<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Image Print Layout — Architecture

🇫🇷 [Version française](ARCHITECTURE.fr.md)

This document is the canonical technical contract of Image Print Layout. Where
`PROJECT_CONTEXT.md` describes the vision and the principles, this one defines the concrete: technical
stack, packages, interfaces, formulas, project format, checks and delivery
plan. If the two seem to contradict each other, this document prevails for everything
concrete (interfaces, formulas, formats, default values).

## 1. Purpose and scope

This document says how to build Image Print Layout v1.0 as defined by the specification. It does not repeat the requirements; it says how to meet them and where the risk is concentrated.

Assumptions retained for v1, to be confirmed:

- **Platforms**: the Debian family (Debian, Ubuntu and their derivatives), the Red Hat family (RHEL, CentOS Stream and their derivatives, Fedora) and Windows 10 and 11. The criterion is the version of the libraries, not the name of the distribution: see "Supported platforms" (§4).
- **Horizontal cuts only.** A vertical cut creates columns of pages, hence a tiled assembly, out of scope (§14 of the specification) and in tension with test 12 (same width on all pages). The data model keeps an axis field so as not to close the door.
- **Mode B (fit to height)**: the scale is computed so that the selected portion, by default the first page, exactly fills the usable height. If the resulting width exceeds the usable width, the page goes into overflow state; nothing is silently reduced.
- **Default margins**: 10 mm at the top and bottom, 5 mm on the left and right. They give 200 mm of usable width on A4, which makes the acceptance test achievable without a warning.
- **One document per project**: an SVG or, since 1.2, an image (JPEG, PNG, TIFF, WebP, BMP, GIF). The project references it by relative path, with a fallback to the absolute path.
- **Out of v1**: all of §14 of the specification (tiling, join marks, footers, automatic rotation, PDF and raster imports). The import of raster images came afterwards, in 1.2 (§12, "Raster images"): for everything upstream of the engine, a document is a size, a drawing and blocks, whether it comes from an SVG or an image.

Order of magnitude of the acceptance test: 200 mm of width for a drawing of 1,280 × 5,479 units gives 0.15625 mm per unit, i.e. 856.1 mm of height. With 277 mm usable per A4 portrait page, at least 4 pages are needed.

## 2. Structuring decisions

Eight decisions carry the design; the first four guarantee the strictest requirements of the specification: no silent resizing and a preview faithful to the PDF.

1. **A single page plan, two surfaces.** A pure module computes the plan (scale, strips, positions). The preview and the PDF draw it with the same function, on two different surfaces, so they cannot diverge.
2. **The drawing is never cut, only windowed.** Each page is the same drawing, translated then cropped by a rectangular clip at the cut lines. A stroke or a text that crosses a cut is cropped exactly, without geometric computation on paths.
3. **A single global scale**, in mm per SVG unit, stored in one place. No page can have a different width, and adding or moving a cut does not touch it.
4. **Cuts in document coordinates** (SVG units), not in millimeters. Changing the scale or the paper leaves each cut at the same place in the drawing; only the page states are recomputed.
5. **SVG rendering delegated to cairo and librsvg**, through a thin cgo layer behind a `Document` interface. No SVG interpreter is written in Go: we get fidelity (text, gradients, masks) and a vector PDF with a native clip. A raster image goes through the same interface: gdk-pixbuf decodes it and cairo draws it (§12, "Raster images"), so the plan, the cuts, the preview and the export do not know whether the document is an SVG or an image.
6. **GTK4 interface via gotk4.** It shares the engine's graphics libraries (cairo, pango): a single set to embed under Windows.
7. **Logic in pure Go**, testable without cgo: model, units, layout, SVG analysis, project. The computation tests run everywhere, without any graphics library.
8. **Four languages from the first interface**: French, English, Spanish, German. No text meant for the user is written in the code: it carries a key, which the `i18n` package translates (§4, "Languages").

Decision 5 is the main technical bet. Milestone M0 validates it on the real file before any interface development; §3 describes the fallback if it fails.

## 3. Technology choices

**Retained: Go with GTK4 (gotk4) for the interface, cairo with librsvg for rendering and PDF export.** The rendering engine is the riskiest choice; milestone M0 validates it on the real file before the rest.

### SVG rendering engine and PDF export

| Option | SVG fidelity | Exact cut per page | Preview identical to the PDF | Verdict |
| --- | --- | --- | --- | --- |
| A. SVG interpreter written in Go, PDF written by hand | To be built: text, gradients, masks, CSS | Yes, clip written by hand | Yes, same drawing tree | Distant fallback, maximum effort |
| B. [tdewolff/canvas](https://pkg.go.dev/github.com/tdewolff/canvas/renderers/rasterizer) as a drawing target | Drawing target, not an SVG interpreter | No: the engine only draws paths, text and images | Yes | Rejected: no clip, module without a stable version |
| C. cairo + librsvg through cgo | High: many SVG 1.1 and SVG 2 features | Yes, cairo clip | Yes, same drawing function on two surfaces | **Retained** |
| D. [svg2pdf](https://pypi.org/project/svg2pdf-py/) (Rust) in a separate process | High, real text optional | Possible through a reusable PDF object | No: the preview requires a second engine | Fallback if C fails |

Why C:

- **Fidelity.** [librsvg](https://gnome.pages.gitlab.gnome.org/librsvg/Rsvg-2.0/overview.html) handles neither script, nor animation, nor SVG fonts; it relies on the system fonts through Cairo and Pango.
- **Any cairo surface.** The same code draws on an image surface for the preview and on a PDF surface for the export, and a loaded document can be rendered as many times as necessary.
- **No default background.** librsvg does not paint a background: the page background is filled by the application (§8).
- **Framing.** `rsvg_handle_render_document` scales the document proportionally into a viewport and centers it. It is given a viewport of the same proportions as the drawing, so as not to depend on any implicit centering. The engine imposes 96 dpi on librsvg, whose default resolution is 90: without this, lengths in `mm`, `in` or `pt` would be badly converted (M0, `docs/M0.md`).
- **Element boxes.** [librsvg measures an element by its identifier](https://valadoc.org/librsvg-2.0/Rsvg.Handle.get_geometry_for_element.html), without taking clips into account. The engine uses `rsvg_handle_get_geometry_for_layer`, which gives the box in the document's coordinate system (the "element" variant gives it relative to the element); on the Mini-CAD, the 36 groups, 121 rectangles and 459 texts are measured, none outside the document. The analysis adds identifiers to the working copy to measure each block (§7).

Accepted costs:

- **cgo and a thin C layer** in the `engine` package: librsvg is not a dependency of gotk4.
- **Windows distribution.** The installer must embed the GTK binaries and their runtime dependencies ([GTK](https://gtk.org/docs/installations/windows/)), and the GNU licenses require providing the sources on request; the check also applies to cairo and librsvg. GTK4 packaging under Windows is already practiced on Planche-Contact (NSIS installer).
- **Determinism** guaranteed at equal library versions: the versions embedded under Windows are frozen (§11).
- **System fonts**: a missing font is substituted, therefore detected and flagged (§12).

### Interface

| Option | Pros | Cons |
| --- | --- | --- |
| [GTK4 via gotk4](https://github.com/diamondburned/gotk4-libgirepository) | Same graphics stack as the engine (cairo, pango): a single set to embed. Packages under MPL-2.0, cairo bindings under MIT | The project reports memory leaks and possible crashes in some parts of the API. Windows build to be validated in M0 |
| [Fyne](https://github.com/fyne-io/fyne) | Idiomatic Go, BSD-3-Clause license, no library required at runtime | A C compiler is also required. No link with cairo: the preview goes through an image copy. First Windows build possibly taking up to 10 minutes |

GTK4 is retained. Since `Document` returns images (§4), Fyne remains a possible fallback for the interface alone.

**gotk4 version.** `github.com/diamondburned/gotk4/pkg` v0.3.1, which compiles with GLib 2.80 and GTK 4.14 (Ubuntu 24.04, RHEL 10): v0.4 requires GLib 2.84 or later, which these systems do not have. The version is that of `go.mod`. Moving to a more recent version is a separate change, which must keep the packages buildable on the oldest supported systems (Ubuntu 24.04, RHEL 10 and its clones). gotk4 compiles slowly (several minutes the first time, the build cache afterwards); the CI keeps this cache.

### Raster images

Images (JPEG, PNG, TIFF, WebP, BMP, GIF) are read by **gdk-pixbuf**, which GTK 4 and librsvg already require: no extra dependency under Linux, and a single set of loaders to embed under Windows. Go's standard library reads neither TIFF, nor WebP, nor EXIF orientation: one more decoder would have been a second path to the same pixels, to be kept at the level of the first. Cairo draws them on the same surface, image or PDF, as the SVG. The detail is in §12, "Raster images".

### Fallback if M0 fails

If fidelity or Windows packaging does not pass, engine D takes over behind the same `Document` interface. The page computation, the SVG analysis, the project and the interface do not change; only the preview and the export change engine.

The rest relies on the standard library (XML, JSON, SHA-256). Rereading the produced PDF, for the checks of §11, requires no PDF library: M0 shows that cairo's content streams are Flate, that `compress/zlib` decompresses, and that the clip and the offset of each page can be read directly in them (§11).

## 4. Modular architecture

The code is split into eight modules on four levels, set on four foundation packages (`model`, `units`, `diag`, `i18n`). The rule: the computation logic depends neither on the interface nor on cgo.

```
ui        GTK4 interface: window, panels, preview, mouse gestures              [cgo]
 ├──▶ viewport  View arithmetic: zoom, panning, window to draw                 [pure Go]
 └──▶ app       Session: project, history, plan; opening a document            [pure Go]
       ├──▶ layout    Scale, strips, cuts                                      [pure Go]
       ├──▶ project   JSON file, versions, migration                           [pure Go]
       ├──▶ svgdoc    Analysis, working copy, blocks, background, fonts        [pure Go]
       └──▶ export    Checks before and after, PDF and HTML page (M4) ─┐ through the types
                                                                     ▼ of engine
             engine   Engine: cairo + librsvg + gdk-pixbuf + fontconfig, loading     [cgo]
                      of an SVG or an image, block boxes, preview windows and multipage PDF

Foundation: model · units · diag · i18n
            project types, unit conversions, shared diagnostics, translation; imported by all
```

The arrows go from the caller to the callee. The `engine` package has two parts: the types and the `Document` interface, in pure Go (`engine.go`; `format.go`, which recognizes a format by the first bytes of a file; `image.go`, which describes an image and its errors; `jpeg.go`, which reads the header of a JPEG; `resolution.go` and `filefacts.go`, which read in the bytes of a file its resolution, its pages, its animation and the decoder's messages), and their implementation, in cgo: through librsvg for an SVG (`rsvg_cgo.go`) and through gdk-pixbuf and cairo for an image (`image_cgo.go`). `LoadDocument` chooses according to the format, and a stub `engine_nocgo.go` refuses to load. `app` and `svgdoc` know the engine only through the types of the first part: the program (`cmd/imageprintlayout`) and the interface give them the real `Loader`, and a fake engine replaces it in the tests. `ExportPDF` takes a list of `PageSpec`, not a `layout.Plan`: `engine` does not depend on `layout`, and it is `export` that translates the plan (M4). Only `ui` and `engine` depend on cgo. The entry point `cmd/imageprintlayout` just wires these packages together.

The GitHub repository is called `imageprintlayout`, the Go module `github.com/gillesmagneville/image-print-layout` and the binary `imageprintlayout`. The file layout is given after the `Document` interface.

```go
// A loaded document: used by one thread at a time, and to be closed.
type Document interface {
    Size() (w, h float64)                                          // document units, at 96 dpi
    Measure(id string) (Box, error)                                // box of an element, in units
    MeasureBlocks(ids []string) map[string]Box                     // an element that cannot be measured is absent
    Render(win Box, pxPerUnit float64, bg color.Color) (*image.RGBA, error)
    ExportPDF(out io.Writer, pages []PageSpec, opt PDFOptions) error
    Close()
}

type Loader func(data []byte, path string) (Document, error)   // an SVG or an image, recognized by its first bytes; path: folder of linked images
type FontMatcher func(family string) string                    // installed family for a requested family
```

There are no tiles (§9): the preview asks the engine only for the visible window, at screen scale.

### Repository layout

```
imageprintlayout/
├── go.mod
├── VERSION                    # current version (SemVer), see CONTRIBUTING.md
├── build-deb.sh               # build the packages at the root (see "Packages"),
├── build-rpm.sh               #   with the options and the handling of VERSION of
├── build-windows.ps1          #   Planche-Contact; this one relays to windows/
├── cmd/imageprintlayout/        # entry point: wires the packages together; -version, -selftest
├── internal/
│   ├── model/                 # shared data types, no logic
│   ├── units/                 # SVG / px / pt / mm conversions, SVG lengths,
│   │                          #   paper formats, margins, usable area, scale
│   ├── diag/                  # diagnostics meant for the user (code and values, no text)
│   ├── i18n/                  # translation: French, English, Spanish, German; numbers
│   ├── layout/                # page plan, states, automatic cuts
│   ├── project/               # .imgpl project file
│   ├── svgdoc/                # SVG analysis, working copy, blocks, background detection, requested fonts
│   ├── engine/                # rendering engine behind the Document interface: SVG through librsvg, images through gdk-pixbuf (cgo)
│   ├── testsvg/               # deterministic test SVG generator (pure Go)
│   ├── testimg/               # deterministic test image generator: PNG, TIFF, JPEG, orientation, resolution (pure Go)
│   ├── export/                # exports: pages of the plan, checks, reread PDF, HTML page (written then reread), atomic writing
│   ├── app/                   # session: project, history, plan; opening a document
│   ├── viewport/              # view arithmetic (zoom, panning, window to draw)
│   ├── ui/                    # GTK4 interface (cgo)
│   ├── acceptance/            # the acceptance tests of §13 on the whole chain (no code of its own)
│   └── repocheck/             # tests on the repository itself (headers, versions, docs)
├── debian/                    # copyright, postinst, postrm, lintian-overrides of the .deb
├── metainfo/                  # menu entry (.desktop) and AppStream metadata
├── icons/                     # application icon (SVG and PNG); the Go package `icons` embeds the PNG ("About" window)
├── windows/                   # Windows build script, NSIS installer, .ico icon
├── docs/                      # milestone reports (M0.md, M0.fr.md), manual page (imageprintlayout.1)
└── testdata/                  # reference SVGs
```

Each package of `internal/` is described in this document; a test fails if one is missing.
The packages other than `ui` and `engine` do not use cgo, and a test checks it.

### Languages

The interface is in French, English, Spanish and German; the `i18n` package (pure Go, nothing to install: the texts are compiled into the program) provides them.

- **One key per text.** The code asks for `tr.T("panel.paper.title")`; the text comes from the catalog of the language (`catalog_fr.go`, `catalog_en.go`, `catalog_es.go`, `catalog_de.go`). French is the reference catalog: a key missing from a language is completed by English, then by French, and a key unknown to all is displayed as is. The tests make these cases impossible in practice: the four catalogs have exactly the same keys, no empty text, the same parameters, and every key asked for in the code exists.
- **Named parameters.** A text is written `{count}`, `{path}` or `{w}`; a translation places the values in the order of its language. Decimal numbers are written with a comma in French, Spanish and German, with a point in English; `Number` fixes the number of decimals.
- **Plurals.** `tr.N("pages.count", n)` reads `pages.count.one` or `pages.count.other` according to the language (French counts 0 as singular, the other three do not).
- **Reading a number.** `ParseNumber` accepts the comma as well as the point and ignores spaces, including the non-breaking thousands space: a person who types "200,5" or "200.5" in a width field gets the same value, whatever the language of the interface.
- **French typography.** The French catalog is written with ordinary spaces; at startup, the non-breaking space is put before ":", ";", "?", "!" and inside the guillemets, so that a label does not break before its colon.
- **Choice of language.** The setting is a code (`fr`, `en`, `es`, `de`) or `auto`. `auto` follows the system: variables `LANGUAGE`, `LC_ALL`, `LC_MESSAGES`, `LANG` under Linux, the user's language under Windows (`GetUserDefaultLocaleName`). A system language that is not one of the four gives English. The `-lang` command-line option prevails; the choice made in the interface (window menu, immediate effect) is saved in the user's preferences, `settings.json` in the system's configuration folder (`~/.config/imageprintlayout/` under Linux, `%AppData%\imageprintlayout\` under Windows), outside the project file: the project does not depend on the language of whoever opens it.
- **Error messages and diagnostics.** A diagnostic (`diag`) carries a code and values, never a text: the interface translates it with the key `diag.<code>`. This way an engine or export error is displayed in the user's language without the computation packages knowing the languages.
- **GTK components.** The buttons of the standard dialogs ("Cancel", "Open"…) are translated by GTK itself, from the chosen language: the interface gives it the language through the `LANGUAGE` variable before initializing it (under Windows also in the C library's environment, where gettext reads it), and the Windows package embeds the GTK catalogs for French, Spanish and German.
- **Files read by the system.** The menu entry (`.desktop`), the AppStream sheet and the manual page exist in the four languages, English being the text without a suffix: the system takes it when its language is not one of the four. `repocheck` tests check that each language has its lines, and that the four manual pages carry the same version. The Windows installer (NSIS) also speaks the four languages, according to that of Windows.
- **What stays in French.** The `-version` and `-selftest` diagnostic outputs (useful to support, reread by the package scripts); the code's identifiers and comments are in English.
- **Documents.** The documents of the repository (`README`, `ARCHITECTURE`, `CONTRIBUTING`, `PROJECT_CONTEXT`, `CHANGELOG`, `docs/M0`, `windows/README`, `testdata/README`) exist in English (`X.md`, the default file) and in French (`X.fr.md`), each linking to the other at the top of the file. They are not translated into Spanish or German.
- **Adding a text or a language.** A text: add the key to the four catalogs (the test says which ones forget it). A language: a catalog, its name in `Name` and `Languages`, its plural rule in `isSingular`, its decimal separator in `Number`.

### Supported platforms

Three families of systems. What decides is not the name of the distribution but the versions
of its libraries: **GTK 4.14, GLib 2.80, Pango 1.52 and librsvg 2.52 at least**. These thresholds
come from gotk4 v0.3.1, which binds almost the whole API of GTK 4.14 and GLib 2.80: on an
older system, the compilation stops on hundreds of unknown symbols. The
package dependencies are then deduced from the binary (see "Packages").

| Family | Supported | Package | Tried by the CI |
| --- | --- | --- | --- |
| Debian and derivatives | Debian 13 and later; Ubuntu 24.04 LTS and later; derivatives based on these versions (Linux Mint 22, Pop!_OS 24.04, elementary OS 8…) | `.deb` | Debian 13, Ubuntu 24.04, Ubuntu 26.04 |
| Red Hat and derivatives | RHEL 10 and its clones (CentOS Stream 10, AlmaLinux 10, Rocky Linux 10…); Fedora (current versions) | `.rpm` | CentOS Stream 10, AlmaLinux 10, Rocky Linux 10, Fedora |
| Windows | Windows 10 and 11, 64-bit | portable `.zip` and installer | Windows Server 2022 and Windows Server 2025, which serve as stand-ins |

| Library | Debian 13 | Ubuntu 24.04 | Ubuntu 26.04 | RHEL 10 and clones | Fedora 44 | Windows (MSYS2) |
| --- | --- | --- | --- | --- | --- | --- |
| GTK | 4.18.6 | 4.14.5 | 4.22.4 | 4.16.7 | 4.22.5 | 4.24.1 |
| GLib | 2.84.4 | 2.80.0 | 2.88.0 | 2.80.4 | 2.88.4 | 2.90.1 |
| librsvg | 2.60.0 | 2.58.0 | 2.61.3 | 2.57.1 | 2.62.4 | 2.63.2 |
| cairo | 1.18.4 | 1.18.0 | 1.18.4 | 1.18.2 | 1.18.6 | 1.18.6 |

**Not supported: Debian 12, Ubuntu 22.04 (and Linux Mint 21), RHEL 9 and its clones.** They have
GTK 4.8, 4.6 and 4.12, GLib 2.74, 2.72 and 2.68, and RHEL 9 has librsvg 2.50. `build-deb.sh` and
`build-rpm.sh` notice it before compiling and say so in French; the CI verifies this refusal
on these three systems (the "refus attendu", or expected refusal, jobs). They could be served later by a
package that embeds GTK (Flatpak or AppImage), which changes how the software is distributed;
this remains out of v1 (§15, question 7).

In the CI, the window opens under Xvfb on the Debian family and on Fedora (with
screenshots), and on a headless Wayland compositor (`weston`, launched by `wlheadless-run`) on
the RHEL 10 family, which no longer has an X server: there is no screenshot in that case.

What the CI does not prove:

- **Windows 10 and 11 themselves.** It only has Windows Server: 2022 (the oldest offered)
  stands in for Windows 10, 2025 for Windows 11. The manual acceptance on a real
  Windows machine was done for 1.0.0 (M5).
- **RHEL itself**, which requires a subscription: CentOS Stream 10 (its upstream) and its
  clones AlmaLinux and Rocky Linux are tried.
- **Ubuntu and Debian derivatives** (Mint, Pop!_OS, elementary OS…): they share the
  libraries of their base, hence the same thresholds.
- **aarch64.** The scripts know how to build for `arm64` and `aarch64`, but the CI only tries
  `amd64` and `x86_64`.

### Packages

Three scripts, at the root of the repository, build the packages from the code; they
take over the options and the handling of the `VERSION` file of `build-deb.sh` and of
`windows/build-windows.ps1` from Planche-Contact, and write the packages at the root (not
versioned: `.gitignore`).

| Package | Script | Build tool |
| --- | --- | --- |
| `.deb` (Debian, Ubuntu and derivatives) | `build-deb.sh` | `dpkg-deb`, dependencies by `dpkg-shlibdeps` |
| `.rpm` (Fedora, RHEL and derivatives) | `build-rpm.sh` | `rpmbuild`, dependencies deduced from the binary |
| Windows: portable `.zip` and installer | `build-windows.ps1`, `windows/` | MSYS2 UCRT64 (Go, gcc, GTK 4, librsvg, cairo), NSIS |

- **Options and version.** `--major`, `--minor`, `--patch`, `--no-version-change`, `--clean`,
  `--help` (`-Major`… under PowerShell); without an option, a menu offers the four choices and the
  version that would result, then the script asks for confirmation. The `VERSION` file is only
  written once the package is built: a failure never leaves a version without a package.
  A pre-release version (`0.1.0-dev`) is published first: `--minor` and `--patch`
  give `0.1.0`, `--major` gives `1.0.0` (node-semver rule); without a suffix, it is
  the usual increment. The three scripts have no code in common, as at
  Planche-Contact: a `repocheck` test checks that they have the same options and the same
  computation function (`next_version`, identical in the two shell scripts), that `VERSION`
  is written only once after the build, and the CI runs the scenarios
  (`.github/scripts/test-version-handling.sh` for Linux, `windows-package.ps1` for Windows).
  The Windows script has one more option, `-Oui`, which answers "yes" to its questions: the CI
  cannot answer a `Read-Host`.
- **Native tools.** `dpkg-deb` and `rpmbuild` are on every machine of the family, without
  Ruby or a module repository (fpm and nfpm are not used, whereas Planche-Contact
  uses fpm: here the program is a binary, not a Python environment); they
  package a folder that the scripts prepare, without compiling anything. The `control`
  file of the `.deb` and the `.spec` of the `.rpm` are written by the scripts, because their
  dependencies, their size and their version number are computed at build time.
- **Numbers.** A pre-release version sorts before the final one: `0.1.0-dev` gives
  `0.1.0~dev` at Debian and the release `0.1.dev` at RPM; `0.1.0` gives `0.1.0` at Debian
  (file `imageprintlayout_0.1.0_amd64.deb`, without a Debian revision, as at Planche-Contact and
  LinuxDoctor) and the release `1` at RPM (`imageprintlayout-0.1.0-1.x86_64.rpm`). Windows keeps `0.1.0-dev` (four numbers in the properties of the `.exe`).
- **Contents of the Linux packages.** `/usr/bin/imageprintlayout`, the menu entry and the
  AppStream metadata (identifier `io.github.gillesmagneville.imageprintlayout`, reverse
  domain name: the `.desktop` file and `<launchable>` must agree with it, and
  `desktop-file-validate` and `appstreamcli validate` check it in the CI). **The program announces
  this identifier to the desktop as its name** (`glib.SetPrgname`, `newApplication`): under Wayland, GNOME
  associates a window with its menu entry by the identifier (`app_id`) it announces, and GTK otherwise announces
  the name of the executable (`imageprintlayout`), which matches no `.desktop` file; the
  window then lost its icon in the overview. `StartupWMClass` says it again for X11; the icon (SVG and PNG
  256), the manual pages (English in `man1`, French, Spanish and German in the
  folder of their language), the README and the changelog (each in English and in French), copyright (`.deb`) or license
  (`.rpm`). The dependencies include GTK 4 (`libgtk-4-1`).
- **Reproducibility.** `-trimpath`, `-buildvcs=false`, an empty build identifier,
  `SOURCE_DATE_EPOCH` (date of the last commit), order, owners and permissions fixed:
  two builds give the same file, at equal Go, librsvg and cairo versions
  (the CI checks it for the `.deb` and the `.rpm`).
- **Packages tied to their build machine.** The dependencies of a `.deb` or an `.rpm`
  are deduced from the binary: you build on the targeted distribution (see "Supported
  platforms"). The `t64` names of Debian packages do not exist before Debian 13 and Ubuntu 24.04.
- **Working folder.** Outside the repository, as at Planche-Contact: `~/deb-build/build`,
  `~/rpm-build/build` and `%LOCALAPPDATA%\imageprintlayout-build`. `--clean` deletes them, and
  deletes the packages already built at the root.
- **Windows layout.** The delivered folder is a prefix (`bin/` for the programs and the
  DLLs, `etc/fonts/`, `lib/gdk-pixbuf-2.0/`, `share/glib-2.0/schemas/`, `share/icons/`,
  `share/locale/`, `share/licenses/`): the program can be installed anywhere. GLib and
  GTK find their files again from the location of their DLLs, hence this prefix; without
  the settings schemas, GTK stops when a file chooser opens, and without the icon
  theme the window's buttons have no image. There are two executables:
  `imageprintlayout.exe` (Windows subsystem: no console opens behind the
  window, but nothing is written to a terminal) and `imageprintlayout-console.exe`, the same
  program in the console subsystem, for `-version`, `-selftest` and error messages;
  the build script checks the subsystem of each. Fontconfig, gdk-pixbuf and pango do not know where their
  configuration is in this case; `engine.ConfigureRuntime` tells them at startup (under
  Windows, when this folder exists). Pango uses fontconfig, as under Linux: the fonts
  of a document are resolved, and replaced, in the same way everywhere (§12). The package
  embeds no font; it embeds the license of each library. An NSIS installation
  script only deletes the files it installed.
- **Smoke test.** `imageprintlayout -selftest` loads a built-in SVG (background, block, accented
  text, embedded PNG image), measures it, draws it and exports it to PDF, and verifies
  that a font is embedded and that two exports are identical. The Windows script
  runs it on the package folder with only the Windows PATH before producing the `.zip`;
  the CI runs it on each installed package. It also opens the installed program's
  window (option `-ready-file`, which creates a file when the window is displayed and the
  document drawn) on a virtual screen, takes a screenshot, and does so in the
  four languages for the portable Windows package.
- **What the packages do not do yet**: the program they install opens an SVG,
  analyzes it, displays it with its pages and its settings (paper, margins, scale, positioning)
  and proposes cuts, which you place, move and delete with the mouse (M3); it replaces or removes
  the document background, shows the sheets as they will be printed and exports the PDF
  after rereading it (M4). It saves and reopens the project (M5), with the asterisk and the
  question before losing changes. Nothing is signed, no package repository exists, and the arm64 architecture is recognized
  by the Linux scripts without having been tried.

## 5. Geometric model

Everything rests on three coordinate systems and a single scale: the document units, the millimeters on the sheet and the points of the PDF.

| Coordinate system | Unit | Used for |
| --- | --- | --- |
| Document | SVG user unit, origin at the top left | Storing the cuts and measuring the elements |
| Sheet | Millimeter, origin at the top left corner of the sheet | Margins, positions, display of dimensions |
| PDF and cairo | Point (1/72 inch) | Drawing; 1 mm = 72 / 25.4 = 2.834646 points, fixed value |

### Document dimensions

The document unit is the **CSS pixel of its declared size** (96 per inch): this is what librsvg reads (`rsvg_handle_get_intrinsic_size_in_pixels`) and what the engine draws. For an SVG whose `viewBox` and `width` and `height` agree (the case of the Mini-CAD, in pixels), it is the unit of the `viewBox`. When the two differ, the drawing is fitted to the declared size, and the positions of the cuts are counted in this unit.

- `width` and `height` in an absolute unit (mm, cm, inch, pt, pc, px): standard conversion.
- Without a unit, as a percentage or absent: one unit is worth one CSS pixel at 96 dpi, i.e. 25.4 / 96 = 0.26458 mm.
- Neither `viewBox` nor dimensions: the engine refuses the document with an explicit error (the extent of the content is not used to guess a size).

An **image** has no unit of its own: its size in units is `pixels × 96 / resolution`, on each axis, with the resolution that the file states (JFIF density for a JPEG, failing that the Exif resolution; `pHYs` for a PNG; the resolution tags of a TIFF; value kept between 10 and 100,000 dots per inch). The engine reads it **in the bytes of the file** (`internal/engine/resolution.go`, pure Go) and not from gdk-pixbuf, whose recent loaders do not report it (§12). Without a valid resolution, it is 96: one pixel is worth one unit, and a 1,000-pixel image is 264.6 mm at 100%, like a 1,000-unit SVG. A photo at 300 dots per inch is 84.7 mm wide at 100%: its "paper" size. When the resolutions of the two axes differ, it is the shape of the image on paper that is the document's; the scale, for its part, stays single. The orientation (EXIF, TIFF) is applied: the size is that of the straightened image. From there on, nothing distinguishes an image from an SVG: the scale, the strips and the positions are computed in the same way.

The factor of the native scale is therefore always 25.4 / 96 mm per unit (`layout.NativeMMPerUnit`). If the Mini-CAD is in pixels, its 1,280 × 5,479 units make 338.7 × 1,449.7 mm at 100%.

### Scale

The scale `s` is a single number, in mm per unit, stored in one place. No page keeps a copy of it: they read it.

```latex
s = \frac{L}{W_d} \qquad h_i = (y_i - y_{i-1})\, s \qquad w = W_d\, s = L
```

`W_d` is the width of the document in units, `L` the chosen width in mm, `y_i` the position of cut i in units, `h_i` the height of page i in mm. `s` depends only on the paper, the margins, the mode and the size of the document, never on the cuts: `layout.ScaleOf` is the only place where it is computed.

| Mode | Computation of `s` |
| --- | --- |
| A. Width | `L` = usable width of the paper (width of the paper minus left and right margins) |
| B. Height | `s` = usable height divided by the height in units of the chosen **portion** (`portionUnits`, the whole document if it is zero). The portion is a setting, not a page: moving a cut therefore does not change the scale. The interface offers to set it equal to the height of an existing page. The resulting width is then compared with the usable width |
| C. Manual | `L` entered in mm (mode `width`), or factor `f` in % (mode `factor`): `s` = `f` × native scale |

The width is displayed in mm and in cm. The acceptance test gives `s` = 200 / 1,280 = 0.15625 mm per unit, i.e. 856.1 mm of total height.

### Sheet and usable area

| Format | Portrait, width × height (mm) |
| --- | --- |
| A0 | 841 × 1,189 |
| A1 | 594 × 841 |
| A2 | 420 × 594 |
| A3 | 297 × 420 |
| A4 | 210 × 297 |
| A5 | 148 × 210 |
| Letter | 215.9 × 279.4 |
| Legal | 215.9 × 355.6 |
| Tabloid | 279.4 × 431.8 |
| Custom | entered in mm |

Landscape swaps width and height. The usable area is the paper minus the four margins, set independently.

### Numerical precision

All computations are in `float64`, and rounding takes place only at display time (0.1 mm). Comparisons of dimensions tolerate 0.005 mm: a width of 200 mm in a usable area of 200 mm must never be refused because of a rounding error. The cuts are recorded as they are, without rounding.

## 6. Rendering and PDF export pipeline

The document is loaded once, the plan computed once, then a single function draws each page: on an image surface for the preview, on a PDF surface for the export.

```
Source file             SVG or image, read-only, never modified
    │
    ▼
Working copy            SVG only: background replaced or removed here only
    │
    ▼
Loaded document         loaded only once, drawing kept in memory
    │
    ├── drawing size ──▶ Page plan ◀── Settings
    │                    pure function:   paper, margins, scale,
    │                    strips, positions alignments, cuts
    │                    in mm, states
    │                              │
    │ the drawing                  │ the plan
    ▼                              ▼
Drawing of the pages: one page = one clip, one offset, one scale
    │
    ├──▶ Preview       same function, image surface
    │
    └──▶ PDF export    same function, multipage PDF surface
              │
              ▼
         Checks: reread PDF (number of pages, dimensions, clip manifest)
```

The plan remains the only source of positions: the preview and the export both read it, and neither recomputes the scale.

### Window of a page

A page is the same drawing, shifted then cropped. The values are in mm; they are converted to points at the moment of drawing.

```latex
M_i = T(x_i,\, y_i) \cdot S(s) \cdot T(0,\, -y_{i-1}) \qquad \text{clip} = [x_i,\, x_i + w] \times [y_i,\, y_i + h_i]
```

The clip is the rectangle of the strip, never that of the usable area: it is what materializes the cut. A stroke or a text that crosses it is therefore cropped exactly, and nothing is recomputed on the paths.

### Sharing the drawing between pages

Each page redraws the document directly under its clip: cairo writes into the PDF only the operations that touch the window, so the file does not contain the drawing as many times as it has pages. M0 measured it on the Mini-CAD: 77,080 bytes for 4 pages, 71,695 for the whole drawing on a single page. Text stays text, with its fonts embedded as subsets.

The initial plan was to draw the document once in a cairo recording surface, then paint it on each page. It is abandoned: with cairo 1.18, the PDF then writes only a single object whose extent is that of its first use, the following pages come out empty, and the text becomes outlines (`docs/M0.md`). The cost is an export time proportional to the number of pages (about 0.25 s per page on the Mini-CAD).

### Images

An image follows the same path, with one difference: it has no drawing to reread for each page. It is decoded only once into a cairo image surface (opaque, RGB24, unless a pixel is really transparent: ARGB32) that the engine keeps as long as the document is open, and each page paints it whole, shifted and cropped by the clip of its strip (it is the same `drawPage`, §6, "Window of a page"). Cairo then writes the image **only once** in the PDF, each page referencing it with its matrix: the file is not N times bigger (`TestAnImageIsWrittenOnceAndShownOnEveryPage`). A JPEG that the PDF can read as is (baseline or progressive JPEG, 8 bits, in gray or in YCbCr color, of the size of the file's frame) goes in with its original bytes (`CAIRO_MIME_TYPE_JPEG`): neither recompression nor loss, and a PDF the size of the photo. The others (PNG, TIFF…, or a CMYK JPEG) are written as pixels, losslessly compressed. The orientation is a drawing matrix, never a recomputation of the pixels, and the filter is cairo's "GOOD" (average when shrinking, interpolation when enlarging), without extended edges: a page receives no pixels from its neighbor.

The export rereads the PDF like that of an SVG (§11); `export.Verify` also reads the image's matrix (`cm`) and checks that it is painted where the plan puts it, to within 1/256 of a point.

### Preview

In the Document view, the drawing is an image of the visible window (§9) and the page limits are overlays. In the Sheets view, each page is painted with its own matrix on a white sheet. Cuts, handles and numbers belong to the interface: they are never in the PDF.

### Export

The engine exposes a single function to draw a page (`drawPage`): the PDF export and the sheet preview (`RenderPage`, on an image surface) call it, so what the Sheets view shows is what the PDF contains. The PDF is first written to a temporary file next to the destination (`.name.pdf.part`), synchronized to disk, reread and verified (§11), then renamed: a refused, canceled or failed export leaves neither a half-written file nor an overwritten old file. Progress is reported page by page and cancellation is taken between two pages. The export keeps its system thread from the first page to the last (`runtime.LockOSThread`): Pango, with which librsvg draws the text, keeps its font table per thread, and cairo writes a subset of each font per table, so an export resumed by Go on another thread along the way embedded the same fonts twice (a bigger PDF, and two exports of the same project that differ; seen under Windows). The export tests note the thread after each page and fail if it changes.

For each page: page size set, page background (§8), context save, clip, matrix, document rendering, restore, next page. The exporter reads `s` in the plan and does not modify it. Clip and path positions are written by cairo in 24.8 fixed point, i.e. to 1/256 of a point (0.0014 mm): the position of a cut in the PDF is exact to this precision, under the 0.005 mm tolerance of §5.

### HTML page

The same plan gives a second output: **a single HTML page**, which opens in a browser with nothing to download (1.3). It is written by `export.WriteHTML`, in pure Go, without the engine: the document's image is written there **only once**, in base64, as the background of a style rule (`.doc{background-image:url("data:…")}`), and each piece is just an empty box that shows a strip of it. An SVG and an image make no difference there.

- **Whole or cut.** Whole, the page carries a single piece: the entire drawing. Cut (box "Cut the image at the cuts", checked by default), it carries one piece per page of the plan, one below the other, each with its caption ("Page 2 of 4"): the strips are those of the plan (`Plan.Pages`, `Top` and `Bottom`), hence the cuts placed in the preview. Printing the page from the browser puts one piece per sheet, without the captions.
- **The numbers.** For a strip of height `h` at position `t` in a document of `W × H` units, the box has `aspect-ratio: W / h`, `background-size: 100% (H / h × 100) %` and `background-position: 0 (t / (H − h) × 100) %` (a position percentage is a share of the box minus the image, hence `H − h`). The width of the column is that of the plan in millimeters (`min(100%, w mm)`): it shrinks with the window, without ever distorting the image, and each piece keeps the proportions of its strip. The numbers are written with at most six decimals (`htmlNum`): same inputs, same bytes, on all machines.
- **The background.** The project's page background (§8) is taken over: opaque white, or transparent (the browser page shows through the image). For an SVG, the document background follows the project setting, exactly as in the PDF: the page carries the working copy (`svgdoc.Analysis.ComposeStandalone`), not the source file, which is never modified.
- **What is written.** An SVG is written as SVG, hence vector: a browser redraws it at any size. Its images linked by a relative path are **embedded** in the page (PNG, JPEG, GIF, WebP, BMP, SVG; at most 64 MB each), because a page read elsewhere than in the document's folder would not find them again; a linked image that cannot be embedded (unreadable file, too big, or of a type that browsers do not read, such as a TIFF) keeps its link, and will not appear in the page read elsewhere. Its **text is drawn with the fonts of the computer that opens the page**, and not with those of the PDF: this is stated in the Export section. An image is written **as the file is** when a browser shows it like the engine (JPEG read by the PDF as is, PNG without animation or orientation); every other case (TIFF, GIF, WebP, BMP, animated PNG, image to rotate) is drawn by the engine, straightened and at its first frame, then written as a PNG, which keeps transparency (`export.ImageSource`; at most 64 million pixels, beyond which the image is drawn smaller). A page of more than 256 MB (`MaxHTMLBytes`) is refused before being written (`export.html_too_large`).
- **Rereading.** Like the PDF (§11), the page is written next to its destination under a temporary name (`.name.html.part`), reread from disk and compared with the plan, then renamed: a refused or failed export leaves neither a half-written file nor an overwritten old file (`export.RunHTML`). `export.VerifyHTML` rereads the file with only the numbers it contains, and finds again the top and the height of each strip, the height of the document, the width of the column and the image itself.

## 7. Layout engine

The `layout` package is a pure function: same parameters, same plan. It reads no file, touches no interface and never modifies the scale.

**Inputs**: document size, scale `s`, paper and orientation, margins, alignments, cuts `y_1 < … < y_k` in units, per-page position overrides. **Output**: a plan, that is, for each page its bounds in units (`y_{i-1}`, `y_i`), its content width and height in mm, its position on the sheet, its clip rectangle, its state and its messages.

The strips are `[0, y_1]`, `[y_1, y_2]`, … `[y_k, H_d]`. They cover the document with no gap or overlap by construction, and a test checks it.

### Positioning

| Setting | Position on the sheet |
| --- | --- |
| Horizontal: left | `x` = left margin |
| Horizontal: centered | `x` = left margin + (usable width − `w`) / 2 |
| Horizontal: right | `x` = paper width − right margin − `w` |
| Horizontal: custom | `x` = left margin + entered offset |
| Vertical: top | `y` = top margin + entered offset |
| Vertical: centered | `y` = top margin + (usable height − `h_i`) / 2 |

These formulas use `s` only through `w` and `h_i`. Changing an alignment therefore cannot change the width of the drawing, and a test guarantees it.

### Page states

The content of a page occupies the rectangle `[x, x+w] × [y, y+h_i]`.

| State | Condition | Effect |
| --- | --- | --- |
| Conforming | The rectangle is within the usable area (tolerance 0.005 mm) | Export allowed |
| Margin overflow | Within the sheet, outside the usable area | Warning, confirmation at export |
| Sheet overflow | Outside the sheet | Export blocked |
| Empty page | No block measured in the strip | Warning |

### Proposed corrections, never applied on their own

When a page overflows in height, the plan provides four numbered proposals, each behind a button:

1. **Add a cut** at the most favorable position of the strip (see below).
2. **Move the neighboring cut** to the maximum height that the page accepts.
3. **Reduce the scale** to the value that makes the tallest strip fit, with the resulting width (for example "78%, width 156 mm").
4. **Change the format**: first format, from the smallest to the largest and in both orientations, for which all the pages fit (the plan is recomputed for each trial, because in mode A the scale follows the paper). Nothing is proposed if none fits.

Each proposal carries a "solves everything" indicator: once applied, no page overflows anymore (a cut added to an 856 mm document is not enough on its own, the next proposal takes over). The scale reduction rounds the width down to the tenth of a millimeter and applies in mode C, width.

For an overflow in width, only proposals 3 and 4 exist. None is applied without a click, which guarantees the absence of silent reduction.

### Cuts

A cut carries an identifier, an axis (reserved, `y` only in v1) and a position in units. Invariants: `0 < y < H_d`, strict order, minimum physical gap of 0.5 mm with the edges and the other cuts. Adding, moving and deleting are pure commands that return a new state; a tested property establishes that `s` is identical before and after each of them. The plan requires only the structure (strict order, inside the document): a change of scale can bring two cuts closer than 0.5 mm without making the project unreadable, only the commands enforce the gap. The identifier of a cut does not change when it moves and is never reused; it is not saved, the cuts are renumbered at each opening.

During a drag, the cut snaps to the detected gaps within a radius of 8 screen pixels; holding Alt disables snapping.

### Blocks and crossing detection

The **blocks** are the top-level groups that carry visible content, descending into Inkscape layers. The detected background rectangle (§8) is excluded, otherwise every position would cross it. The engine measures each block; `layout` only manipulates these boxes.

A cut crosses a block if its position is strictly inside the block's box (tolerance 0.05 unit). It then receives a warning icon, and the right panel names the blocks crossed.

**Documents with very many blocks.** Measuring an element forces librsvg to go through the whole drawing: measuring each block costs a time proportional to the square of their number (4,000 blocks: less than a second; 100,000: more than six minutes, measured). Beyond 3,000 blocks (`app.MaxExactBlocks`), `app` no longer measures them one by one. What `layout` reads from the blocks is the union of their vertical intervals, and a row of the image without any ink is a gap: the document, without its background rectangle, is therefore drawn once at 1 pixel per unit (at most 16 million pixels), and each run of rows that carry ink becomes a block. It is the same information, at the resolution of the image; a gap thinner than one row is lost, which is on the safe side (the cut is not proposed there). The finding `svg.coarse_blocks` tells the user. A document of 100,000 elements opens in about four seconds.

**Blocks of an image.** An image has neither group nor element: its blocks are read from its pixels (`app.rasterBlocks`), as for a too-heavy SVG (above), but without a background to remove. The image is drawn once at its native resolution, brought down to 16 million pixels at most, then laid on white if it is transparent. The **background color** is the one that covers at least half of the perimeter, in classes of 16 levels per channel: it can be white, black or colored. A row is **ink** if more than 0.1% of its pixels differ from the background by more than 16 levels out of 255 (the noise of a JPEG on a flat area, the grain of a scan and dust do not count), and each run of ink rows is a block (`blocksOfRows`, the same code as for a too-heavy SVG). A **photograph** has no plain background on its perimeter, or has no empty row: it then has no blocks, a cut crosses none and the automatic cuts are spread evenly. This is intended: cutting a photograph "between two blocks" would make no sense, and the proposal does not pretend to find any.

### Automatic cuts

1. **Minimum number of pages**: `n_min` = ceiling of (document height in mm / usable height).
2. **Gaps**: the union of the vertical intervals of the blocks, complemented, gives the zones where nothing is crossed. The cost of a position is the number of blocks it crosses, zero in a gap.
3. **Search** for `n` from `n_min` to `n_min + 2`: dynamic programming over the candidate positions (center of each gap, ideal positions `j × H_d / n` and, near each of them, the point of a gap closest to it, one unit from the edge), with the constraints that each strip fit within the usable height and not be less than 40% of the ideal strip. Criteria, in order: fewer blocks crossed, then fewer cuts within 2 mm of a block, then fewer pages, then the most balanced strips (sum of the squares of the deviations relative to the ideal strip).
4. **Proximity**: the center of a gap maximizes the distance to the neighboring blocks; a cut within 2 physical mm of a block, without crossing it, counts as "nearby" and produces a warning.
5. **Result**: a proposal, applied by a single undoable command (`SetCuts`, which drops the page overrides that have become pointless). Since the ideal positions are candidates and always fit within the usable height as soon as `n ≥ n_min`, a solution always exists; when a cut cannot avoid a block, the proposal says so ("cut 2 crosses blocks"). The user then moves each cut.

## 8. Background handling

Three backgrounds have nothing to do with each other: the background of an element of the SVG, the background of the PDF page and the color of the interface's work area. The application never mixes the three.

| Background | Who produces it | Setting |
| --- | --- | --- |
| Document | An element of the SVG, detected by the analysis | Keep, replace with white, remove |
| Page | The application, before painting the drawing | Opaque white by default, or transparent |
| Work area | The interface, with the theme color | None: never exported |

librsvg paints no background by itself. The document therefore has a background only if one of its elements provides one.

**Page background.** In opaque white, each PDF page receives a white rectangle the size of the sheet, painted before the drawing. In transparent, this rectangle is omitted. A PDF has no background of its own: a viewer most often displays white, so "transparent" changes the result only if the PDF is placed on something else.

**Preview.** It paints in this order the work area (neutral gray), then the sheet with the page color, then the drawing. A gray background of the SVG is therefore never confused with the interface color.

**Image.** An image has no background of its own: its pixels are printed as they are, and there is neither detection nor replacement (`Session.SetDocBackground` answers `background.none`, and the Background section explains this instead of offering grayed-out choices). Only the page background acts, and it shows only through the transparent parts of the image: opaque white (the transparent pixels come out white) or transparent (they stay so).

### Background rectangle detection

The candidate is a `rect`, or a path of four orthogonal segments, that meets the following conditions:

- its box, once its ancestors' transformations are applied, covers at least 99% of the `viewBox` (tolerance of 0.5 unit per edge);
- its fill is not `none`;
- it is the first painted, in document order, among the candidates.

If nothing matches, the setting displays "No background detected" and the replacement choices are grayed out.

### Operations on the working copy

| Choice | Operation |
| --- | --- |
| Keep | None |
| Replace with white | The element's fill becomes `#ffffff`, opacity 1 |
| Remove | The element is removed from the tree |

The choice is saved in the project as an instruction: mode and identifier of the element, or failing that its index path (for example `/svg/g[1]/rect[1]`). It is replayed at each load. The source SVG file is never written.

The preview is immediate: the modified copy is reloaded into the engine, off the interface thread (`app.Rebuild`), then redrawn; the old image stays displayed until the new one arrives, without flicker. The project is the only source of truth: the window compares the project's background with the one the engine was loaded with, so undo and redo have no code of their own. If the loading fails, the project goes back to the background the engine has. The page background, for its part, reloads nothing: the preview paints a checkerboard behind a transparent page, so that it can be told apart from a white page. After the export, a pixel of the top margin must be opaque white if this background was requested (test 9, §13).

## 9. Graphical interface

A single window with three areas, with no dialog for common operations: the settings are made in the panels and the mouse acts directly on the preview. The title bar carries Open (an image or an SVG: Ctrl+O), the Project menu (Open a project, Save, Save as: Ctrl+Shift+O, Ctrl+S, Ctrl+Shift+S), Undo and Redo and the menu; the export button is in the Export section of the left panel, and nowhere else (Ctrl+E leads there too). The "About" window (menu) is a window of the program, in the interface language: the icon (embedded, `icons` package), the name, the version, "Developed by Gilles MAGNEVILLE", "Designed with the help of claude.ai", the website and the license; no text in it is selectable, and the Close button has the focus. `imageprintlayout file.imgpl` opens a project like a document. The file chooser offers, by default, "Images and SVG documents" (all the formats this installation reads), then SVGs alone and images alone (the latter absent if no image format is read); the program recognizes a file by its content, never by its extension, and a file that is neither an image nor an SVG is refused with a message saying so (`file.unrecognised`).

### Left panel: settings

Collapsible sections, in the order of the specification: source file (path, native size, number of analysis warnings; for an image, its format, its pixels and its resolution, or the fact that it states none), paper, margins, scale, positioning, background, export. The Background section distinguishes the three backgrounds of §8: that of the document (keep, replace with white, remove, grayed out when no background rectangle is detected), that of the page (opaque white or transparent) and a note on the gray work area. The Export section summarizes what would be written (number of pages, format), says whether the export is ready, to be confirmed or blocked, and carries the export button, the sheet preview button and, during the export, the progress bar with its cancel button. Under a separator, a second "HTML page" block writes the same layout as an HTML page (§6, "HTML page"): a "Cut the image at the cuts" box (checked by default, and specific to the session: it is not in the project), the "Export to an HTML page…" button (Ctrl+Shift+E) and a note saying that the text of an SVG takes the fonts of the computer that opens the page. The margins are four independent fields with a button to link them. A value entered applies after 150 ms without a keystroke: there is no Apply button.

### Central area: interactive preview

Two views, chosen by tab:

- **Document** (default): the entire drawing, with the cut lines, the page limits and numbers. Nothing is painted over the drawing: it is shown as it will be printed, without a tint on a page (a tint would pass for a background that the "white" or "remove" choice had not reached). The extent of each page is given by a bar along the drawing, on the outside, alternately light and blue.
- **Sheets**: the pages side by side as they will be printed, with margins and positioning. Each sheet is drawn by the function that writes the PDF (`RenderPage`), at the scale of the view's height, on a sheet of the page color (checkerboard if it is transparent); a change of page background redraws the sheets without reloading the document.

A `GtkOverlay` stacks, in order: a work area, the drawing (a `GtkPicture` placed by a `GtkFixed`), then an area that paints the page limits and numbers, the cut lines and the handles, and receives the gestures.

| Gesture | Effect |
| --- | --- |
| Wheel | Vertical scrolling |
| Shift + wheel | Horizontal scrolling |
| Ctrl + wheel | Zoom around the cursor |
| Drag on the background (left or middle button) | Panning the view |
| Double-click on the drawing, or "Add a cut" button | New cut at this position (in the middle of the view with the button), snapped to a nearby gap |
| Drag a line (6-pixel grab zone) | Move; badge in mm and in units; snapping to the gaps (radius of 8 pixels); the line stops against its neighbors |
| Alt during a double-click or a drag | Disables snapping |
| Click on a page | Selects the page (highlight, and row of the list) |
| Delete or Backspace | Deletes the selected cut |
| Up and down arrows | Move the selected cut by 1 unit; Shift: 10; Ctrl: 0.1 mm |
| Esc | Deselects |
| Right click on a line | Pop-up: enter a precise position (mm from the top of the drawing), or delete |

A line that crosses a block of the drawing carries a warning sign and the list of cuts names the first blocks crossed. An entered position that is not allowed is refused with its reason, never silently corrected; a drag, for its part, stops against the neighboring line.

During a drag, only the cuts layer is redrawn: the drawing image stays in place and the plan is recomputed at each movement, which is cheap since `layout` is pure.

**Visible window, no tiles.** A vector drawing can be redrawn at any scale: rather than cutting the document into fixed-size tiles, the interface asks the engine only for the visible part, plus a quarter of its size on each side so that a pan does not show white (`viewport.Window`), at screen scale, that is, the zoom multiplied by the window's scale factor (HiDPI screens). The image is limited to 12 million pixels; beyond that, the margin shrinks then disappears. It is produced off the interface thread, with a generation counter: a result that is no longer the one expected is dropped, unless it still covers what is visible, which avoids flicker when requests follow one another. Between two renderings, GTK scales the image: the drawing stays visible, slightly blurred, until the new image arrives. A zoom change of less than 2% redraws nothing. All the arithmetic (zoom around a point, panning, bounds, window, need to redraw) is in the `viewport` package, tested without a screen.

### Right panel: list of pages

At the top: number of pages, content width in mm and in cm, scale, then two buttons: "Add a cut" and "Automatic cuts" (the layout engine's proposal replaces the cuts, in a single history entry that can be undone; each cut is then moved by hand). Below, one card per page: number, width × height of the content (mm), scale, start and end (in units and in mm from the top of the document), state and warnings. Selecting a card highlights it in the preview and brings it into view there, and vice versa.

Then the list of cuts (rank, position in mm, number of blocks crossed and their names), linked to the preview: selecting a row selects the cut, and vice versa. At the bottom: the proposed corrections (§7) when a page overflows, each on its own button.

The "Preview" button at the foot of each card opens the Sheets view on that page, rendered as it will be exported; "Sheet preview", in the Export section, opens the same view without a selection, for all the pages before the export.

### Undo and long tasks

- **Undo and redo**: each change is a command that produces a new immutable state of the project. A drag counts as a single history entry. Ctrl+Z and Ctrl+Shift+Z; an asterisk before the name in the title bar signals unsaved changes (the session's project is compared with that of the last save or the last opening: going back to the saved state makes the asterisk disappear). Closing the window, opening another document or another project with unsaved changes opens a three-answer question: cancel, don't save, save.
- **Interface thread**: the engine is driven by a single actor (a goroutine), so a loaded document is never used by two threads at once. Results come back to the GTK thread through `glib.IdleAdd`.
- **Export**: Ctrl+E or the button in the Export section of the left panel. The checks of §11 go first: an error opens a report (page, reason) and nothing is written; a warning asks for a confirmation that describes it; then the file chooser (folder of the last export, name of the SVG with `.pdf`), the export on the engine thread with its progress bar, cancelable, and finally the rereading. The final state gives the file name, the number of pages and its size, and says that the reread PDF matches the preview.
- **Export to an HTML page**: Ctrl+Shift+E or the button of the "HTML page" block of the Export section. Same course, with the checks specific to the page (§11, `export.CheckHTML`), a file chooser that offers `<name>.html` and the page written on the engine thread (the engine is called only to draw an image that a browser does not read, §6). The final state gives the name, "whole image" or the number of pieces, the size, and says that the reread page matches the preview.
- **Errors**: each message says what failed, why and what to do, with no raw technical code.

## 10. Project format

The project is a readable, versioned JSON file, with the extension `.imgpl`. It records settings, never the drawing: the SVG stays in its original file, and nothing derived (number of pages, states, page dimensions) is stored, everything is recomputed.

```json
{
  "format": "imageprintlayout-project",
  "version": 1,
  "source": {
    "path": "schemas/mini-cad.svg",
    "absolutePath": "/home/…/schemas/mini-cad.svg",
    "sha256": "…",
    "width": 1280,
    "height": 5479
  },
  "paper": { "format": "A4", "orientation": "portrait", "customMm": null },
  "marginsMm": { "top": 10, "bottom": 10, "left": 5, "right": 5 },
  "scale": { "mode": "width", "widthMm": 200, "percent": 0, "portionUnits": 0 },
  "placement": { "horizontal": "center", "vertical": "top", "offsetXMm": 0, "offsetYMm": 0 },
  "background": { "document": "whiten", "element": "rect4521", "page": "white" },
  "cuts": [
    { "axis": "y", "position": 1380.5 },
    { "axis": "y", "position": 2760 },
    { "axis": "y", "position": 4120.25 }
  ],
  "pageOverrides": [ { "page": 2, "vertical": "center" } ],
  "export": { "outputPath": "mini-cad.pdf" }
}
```

Reading and writing rules:

- **Deterministic writing.** UTF-8, keys in a fixed order, 2-space indentation, numbers in the shortest decimal representation, atomic writing (temporary file then rename). The same project gives the same bytes.
- **Complete settings, strict reading.** All the settings are written, even those of a mode that is not chosen (the percentage when the mode is "width": switching from one mode to the other in the interface does not lose the last value entered). On reading, an unknown field or an invalid value (nonexistent paper format, negative margin, cuts out of order or outside the document, override of a page that does not exist) refuses the file, naming the field; an absent orientation, alignment and page background take their default value. The cut identifiers are not written.
- **Versions.** `version` is an integer. Each evolution adds a migration function `vN → vN+1`, tested on sample files. A file of a more recent version is refused with a clear message, never opened partially.
- **Paths.** Relative to the project file, with `/` as the separator on all platforms, and a fallback to the absolute path. If the SVG cannot be found, a dialog offers to locate it and the settings are kept; the project then carries the new location at the next save.
- **Source integrity.** The SHA-256 (that of the file as it was read at opening, not the one it has on disk at the time of saving) and the dimensions are recorded. If the hash has changed on reopening, a non-blocking warning explains that the cuts are in document units and may no longer fall where they had been placed. If the dimensions have changed, the cuts that have ended up outside the document are flagged then removed after confirmation (`app.OutsideCuts`, then `Session.OpenProject`); the project is then modified relative to the file, and the asterisk says so. A background instruction (whiten or remove an element) that no longer finds its element is dropped, with a warning (`project.background_lost`): the project must say what the preview shows. A project that the layout refuses (a nonexistent paper for this document, for example) changes nothing in the session.
- **Former name.** Up to 1.0.x the program was called "SVG Print Layout": its projects had the extension `.svgpl` and `"format": "svgprintlayout-project"`. They are version 1 files like the others: `project.Decode` accepts both format names (`project.LegacyFormat`), the file chooser offers both extensions (`project.IsProjectPath`), and the program only writes `.imgpl`. The original file is never modified; `testdata/legacy-1.0.svgpl` is the example, compared with the current reference file. The user's settings follow the same rule: `app.ImportLegacySettings` takes over `svgprintlayout/settings.json` once when `imageprintlayout/settings.json` does not exist.
- **In the session.** `app.ReadProject` reads the file and looks for the document; `Session.OpenProject` sets the document and the settings in one block, returns the warnings and clears the history; `Session.SaveProject` writes the project with the description of the document. The warnings block nothing: the user decides.
- **Per-page overrides.** Adding a cut copies the override of the page that was cut onto the two resulting pages. Deleting a cut keeps that of the page above.
- **Images.** A project on an image is a project like the others: the same version 1, the same extension, the same format. The document is described there by its path, its SHA-256 and its size **in units** (pixels at 96 dots per inch, or at the file's resolution), and the document background field is `keep` (an image has none).
- **Intact source.** The document is never opened for writing; a test compares its SHA-256 before and after each scenario.

## 11. Checks before export and determinism

Seven checks apply to the plan before the export, and two to the reread PDF afterwards. An error blocks; a warning asks for a confirmation. They cover, one by one, the points of §9.2 of the specification.

| # | Check | If it fails |
| --- | --- | --- |
| 1 | Cuts in strict order, all within `]0, H_d[`, minimum gap respected | Error |
| 2 | The sum of the strip heights equals `H_d` (tolerance 1e-9 unit) | Error |
| 3 | All pages have the same width `w = W_d × s` | Error |
| 4 | No content outside the sheet (nothing is cropped by the edge) | Error |
| 5 | Content within the usable area, otherwise margin overflow | Warning |
| 6 | Requested background consistent with the detected background | Warning |
| 7 | Unsupported SVG features, substituted fonts (§12) | Listed warning |
| 8 | Reread PDF: number of pages equal to the number of strips | Error |
| 9 | Reread PDF: size of each page equal to the requested paper, tolerance 0.02 mm | Error |

In the code (`export.Check`, `export.Verify`), each check produces a registered diagnostic (§4, `diag`): `export.no_pages`, `export.cuts_invalid`, `export.bands_sum`, `export.width_differs`, `export.outside_sheet`, `export.outside_margins`, `export.no_background` and `export.background_covers` before the export; `export.pdf_pages`, `export.pdf_size`, `export.pdf_clip`, `export.pdf_offset` and `export.pdf_unreadable` afterwards. They carry the page number and their values, never a text: the interface writes them in the chosen language.

The **HTML page** (§6, "HTML page") has no sheet: `export.CheckHTML` applies neither check 4 nor 5 to it (a plan with a page that overflows its sheet can be exported), nor the transparent-background default of check 6, which concerns only the PDF; cut, it passes checks 1 to 3 on the strips, whole it passes none of them, and the warnings of 7 apply in both cases. Once written, it is reread by `export.VerifyHTML`, which knows only the numbers of the file and compares them with the plan: `export.html_pieces` (number of pieces), `export.html_picture` (the image is there, once, and it is the one that was given), `export.html_width` (width of the column) and `export.html_band` (the top, the height and the proportion of each strip, and the height of the document, to within 0.001 unit, or to a millionth of the document's height for very large documents). A rereading error leaves no file (`VerifyError`, type "HTML").

Check 9 compares points: an A4 is 595.28 × 841.89 pt. The correspondence of the cuts with the preview comes from the construction (same plan, §6); checks 8 and 9 reread the written PDF (`export.ReadPDF`, without a PDF library: reference table, object streams, compressed contents), and this same survey proves it: the clip rectangle and the offset of the drawing of each page can be read in the content stream, and are compared with the plan to within 1/256 of a point (this is the method of the `engine` tests, `docs/M0.md`). Cairo writes a clip only for an operation that needs one: a document without a background rectangle (the background removed) would leave none for the strip. With a white page, the engine therefore paints white on the strip, inside the clip (no effect on the image, the sheet being already white), so that the clip of the strip is written in full at the start of the drawing: `export.Verify` then requires it. With a transparent page, nothing of the kind is possible without changing the image: the file keeps only the clips of the elements that cross a cut, each brought back to the strip, and `Verify` checks that none goes outside the strip's window. The equivalent visual check is done by the tests (§13).

### Determinism

- **No timestamp and no random identifier** in the PDF: cairo writes the creation and modification dates by default, the engine fixes them at `2000-01-01T00:00:00Z`. The "producer" field contains the cairo version.
- **Stable inputs**: the background instructions are replayed in a fixed order, and `layout` is sequential, with no parallelism that would change the order of floating-point operations.
- **Accepted limit**: byte equality is guaranteed at identical cairo and librsvg versions, and identical fonts. Between two different machines, the tests compare rasters with a tolerance rather than bytes.
- **Proven in M0, on one machine**: three exports of the same document (one of them after reloading, a second apart) give files with the same SHA-256, on the synthetic SVG and on the Mini-CAD.

## 12. SVG coverage and reporting

librsvg renders most documents; the application adds its own analysis to tell the user, before the export, what will be rasterized, replaced or ignored. It never tries to work around a limit of the engine: what it does not draw is flagged, not reconstructed.

The analysis is done in two passes:

1. **XML pass** (`svgdoc`): a single walk through the working copy, guided by a catalog of rules (element or attribute, level, message).
2. **Engine pass**: resolution of the requested font families and verification of the linked images.

| Finding | Level | Message |
| --- | --- | --- |
| `filter` element or `filter` attribute | Warning | Rasterized effect: it may appear as a bitmap in the PDF |
| `script`, `animate`, `set` | Warning | Ignored: librsvg handles neither script nor animation |
| SVG fonts (`font`, `glyph`) | Warning | Not supported: a system font is used instead |
| `foreignObject` element, `flowRoot` flowed text, `use` pointing to another file | Warning | Not drawn: it will be missing from the preview and the PDF |
| Web font (`@font-face`) | Warning | Not loaded: an installed font replaces it |
| Font family missing from the system | Warning | Font "X" missing, replaced by "Y" |
| Image linked by path, file not found | Warning | Linked image not found, expected name and path |
| Image linked by remote URL | Warning | Not downloaded: the application never accesses the network |
| Embedded raster image | Information | Kept as a bitmap, at its original resolution |
| Neither dimension nor usable `viewBox` | Error | Document size undetermined |

Each category is flagged at most 25 times; the rest is counted in a single finding ("N other similar findings"). A `foreignObject`, a `flowRoot` and a web font are warnings: librsvg does not draw them or does not load the font, and nothing is reconstructed. A document whose XML is malformed (unclosed tag, end tag that does not match) is refused by the engine; the analysis says so with the reason.

The report is displayed in the "Source file" section, with a counter per level; each finding is a translated text (`diag.svg.<code>`). Each finding on an element carries its reference (its identifier, or its index path): clicking an entry will select the element in the preview (not done yet). The report can be copied as plain text.

The text is drawn with the system fonts. M0 shows that it stays text in the PDF, selectable, with embedded TrueType subsets. It also shows that a missing font is replaced **without a word** (IBM Plex replaced by DejaVu, and the PDF embeds the replacement font): the flagging above is therefore the only protection, and it requires a font resolution by fontconfig in `engine`. It is done: `MatchFont` asks fontconfig for the family that the system would give, and `svgdoc.CheckFonts` flags every `font-family` list whose first family is not installed, with the family that will actually be drawn (the next one in the list that is installed, failing that the generic family or what fontconfig gives); a list that starts with a generic family (`sans-serif`, `monospace`) is never flagged, the document having asked for nothing specific.

### Raster images

The same principle: what the engine cannot do is stated, not worked around. A file is recognized by its first bytes (`engine.Detect`: SVG, PNG, JPEG, TIFF, GIF, BMP, WebP), never by its name; `app.Prepare` sends images to `prepareImage`, the rest to the SVG analysis, and a file that is neither is refused (`file.unrecognised`, with the reason).

| Finding | Level | Message |
| --- | --- | --- |
| File unreadable by the decoder | Error | `image.unreadable`, with the decoder's reason |
| Known format, decoder absent (WebP without a loader under Linux) | Error | `image.unsupported`: this installation does not read this format |
| More than 100 million pixels (`engine.MaxImagePixels`), or a side that cairo cannot have (32,767) | Error | `image.too_large`: refused **before** decoding, the size being read from the header ("size-prepared"), so an image that is too large does not eat up memory |
| PNG or JPEG whose end is missing | Warning, or error depending on the decoder | `image.truncated` with gdk-pixbuf 2.42 (Debian 13, Ubuntu 24.04, RHEL 10): the decoder returns what it read without an error, the engine compares with the expected end (`engine.Complete`). With gdk-pixbuf 2.44 (glycin: Ubuntu 26.04, Fedora 44), the loader refuses the file: `image.unreadable`, with the decoder's reason stripped of its jargon (`tidyDecoderMessage`) |
| Multipage TIFF | Warning | `image.multipage`: only the first page is read |
| Animated GIF | Warning | `image.animated`: only the first frame is read |
| Resolution absent or invalid | Information | `image.no_dpi`: counted at 96 dots per inch, at 100% the size of a screen |

The findings are the same diagnostics as those of an SVG (§4, `diag`): a code, values, a text per language. The formats read by this installation come from gdk-pixbuf (`engine.SupportedImageFormats`): the file chooser's filters announce only those. A TIFF is read at its first page.

**What the engine reads itself in the file.** The two generations of gdk-pixbuf do not report the same things: 2.42 gives `x-dpi`, `y-dpi` and `multipage` and itself applies some of a TIFF's orientations; 2.44 loads through glycin, which reports only the orientation (faithful to the tag, pixels not rotated) and the original size, with no resolution, no page count and no animation. The engine therefore relies on the decoder neither for the **resolution** (`fileResolution`: `pHYs` of a PNG, JFIF then Exif of a JPEG, tags of a TIFF), nor for the **pages** of a TIFF (`fileMultiPage`: a second directory), nor for the **animation** (`fileAnimated`: several frames of a GIF, animation flag of a WebP, `acTL` of a PNG), all in pure Go, without panicking on a damaged file (`TestFileResolutionSurvivesDamagedFiles`). It keeps the decoder's orientation, correct with both: the tests therefore verify the visible result, straightened image, for the eight orientations, and not the tag. That of a JPEG (EXIF) is applied by a matrix at drawing time. What is not supported: PDF and RAW formats as input, ICC color profiles (the pixels are printed as the decoder renders them) and animated images other than through their first frame.

## 13. Test strategy

The 14 acceptance tests are all automated except the mouse gesture, checked by hand on each platform; the commands it triggers (double-click, drag, entry, keyboard), for their part, are tested automatically, and the window is tested under Xvfb. They are all in `internal/acceptance` (`TestAcceptance1To7`, `TestAcceptance8To13OnTheScenario`, `TestAcceptance14ReopenTheProject`) and the CI runs them on Ubuntu, on each distribution tried (Debian family and Red Hat family) and under MSYS2 on Windows. Tests 8 to 13 are in `internal/acceptance`, which makes the session, the engine, the export and the written PDF work together (`TestAcceptance8To13OnTheScenario`, with the determinism and the integrity of the source SVG); they rasterize the PDF with poppler (`pdftoppm`, `pdfinfo`), so the CI's `engine` job installs it.

| # | Acceptance test | Verification | Type |
| --- | --- | --- | --- |
| 1 | Import the Mini-CAD | Dimensions read: 1,280 × 5,479, no blocking error | Auto |
| 2 | A4 portrait | Usable area 200 × 277 mm with the default margins | Auto |
| 3 | Width of 200 mm | `s` = 0.15625 and `w` = 200 mm, tolerance 0.005 mm | Auto |
| 4 | Proportions kept | A single scale in the model: no function accepts two factors | Auto |
| 5 | Cuts with the mouse | Scenario under Xvfb for GTK; commands tested in pure Go | Auto, gesture by hand |
| 6 | Add a cut without changing the width | Property: `w` before equals `w` after, for any addition | Auto |
| 7 | Move a cut without changing the scale | Property: `s` before equals `s` after, for any move | Auto |
| 8 | Pages 10 mm from the top | `y` = 10 mm on each page; first non-white pixel of the PDF raster at 10 mm, tolerance 0.2 mm | Auto |
| 9 | Gray background replaced by white | Background detected; pixels of the former gray area white in the export raster | Auto |
| 10 | Export of a multipage PDF | Number of pages equal to the number of strips | Auto |
| 11 | Dimensions of the PDF pages | Page size reread: 595.28 × 841.89 pt, tolerance 0.02 mm | Auto |
| 12 | Same width on all pages | Plan, then raster measurement of a full-width element on each page | Auto |
| 13 | Cuts consistent with the preview | Seam test (below): clip and offset read in the PDF equal to the plan, and pixels by squares | Auto |
| 14 | Reopening the project | Save, reload, deep equality of the settings; reference JSON; the PDF exported after the reopening has the same bytes as the one before | Auto |

### HTML page

Three levels, like the PDF. In pure Go: `export/html_test.go` (whole and cut pieces, deterministic writing, **simulation of the style rule** `background-size` / `background-position` which finds again the top and the height of each strip, a rereading that fails if a strip, the width or the image changes, white background, refusal, `.part` file never left behind and good file never overwritten, `CheckHTML`, `ImageSource`) and `svgdoc/standalone_test.go` (linked images embedded, link lost with a removed background). With the engine: `acceptance/html_cgo_test.go` writes the page of a drawing (cut and whole), of an image rotated by its tag (the red quadrant is where the preview puts it) and of a cut image. In the window: `ui_test.go` exports a cut page (four pieces, captions "Page 3 of 4") and a whole page, and reads the final state. The formulas were also compared once, outside the repository's tests, with the rendering of a real browser (Chromium): each piece coincides with the strip rendered by the engine, to within a pixel, and the orientation is right.

### Seam test

Two complementary checks, because neither is both exact and robust (measurements and reasons in `docs/M0.md`):

- **The geometry written in the PDF.** In the content stream of each page we read the clip rectangle and the place where the drawing is painted, and we compare them with the plan, to within 1/256 of a point. It is the exact check, valid for any cut position; it also establishes the equality of width.
- **The pixels.** We rasterize the produced PDF, cut each page to its content window, and compare it with the rendering of the whole document at the same scale, by squares of 16 × 16 pixels whose gray average is compared (24 levels tolerated; 14 measured at most). This check sees content that is lost, repeated or moved by a few millimeters. It does not compare pixel by pixel: a rasterizer places glyphs on a quarter-pixel grid and snaps thin strokes to the pixel grid, so a hundredth-of-a-pixel discrepancy at the origin of a page moves a stroke by a whole pixel.

A test verifies that each of the two checks fails when a page is shifted. Together, they answer the final criterion of the specification: a PDF faithful to the preview, without silent resizing.

### Organization

- **Three levels.** Pure Go (model, units, layout, analysis, project) with `go test` everywhere without cgo; engine integration (loading, preview windows, export) with librsvg installed; interface under Xvfb on Linux, plus a manual checklist on Windows.
- **Property tests** on `layout`: papers, margins, scales and cuts drawn at random, with exact coverage, constant `w`, `s` unchanged by the commands and consistent states as invariants.
- **Test images.** `internal/testimg` (pure Go) makes reference images with no file in the repository: a checkerboard of four quadrants of distinct colors (which makes any rotation or symmetry visible), PNG with resolution, TIFF with orientation, JPEG with JFIF density, resolution or EXIF orientation, and a PNG that announces a huge size without containing it. The tests of the engine, of `app`, of the export and the acceptance tests use them: the 14 acceptance tests also pass on an image (`TestTheScenarioOnAPicture`), and a test runs them for each format and each of the eight orientations (`TestPicturesOfEveryKindAreExportedWhereThePlanPutsThem`).
- **Test set.** The real Mini-CAD is not in the repository: it describes a real infrastructure. The tests take it when `SVGPL_MINICAD` gives its path. The deterministic generator `internal/testsvg` produces a public equivalent of 1,280 × 5,479 units (gray background, blocks with text, full-width banner, transformed rectangle); with `Filler`, it also serves as a 100,000-element load test (`internal/app/load_cgo_test.go`, opening in 4 s on the development machine).
- **Continuous integration** (all in place): an `engine` job on Ubuntu with `librsvg2-dev`, `poppler-utils` and `fonts-ibm-plex`; a `ui` job (GTK4 under Xvfb: window tests, then screenshots in the four languages, deposited as an artifact); a `linux-packages` job (build of the `.deb` and the `.rpm`, lintian, AppStream, installation, `-selftest`, installed program's window, identical builds); a `distributions` job (the family's package is built in a container of each supported system, installed by `apt` or `dnf`, then the installed program runs `-selftest` and the window in the four languages; on Debian 12, Ubuntu 22.04 and AlmaLinux 9, the test verifies that the build is refused with a clear message) and a `windows-package` job, on `windows-2022` and `windows-latest`, under MSYS2 UCRT64 (build by `build-windows.ps1`, `-selftest` then window in the four languages of the portable version, installed program, uninstallation). The `release.yml` workflow publishes a version: a `vX.Y.Z` tag on a commit of `main` (the tag must be that of `VERSION`, and both CHANGELOGs must have its section) builds without changing `VERSION` the `.deb` on Ubuntu 24.04 and the `.rpm` on AlmaLinux 10, with the script of the `distributions` job (build, installation, installed program, 14 acceptance tests), then the Windows packages on `windows-2022` (build, portable version, installer), and creates the GitHub release with the packages, their SHA-256 checksums and the section of the CHANGELOG (English, then French); it also runs, without publishing anything, when it is modified in a pull request or on demand. The other packages are built and tested without cgo (`CGO_ENABLED=0`), where `ui` is a stub.
- **References.** The reference rasters are stored per platform, with a tolerance, and never compared from one platform to another.

## 14. Delivery plan

Six milestones, of which M0 is the decision point: the page computation (M1) does not depend on the engine and can proceed in parallel, whereas M2 to M5 wait for the verdict of M0.

| Milestone | Content | Exit criterion |
| --- | --- | --- |
| M0. Truth prototype | Minimal chain, first on the synthetic SVG then on the real Mini-CAD: loading by librsvg, rendering of a window, 4-page A4 PDF with clip. Build under Linux and under Windows. Measurements: PDF size, text, resolution of rasterized effects, determinism | 4 pages of 595.28 × 841.89 pt, `w` = 200 mm, seam test passed, two exports with the same SHA-256, PDF size not multiplied by the number of pages, Windows build that produces the same plan. Otherwise: switch to engine D |
| M1. Core in pure Go | Model, units, layout (scale, strips, positioning, states, corrections), project format, property tests | Green tests without cgo; tests 2, 3, 4, 6, 7 and 14 passed at the logical level |
| M2. Document and preview | Working copy, analysis and report, engine (loading, preview windows), three-panel window, zoom, panning, page limits | Mini-CAD displayed in full, smooth zoom, readable analysis report |
| M3. Cuts | Mouse interaction, list of pages, snapping, undo, automatic cuts, crossing detection | Tests 5, 6 and 7 passed; on the Mini-CAD, the automatic proposal crosses no block when a gap exists |
| M4. Background and export | Background detection and replacement, PDF export, checks, page preview | Tests 8 to 13 passed |
| M5. Project, packages, acceptance | Saving and reopening in the interface, .deb package, Windows installer, complete acceptance | The 14 tests pass on each supported system: Debian 13 and Ubuntu 24.04 (Debian family), RHEL 10 or a clone (Red Hat family), Windows 10 and Windows 11 |

**Status.** The repository skeleton, the `units` package, the engine prototype (M0a, `docs/M0.md`), the scripts that build the `.deb`, `.rpm` and Windows packages, the `i18n` translation package and a first GTK4 window (three panels, four languages, opening an SVG displayed in full; M0b) exist, with their CI, which builds and tries the package on seven Linux distributions (Debian and Red Hat families) and on Windows. M0 is not finished: the comparison of the plan produced under Windows and under Linux remains to be done. **M1 is done**: `model`, `diag`, `layout` (scale, sheet, plan, states, corrections, cuts, snapping, automatic cuts) and `project` (`.imgpl` format) are written and tested without cgo, with property tests; the window's panels do not use them yet. **M2 is done**: `svgdoc` (analysis, findings, fonts, blocks, background, working copy), the engine's `Document` interface (loading by path for linked images, fonts through fontconfig), `app` (session, history, plan), `viewport` and the window (Source file, Paper, Margins, Scale, Positioning panels; list of pages with the proposed corrections; preview with zoom, panning, page limits and numbers, undo and redo) exist and are tried under Xvfb. **M3 is done**: `app` gives the session the commands on the cuts (add, drag, move, nudge by a step, delete, automatic cuts), each a history entry, and none touches the scale (tests 5, 6 and 7); the preview receives them through the mouse (double-click, drag, snapping to gaps, Alt to disable it, right click, keyboard) and the list of pages shows the cuts and their crossed blocks, with a shared selection. **M4 is done**: `export` (pages of the plan, checks before the export, rereading of the PDF, atomic writing), the engine (`RenderPage`, progress, cancellation), the background commands of `app` (`SetDocBackground`, `SetPageBackground`, `Rebuild`), the Background and Export sections, the Sheets view, the preview button of each page and `internal/acceptance` (tests 8 to 13, determinism, source SVG intact) exist, and the window exports a reread and verified PDF. **M5 is done**: `app` reads, writes and sets a project (`ReadProject`, `SaveProject`, `OpenProject`), the window has the Project menu (open, save, save as), locates a moved document, flags what a project no longer finds (modified document, changed size, cuts outside the document, lost background), shows the asterisk of unsaved changes and asks before losing them; `internal/acceptance` automates the 14 tests, and the CI runs them on each distribution tried and under Windows. The manual acceptance was done under Ubuntu and under Windows (on a real machine, with the CI's packages, which only has Windows Server): version 1.0.0 is published. **Version 1.1.0** changes the name of the program (Image Print Layout, formerly "SVG Print Layout") to prepare for the opening of images, and carries over what exists: `.svgpl` projects, settings, installed packages. **Version 1.2.0** opens images (JPEG, PNG, TIFF, WebP, BMP, GIF) as documents: same plan, same cuts, same preview, same PDF (§12, "Raster images"). **Version 1.3.0** adds the export to a **single HTML page**, whole image or cut at the cuts (§6, "HTML page"). The `CHANGELOG.md` holds the detail.

After v1, §14 of the specification remains the list of possible follow-ups; the import of raster images is done (1.2), the export to an HTML page too (1.3); the import of a PDF remains to be done. A preview of the real PDF reread by an independent PDF engine (the specification asks for "a preview of the final PDF") is the first candidate: v1 sticks to the preview produced by the same drawing function, which is faithful to the PDF by construction.

## 15. Risks and open questions

The first five risks are settled at milestone M0. M0a settled one and advanced two (see under the table); the M0b window goes no further than what it shows, and engine D remains the fallback until the verdict is delivered.

| Risk | Effect | Mitigation | Settled in |
| --- | --- | --- | --- |
| Windows packaging of GTK4, librsvg and their dependencies | Heavy installer, or impossible build | Build under MSYS2 UCRT64, trial on a clean Windows machine, engine D as a fallback | M0 |
| librsvg fidelity on the real Mini-CAD (text, fonts) | Rendering different from the original | Trial of the real file, compared with an Inkscape rendering | M0 |
| PDF size: the drawing repeated on each page | File N times bigger | Direct rendering on each page, under a clip: cairo writes only what touches the window (the recording surface is ruled out, §6) | M0a, settled: ×1.08 |
| Resolution of rasterized elements (filters) in the PDF | Blur or visible pixels when printed | Measurement, warning from the analysis on, study of the fallback resolution setting | M0 |
| Determinism depending on the cairo and librsvg versions | Different bytes from one machine to another | Versions frozen in the Windows packages; comparison by rasters between platforms | M0 then M5 |
| Maturity of gotk4 | Memory leaks, crashes | Thin interface, engine independent of gotk4, scenarios under Xvfb, Fyne fallback | M2 |
| Very heavy documents | Slow loading and rendering, memory | 100,000-element test, rendering of the visible window only, loading off the interface thread | M2 |
| Font missing on the export machine | Substituted text | Detection and warning before the export | M2 |
| Huge or truncated image, or format without a decoder (1.2) | Memory exhausted, image silently cut, late refusal, wrong scale for lack of resolution | Size read from the header before any decoding (100 million pixels at most), end of the file verified, formats announced according to the loaders present, resolution, pages and animation read in the file, hence independent of the gdk-pixbuf version (§12) | 1.2 |

After M0a (`docs/M0.md`):

- **PDF size**: settled, ×1.08 on the Mini-CAD.
- **librsvg fidelity**: advanced. The rendering agrees with that of Chromium (mean difference of one gray level); the comparison with Inkscape remains to be done.
- **Determinism**: advanced. Established on one machine, with fixed dates; it remains to be observed between versions and between machines.
- **Rasterized elements**: not advanced, the Mini-CAD has no filter.
- **Windows packaging**: advanced. The package (DLLs of GTK 4, cairo and librsvg, fontconfig, image loaders, schemas, icons and GTK translations) builds under MSYS2 UCRT64, passes `-selftest` with only the Windows PATH and opens its window in the CI; the comparison of the plan produced under Windows and under Linux remains to be done.
- **Missing font**: the risk is greater than was thought, because the substitution is silent and the PDF embeds the replacement font. Detection becomes mandatory, not just desirable.
- **Export time** proportional to the number of pages (about 0.25 s per page on the Mini-CAD), and rendering of a preview window of 0.24 to 0.41 s: handled in M2 by rendering only the visible window (a quarter margin, capped at 12 million pixels) and by keeping the previous image, scaled, while the next one is being computed.

### Open questions

1. **Real file.** The Mini-CAD was provided and is used for M0a (not versioned). A second representative SVG, with filters, images or embedded fonts, would be useful for the rasterized elements.
2. **Vertical cuts.** Are they really out of v1, as §14 of the specification and test 12 suggest?
3. **Mode B.** Does the definition of §1 fit: a scale that makes the chosen portion fit in the usable height?
4. **Preview of the final PDF (§9.2).** v1 provides it with the same drawing function as the export. Should the PDF file also be reread with an independent PDF engine from v1?
5. **Text in the PDF.** The engine gives it effortlessly: selectable text, fonts embedded as subsets. It remains to be decided whether this is a guarantee of the product, or only the appearance.
6. **Interface language.** *Settled: French, English, Spanish and German from the first version of the interface (§4, "Languages").*
7. **Linux versions to support.** *Settled for v1: the systems that have GTK 4.14, GLib 2.80 and librsvg 2.52 at least (§4, "Supported platforms"), i.e. Debian 13, Ubuntu 24.04, the RHEL 10 family and Fedora, with Windows 10 and 11.* It remains to be decided whether Debian 12, Ubuntu 22.04 and RHEL 9, which are still very widespread, justify later a package that embeds GTK (Flatpak or AppImage), or an older version of gotk4 and a reduced interface. The librsvg version varies from one system to another (2.57 to 2.63), which affects fidelity and determinism: PDFs are only identical at equal libraries (§11).

## Sources

Pages consulted on 19 October 2026.

- [librsvg: overview and rendering to cairo](https://gnome.pages.gitlab.gnome.org/librsvg/Rsvg-2.0/overview.html)
- [librsvg: measuring the geometry of an element](https://valadoc.org/librsvg-2.0/Rsvg.Handle.get_geometry_for_element.html)
- [GTK: installation and distribution under Windows](https://gtk.org/docs/installations/windows/)
- [gotk4: GTK4 bindings for Go](https://github.com/diamondburned/gotk4-libgirepository)
- [Fyne: prerequisites and license](https://github.com/fyne-io/fyne)
- [tdewolff/canvas: rasterization engine](https://pkg.go.dev/github.com/tdewolff/canvas/renderers/rasterizer)
- [svg2pdf-py: svg2pdf binding, vector PDF](https://pypi.org/project/svg2pdf-py/)

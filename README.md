<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Image Print Layout

🇫🇷 [Version française](README.fr.md)

Desktop application (Linux — Debian, Ubuntu, Red Hat and their derivatives —, Windows 10 and 11, written in Go) to **lay out, cut and print an SVG document or an image (JPEG, PNG, TIFF…) on several sheets of standard paper**, then export it as a **multi-page PDF** (vector for an SVG) or as a **single HTML page**, the whole image or cut at the breaks.

Developed by Gilles MAGNEVILLE, designed with the help of [claude.ai](https://claude.ai).

> **Image Print Layout was called "SVG Print Layout" up to version 1.0.x.** The name changed because the program also opens images (JPEG, PNG, TIFF…), which it has done since 1.2.0. The `.svgpl` projects of 1.0.x still open, the chosen language is kept and installing a package replaces the old one (see the [CHANGELOG](CHANGELOG.md)).

> **Status: version 1.3.1, all the functions of the specification are there (milestones M1 to M5), and the acceptance run was done by hand on Ubuntu and on Windows.** The GTK4 window, in **French, English, Spanish and German**, opens an SVG or an image (JPEG, PNG, TIFF, WebP, BMP, GIF), analyzes it (unsupported features, missing fonts, missing images; for an image, its resolution, a truncated file or a multi-page one), computes its pages with the paper, margin, scale and positioning settings, and shows the preview with the page limits and numbers, the list of pages and the corrections offered when a page overflows. You add, move and delete the cuts with the mouse (double-click, drag, snapping to empty areas, keyboard) or ask for automatic cuts, without ever changing the scale. You choose the background of the document (keep it, replace it with white, remove it, with an immediate preview) and that of the page (opaque white or transparent), you see each sheet as it will be printed, then you export a multi-page PDF that the program rereads and checks before delivering it, or a single HTML page (Ctrl+Shift+E), also reread. You save the project (settings, cuts, background, positions) and reopen it later: the original file is never modified, a moved document can be relocated, and an asterisk in the title flags unsaved changes. The 14 acceptance tests of the specification are automated (`internal/acceptance`). The repository also contains the engine prototype (librsvg + cairo: rendering of a window and multi-page PDF export, validated on the Mini-CAD, see [`docs/M0.md`](docs/M0.md)), the scripts that build the `.deb`, `.rpm` and Windows packages (see [Packages](#packages)) and the CI.

## Principle

1. You open an SVG of any dimensions (for example a diagram of 1,280 × 5,479 units) or an image (a photo, a scan, a long screenshot).
2. You choose the paper format, the orientation, the margins and the width of the drawing (e.g. 200 mm on A4 portrait).
3. You place the cuts with the mouse on the preview, or accept an automatic proposal.
4. You check the result page by page, then export a single PDF, or a single HTML page (see [HTML page](#html-page)).

Fundamental rules, taken from the specification:

- **A single scale for the whole drawing**: proportions are never modified, and no page is silently shrunk to "make a portion that is too tall fit" — the user is warned and chooses (add a cut, move a cut, change the scale or the paper).
- **The number of pages follows from the dimensions and the cuts**; it is never imposed.
- **The original file is never modified**: all transformations (white background, removal of an SVG's background…) are applied to a working copy.
- **The PDF is faithful to the preview** and deterministic: same parameters, same result.
- **No online service**.

## Formats it opens

The format is recognized from the content of the file, never from its extension. An image is cut and printed like an SVG: same single scale, same cuts, same preview, same PDF.

| Format | What the program does with it |
| --- | --- |
| SVG | Vector rendering by librsvg; the PDF keeps the paths and the text. |
| JPEG | Resolution (JFIF, otherwise Exif) and orientation (EXIF) read. The JPEG goes into the PDF with its original bytes, without recompression, when the PDF can read it (8 bits, gray or YCbCr). |
| PNG | Resolution and transparency read; the page background shows through the transparent parts. |
| TIFF | First page; resolution and orientation read; a multi-page file is flagged. |
| WebP | Read when the gdk-pixbuf WebP loader is installed (`webp-pixbuf-loader` on Linux, which the packages recommend). Otherwise, the file is refused, with a message saying so. |
| BMP, GIF | Read; for an animated GIF, only the first frame. |

The size of an image on paper is the one its file states: its pixels divided by its resolution (a 3,000-pixel photo at 300 dots per inch is 254 mm at 100%). Without a resolution, it is counted at 96 dots per inch, like a screen, and the program says so. An image is at most 100 million pixels: beyond that, it is refused before being decoded. An image has no separate background to replace: only the page background acts, through its transparent parts. Its automatic cuts fall between its blocks when it has a plain background (a screenshot, a diagram, a scanned page) and are spread evenly when it has none (a photograph).

## HTML page

The "Export as HTML page…" button of the Export section (Ctrl+Shift+E) writes the layout as **a single `.html` file**, which opens in any browser, offline, without an accompanying folder: the image is written only once.

- **Cut at the breaks** (box checked by default): one piece per page of the plan, one below the other, with their caption ("Page 2 of 4"). These are exactly the strips of the preview. The browser prints one piece per sheet.
- **Whole** (box unchecked): the image in one piece, at the width of the plan.
- An SVG stays vector in the page (its text uses the fonts of the computer opening the page) and its linked images are embedded in it. A JPEG or a PNG is written as is; a TIFF, a GIF, a WebP or a BMP, as the preview shows it, as a PNG.
- Like the PDF, the page is reread after writing and compared with the plan; if it does not match, nothing is delivered. Beyond 256 MB, the export is refused.

## Code organization

```
cmd/imageprintlayout/   entry point: the window, or -version, -selftest, -lang
build-deb.sh, build-rpm.sh, build-windows.ps1   build the packages (see Packages)
debian/, metainfo/, icons/, windows/   files they package; docs/imageprintlayout.1 (+ fr/, es/, de/): manual pages
internal/
  units/     SVG / px / pt / mm conversions, paper formats, margins      (pure Go, tested)
  model/     shared data types                                           (pure Go)
  layout/    page computation, strips, statuses, automatic cuts          (pure Go)
  project/   .imgpl project file (versioned JSON, deterministic writing) (pure Go)
  svgdoc/    SVG reading: findings, fonts, blocks, background, working copy   (pure Go)
  engine/    rendering engine: Document interface, librsvg (SVG) + gdk-pixbuf (images) + cairo + fontconfig through cgo
  export/    multi-page PDF and HTML page exports: checks before, rereading and verification of the written file   (pure Go)
  app/       session (project, plan, history), opening a document        (pure Go)
  viewport/  view arithmetic: zoom, panning, window to draw              (pure Go)
  diag/      warnings and errors meant for the user (a code and values)
  i18n/      translation: French, English, Spanish, German; numbers     (pure Go, tested)
  ui/        graphical interface: GTK4 through gotk4 (stub without cgo)
  testsvg/   deterministic test SVG generator                           (pure Go)
  testimg/   deterministic test image generator (PNG, TIFF, JPEG)       (pure Go)
  acceptance/ the acceptance tests of the specification over the whole chain
  repocheck/ tests on the repository itself (licence headers, versions, documentation)
docs/        milestone reports (M0.md, M0.fr.md)
testdata/    reference documents
```

Dependencies point from `ui` to `app` and `viewport`, then from `app` to `layout`, `project`, `svgdoc` and `export`; `app` only calls `engine` through its `Document` interface. Only `ui` and `engine` use cgo: all the other packages are tested everywhere, without any graphics library installed.

## Documentation

Every document exists in two languages: English (the default file, `X.md`) and French (`X.fr.md`), each linking to the other at the top of the file.

- [`PROJECT_CONTEXT.md`](PROJECT_CONTEXT.md): why the project exists, its principles and its scope.
- [`ARCHITECTURE.md`](ARCHITECTURE.md): the technical contract (packages, interfaces, formulas, project format, checks, delivery plan). It is authoritative for everything concrete.
- [`CONTRIBUTING.md`](CONTRIBUTING.md): report a bug, propose an improvement, contribute code.
- [`docs/M0.md`](docs/M0.md): report on the rendering engine prototype (measurements, discoveries, what remains to be validated).
- [`CHANGELOG.md`](CHANGELOG.md): the history of changes.

## Development

Prerequisite: Go 1.24 or newer.

```sh
gofmt -l .          # must print nothing
go vet ./...
go test ./...
go run ./cmd/imageprintlayout -version
```

The CI (GitHub Actions) runs these commands on Ubuntu and Windows, with `CGO_ENABLED=0`.

The rendering engine (`internal/engine`) requires cgo, librsvg, gdk-pixbuf and cairo, and its tests read PDFs with poppler. On Ubuntu 24.04:

```sh
sudo apt-get install librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev poppler-utils fonts-ibm-plex
CGO_ENABLED=1 go test ./internal/engine ./internal/testsvg ./internal/testimg
```

Without `CGO_ENABLED=1`, the `engine` package is empty and the rest builds and is tested without any graphics library. A missing poppler tool makes the test concerned skip, except when the `CI` variable is set.

The window (`internal/ui`) additionally requires GTK 4.14 or newer (gotk4 v0.3.1): without cgo, `ui` is a stub and the program says it was built without a graphical interface. On Ubuntu 24.04:

```sh
sudo apt-get install librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev poppler-utils fonts-ibm-plex
go run ./cmd/imageprintlayout                       # the window
go run ./cmd/imageprintlayout -lang de plan.svg     # in a given language, with a document (SVG or image)
```

The window tests open real windows: without a display (terminal, CI), they run on a virtual display, with `sudo apt-get install xvfb xauth`:

```sh
xvfb-run -a go test ./internal/ui ./cmd/imageprintlayout
```

The first compilation of gotk4 takes several minutes; later ones use Go's cache. On each compilation of gotk4, gcc prints warnings about `free` (`-Wbuiltin-declaration-mismatch`): they come from the library, and the package scripts hide them. To hide them in development too: `export CGO_CFLAGS="-O2 -g -Wno-builtin-declaration-mismatch"`. For Windows (MSYS2 UCRT64), see [`windows/README.md`](windows/README.md).

## Supported systems

| Family | Systems | Package |
| --- | --- | --- |
| Debian and derivatives | Debian 13 and later, Ubuntu 24.04 LTS and later, and their derivatives (Linux Mint 22, Pop!_OS 24.04, elementary OS 8…) | `.deb` |
| Red Hat and derivatives | RHEL 10 and its clones (CentOS Stream 10, AlmaLinux 10, Rocky Linux 10…), Fedora | `.rpm` |
| Windows | Windows 10 and 11, 64-bit | portable `.zip` or `.exe` installer |

On Linux, what matters is the version of the libraries: GTK 4.14, GLib 2.80 and librsvg 2.52
at least. **Debian 12, Ubuntu 22.04 (and Linux Mint 21) and RHEL 9 (and its clones) are too
old**: the build scripts say so instead of compiling. The CI builds and
installs the package in a container of each of the supported systems and runs the
installed program (details in [`ARCHITECTURE.md`](ARCHITECTURE.md), §4 "Supported
platforms").

## Packages

Three scripts, at the root, build the packages (as in Planche-Contact):

```sh
./build-deb.sh              # Debian, Ubuntu and derivatives: imageprintlayout_<version>_amd64.deb
./build-rpm.sh              # Fedora, RHEL and derivatives: imageprintlayout-<version>-<release>.x86_64.rpm
```

```powershell
.\build-windows.ps1         # Windows 10 and 11: a portable .zip and an .exe installer
```

The packages of each version are also attached to its [GitHub release](https://github.com/gillesmagneville/image-print-layout/releases) (`.deb`, `.rpm`, Windows installer and portable version, with their SHA-256 checksums).

The packages are written at the root of the repository (not versioned). The three scripts have the
same options and handle the `VERSION` file the same way:

| Option (PowerShell) | Effect |
| --- | --- |
| `--major` (`-Major`) | increments the major number (1.2.3 → 2.0.0) |
| `--minor` (`-Minor`) | increments the minor number (1.2.3 → 1.3.0) |
| `--patch` (`-Patch`) | increments the patch number (1.2.3 → 1.2.4) |
| `--no-version-change` (`-NoVersionChange`) | rebuilds without changing the version |
| `--clean` (`-Clean`) | deletes the build folder and the packages already built |
| `--help`, `-h` (`-Help`) | prints the help |

Without an option, the script shows the current version, offers to move to major, minor or
patch or to change nothing, then asks for confirmation (`o`, for the French "oui") before building. **The
`VERSION` file is only updated once the package is built**: a failure never
leaves a version without a package. Under PowerShell, `-Oui` answers "yes" to all
the questions (for unattended use).

A pre-release version such as `0.1.0-dev` is published first: `--minor` and
`--patch` turn it into `0.1.0`, `--major` into `1.0.0`. Ordinary versions
are incremented as usual. When building the three formats of the same version,
increment with the first script and rerun the other two with
`--no-version-change` (`-NoVersionChange`):

```sh
./build-deb.sh --minor
./build-rpm.sh --no-version-change
```

After a version without a suffix, update all the files that carry it: the
`## [X.Y.Z]` section of `CHANGELOG.md` (and of `CHANGELOG.fr.md`), the `<release>` of the AppStream file, the `.TH` line of the four
manual pages, the statuses of the README and of `ARCHITECTURE.md` (see `CONTRIBUTING.md`; a `repocheck` test
fails as long as one of them is behind).

Each script checks its prerequisites, including the version of the libraries. A package is tied to the
libraries of the machine that built it: build it on the system where it will be
installed. Linux packages are installed with `apt install ./…deb` or `dnf install ./…rpm`.
On Debian and Ubuntu, the build prerequisites are:

```sh
sudo apt install golang-go build-essential pkg-config librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev dpkg-dev rpm
```

On Fedora and the RHEL 10 family (`golang` is in AppStream; `-devel` is in CRB, to be
enabled with `sudo dnf config-manager --set-enabled crb`):

```sh
sudo dnf install golang gcc make pkgconf-pkg-config librsvg2-devel gdk-pixbuf2-devel cairo-devel fontconfig-devel gtk4-devel gobject-introspection-devel rpm-build
```

Go 1.24 or newer is required (`go.mod`): if the distribution's version is older,
install the one from <https://go.dev/dl/>.

The organization of the packages (contents, version numbers `0.1.0~dev` / `0.1.dev`,
reproducible build, dependencies) is described in `ARCHITECTURE.md`, §4 "Packages";
the Windows build (MSYS2, NSIS, SmartScreen, troubleshooting) in
[`windows/README.md`](windows/README.md).

The installed program can check itself: `imageprintlayout -selftest` checks the
rendering engine (libraries, fonts, embedded images, reading a JPEG, a PNG and a TIFF, PDF export) on the machine
where it runs. It is the first command to run in case of a problem.

## Author

Image Print Layout is developed by **Gilles MAGNEVILLE**, and designed with the help of [claude.ai](https://claude.ai). The program's "About" window (main menu) says so too, in the interface language, as does the AUTHOR section of the manual page.

## License

This project is distributed under the [GNU General Public License v3.0](LICENSE) (GPL-3.0-only).

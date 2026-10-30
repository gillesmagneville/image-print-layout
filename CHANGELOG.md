<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Changelog

🇫🇷 [Version française](CHANGELOG.fr.md)

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.3.1] - 2026-10-30

Documentation release: the program itself does not change.

### Changed

- **The documentation is now bilingual.** Every document of the project (`README`,
  `ARCHITECTURE`, `CONTRIBUTING`, `PROJECT_CONTEXT`, `CHANGELOG`, `docs/M0`, `windows/README`
  and `testdata/README`) exists in English, the default file that GitHub displays, and in
  French (`X.fr.md`), each linking to the other at the top of the file. The French texts are
  unchanged, apart from their links, which now point to the `.fr.md` versions.
- **Packages and release notes follow.** The `.deb`, `.rpm` and Windows packages ship the
  README and the CHANGELOG in both languages; the GitHub release text gives the section of
  the CHANGELOG in English, then in French; the `repocheck` tests check both CHANGELOGs and both
  ARCHITECTUREs, and that every document exists in both languages with the same sections.
- **Repository clean-up.** The build files that had been added by mistake (`dist/`, `*.sha256`)
  are removed from the repository and are now ignored by Git.

## [1.3.0] - 2026-10-21

The program also exports to a **single HTML page**: one file, which a browser opens without
downloading anything, with the whole image or cut at the cuts. The PDF is unchanged.

### Added

- **Export to an HTML page**: the "Export to an HTML page…" button of the "HTML page" block of the
  Export section, or Ctrl+Shift+E. Same course as the PDF: checks before the export (errors
  that block, warnings that ask for confirmation), file chooser (`<name>.html`
  suggested), writing next to the destination under a temporary name, **rereading from
  disk**, comparison with the plan, and only then the replacement of the file: a refused or
  failed export leaves nothing half-written and does not overwrite a good file.
- **Cut or whole.** The "Cut the image at the cuts" box (checked by default) writes one
  piece per page of the plan, one below the other, each with its caption ("Page 2 of 4"):
  these are the strips of the preview, hence the cuts placed with the mouse. Unchecked, the page shows
  the image in one piece. The browser prints one piece per sheet, without the captions.
- **One file, a single copy of the image.** The image is written once, in base64, in the
  page; each piece is just a box that shows a strip of it, so the page does not grow
  with the number of pieces. The column has the width of the plan in millimeters and shrinks with
  the window without ever distorting the image. The project's page background (opaque white or transparent)
  is taken over, like the document background of an SVG.
- **What the page contains.** An SVG stays vector (the page carries the working copy, never the
  source file); its text is drawn with the fonts of the computer that opens the page, which
  the Export section says. The images it links by a relative path are embedded in the
  page (PNG, JPEG, GIF, WebP, BMP, SVG), otherwise a page read elsewhere would not find
  them again. A JPEG or a PNG is written as the file is; a TIFF, a GIF, a WebP, a BMP, an
  animated PNG or an image to rotate is drawn by the engine (straightened, first frame,
  transparency kept) and written as a PNG.
- **Verification.** `export.VerifyHTML` rereads the page with only the numbers it contains
  and finds again the top and the height of each strip, the height of the document, the width of the column
  and the image itself; a discrepancy is refused with the number of the piece. Beyond 256 MB, the page
  is refused before being written, with the size it would have.
- The messages are translated into French, English, Spanish and German, including the language of
  the page and the caption "Page n of total".
- **Tests**: unit tests (pieces, determinism, simulation of the style rule, a reread that
  fails when a strip, the width or the image changes, `ImageSource`), of `svgdoc` (linked
  images), acceptance tests on a drawing, an image rotated by its tag and a cut image,
  and of the window (page cut into four pieces, whole page).

### Fixed

- **The PDF export keeps its system thread from start to finish.** Pango, with which librsvg draws
  the text, keeps its font table per thread, and cairo writes into the PDF a font subset
  specific to each table: when Go resumed an export on another thread between two pages,
  the same fonts were embedded twice. The PDF was bigger (up to half again as big
  on the test set) and two exports of the same project did not have the same bytes, which
  contradicts the determinism required. Seen under Windows, from time to time (intermittent
  failures of test 14 and of the determinism test in the CI), and never under Linux. The export
  tests now note the thread of each page and fail if the export changes it; they also say,
  when two PDFs should be identical, how they differ.

## [1.2.0] - 2026-10-21

The program opens **images**: JPEG, PNG, TIFF, WebP, BMP and GIF. An image is cut and
printed like an SVG: same single scale, same cuts with the mouse or automatic, same
preview, same multipage PDF reread and verified. This is the sequel announced by the renaming
of 1.1.0; nothing is changed for SVGs.

### Added

- **Opening an image as a document.** Open (Ctrl+O, or a file given to the program on
  the command line) accepts SVGs and images; the file chooser offers "Images and SVG documents", SVGs alone and images alone, with only
  the formats that this installation can read. The type is recognized by the file's **content**,
  never by its name: a `.png` that is a JPEG opens as a JPEG. A file that is
  neither a known image nor an SVG is refused, with the reason.
- **Size on paper from the file's resolution**: the pixels divided by the dots
  per inch that the file states (JFIF density of a JPEG, failing that its Exif resolution; `pHYs` of a PNG;
  resolution tags of a TIFF), on each axis, read by the program in the file
  itself (recent gdk-pixbuf decoders do not report them). Without a valid resolution, 96 dots per inch, and
  the Source file section says so (a 3,000-pixel photo at 300 dots per inch is 254 mm
  at 100%). The scale stays single, and the image is never distorted.
- **Orientation**: the orientation tag of a JPEG (EXIF) or of a TIFF is applied, for all
  eight values, to the preview as to the PDF.
- **JPEG embedded as is in the PDF** when the PDF can read it (baseline or progressive JPEG,
  8 bits, gray or YCbCr): no recompression, no loss, and the file is no bigger
  than the photo. An image is written **only once** in the PDF, each page references it:
  the file does not grow with the number of pages.
- **Transparency**: a transparent PNG, TIFF, WebP or GIF stays transparent; the page
  background (opaque white or transparent) shows through.
- **Automatic cuts for an image**: the program reads the pixels to find the blocks
  of a plain-background image (a screenshot, a diagram, a scanned page): the background is the color
  that borders half of the image, and a run of rows that differ from it is a block. The
  cuts fall between them, and the noise of a JPEG or the dust of a scan changes nothing.
  A photograph, without a plain background, has no blocks: the cuts are spread evenly and
  none is said to cross anything.
- **Safeguards, told to the user**: more than 100 million pixels (refused **before**
  decoding, the size being read from the header, so a huge image does not eat up memory);
  a PNG or JPEG file whose end is missing (opened as far as it could be read, with a warning,
  or refused with the decoder's reason when the decoder, as with gdk-pixbuf 2.44, returns nothing);
  multipage TIFF and animated GIF (only the first frame is read, with a warning);
  unreadable image or format without a decoder on this installation (WebP without its loader, under
  Linux), with a message that says which.
- **Projects on an image**: an `.imgpl` project describes an image like an SVG (relative path,
  fingerprint, size) and reopens the same way, with the same warnings if the file has
  changed. The project format has not changed (version 1): projects of earlier versions
  open as before.
- **Export verification**: `export.Verify` also rereads, in the PDF, the matrix that places
  the image and compares it with the plan, as it does for the clip of each page.
- **`-selftest`** reads a built-in JPEG, PNG and TIFF and compares their pixels, in addition to the SVG;
  the Windows package embeds gdk-pixbuf's TIFF loader.
- **Independence from the gdk-pixbuf version**: the resolution, the pages of a TIFF and
  the animation (GIF, WebP, animated PNG) are read in the bytes of the file, because gdk-pixbuf 2.44
  (glycin: Ubuntu 26.04, Fedora 44) no longer reports them, and the decoder's error messages
  are stripped of their jargon. Same results with 2.42 (Debian 13, Ubuntu 24.04, RHEL 10).
- **Test set**: the `internal/testimg` package makes deterministic reference images
  (PNG, TIFF, JPEG, with resolution and orientation), and the 14 acceptance tests also pass
  on an image, for each format and each of the eight orientations.

### Changed

- The title bar button is called **Open…** (it was called "Open an SVG"), and the
  messages say "the original file" where they said "the original SVG".
- **Document background**: for an image, the Background section explains that it has no separate background
  (its pixels are printed as they are) instead of offering grayed-out choices; only the page
  background acts.
- **Packages**: the engine reads images with gdk-pixbuf, which GTK 4 already requires (no
  extra dependency at installation). The `.deb` and `.rpm` packages recommend
  `webp-pixbuf-loader` for WebP, and the menu entry announces the MIME types of images, for
  "Open with" in the file manager. The build scripts check that
  the gdk-pixbuf development libraries are present.
- The package description, the AppStream sheets, the menu entry and the four manual pages
  talk about SVG documents **and images**. So does the subtitle of the "About" window (it said
  "vector documents", as in the days of SVG Print Layout).

### Fixed

- **The program icon was missing from the overview of windows** in GNOME (a gear in its
  place): GTK announced to the desktop the name of the executable, `imageprintlayout`, instead of
  the identifier `io.github.gillesmagneville.imageprintlayout`, which is the name of the menu entry.
  Since the renaming of 1.1.0, the window therefore no longer found its menu entry again.
  The program now announces its identifier (under Wayland as under X11), and the menu
  entry carries `StartupWMClass`. A test checks it.

## [1.1.0] - 2026-10-21

The program changes its name: **Image Print Layout** replaces "SVG Print Layout". The program will
soon take raster images too (JPEG, PNG, TIFF…): "SVG" in its name would no longer
say what it does, whereas "image" covers both (SVG is a vector image). This version only changes the name and the way existing things are carried over; the
behavior is that of 1.0.0. The entries below keep the name the
program had at their date.

### Changed

- **New name: Image Print Layout.** The command is `imageprintlayout`, the packages are called
  `imageprintlayout` (`imageprintlayout_1.1.0_amd64.deb`, `imageprintlayout-1.1.0-1.x86_64.rpm`,
  `imageprintlayout_1.1.0_windows-setup.exe`…), the Windows installation folder is `Image Print Layout`,
  the application identifier is `io.github.gillesmagneville.imageprintlayout`, the manual page
  is `man imageprintlayout`, and the Go module is `github.com/gillesmagneville/image-print-layout`. The icon, the
  "About" window, the texts in four languages and the documents say Image Print Layout.
- **Projects are saved as `.imgpl`** (format `imageprintlayout-project`, still in
  version 1: only the name changes).
- **Release: one `.sha256` file per package**, attached next to it, instead of a single
  `SHA256SUMS`. `sha256sum -c package.sha256` verifies it when both are in the same folder.
  The release text says which file verifies which package.

### Added

- **Carrying over what exists**, so that the renaming costs nobody anything:
  - the `.svgpl` projects of 1.0.x open (the file chooser offers them) and
    are then saved as `.imgpl`, without touching the original file;
  - the chosen language is taken over from the settings of 1.0.x (`~/.config/svgprintlayout`,
    `%AppData%\svgprintlayout`), once, without erasing them;
  - installing the `.deb` or the `.rpm` removes the old `svgprintlayout` package
    (`Conflicts`/`Replaces` for Debian, `Obsoletes` for RPM);
  - the Windows installer first uninstalls an installation of "SVG Print Layout" (files,
    shortcuts, registry keys), with its own uninstaller.

## [1.0.0] - 2026-10-21

First stable version. The program does everything the specification asks (milestones M1
to M5), the 14 acceptance tests are automated and run in the CI on seven
Linux distributions and under Windows, and the acceptance was done by hand under Ubuntu and under
Windows, with the Mini-CAD. The program's code is that of 0.3.1: this version only changes
the number, the documentation and the way of publishing.

### Added

- **The packages are attached to the GitHub release**: the `.deb` (Ubuntu 24.04 and later, Debian
  13), the `.rpm` (RHEL 10 and its clones, Fedora), the Windows installer and portable version
  (10 and 11, 64-bit), with their SHA-256 checksums.
- **Release workflow** (`.github/workflows/release.yml`): a `vX.Y.Z` tag on main builds
  these packages without changing `VERSION`, tries them like the CI (installation, installed program,
  14 acceptance tests under Linux; portable version, installer and uninstallation under
  Windows), then creates the release with this section of the CHANGELOG as its text. It refuses a tag that
  is not that of `VERSION` or that is not on main, and it runs, without publishing anything,
  when it is modified in a pull request.

### Changed

- The README, `ARCHITECTURE.md`, `windows/README.md` and the CI no longer say that the acceptance on
  a real Windows machine remains to be done.
- Still open, without preventing 1.0.0: the building of the Windows packages by a user
  with their own MSYS2 installations (the CI does it on a fresh MSYS2).

## [0.3.1] - 2026-10-20

Fixes on the document background, number entry and the "About" window, and mention
of the author wherever it belongs.

### Fixed

- **Entering a number (scale factor, width, margins…): the cursor went back to the start of
  the line** after each digit typed. The field rewrote its text when the program gave it
  back the value it had just applied, and GTK then places the cursor at the start: the next
  digit was inserted before the first (typing 12 gave 21). The focus test that was meant to
  prevent this never saw the focus (the focus of a GtkEntry is in the GtkText it
  contains). The field no longer rewrites what it already shows; the canonical notation is written
  when it is left, and the cursor stays at the end when the text is rewritten.
- **"About" window: the name was displayed selected (in red) and the program icon
  was missing.** The GtkAboutDialog gives the focus to its title label, which is selectable,
  and looks for its logo in the icon theme under the full identifier of the application, whereas
  the packages install the icon under the name `svgprintlayout`. It is now a window of the
  program: the icon is embedded in the executable (`icons` package), no text is
  selectable, the Close button has the focus, and the texts are in the language chosen in the
  program and not in that of the system. The report and error windows no longer
  open with a selected text either.
- **Window icon**: the program asks for it under the name `svgprintlayout`, that of the packages,
  and not under the full identifier of the application.
- **Document background: "Replace with white" and "Remove the background" were not visible on
  the last page.** The "Document" preview tinted pale blue every other page (pages
  2, 4…) over the drawing, in a gray almost identical to that of the original background: the
  last page, when it was even, stayed grayish although the background was white. Nothing
  is painted over the drawing anymore; the extent of each page is given by a bar along
  the drawing, on the outside.
- **Export with "Remove the background" refused.** Without a background rectangle, cairo wrote no
  clip for the strip of the page, and the check of the reread PDF refused the export ("The cropping of
  this page in the PDF does not match the preview's cut", on every page). With a
  white page, the engine paints white on the strip, inside the clip (no effect on the image),
  so that the clip is written in full; with a transparent page, `export.Verify` checks
  that the clip of each element that crosses a cut stays within the strip.
- **Sheet preview: a sheet kept the old drawing** when the document background
  changed while it was being drawn; nothing asked for a new one. The sheet
  is redrawn as soon as the drawing in progress has returned.
- **Tests**: acceptance test 9 looks at all the pages, for "replace" as for
  "remove"; the export is tried with each document background and each page background; test
  8 measures the top of the strip by the background kept (the real Mini-CAD draws nothing in its
  first 43 units); the window is tried on the page marks (nothing on the drawing) and
  on the sheets when the background changes.

### Changed

- **The "Export to PDF…" button of the title bar is removed**: it duplicated the one in the
  Export section of the left panel. Ctrl+E remains.
- **The author is named**: "Developed by Gilles MAGNEVILLE" and "Designed with the help of
  claude.ai" in the "About" window (in French, English, Spanish and German), the
  README, `PROJECT_CONTEXT.md`, `CONTRIBUTING.md`, the AUTHOR section of the four manual pages,
  `debian/copyright` and the properties of the Windows executable and installer.
- The repository no longer contains `err.txt`, an empty file added by mistake.

## [0.3.0] - 2026-10-20

First version that does everything the specification asks: you open an SVG, set
the paper, the margins, the scale and the positioning, place the cuts with the mouse (or have
them proposed), choose the background, see each sheet, export a multipage PDF that
the program rereads before delivering it, and save the project to reopen it later. The
14 acceptance tests are automated (`internal/acceptance`) and run in the CI on
each Linux distribution tried and under Windows. What remains, before 1.0.0, is the acceptance on
real Windows 10 and 11 machines and the manual one of the mouse gestures (test 5).

### Added

- **Core of the layout, in pure Go (milestone M1)**, tried by tests without cgo, including
  property tests drawn at random (fixed seed):
  - `internal/model`: the settings of a project (paper, margins, scale, positioning, background,
    cuts, per-page overrides), immutable values copied by each command;
  - `internal/layout`: the sheet and its usable area (formats A0 to A5, Letter, Legal, Tabloid,
    custom in mm; portrait or landscape; four independent margins), the single scale in
    three modes (width, height, manual by width or by percentage), the page plan
    (strips that cover the document without a gap, same width everywhere, horizontal and
    vertical positioning independent of the scale, per-page overrides, states Conforming, Margin overflow,
    Sheet overflow, empty page), the cuts (adding, moving, deleting, bounds of a
    drag, minimum gap of 0.5 mm, stable identifiers), the gaps between blocks, the detection
    of cuts that cross a block, snapping to the gaps, the four corrections proposed
    when a page overflows (never applied on their own) and the automatic cuts
    (dynamic programming: fewer blocks crossed, then fewer pages, then balanced
    pages);
  - `internal/project`: the `.svgpl` project file (readable JSON, deterministic and
    atomic writing, version with migrations, strict reading, path relative to the project with fallback to
    the absolute path, SHA-256 fingerprint and warnings when the SVG has changed);
  - `internal/diag`: the diagnostics, by code and values, with their texts in four languages;
    a test checks that each code has a text in each language and that the code produces no
    other.
- **Acceptance tests 2, 3, 4, 6, 7 and 14 at the logical level**: usable area of 200 × 277 mm in A4
  portrait, scale 0.15625 mm per unit for 200 mm of width, proportions kept, adding and
  moving a cut without a change of scale, saving and reopening a project.
- **Document, analysis and window (milestone M2)**: the window opens an SVG, analyzes it, computes its
  pages with the core of M1 and shows the plan.
  - `internal/svgdoc` (pure Go): reading the SVG without drawing it, with the position of each
    element in the source text; findings translated into four languages (filter, script,
    animation, SVG font, `foreignObject`, `flowRoot`, web font, linked image not found or
    remote, `use` pointing to another file, embedded images, unknown size, malformed XML;
    25 findings at most per category, the rest counted); requested fonts; top-level
    blocks, descending into Inkscape's layers; background rectangle, rectangle or path
    of four segments that covers the frame; working copy (background whitened or removed, an
    identifier on each block) made by replacing a few bytes of the text, without rewriting the
    rest, and always identical for the same inputs. The source file is never written.
  - `internal/engine`: the `Document` interface (in pure Go, replaceable by a fake engine in
    tests), loading by path so that linked images are found next to the
    SVG, and font resolution by fontconfig (`MatchFont`): a missing font is
    flagged with the one that replaces it, instead of being replaced silently.
  - `internal/app`: the session (project, plan, history). Each setting is a command that
    yields a new state; the plan is computed before accepting a change, so a refused
    change leaves no trace; undo and redo; successive keystrokes
    in the same field make only one history entry; a drag makes only one;
    the modification indicator is deduced from the encoding of the project.
  - `internal/viewport` (pure Go): zoom around a point, panning, bounds, the window of the
    drawing to ask of the engine, need to redraw; tested without a screen.
  - Window: left panel (Source file with the analysis report, Paper, linkable Margins,
    Scale in three modes, Positioning), list of pages on the right (width and height of the
    content, scale, start and end in the document, state, proposed corrections on the button
    itself), preview in the center (zoom, panning, page limits and numbers, page tinted according to
    its state, rendering of only the visible part at screen scale, HiDPI included), Undo
    and Redo buttons. The Background and Export panels, the cuts with the mouse and the export
    are still announced as coming.
  - **Heavy documents**: beyond 3,000 blocks, the gaps are read on a single image of the
    drawing and not block by block, because measuring an element forces librsvg to go through the whole
    drawing (100,000 elements: more than six minutes measured one by one, four seconds this way).
    A finding tells the user.
- **Cuts with the mouse and automatic cuts (milestone M3)**:
  - `internal/app`: the session's commands on the cuts (add, drag, enter a
    position, nudge by a step, delete, automatic cuts). Each is a history entry
    (a drag makes only one, successive steps on the same cut neither),
    none reads or writes the scale (acceptance tests 6 and 7), and the plan is computed before
    accepting a change. A drag stops against the neighboring cut instead of failing;
    an entered position that is not allowed is refused with its reason, never silently corrected.
    For each cut: its rank, its position, the number of blocks it crosses and
    their names; the gaps between blocks are used for snapping.
  - Preview: double-click to add a cut, drag a line (handle, badge in mm and
    in units, snapping to the gaps, Alt to disable it), arrows (1 unit, Shift 10, Ctrl
    0.1 mm), Delete, Esc, right click (enter a precise position or delete), click on a
    page to select it. During a drag, only the cuts layer is redrawn.
  - Right panel: "Add a cut" and "Automatic cuts" buttons (the
    proposal replaces the cuts, in a single history entry that can be undone), list of
    cuts with their crossed blocks, selection shared between the preview and the lists.
  - Tests: `app` commands (including those of the scale), window scenarios under Xvfb that
    drive the gestures (adding, dragging, snapping, keyboard, menu, selection) in four languages.
- **Background, verified PDF export and sheet preview (milestone M4)**:
  - `internal/export` (pure Go): the pages of the plan given to the engine; the checks before
    the export (cuts in order, strips that cover the document, same width
    everywhere, content within the sheet, margin overflow, requested page background compared with the detected
    background); the rereading of the written PDF, without a PDF library, which compares the number of pages,
    their size (0.02 mm), the clip rectangle and the offset of the drawing of each page with the plan,
    to within 1/256 of a point; atomic writing (synchronized temporary file, reread, then
    renamed: a refused, canceled or failed export leaves nothing). Thirteen `export.*` diagnostics
    translated into four languages.
  - `internal/engine`: a single function draws a page, for the PDF as for the sheet
    preview (`RenderPage`); page-by-page progress and cancellation.
  - `internal/app`: `SetDocBackground`, `SetPageBackground` (one history entry each),
    `DocumentBackground`, `CheckExport`, and `Rebuild`, which reloads the working copy with the
    chosen background.
  - Window: Background sections (three distinct backgrounds, choices grayed out without a detected
    background rectangle) and Export (summary, state ready, to confirm or blocked, cancelable
    progress), Sheets view next to the Document view, "Preview" button on each page card,
    Ctrl+E and title bar button. The background is changed off the interface thread, the old image
    stays displayed until the new one; undo and redo follow the project. A transparent
    page is painted on a checkerboard. A check error opens a report per
    page, a warning asks for confirmation.
  - `internal/acceptance`: acceptance tests 8 to 13 on the whole chain (Mini-CAD of
    1,280 × 5,479 units at 200 mm, pages 10 mm from the top, gray background replaced by white, PDF
    rasterized by poppler), the determinism of the export and the integrity of the source SVG.
  - Tests: window scenario under Xvfb (background, undo, sheets, export refused then
    written then canceled, texts in four languages).
- **Project in the window and acceptance (milestone M5)**:
  - `internal/app`: `ReadProject`, `Session.SaveProject` and `Session.OpenProject`. The project
    records where the document is (relative to the project file, absolute as a fallback), the fingerprint of the
    file as it was read and its size; reopening returns the warnings that block
    nothing (document modified, size changed, cuts outside the document, project background
    not found: `project.background_lost`), removes the cuts outside the document and sets
    document and settings in one block, changing nothing if the project is refused. The
    "modified" state is compared with the last save or the last opening.
  - Window: **Project** menu (Open a project, Save, Save as; Ctrl+Shift+O,
    Ctrl+S, Ctrl+Shift+S), moved document to locate without losing the settings, question before
    removing cuts, report of the warnings, asterisk before the name in the
    title, "Save, don't save, cancel" question before closing or opening another
    document. `svgprintlayout file.svgpl` opens a project.
  - `internal/acceptance`: tests 1 to 7 and 14 are added to tests 8 to 13; test 14
    exports before saving and after reopening: the two PDFs have the same bytes.
  - CI: the 14 tests run in the engine job, on each Linux distribution tried
    and under MSYS2 on Windows (poppler installed).
  - Tests: window scenario (save, reopen, missing document, modified document,
    cuts outside the document, file that is not a project, closing).
- **Build dependency**: `libfontconfig-dev` (Debian and Ubuntu), `fontconfig-devel`
  (Fedora, RHEL), `mingw-w64-ucrt-x86_64-fontconfig` (MSYS2), added to the scripts, the CI and
  the documentation.

### Changed

- **Name of the `.deb`**: `svgprintlayout_<version>_amd64.deb`, without a Debian revision (`-1`),
  like the Planche-Contact and LinuxDoctor packages; the package version (`Version:`
  of the `control`) is that of `VERSION`. 0.2.0 was called `svgprintlayout_0.2.0-1_amd64.deb`.
  The following versions (0.2.1, 0.3.0…) sort after it, but a 0.2.0 `.deb`
  rebuilt with this script sorts before the one already installed: `sudo dpkg -i`
  installs it anyway, `apt install` asks for `--allow-downgrades`. The `.rpm` keeps its
  release: `svgprintlayout-<version>-1.x86_64.rpm`.
- **Document unit**: it is the CSS pixel of the declared size of the SVG, the one the engine reads and
  draws (ARCHITECTURE §5). Mode B: the portion fitted to the height of a sheet is
  a setting, so that moving a cut never changes the scale.

## [0.2.0] - 2026-10-20

First version with a graphical interface: a three-panel GTK4 window, in
French, English, Spanish and German, which opens an SVG and displays it in full.
The page settings, the cuts and the export are not yet in the window. The
program is supported, and tried by the CI, on Debian 13, Ubuntu 24.04 and later,
RHEL 10 and its clones, Fedora, Windows 10 and 11 (see "Supported systems" in the
README).

### Added

- **Four languages**: the `internal/i18n` package translates the interface into French, English,
  Spanish and German (keys, named parameters, plurals, numbers with comma or
  point, reading a number typed in either notation), detects the system
  language (Linux: `LANGUAGE`, `LC_ALL`, `LC_MESSAGES`, `LANG`; Windows: the user's
  language) and adds the `-lang` option. Tests check that the four catalogs have
  the same keys, the same parameters and no empty text, and that the code only asks for
  keys that exist.
- **First window** (`internal/ui`, GTK4 through gotk4 v0.3.1): three panels (settings,
  preview, list of pages), title bar with "Open an SVG" and a menu (language, About,
  Quit). An SVG opens without blocking the window (the engine runs in its own goroutine)
  and is displayed in full, fitted to the view; a file that is not an SVG is refused with a
  message, the open document stays. The language changes on the fly (automatic, French,
  English, Spanish, German); the choice is saved in `settings.json` in the configuration
  folder (`internal/app`), outside the projects. Without cgo, `ui` is a stub: the rest of the
  repository builds and is tested as before. The page settings, the cuts and the export
  are not yet in the window.
- **System files in four languages**: the menu entry (`.desktop`), the AppStream sheet
  and the manual page, in English (fallback language), French, Spanish and German; the
  translated manual pages are installed in the folder of their language. `repocheck` tests
  check that each language has its lines and that the four manual pages carry
  the version.
- **`-ready-file` option**: creates a file when the window is displayed and the document
  drawn (trials and documentation).
- **Supported systems**: the Debian family (Debian 13 and later, Ubuntu 24.04 and
  later, their derivatives), the Red Hat family (RHEL 10 and its clones, Fedora) and Windows 10
  and 11. The criterion is the version of the libraries (GTK 4.14, GLib 2.80, librsvg 2.52 at
  least, imposed by gotk4 v0.3.1); Debian 12, Ubuntu 22.04 and RHEL 9 are too old.
  `build-deb.sh` and `build-rpm.sh` check these versions before compiling and say so in
  French. Table in the README and in `ARCHITECTURE.md` (§4).
- **Distributions CI**: the `distributions` job builds the package in a container of
  Debian 13, Ubuntu 24.04, Ubuntu 26.04, Fedora, CentOS Stream 10, AlmaLinux 10 and Rocky Linux
  10, installs it with `apt` or `dnf`, runs `-selftest` then the window in the four
  languages, and checks on Debian 12, Ubuntu 22.04 and AlmaLinux 9 that the build is
  refused with a clear message. The result of each system is in the annotation of the
  run. The Windows package is tried on `windows-2022` and `windows-latest`.
- **CI**: a `ui` job (GTK4 under Xvfb: window tests, screenshots in the four languages
  as an artifact), the opening of the installed program's window in the `linux-packages` job, and
  in the `windows-package` job the opening of the portable package's window in the four
  languages then of the installed program's, with screenshots.

### Changed

- **Packages**: the `.deb` and `.rpm` depend on GTK 4 (`libgtk-4-1`); their scripts
  check for the presence of GTK 4 at build time (Ubuntu 24.04 or newer, GTK 4.14).
  The Windows package embeds GTK 4 and what it needs (settings schemas, Adwaita
  and hicolor icon themes, GTK translations) and contains two programs: `svgprintlayout.exe`
  (the window, without a console) and `svgprintlayout-console.exe` (`-version`, `-selftest`). The
  build script checks the subsystem of each. The NSIS installer speaks the four
  languages.

### Fixed

- **Build warnings**: the three gcc warnings about `free`, which
  gotk4 declares differently from `<stdlib.h>`, no longer appear during the building of the
  packages (the other warnings stay visible); `dpkg-shlibdeps` no longer prints
  "binaries to analyze should already be installed…" on a system that is not in
  English (the filter looked for the English message). The scripts also warn that the
  first compilation of GTK takes several minutes without displaying anything.

## [0.1.0] - 2026-10-19

First version. The repository contains the skeleton, the reference documents, the library
of units and paper formats, the prototype of the rendering engine (M0a), the scripts that
build the `.deb`, `.rpm` and Windows packages, and the CI. The program does not yet have
a graphical interface: it only offers `-version` and `-selftest`. The prototype of the GTK4
interface and its build under Windows (M0b) are the next step.

### Added

- **Packages**: `./build-deb.sh`, `./build-rpm.sh` and `.\build-windows.ps1` build at the
  root of the repository a `.deb`, a `.rpm`, a portable version (`.zip`) and an NSIS installer for
  Windows, with the native tools of each platform (`dpkg-deb`, `rpmbuild`, MSYS2 and
  NSIS). They have the options and the handling of `VERSION` of those of Planche-Contact:
  `--major`, `--minor`, `--patch`, `--no-version-change`, `--clean`, `--help` (`-Major`… under
  PowerShell), interactive menu without an option, confirmation before building, `VERSION`
  written once the package is built. A `0.1.0-dev` version is published first (`--patch`
  and `--minor` give `0.1.0`). The Linux packages contain the program, the menu
  entry, the AppStream metadata, the icon and the manual page, with dependencies
  deduced from the binary; the Windows package also embeds the cairo and librsvg DLLs,
  fontconfig, the image loaders and the license of each library. The builds
  are reproducible. Same folder organization as Planche-Contact: `debian/`,
  `metainfo/`, `icons/`, `windows/`. See the README, "Packages", and `ARCHITECTURE.md`.
- **`-version` and `-selftest`**: the program displays its version and those of librsvg and
  cairo, and verifies its rendering engine on the machine where it runs (loading, measurement,
  rendering, embedded PNG image, font embedded in the PDF, deterministic export). The packages
  use it as a smoke test.
- **`engine.ConfigureRuntime`**: under Windows, points fontconfig, gdk-pixbuf and pango to
  the data delivered next to the program.
- **CI**: a `linux-packages` job (build, lintian, AppStream, installation and
  uninstallation of the `.deb`, trial of the `.rpm`, identical builds, handling of `VERSION`
  by the scripts) and a `windows-package` job (build by `build-windows.ps1 -Patch`,
  trial of the portable version with only the Windows PATH, silent installation and
  uninstallation, `-Clean`).
- **`repocheck`**: the license header is also required in the PowerShell scripts, the NSIS
  scripts and the packaging files; the three build scripts must have
  the same options, the same version computation, and write `VERSION` only after the
  build; the version of `VERSION` must be in the changelog, in the
  AppStream metadata and in the manual page; `ARCHITECTURE.md` must keep its sections
  numbered from 1 to 15 without a gap; the files are read with Unix line endings, as they
  are in a Windows working copy.
- **`engine` (M0a prototype)**: loading of an SVG by librsvg (resolution imposed at 96 dpi),
  size of the document, box of an element in the document's coordinate system, rendering of a window to an
  image, and multipage PDF export: each page redraws the document under a rectangular
  clip, at a single scale, with fixed dates (two exports are identical to the
  byte), embedded fonts and selectable text. Written in cgo (`librsvg-2.0`,
  `cairo`); without cgo, the package is empty and the rest of the repository compiles.
- **`testsvg`**: deterministic generator of a 1,280 × 5,479-unit test SVG (gray background,
  banner, blocks with text, transformed rectangle), public equivalent of the Mini-CAD.
- **Engine tests**: exact size of the PDF pages, identical exports, text and fonts,
  file size independent of the number of pages, and seam test in two checks — the
  geometry read in the PDF (clip and offset of each page, exact to 1/256 of a point) and the
  pixels by squares of 16 × 16 — which both fail when a page is shifted. The real
  Mini-CAD, which is not in the repository, passes the same tests with `SVGPL_MINICAD`.
- **`docs/M0.md`**: report of the M0a part. Verdict: the librsvg + cairo engine is retained;
  the cairo recording surface, which emptied pages 2 to 4, is abandoned in favor of
  direct rendering per page; a missing font is replaced without a warning, which
  makes the detection of §12 of `ARCHITECTURE.md` indispensable.
- **Continuous integration**: an `engine` job under Ubuntu (`librsvg2-dev`, `poppler-utils`,
  `fonts-ibm-plex`, cgo enabled) for `engine` and `testsvg`.
- **Repository skeleton**: Go module `github.com/gillesmagneville/svg-print-layout`, entry
  point `svgprintlayout` (which still only displays its version with `-version`), one package
  per role in `internal/` (`model`, `units`, `diag`, `layout`, `project`, `svgdoc`,
  `engine`, `export`, `app`, `ui`).
- **`units`**: conversions between millimeters, points and CSS pixels; reading of SVG lengths
  (`mm`, `cm`, `in`, `pt`, `pc`, `px`, no unit), with an explicit error for `%`, `em` and
  `ex`; formats A0 to A5, Letter, Legal, Tabloid and custom, portrait and landscape;
  independent margins and usable area; scale in millimeters per document unit. The
  figures of the Mini-CAD acceptance test (200 mm for 1,280 units: 856.09375 mm
  high, 4 A4 pages at least) are verified by a test.
- **Reference documents**: `PROJECT_CONTEXT.md` (vision, principles, scope),
  `ARCHITECTURE.md` (technical contract, drawn from the design document), `CONTRIBUTING.md`,
  GPL-3.0 license (`LICENSE`).
- **`repocheck`**: tests on the repository itself — SPDX header in every file, `VERSION`
  consistent with this file, cgo limited to `ui` and `engine`, each package of `internal/`
  described in `ARCHITECTURE.md`.
- **Continuous integration** (GitHub Actions): `gofmt`, `go vet` and `go test` under Ubuntu and
  Windows, with `CGO_ENABLED=0`.

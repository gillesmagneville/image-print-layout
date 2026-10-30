<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Contributing to Image Print Layout

🇫🇷 [Version française](CONTRIBUTING.fr.md)

Thank you for your interest in this project! This document explains how to report a problem,
propose an improvement, or contribute code.

Image Print Layout is developed by Gilles MAGNEVILLE, and designed with the help of claude.ai.

## Reporting a bug

Open an issue stating:

- the system (the distribution and its version, or Windows 10 or 11);
- the version of the application (`imageprintlayout -version`);
- what you did, the observed behavior and the expected behavior;
- if possible, the file concerned (an SVG or an image, or a minimal example that reproduces
  the problem) and the `.imgpl` project file.

## Proposing an improvement

Open an issue describing the need before writing code, in particular for a rule
of SVG analysis or a change to the model or the project format: it is easier to
discuss before than after. The functions listed as out of scope in
`PROJECT_CONTEXT.md` are possible future developments, not oversights.

## Contributing code

1. Fork the repository and create a branch from `main`.
2. Check that everything passes before proposing a pull request:

   ```bash
   gofmt -l .          # must print nothing
   go vet ./...
   go test ./...
   ```

   These commands are exactly those of the continuous integration
   (`.github/workflows/ci.yml`), which runs them on Linux and on Windows with
   `CGO_ENABLED=0`. Do not use a feature newer than the version of Go
   given in `go.mod`.

   If you touch the rendering engine (`internal/engine`), also run its tests with cgo
   (`sudo apt-get install librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev poppler-utils fonts-ibm-plex`,
   then `CGO_ENABLED=1 go test ./internal/engine ./internal/testsvg ./internal/testimg`): this is what the
   `engine` job of the continuous integration does.

   If you touch the window (`internal/ui`), add GTK 4 and a virtual display
   (`sudo apt-get install librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev xvfb xauth
   imagemagick fonts-ibm-plex`) and run `xvfb-run -a go test ./internal/ui
   ./cmd/imageprintlayout`: this is the `ui` job, which also deposits screenshots of the window
   in the four languages (`screenshots` artifact). A text of the interface is added to the
   **four** catalogs of `internal/i18n` (the test says which ones forget it).

   If you touch the package build scripts or their dependencies, try them
   on the target distribution: the CI's `distributions` job does so in one container per
   supported system (`.github/scripts/test-distribution.sh debian|redhat`, run from
   the root of the repository in the container; see `ARCHITECTURE.md`, §4 "Supported
   platforms"). A library newer than those in this table must not be required
   without removing the system that does not have it.

3. Describe in the pull request what changes and why, and say what you
   verified and what you could not verify.

A pull request carries **a single coherent change**. The commit title has the form
`area: summary` (for example `units: …`, `layout: …`, `docs: …`), and the pull request is
merged as a single commit.

## Architecture rules

The complete contract is in `ARCHITECTURE.md`. The rules not to break:

- **Pure Go outside `ui` and `engine`.** Only these two packages may use cgo. The others,
  `layout`, `units`, `model` and `project` in particular, are tested everywhere, without any
  graphics library. A test fails if `import "C"` appears elsewhere.
- **A single scale.** No function accepts two separate factors for the width and
  the height. The scale (in mm per document unit) is stored in one place, and
  the pages read it.
- **Cuts in document units**, not in millimeters.
- **Every unit conversion goes through `units`** (1 mm = 72 / 25.4 points; 1 unit = 1 CSS
  pixel at 96 dpi). No conversion constant copied elsewhere.
- **The source file (SVG or image) is never opened for writing.** Every transformation is applied to the
  working copy.
- **Nothing random or dated in what the application writes** (PDF, project): no
  timestamp, no random identifier, and never a walk over a `map` without sorting
  its keys, because Go's iteration order changes from one run to the next.
- **No silent reduction.** A page that overflows is flagged with proposed
  corrections, never corrected on its own.
- **No network access.**
- **Explicit error messages**: what failed, why, and what can be done,
  with no raw technical code.
- **Project format.** Any change increments `version` and adds a
  migration function `vN → vN+1`, tested on sample files.

## Code style

- `gofmt` before proposing a pull request.
- Identifiers and code comments in English. No user-facing text in
  the code: it carries a key (`tr.T("panel.paper.title")`) whose text is in the
  four catalogs of `internal/i18n` (French, English, Spanish, German). French
  is the reference; a test fails if a language forgets a key, a text or a `{name}`
  parameter, or if the code asks for a key that does not exist. See "Languages" in
  `ARCHITECTURE.md`.
- Standard library first. Every new dependency must be justified: what it
  brings that the standard library does not offer, its license, its maintenance.
- Every source file, document or CI file starts with
  `SPDX-License-Identifier: GPL-3.0-only` (in a comment, with an empty line after it in
  a Go file, so that it does not merge with the package comment). A test fails otherwise.

## Tests

- The computation logic is tested in pure Go, with `go test` everywhere.
- For `layout`, prefer property tests (papers, margins, scales and cuts
  drawn at random with a fixed seed) to examples alone: exact coverage of the strips,
  constant width, scale unchanged by the commands.
- The reference SVGs go in `testdata/`. The real Mini-CAD is not there: it describes a real
  infrastructure. The deterministic generator `internal/testsvg` provides an
  equivalent, and `SVGPL_MINICAD=/path/to/file.svg` makes the engine tests run on
  the real file (see `docs/M0.md`).
- A test image is not added to `testdata/`: `internal/testimg` makes them, in a
  deterministic way (PNG, TIFF and JPEG with resolution and orientation, four-color checkerboard
  that makes any rotation visible). It keeps the repository light and the method readable.
- A rendering test never compares two rasters pixel by pixel: a rasterizer does not
  redraw the same image identically twice. Compare the geometry written in the
  PDF, or averages over squares (see "Seam test" in `ARCHITECTURE.md`).

- The package build scripts (`build-deb.sh`, `build-rpm.sh`,
  `windows/build-windows.sh`) pass `shellcheck` without warnings. A change that
  touches them is checked by building the package, installing it and running
  `imageprintlayout -selftest`; the CI does this for the three formats. Two builds of the
  same code must give identical files: put in a package neither a date, nor a
  path of the machine, nor a file order left to chance.
- The three scripts (`build-deb.sh`, `build-rpm.sh`, `windows/build-windows.ps1`) have the
  same options and the same handling of `VERSION` as those of Planche-Contact, and share
  no code: if you change one of these behaviors, change it in all three. The `repocheck`
  test and the CI (`.github/scripts/`) flag the discrepancies.
- The `.ps1` files and `installer.nsi` are UTF-8 **with** a byte order mark
  (BOM), needed by Windows PowerShell 5.1 and by NSIS to read accents; the scripts
  read by bash keep Unix line endings (`.gitattributes`).

## Adding a package in `internal/`

Describe it in the "Repository layout" section of `ARCHITECTURE.md`: a test fails
as long as a package of `internal/` is not listed there.

## Documentation

Every document of the project exists in two languages: English (the default file,
`X.md`) and French (`X.fr.md`), each linking to the other at the top of the file. When you
change one, change the other too (or say in your pull request that the other still has to
be updated). The documents are: `README`, `ARCHITECTURE`, `CONTRIBUTING`, `PROJECT_CONTEXT`,
`CHANGELOG`, `docs/M0`, `windows/README` and `testdata/README`. `go test ./internal/repocheck`
checks that both versions exist, link to each other and have the same sections, and that the
two changelogs list the same releases.

If your change modifies the observable behavior (new function, new setting,
new file format), update `CHANGELOG.md` in the same pull request, under
`[Unreleased]`, and `CHANGELOG.fr.md`, under `[Non publié]`. If the technical contract changes (interface, formula, format, default
value), also update `ARCHITECTURE.md` (and `ARCHITECTURE.fr.md`): it is the authoritative one.

## Versions

The project follows [Semantic Versioning](https://semver.org/). The current version
is in the `VERSION` file, which is injected at compile time:

```bash
go build -ldflags "-X main.version=$(cat VERSION)" ./cmd/imageprintlayout
```

The package build scripts handle `VERSION`: with `--major`, `--minor` or
`--patch` (or the menu they display without an option), they increment it once the package
is built; with `--no-version-change`, they leave it as is. **For a trial
build, use `--no-version-change`**, otherwise you modify `VERSION` without
meaning to. A version that carries a suffix (`0.1.0-dev`) is published first: `--minor` and
`--patch` give `0.1.0`, `--major` gives `1.0.0`.

A release is a separate pull request, "Release X.Y.Z":

1. build the first format with the desired increment (`./build-deb.sh --minor`): the
   script updates `VERSION`;
2. build the others with `--no-version-change` (`./build-rpm.sh --no-version-change`,
   `.\build-windows.ps1 -NoVersionChange`);
3. update **all the files that name the version**, in the same pull request:
   - `CHANGELOG.md` and `CHANGELOG.fr.md`: move the notes from `[Unreleased]` (`[Non publié]` in French) under
     `## [X.Y.Z] - YYYY-MM-DD` (the most recent first), with the same date in both;
   - `metainfo/io.github.gillesmagneville.imageprintlayout.metainfo.xml`: add
     `<release version="X.Y.Z" date="YYYY-MM-DD">` at the top of `<releases>`, with its
     description;
   - `docs/imageprintlayout.1`, `docs/fr/imageprintlayout.1`, `docs/es/imageprintlayout.1` and
     `docs/de/imageprintlayout.1`: the date and `imageprintlayout X.Y.Z` of the `.TH` line;
   - `README.md`, `ARCHITECTURE.md` (§14, "Status") and the reports in `docs/` when what
     they announce has changed with the version (and their `.fr.md` versions);

   `go test ./internal/repocheck` fails as long as `VERSION`, the CHANGELOG, the AppStream
   file and the four manual pages do not say the same version and the same date;
   the CI runs it. A text of the AppStream file (description of the release) is written in
   English and in the three other languages: `repocheck` checks this too;
4. merge, then tag the merge commit:

   ```bash
   git tag vX.Y.Z && git push origin vX.Y.Z
   ```

   The `Release` workflow (`.github/workflows/release.yml`) then builds the `.deb`, the
   `.rpm` and the Windows packages of the version in `VERSION`, tries them, then creates the GitHub
   release with the packages, their SHA-256 checksums and the section of the CHANGELOG (English,
   then French). It stops
   before building if the tag is not `v` followed by `VERSION`, if the tag is not on
   `main` or if a CHANGELOG has no section for this version. The packages built in
   steps 1 and 2 are only for trying: those of the release are built by the
   workflow. To redo a release whose workflow failed before creating it, rerun it
   from the Actions tab; if the release already exists, delete it first.

As long as `VERSION` carries a suffix (`0.1.0-dev`), no version is published.

## Questions

For any question that does not fit the cases above, simply open an issue.

## License

By contributing, you agree that your contributions will be published under the same license as
the project (GPL-3.0-only).

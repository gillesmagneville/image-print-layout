#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
set -euo pipefail

# Builds the RPM package of Image Print Layout, for Fedora, RHEL and their
# derivatives. Same options and same handling of the VERSION file as build-deb.sh.
# rpmbuild packages a folder prepared by the script; it compiles nothing
# itself.

# ====================== CONFIGURATION ======================
PACKAGE_NAME="imageprintlayout"
APP_ID="io.github.gillesmagneville.imageprintlayout"
MAINTAINER="Gilles MAGNEVILLE <gilles@magneville.fr>"
# The summary and the description are in English, the language of the packages,
# and the .rpm also carries them in French, Spanish and German (Summary(xx) and
# %description -l xx): rpm -qi shows the one of the language of the system.
DESCRIPTION="Lays out an SVG drawing or an image over several sheets and exports it as a PDF"
URL="https://github.com/gillesmagneville/image-print-layout"
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="$HOME/rpm-build"
BUILD_DIR="$WORK_DIR/build"
VERSION_FILE="$PROJECT_DIR/VERSION"
# ===========================================================

show_help() {
    cat << EOF
Usage: ./build-rpm.sh [options]

Script that builds the RPM package for Image Print Layout.

OPTIONS:
  --major               Increment the major number    (e.g. 1.2.3 → 2.0.0)
  --minor               Increment the minor number    (e.g. 1.2.3 → 1.3.0)
  --patch               Increment the patch number    (e.g. 1.2.3 → 1.2.4)
  --no-version-change   Rebuild the package without changing the version
  --clean               Clean the build folder and delete every .rpm
  --help, -h            Show this help

Without an option → interactive mode (asks major / minor / patch / no-version-change)

The VERSION file is only updated once the package has been built. A
pre-release version (0.1.0-dev) is released by --minor or --patch
(→ 0.1.0); --major gives 1.0.0.

The package is created at the root of the repository:
imageprintlayout-<version>-<release>.<arch>.rpm. A pre-release version
(0.1.0-dev) becomes release "0.1.dev" and sorts before 0.1.0.

Prerequisites:
  Fedora        : sudo dnf install golang gcc pkgconf librsvg2-devel gdk-pixbuf2-devel cairo-devel fontconfig-devel gtk4-devel gobject-introspection-devel rpm-build
  Debian/Ubuntu : sudo apt install golang-go build-essential pkg-config librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev rpm

The dependencies of the package (shared libraries) are deduced from the binary
by rpmbuild. The package is tied to the library versions of the build
machine: to target a distribution, build it on that distribution (or in a
container of that distribution).

For an identical build from one run to the next, the date of the files
is that of the last commit; SOURCE_DATE_EPOCH overrides it when it is set.

EOF
}

INCREMENT=""
NO_VERSION_CHANGE=false
CLEAN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --major)              INCREMENT="major"; shift ;;
        --minor)              INCREMENT="minor"; shift ;;
        --patch)              INCREMENT="patch"; shift ;;
        --no-version-change)  NO_VERSION_CHANGE=true; shift ;;
        --clean)              CLEAN=true; shift ;;
        --help|-h)            show_help; exit 0 ;;
        *)
            echo "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

cd "$PROJECT_DIR"

# === Clean mode ===
if [ "$CLEAN" = true ]; then
    echo ">>> Cleaning the build folder..."
    rm -rf "${WORK_DIR:?}"
    echo ">>> Deleting the existing .rpm packages..."
    rm -f ./*.rpm
    echo ">>> Clean-up done."
    exit 0
fi

# === Checking the tools ===
fail() {
    echo "" >&2
    echo "Error: $*" >&2
    exit 1
}

command -v rpmbuild >/dev/null 2>&1 || fail "rpmbuild is not installed. Install it with: sudo dnf install rpm-build (Debian/Ubuntu: sudo apt install rpm)"
command -v go >/dev/null 2>&1 || fail "Go is not installed (see go.mod for the version): https://go.dev/doc/install"
command -v gcc >/dev/null 2>&1 || fail "gcc is not installed. Install it with: sudo dnf install gcc (Debian/Ubuntu: sudo apt install build-essential)"
command -v pkg-config >/dev/null 2>&1 || fail "pkg-config is not installed. Install it with: sudo dnf install pkgconf (Debian/Ubuntu: sudo apt install pkg-config)"
pkg-config --exists librsvg-2.0 gdk-pixbuf-2.0 cairo fontconfig || fail "the development libraries of librsvg, gdk-pixbuf, cairo and fontconfig are missing.
  Fedora        : sudo dnf install librsvg2-devel gdk-pixbuf2-devel cairo-devel fontconfig-devel
  Debian/Ubuntu : sudo apt install librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev"
pkg-config --exists gtk4 gobject-introspection-1.0 || fail "the GTK 4 development libraries are missing (GTK 4.14 or newer).
  Fedora        : sudo dnf install gtk4-devel gobject-introspection-devel
  Debian/Ubuntu : sudo apt install libgtk-4-dev libgirepository1.0-dev"
pkg-config --atleast-version=4.14 gtk4 && pkg-config --atleast-version=2.80 glib-2.0 && pkg-config --atleast-version=2.52 librsvg-2.0 ||
    fail "this distribution's libraries are too old: GTK $(pkg-config --modversion gtk4), GLib $(pkg-config --modversion glib-2.0), librsvg $(pkg-config --modversion librsvg-2.0).
  At least GTK 4.14, GLib 2.80 and librsvg 2.52 are required (gotk4 v0.3.1): Debian 13, Ubuntu 24.04, Fedora,
  RHEL 10 and their derivatives. Debian 12, Ubuntu 22.04 and RHEL 9 are not suitable."

case "$(uname -m)" in
    x86_64)  RPM_ARCH="x86_64" ;;
    aarch64) RPM_ARCH="aarch64" ;;
    *) fail "architecture $(uname -m) is not supported (x86_64 and aarch64 only)." ;;
esac

# === Reading the current version ===
[ -f "$VERSION_FILE" ] || fail "VERSION file not found: $VERSION_FILE"
CURRENT_VERSION=$(tr -d '[:space:]' < "$VERSION_FILE")

# A version is X.Y.Z, optionally followed by a pre-release suffix
# (X.Y.Z-dev).
SEMVER='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'
[[ "$CURRENT_VERSION" =~ $SEMVER ]] || fail "VERSION contains '$CURRENT_VERSION', which is not an X.Y.Z or X.Y.Z-suffix version."

# next_version VERSION major|minor|patch prints the next version. Without a
# suffix, it is the usual increment (1.2.3 → 1.2.4). With a suffix, the
# pre-release version is released first: 0.1.0-dev → 0.1.0 with
# --minor or --patch, since 0.1.0 does not exist yet; --major gives 1.0.0.
next_version() {
    local version="$1" part="$2" pre=""
    [[ "$version" == *-* ]] && pre="${version#*-}"
    local major minor patch
    IFS='.' read -r major minor patch <<< "${version%%-*}"
    case $part in
        major)
            if [ "$minor" -ne 0 ] || [ "$patch" -ne 0 ] || [ -z "$pre" ]; then major=$((major + 1)); fi
            minor=0
            patch=0
            ;;
        minor)
            if [ "$patch" -ne 0 ] || [ -z "$pre" ]; then minor=$((minor + 1)); fi
            patch=0
            ;;
        patch)
            if [ -z "$pre" ]; then patch=$((patch + 1)); fi
            ;;
    esac
    echo "${major}.${minor}.${patch}"
}

# === Interactive mode when there is no option ===
if [ -z "$INCREMENT" ] && [ "$NO_VERSION_CHANGE" = false ]; then
    echo ""
    echo "Current version: $CURRENT_VERSION"
    echo ""
    echo "What do you want to do?"
    echo "  1) major              (→ $(next_version "$CURRENT_VERSION" major))"
    echo "  2) minor              (→ $(next_version "$CURRENT_VERSION" minor))"
    echo "  3) patch              (→ $(next_version "$CURRENT_VERSION" patch))"
    echo "  4) no-version-change  (rebuild as $CURRENT_VERSION)"
    echo ""
    read -r -p "Your choice [1/2/3/4]: " CHOICE || CHOICE=""

    case $CHOICE in
        1) INCREMENT="major" ;;
        2) INCREMENT="minor" ;;
        3) INCREMENT="patch" ;;
        4) NO_VERSION_CHANGE=true ;;
        *)
            echo "Invalid choice. Cancelled."
            exit 1
            ;;
    esac
fi

# === Computing the new version ===
if [ "$NO_VERSION_CHANGE" = true ]; then
    NEW_VERSION="$CURRENT_VERSION"
else
    NEW_VERSION=$(next_version "$CURRENT_VERSION" "$INCREMENT")
fi

# RPM version numbers. A pre-release version (0.1.0-dev) must sort before the
# final version (0.1.0): its release is "0.1.dev", that of a final version
# is "1".
RPM_VERSION="${NEW_VERSION%%-*}"
PRERELEASE=""
if [[ "$NEW_VERSION" == *-* ]]; then
    PRERELEASE="${NEW_VERSION#*-}"
    PRERELEASE="${PRERELEASE//-/.}"
fi
RPM_RELEASE="${PRERELEASE:+0.1.$PRERELEASE}"
RPM_RELEASE="${RPM_RELEASE:-1}"

# === Asking for confirmation ===
echo ""
echo "========================================"
if [ "$NO_VERSION_CHANGE" = true ]; then
    echo "  Mode           : Rebuild without changing the version"
    echo "  Version        : $NEW_VERSION"
else
    echo "  Current version : $CURRENT_VERSION"
    echo "  New version     : $NEW_VERSION"
fi
echo "========================================"
echo ""
read -r -p "Build the package as version $NEW_VERSION? [y/N] " CONFIRM || CONFIRM=""

if [[ ! "$CONFIRM" =~ ^[yY]$ ]]; then
    echo "Build cancelled. No file was modified."
    exit 0
fi

# === From here on, the build is confirmed ===

echo ">>> Deleting the existing .rpm packages..."
rm -f ./*.rpm

RPM_FILE="./${PACKAGE_NAME}-${RPM_VERSION}-${RPM_RELEASE}.${RPM_ARCH}.rpm"

# Date of the package files: that of the last commit, so that two builds of
# the same code give the same package.
if [ -z "${SOURCE_DATE_EPOCH:-}" ]; then
    SOURCE_DATE_EPOCH="$(git -c safe.directory="$PROJECT_DIR" -C "$PROJECT_DIR" log -1 --format=%ct 2>/dev/null || true)"
    SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(date +%s)}"
fi
export SOURCE_DATE_EPOCH

echo ""
echo "========================================"
echo " Building package $PACKAGE_NAME v$NEW_VERSION ($RPM_ARCH)"
echo "========================================"

rm -rf "${WORK_DIR:?}"
mkdir -p "$BUILD_DIR"

# -trimpath, an empty build id and the absence of VCS information make the
# binary independent of the folder and of the time of the build; -buildmode=pie
# lets Linux place the program at a random address (hardening). The package
# version number is written into the program (imageprintlayout -version), even
# though the project's VERSION file is only updated after a successful build
# (see below).
# gotk4 (the GTK binding for Go) declares "free" differently from <stdlib.h>:
# gcc reports it three times at every compilation (-Wbuiltin-declaration-mismatch).
# That is the library's code and not a defect; only this warning is hidden,
# without touching cgo's default options
# (-O2 -g). The other warnings stay visible.
echo ">>> Compiling (cgo, GTK4, librsvg, gdk-pixbuf, cairo)..."
echo "    The first compilation of GTK (gotk4) takes several minutes and prints nothing;"
echo "    the following ones are almost instant (Go cache)."
BINARY="$WORK_DIR/$PACKAGE_NAME"
( cd "$PROJECT_DIR" && CGO_ENABLED=1 CGO_CFLAGS="${CGO_CFLAGS:--O2 -g} -Wno-builtin-declaration-mismatch" go build -trimpath -buildvcs=false -buildmode=pie \
    -ldflags "-s -w -buildid= -X main.version=$NEW_VERSION" -o "$BINARY" ./cmd/imageprintlayout )

echo ">>> Creating the file tree..."
install -D -m 0755 "$BINARY" "$BUILD_DIR/usr/bin/$PACKAGE_NAME"
install -D -m 0644 "$PROJECT_DIR/metainfo/$APP_ID.desktop" "$BUILD_DIR/usr/share/applications/$APP_ID.desktop"
# AppStream metadata: it lets software centres (GNOME Software,
# KDE Discover) show the package with its full description.
install -D -m 0644 "$PROJECT_DIR/metainfo/$APP_ID.metainfo.xml" "$BUILD_DIR/usr/share/metainfo/$APP_ID.metainfo.xml"
install -D -m 0644 "$PROJECT_DIR/icons/imageprintlayout.svg" "$BUILD_DIR/usr/share/icons/hicolor/scalable/apps/imageprintlayout.svg"
install -D -m 0644 "$PROJECT_DIR/icons/imageprintlayout-256.png" "$BUILD_DIR/usr/share/icons/hicolor/256x256/apps/imageprintlayout.png"
# Manual page: in English in man1 (fallback language), translated in the
# folder of each language (man-db picks the one matching the user's language).
install -d -m 0755 "$BUILD_DIR/usr/share/man/man1"
gzip -9n -c "$PROJECT_DIR/docs/imageprintlayout.1" > "$BUILD_DIR/usr/share/man/man1/$PACKAGE_NAME.1.gz"
for LANG_CODE in fr es de; do
    install -d -m 0755 "$BUILD_DIR/usr/share/man/$LANG_CODE/man1"
    gzip -9n -c "$PROJECT_DIR/docs/$LANG_CODE/imageprintlayout.1" > "$BUILD_DIR/usr/share/man/$LANG_CODE/man1/$PACKAGE_NAME.1.gz"
done
chmod 0644 "$BUILD_DIR"/usr/share/man/man1/*.gz "$BUILD_DIR"/usr/share/man/*/man1/*.gz
install -D -m 0644 "$PROJECT_DIR/README.md" "$BUILD_DIR/usr/share/doc/$PACKAGE_NAME/README.md"
install -D -m 0644 "$PROJECT_DIR/README.fr.md" "$BUILD_DIR/usr/share/doc/$PACKAGE_NAME/README.fr.md"
install -D -m 0644 "$PROJECT_DIR/CODE_OF_CONDUCT.md" "$BUILD_DIR/usr/share/doc/$PACKAGE_NAME/CODE_OF_CONDUCT.md"
install -D -m 0644 "$PROJECT_DIR/CODE_OF_CONDUCT.fr.md" "$BUILD_DIR/usr/share/doc/$PACKAGE_NAME/CODE_OF_CONDUCT.fr.md"
install -D -m 0644 "$PROJECT_DIR/CHANGELOG.md" "$BUILD_DIR/usr/share/doc/$PACKAGE_NAME/CHANGELOG.md"
install -D -m 0644 "$PROJECT_DIR/CHANGELOG.fr.md" "$BUILD_DIR/usr/share/doc/$PACKAGE_NAME/CHANGELOG.fr.md"
install -D -m 0644 "$PROJECT_DIR/LICENSE" "$BUILD_DIR/usr/share/licenses/$PACKAGE_NAME/LICENSE"

# install -m already sets the wanted permissions, whatever the umask of the
# build machine; here we set the dates, so that the package does not depend on
# the time of the build.
echo ">>> Normalising the permissions..."
find "$BUILD_DIR" -type d -exec chmod 755 {} +
find "$BUILD_DIR" -exec touch -h -d "@$SOURCE_DATE_EPOCH" {} +

echo ">>> Writing the .spec file..."
TOP="$WORK_DIR/rpmbuild"
SPEC="$TOP/SPECS/$PACKAGE_NAME.spec"
mkdir -p "$TOP/SPECS"
{
    # There is no %prep, %build or %install: the program is already
    # compiled and installed in BUILD_DIR, rpmbuild only packages it.
    cat << EOF
# The binary is already stripped at link time; no debuginfo package.
%global debug_package %{nil}
%global __strip /bin/true
%define _build_id_links none

# The same content gives the same package: dates fixed by SOURCE_DATE_EPOCH.
%define use_source_date_epoch_as_buildtime 1
%define clamp_mtime_to_source_date_epoch 1
%define _buildhost reproducible

Name:           $PACKAGE_NAME
Version:        $RPM_VERSION
Release:        $RPM_RELEASE
Summary:        $DESCRIPTION
Summary(fr):    Met en page un dessin SVG ou une image sur plusieurs feuilles et l'exporte en PDF
Summary(es):    Reparte un dibujo SVG o una imagen en varias hojas y lo exporta como PDF
Summary(de):    Verteilt eine SVG-Zeichnung oder ein Bild auf mehrere Blätter und exportiert das Ergebnis als PDF
License:        GPL-3.0-only AND CC-BY-SA-4.0
URL:            $URL
Packager:       $MAINTAINER
# The gdk-pixbuf image loaders are partly in separate packages:
# TIFF in gdk-pixbuf2-modules, WebP in webp-pixbuf-loader. Without them the
# program refuses that format and says so, and reads the others.
Recommends:     gdk-pixbuf2-modules webp-pixbuf-loader
# The program was called "svgprintlayout" up to version 1.0.x: installing
# this one removes the old package.
Obsoletes:      svgprintlayout < 1.1.0

%description
Image Print Layout prepares a document of any size for printing on several
sheets of standard paper: a vector SVG drawing, or an image (JPEG, PNG,
TIFF, WebP, BMP, GIF). You choose the format, the margins and the width,
place the cuts with the mouse, then export a single multi-page PDF
(vector for an SVG).

The document keeps a single scale: no page is silently shrunk to fit a
portion that is too tall. The original file is never modified.

%description -l fr
Image Print Layout prépare l'impression d'un document de dimensions
quelconques sur plusieurs feuilles de papier standard : un dessin vectoriel
SVG, ou une image (JPEG, PNG, TIFF, WebP, BMP, GIF). On choisit le format,
les marges et la largeur, on place les coupures à la souris, puis on exporte
un seul PDF multipage (vectoriel pour un SVG).

Le document garde une échelle unique : aucune page n'est réduite en silence
pour faire tenir une portion trop haute. Le fichier d'origine n'est jamais
modifié.

%description -l es
Image Print Layout prepara la impresión de un documento de cualquier tamaño
en varias hojas de papel estándar: un dibujo vectorial SVG o una imagen
(JPEG, PNG, TIFF, WebP, BMP, GIF). Se elige el formato, los márgenes y el
ancho, se colocan los cortes con el ratón y se exporta un único PDF de
varias páginas (vectorial si es un SVG).

El documento conserva una única escala: ninguna página se reduce en silencio
para que quepa una parte demasiado alta. El archivo original nunca se
modifica.

%description -l de
Image Print Layout bereitet ein Dokument beliebiger Größe für den Druck auf
mehreren Blättern Standardpapier vor: eine SVG-Vektorzeichnung oder ein Bild
(JPEG, PNG, TIFF, WebP, BMP, GIF). Man wählt Format, Ränder und Breite,
setzt die Schnitte mit der Maus und exportiert ein einziges mehrseitiges PDF
(bei SVG als Vektor-PDF).

Das Dokument behält einen einzigen Maßstab: Keine Seite wird
stillschweigend verkleinert, damit ein zu hoher Abschnitt passt. Die
ursprüngliche Datei wird nie verändert.

# Updates the icon and menu caches; a failure is not an error.
%post
/usr/bin/gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor >/dev/null 2>&1 || :
/usr/bin/update-desktop-database -q /usr/share/applications >/dev/null 2>&1 || :

%postun
/usr/bin/gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor >/dev/null 2>&1 || :
/usr/bin/update-desktop-database -q /usr/share/applications >/dev/null 2>&1 || :

EOF
    echo "%files"
    echo "%defattr(-,root,root,-)"
    echo "%attr(0755,root,root) /usr/bin/$PACKAGE_NAME"
    echo "%dir /usr/share/doc/$PACKAGE_NAME"
    echo "%dir /usr/share/licenses/$PACKAGE_NAME"
    # The notes and the licence get their marks (%doc, %license): dnf
    # can then exclude them or list them separately.
    ( cd "$BUILD_DIR" && find usr -type f ! -path "usr/bin/*" | sort | sed \
        -e 's|^usr/share/doc/|%doc /usr/share/doc/|' \
        -e 's|^usr/share/licenses/|%license /usr/share/licenses/|' \
        -e 's|^usr/share/man/|%doc /usr/share/man/|' \
        -e 's|^usr/|/usr/|' )
    echo ""
    echo "%changelog"
    echo "* $(LC_ALL=C date -u -d "@$SOURCE_DATE_EPOCH" '+%a %b %d %Y') $MAINTAINER - $RPM_VERSION-$RPM_RELEASE"
    echo "- Package built from the repository; the history is in CHANGELOG.md (English) and CHANGELOG.fr.md (French)."
} > "$SPEC"

echo ">>> Creating the .rpm package..."
rpmbuild -bb "$SPEC" \
    --quiet \
    --buildroot "$BUILD_DIR" \
    --target "${RPM_ARCH}-linux" \
    --define "_topdir $TOP" \
    --define "_rpmdir $PROJECT_DIR" \
    --define "_rpmfilename %%{NAME}-%%{VERSION}-%%{RELEASE}.%%{ARCH}.rpm"
[ -f "$RPM_FILE" ] || fail "rpmbuild did not produce $RPM_FILE."

echo ""
echo "✅ Package generated:"
echo "   $RPM_FILE"
echo "   $(du -h "$RPM_FILE" | cut -f1) — sha256 $(sha256sum "$RPM_FILE" | cut -c1-16)…"

# The project's VERSION file is only updated here, once the .rpm has really
# been built. This way, a failed build (compilation, rpmbuild...)
# never leaves the project in an inconsistent state (VERSION incremented
# without a matching .rpm).
if [ "$NO_VERSION_CHANGE" = true ]; then
    echo "   Version unchanged: $NEW_VERSION"
else
    echo "$NEW_VERSION" > "$VERSION_FILE"
    echo "   Version recorded: $NEW_VERSION"
    if [[ "$NEW_VERSION" != *-* ]]; then
        echo "   Remember to add '## [$NEW_VERSION]' to CHANGELOG.md and CHANGELOG.fr.md (see CONTRIBUTING.md)."
    fi
fi
echo ""
echo "To try it: sudo dnf install \"$RPM_FILE\" && $PACKAGE_NAME -selftest"

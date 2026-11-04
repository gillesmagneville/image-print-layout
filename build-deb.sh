#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
set -euo pipefail

# Builds the Debian package (.deb) of Image Print Layout, for Debian, Ubuntu and
# their derivatives. Same options and same handling of the VERSION file as the
# build-deb.sh of Planche-Contact. The program is compiled in Go (cgo, librsvg,
# gdk-pixbuf, cairo) and packaged with dpkg-deb: no fpm, no embedded Python
# environment.

# ====================== CONFIGURATION ======================
PACKAGE_NAME="imageprintlayout"
APP_ID="io.github.gillesmagneville.imageprintlayout"
MAINTAINER="Gilles MAGNEVILLE <gilles@magneville.fr>"
# A .deb has a single description, and by custom it is in English. The software
# centres show the AppStream sheet (metainfo/), which has the four languages.
DESCRIPTION="Lays out an SVG drawing or an image over several sheets and exports it as a PDF"
URL="https://github.com/gillesmagneville/image-print-layout"
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="$HOME/deb-build"
BUILD_DIR="$WORK_DIR/build"
VERSION_FILE="$PROJECT_DIR/VERSION"
# ===========================================================

show_help() {
    cat << EOF
Usage: ./build-deb.sh [options]

Script that builds the Debian package for Image Print Layout.

OPTIONS:
  --major               Increment the major number    (e.g. 1.2.3 → 2.0.0)
  --minor               Increment the minor number    (e.g. 1.2.3 → 1.3.0)
  --patch               Increment the patch number    (e.g. 1.2.3 → 1.2.4)
  --no-version-change   Rebuild the package without changing the version
  --clean               Clean the build folder and delete every .deb
  --help, -h            Show this help

Without an option → interactive mode (asks major / minor / patch / no-version-change)

The VERSION file is only updated once the package has been built. A
pre-release version (0.1.0-dev) is released by --minor or --patch
(→ 0.1.0); --major gives 1.0.0.

The package is created at the root of the repository: imageprintlayout_<version>_<arch>.deb

Prerequisites (Debian, Ubuntu):
  sudo apt install golang-go build-essential pkg-config librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev dpkg-dev

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
    echo ">>> Deleting the existing .deb packages..."
    rm -f ./*.deb
    echo ">>> Clean-up done."
    exit 0
fi

# === Checking the tools ===
fail() {
    echo "" >&2
    echo "Error: $*" >&2
    exit 1
}

command -v dpkg-deb >/dev/null 2>&1 || fail "dpkg-deb is not installed. Install it with: sudo apt install dpkg"
command -v dpkg-shlibdeps >/dev/null 2>&1 || fail "dpkg-shlibdeps is not installed. Install it with: sudo apt install dpkg-dev"
command -v go >/dev/null 2>&1 || fail "Go is not installed (see go.mod for the version): https://go.dev/doc/install"
command -v gcc >/dev/null 2>&1 || fail "gcc is not installed. Install it with: sudo apt install build-essential"
command -v pkg-config >/dev/null 2>&1 || fail "pkg-config is not installed. Install it with: sudo apt install pkg-config"
pkg-config --exists librsvg-2.0 gdk-pixbuf-2.0 cairo fontconfig || fail "the development libraries of librsvg, gdk-pixbuf, cairo and fontconfig are missing.
  Install them with: sudo apt install librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev"
pkg-config --exists gtk4 gobject-introspection-1.0 || fail "the GTK 4 development libraries are missing (GTK 4.14 or newer).
  Install them with: sudo apt install libgtk-4-dev libgirepository1.0-dev"
pkg-config --atleast-version=4.14 gtk4 && pkg-config --atleast-version=2.80 glib-2.0 && pkg-config --atleast-version=2.52 librsvg-2.0 ||
    fail "this distribution's libraries are too old: GTK $(pkg-config --modversion gtk4), GLib $(pkg-config --modversion glib-2.0), librsvg $(pkg-config --modversion librsvg-2.0).
  At least GTK 4.14, GLib 2.80 and librsvg 2.52 are required (gotk4 v0.3.1): Debian 13, Ubuntu 24.04, Fedora,
  RHEL 10 and their derivatives. Debian 12, Ubuntu 22.04 and RHEL 9 are not suitable."

case "$(uname -m)" in
    x86_64)  DEB_ARCH="amd64" ;;
    aarch64) DEB_ARCH="arm64" ;;
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

# Debian version number. A pre-release version (0.1.0-dev) must sort before the
# final version (0.1.0): that is the role of the "~". No Debian revision
# ("-1"): as with Planche-Contact and LinuxDoctor, the file name is
# imageprintlayout_<version>_<arch>.deb; the "-1" stays that of the .rpm
# (release).
VERSION_BASE="${NEW_VERSION%%-*}"
PRERELEASE=""
if [[ "$NEW_VERSION" == *-* ]]; then
    PRERELEASE="${NEW_VERSION#*-}"
    PRERELEASE="${PRERELEASE//-/.}"
fi
DEB_VERSION="${VERSION_BASE}${PRERELEASE:+~$PRERELEASE}"

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

echo ">>> Deleting the existing .deb packages..."
rm -f ./*.deb

DEB_FILE="./${PACKAGE_NAME}_${DEB_VERSION}_${DEB_ARCH}.deb"

# Date of the package files: that of the last commit, so that two builds of
# the same code give the same package.
if [ -z "${SOURCE_DATE_EPOCH:-}" ]; then
    SOURCE_DATE_EPOCH="$(git -c safe.directory="$PROJECT_DIR" -C "$PROJECT_DIR" log -1 --format=%ct 2>/dev/null || true)"
    SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(date +%s)}"
fi
export SOURCE_DATE_EPOCH

echo ""
echo "========================================"
echo " Building package $PACKAGE_NAME v$NEW_VERSION ($DEB_ARCH)"
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
# AppStream metadata: it lets software centres (GNOME Software, App Center)
# show the package with its full description.
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

DOC_DIR="$BUILD_DIR/usr/share/doc/$PACKAGE_NAME"
install -D -m 0644 "$PROJECT_DIR/README.md" "$DOC_DIR/README.md"
install -D -m 0644 "$PROJECT_DIR/README.fr.md" "$DOC_DIR/README.fr.md"
install -D -m 0644 "$PROJECT_DIR/CODE_OF_CONDUCT.md" "$DOC_DIR/CODE_OF_CONDUCT.md"
install -D -m 0644 "$PROJECT_DIR/CODE_OF_CONDUCT.fr.md" "$DOC_DIR/CODE_OF_CONDUCT.fr.md"
install -D -m 0644 "$PROJECT_DIR/debian/copyright" "$DOC_DIR/copyright"
# Names that Debian expects for the change logs: that of the project
# (changelog.gz, in English, with its French version changelog.fr.gz) and that
# of the package (changelog.Debian.gz, in Debian format).
gzip -9n -c "$PROJECT_DIR/CHANGELOG.md" > "$DOC_DIR/changelog.gz"
gzip -9n -c "$PROJECT_DIR/CHANGELOG.fr.md" > "$DOC_DIR/changelog.fr.gz"
{
    echo "$PACKAGE_NAME ($DEB_VERSION) unstable; urgency=low"
    echo ""
    echo "  * Package built from the repository; the history is in changelog.gz"
    echo "    (English) and changelog.fr.gz (French)."
    echo ""
    echo " -- $MAINTAINER  $(date -u -d "@$SOURCE_DATE_EPOCH" -R)"
} | gzip -9n -c > "$DOC_DIR/changelog.Debian.gz"
install -D -m 0644 "$PROJECT_DIR/debian/lintian-overrides" "$BUILD_DIR/usr/share/lintian/overrides/$PACKAGE_NAME"

# === Normalising the permissions ----------------------------------------
# install -m already sets the wanted permissions, whatever the umask of the
# build machine (a restrictive umask would otherwise leave files unreadable
# for a normal user once the package is installed). Here we normalise the
# folders and the dates, so that the package does not depend on the time of
# the build either.
echo ">>> Normalising the permissions..."
chmod 0644 "$DOC_DIR/changelog.gz" "$DOC_DIR/changelog.Debian.gz" "$BUILD_DIR"/usr/share/man/man1/*.gz "$BUILD_DIR"/usr/share/man/*/man1/*.gz
find "$BUILD_DIR" -type d -exec chmod 755 {} +

# Dependencies: dpkg-shlibdeps reads the libraries that the binary loads and
# deduces the packages from them (with the minimum versions): librsvg, cairo and
# GTK 4 (libgtk-4-1) for the window.
echo ">>> Computing the dependencies (dpkg-shlibdeps)..."
SHLIBS_TMP="$(mktemp -d)"
trap 'rm -rf "${SHLIBS_TMP:?}"' EXIT
# dpkg-shlibdeps expects a debian/ folder with a control file. It warns that
# the binary is not yet in its place in a package: that is normal, we examine it
# before building the package, and we do not keep that warning.
# LC_ALL=C: the message is filtered on its English text, so it must be
# written in English whatever the language of the system.
mkdir "$SHLIBS_TMP/debian"
printf 'Source: %s\n\nPackage: %s\nArchitecture: any\n' "$PACKAGE_NAME" "$PACKAGE_NAME" > "$SHLIBS_TMP/debian/control"
DEPENDS="$(cd "$SHLIBS_TMP" && LC_ALL=C dpkg-shlibdeps -O "-e$BINARY" 2> "$SHLIBS_TMP/warnings" | sed -n 's/^shlibs:Depends=//p')"
grep -v 'should already be installed' "$SHLIBS_TMP/warnings" >&2 || true
[ -n "$DEPENDS" ] || fail "dpkg-shlibdeps found no dependency: the binary is not linked to the expected libraries."
echo "    Depends: $DEPENDS"

echo ">>> Creating the control files..."
INSTALLED_SIZE="$(du -sk "$BUILD_DIR" | cut -f1)"
mkdir -p "$BUILD_DIR/DEBIAN"
cat > "$BUILD_DIR/DEBIAN/control" << EOF
Package: $PACKAGE_NAME
Version: $DEB_VERSION
Section: graphics
Priority: optional
Architecture: $DEB_ARCH
Maintainer: $MAINTAINER
Installed-Size: $INSTALLED_SIZE
Depends: $DEPENDS
Recommends: webp-pixbuf-loader
Conflicts: svgprintlayout (<< 1.1.0)
Replaces: svgprintlayout (<< 1.1.0)
Homepage: $URL
Description: $DESCRIPTION
 Image Print Layout prepares a document of any size for printing on several
 sheets of standard paper: a vector SVG drawing, or an image (JPEG, PNG,
 TIFF, WebP, BMP, GIF). You choose the format, the margins and the width,
 place the cuts with the mouse, then export a single multi-page PDF
 (vector for an SVG).
 .
 The document keeps a single scale: no page is silently shrunk to fit a
 portion that is too tall. The original file is never modified.
EOF
# Maintainer scripts: dpkg-deb handles them natively when they are in DEBIAN/.
install -m 0755 "$PROJECT_DIR/debian/postinst" "$BUILD_DIR/DEBIAN/postinst"
install -m 0755 "$PROJECT_DIR/debian/postrm" "$BUILD_DIR/DEBIAN/postrm"
( cd "$BUILD_DIR" && find usr -type f -print0 | sort -z | xargs -0 md5sum ) > "$BUILD_DIR/DEBIAN/md5sums"
chmod 0644 "$BUILD_DIR/DEBIAN/md5sums"
find "$BUILD_DIR" -exec touch -h -d "@$SOURCE_DATE_EPOCH" {} +

echo ">>> Creating the .deb package..."
dpkg-deb --root-owner-group -Zxz --build "$BUILD_DIR" "$DEB_FILE" > /dev/null

echo ""
echo "✅ Package generated:"
echo "   $DEB_FILE"
    echo "   $(du -h "$DEB_FILE" | cut -f1) — sha256 $(sha256sum "$DEB_FILE" | cut -c1-16)…"

# The project's VERSION file is only updated here, once the .deb has really
# been built. This way, a failed build (compilation,
# dpkg-shlibdeps, dpkg-deb...) never leaves the project in an inconsistent
# state (VERSION incremented without a matching .deb).
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
echo "To try it: sudo apt install \"$DEB_FILE\" && $PACKAGE_NAME -selftest"

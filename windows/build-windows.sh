#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
set -euo pipefail

# Builds the Windows package of Image Print Layout: a portable version (.zip)
# and, if NSIS is installed, an installer (.exe), at the root of the repository.
#
# This script runs in an MSYS2 UCRT64 terminal (that is where Go, the C
# compiler, GTK 4, librsvg and cairo for Windows are found). It is best run
# through build-windows.ps1, at the root of the repository: that script handles
# the options and the VERSION file like build-deb.sh, prepares MSYS2 and NSIS,
# then calls this script with the chosen version number. This script never
# modifies the VERSION file.

# ====================== CONFIGURATION ======================
PACKAGE_NAME="imageprintlayout"
WINDOWS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$WINDOWS_DIR")"
# Working folder: the one that build-windows.ps1 chooses (variable
# SVGPL_BUILD_DIR, as a Windows or MSYS2 path), otherwise a folder in the
# home directory. Set by main().
WORK=""
# ===========================================================

step() { echo ">>> $*"; }

die() {
    echo "" >&2
    echo "Error: $*" >&2
    exit 1
}

# require_tool COMMAND "installation advice"
require_tool() {
    command -v "$1" >/dev/null 2>&1 || die "$1 not found. $2"
}

# MSYS2 packages needed for the build (the runtime libraries come with
# GTK 4, librsvg and cairo). The icon themes are shipped with the program:
# without them, the buttons of the window have no image.
MSYS_PACKAGES=(
    mingw-w64-ucrt-x86_64-go
    mingw-w64-ucrt-x86_64-gcc
    mingw-w64-ucrt-x86_64-pkgconf
    mingw-w64-ucrt-x86_64-librsvg
    mingw-w64-ucrt-x86_64-gdk-pixbuf2
    mingw-w64-ucrt-x86_64-cairo
    mingw-w64-ucrt-x86_64-fontconfig
    mingw-w64-ucrt-x86_64-gtk4
    mingw-w64-ucrt-x86_64-gobject-introspection
    mingw-w64-ucrt-x86_64-adwaita-icon-theme
    mingw-w64-ucrt-x86_64-hicolor-icon-theme
    zip
    git
)

# Two executables: imageprintlayout.exe, the one of the window (Windows
# subsystem: no console opens behind it), and imageprintlayout-console.exe,
# the same program in the console subsystem, for -version, -selftest and the
# error messages in a terminal (a windowed program writes nothing in a
# terminal).
GUI_EXE="$PACKAGE_NAME.exe"
CONSOLE_EXE="$PACKAGE_NAME-console.exe"

# GTK languages shipped with the program (the text of its own buttons);
# English is in the code of GTK.
GTK_LOCALES=(fr es de)

# Image formats that the shipped gdk-pixbuf loaders can read: those of the
# images the program opens (JPEG, PNG, TIFF, BMP, GIF, WebP) and those of the
# images embedded in SVGs. The first five are essential: without one of
# them, the program would not do what it announces, and the build
# stops. WebP (a separate MSYS2 package) and SVG are shipped if they are there.
PIXBUF_REQUIRED=(png jpeg tiff gif bmp)
PIXBUF_OPTIONAL=(webp svg)

show_help() {
    cat << EOF2
Usage: windows/build-windows.sh [options]

Builds, at the root of the repository:
  imageprintlayout_<version>_windows-portable.zip   portable version
  imageprintlayout_<version>_windows-setup.exe      installer (if NSIS is there)

OPTIONS:
  --version X.Y.Z   Version number to build (default: the content of the
                    VERSION file). This script never modifies VERSION.
  --install-deps    Install the missing MSYS2 packages with pacman
  --check-deps      Only check that the MSYS2 packages are present
  --help, -h        Show this help

To be run in a "MSYS2 UCRT64" terminal (not MSYS or MINGW64). Simpler:
.\\build-windows.ps1 at the root of the repository, from PowerShell: it accepts
-Major, -Minor, -Patch and -NoVersionChange, like build-deb.sh.

The working folder is the environment variable SVGPL_BUILD_DIR
(default ~/imageprintlayout-build).
EOF2
}

INSTALL_DEPS=false
CHECK_DEPS=false
VERSION=""

# ensure_packages checks the MSYS2 packages; with --install-deps, it installs
# the missing ones, with --check-deps it only lists them.
ensure_packages() {
    local missing=() p
    for p in "${MSYS_PACKAGES[@]}"; do
        pacman -Q "$p" >/dev/null 2>&1 || missing+=("$p")
    done
    [ ${#missing[@]} -eq 0 ] && return 0
    if [ "$CHECK_DEPS" = true ]; then
        echo "Missing MSYS2 packages:" >&2
        printf '  %s\n' "${missing[@]}" >&2
        return 1
    fi
    if [ "$INSTALL_DEPS" != true ]; then
        die "missing MSYS2 packages:
$(printf '  %s\n' "${missing[@]}")
Install them with: pacman -S --needed ${missing[*]}
or run this script again with --install-deps."
    fi
    step "Installing the missing MSYS2 packages..."
    pacman -S --needed --noconfirm "${missing[@]}"
}

# Date of the package files: that of the last commit, so that two builds of
# the same code give the same package.
set_source_date_epoch() {
    if [ -z "${SOURCE_DATE_EPOCH:-}" ]; then
        SOURCE_DATE_EPOCH="$(git -c safe.directory="$PROJECT_DIR" -C "$PROJECT_DIR" log -1 --format=%ct 2>/dev/null || true)"
        SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(date +%s)}"
    fi
    export SOURCE_DATE_EPOCH
}

# read_version sets VERSION (--version option, otherwise the VERSION file) and
# VERSION_BASE, its numeric part: a pre-release version
# (0.1.0-dev) keeps its suffix in the texts, but Windows version numbers
# only have four numbers.
read_version() {
    if [ -z "$VERSION" ]; then
        [ -f "$PROJECT_DIR/VERSION" ] || die "VERSION file not found: $PROJECT_DIR/VERSION"
        VERSION="$(tr -d '[:space:]' < "$PROJECT_DIR/VERSION")"
    fi
    local semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'
    [[ "$VERSION" =~ $semver ]] || die "'$VERSION' is not an X.Y.Z or X.Y.Z-suffix version."
    VERSION_BASE="${VERSION%%-*}"
}

check_go_prerequisites() {
    require_tool go "Install the MSYS2 package mingw-w64-ucrt-x86_64-go."
    require_tool gcc "Install the MSYS2 package mingw-w64-ucrt-x86_64-gcc."
    require_tool pkg-config "Install the MSYS2 package mingw-w64-ucrt-x86_64-pkgconf."
    pkg-config --exists librsvg-2.0 gdk-pixbuf-2.0 cairo fontconfig || die "the development libraries of librsvg, gdk-pixbuf, cairo and fontconfig are missing (MSYS2 packages mingw-w64-ucrt-x86_64-librsvg, -gdk-pixbuf2, -cairo and -fontconfig)."
    pkg-config --exists gtk4 gobject-introspection-1.0 || die "the GTK 4 development libraries are missing (MSYS2 packages mingw-w64-ucrt-x86_64-gtk4 and -gobject-introspection)."
}

# build_binary DESTINATION SUBSYSTEM compiles the program, in the "windows"
# subsystem (the window) or "console". -trimpath, an empty build id and the
# absence of VCS information make the binary independent of the folder and
# of the time of the build. The version number is written into the program
# (imageprintlayout -version).
build_binary() {
    local destination="$1" subsystem="$2" flags="-s -w -buildid= -X main.version=$VERSION" expected
    if [ "$subsystem" = "windows" ]; then
        flags="$flags -H windowsgui"
        expected="Windows GUI"
    else
        expected="Windows CUI"
    fi
    step "Compiling $(basename "$destination") $VERSION (cgo, GTK4, librsvg, gdk-pixbuf, cairo; $subsystem subsystem)..."
    if [ "$subsystem" = "windows" ]; then
        echo "    The first compilation of GTK (gotk4) takes several minutes and prints nothing;"
        echo "    the following ones are almost instant (Go cache)."
    fi
    mkdir -p "$(dirname "$destination")"
    # gotk4 declares "free" differently from <stdlib.h>: gcc reports it at every
    # compilation. It is not a defect of this repository; only this warning is
    # hidden, and cgo's default options are kept.
    ( cd "$PROJECT_DIR" && CGO_ENABLED=1 CGO_CFLAGS="${CGO_CFLAGS:--O2 -g} -Wno-builtin-declaration-mismatch" go build -trimpath -buildvcs=false \
        -buildmode=exe -ldflags "$flags" -o "$destination" ./cmd/imageprintlayout )
    # A windowed executable started from a terminal writes nowhere, a console
    # executable opens a black window behind the window: we check that each
    # has the wanted subsystem.
    objdump -p "$destination" | grep -E '^Subsystem' | grep -q "$expected" \
        || die "$(basename "$destination") does not have the '$subsystem' subsystem: $(objdump -p "$destination" | grep -E '^Subsystem')"
}

# normalize_tree ROOT sets the date of the whole content.
normalize_tree() {
    find "${1:?normalize_tree: missing folder}" -exec touch -h -d "@$SOURCE_DATE_EPOCH" {} +
}

# find_makensis prints the path of makensis (NSIS), if it is installed.
find_makensis() {
    local candidate
    for candidate in \
        "${NSIS_HOME:+$NSIS_HOME/makensis.exe}" \
        "$(cygpath -u "${PROGRAMFILES:-C:\\Program Files} (x86)" 2>/dev/null)/NSIS/makensis.exe" \
        "$(cygpath -u "${PROGRAMFILES:-C:\\Program Files}" 2>/dev/null)/NSIS/makensis.exe"; do
        if [ -n "$candidate" ] && [ -f "$candidate" ]; then
            echo "$candidate"
            return 0
        fi
    done
    command -v makensis 2>/dev/null || command -v makensis.exe 2>/dev/null || true
}

# make_resources FOLDER writes, in the program's folder, the resource that
# gives it its icon and its properties (right click > Properties), in English
# and without accents: windres reads this file in the Windows code page, not
# in UTF-8. Go links it
# by itself: any .syso file in the folder of a package is added to the
# link. It is deleted when the script exits.
make_resources() {
    local cmd_dir="$1"
    local rc_dir="$WORK/resources"
    local file_version="${VERSION_BASE//./,},0"
    mkdir -p "$rc_dir"
    cp "$WINDOWS_DIR/imageprintlayout.ico" "$rc_dir/"
    cat > "$rc_dir/imageprintlayout.rc" << EOF2
1 ICON "imageprintlayout.ico"
1 VERSIONINFO
FILEVERSION $file_version
PRODUCTVERSION $file_version
FILEOS 0x4
FILETYPE 0x1
BEGIN
  BLOCK "StringFileInfo"
  BEGIN
    BLOCK "040904b0"
    BEGIN
      VALUE "ProductName", "Image Print Layout"
      VALUE "FileDescription", "Layout and printing of SVG drawings"
      VALUE "FileVersion", "$VERSION"
      VALUE "ProductVersion", "$VERSION"
      VALUE "CompanyName", "Gilles MAGNEVILLE"
      VALUE "LegalCopyright", "Gilles MAGNEVILLE - GNU GPL v3"
      VALUE "OriginalFilename", "imageprintlayout.exe"
    END
  END
  BLOCK "VarFileInfo"
  BEGIN
    VALUE "Translation", 0x0409, 1200
  END
END
EOF2
    SYSO="$cmd_dir/rsrc_windows_amd64.syso"
    ( cd "$rc_dir" && windres -O coff -i imageprintlayout.rc -o "$SYSO" )
    trap 'rm -f "$SYSO"' EXIT
}

# dll_closure BINARY... prints, one per line, the MSYS2 DLLs that the
# binaries depend on, directly or not. The Windows DLLs (kernel32,
# ucrtbase...) are not part of it: they are on every machine.
# Each DLL found is examined in turn, until there is nothing new
# left: we do not assume that ldd gives the indirect dependencies.
dll_closure() {
    local -A seen=()
    local queue=("$@") item listing dll
    while [ ${#queue[@]} -gt 0 ]; do
        item="${queue[0]}"
        queue=("${queue[@]:1}")
        listing="$(ldd "$item")"
        if grep -q "not found" <<< "$listing"; then
            die "libraries not found for $item:
$(grep 'not found' <<< "$listing")"
        fi
        while IFS= read -r dll; do
            if [ -n "$dll" ] && [ -z "${seen[$dll]:-}" ]; then
                seen[$dll]=1
                queue+=("$dll")
            fi
        done < <(awk -v prefix="$MINGW_PREFIX/" '$2 == "=>" && index($3, prefix) == 1 { print $3 }' <<< "$listing")
    done
    printf '%s\n' "${!seen[@]}" | LC_ALL=C sort
}

# stage_windows ROOT GUI_EXE CONSOLE_EXE prepares the "prefix-like" tree
# that the zip and the installer reproduce identically
# (GLib and GTK find their files from the location of their DLLs,
# which must therefore be in a bin/ folder next to share/):
#   bin/        the two executables and the DLLs
#   etc/fonts/  the fontconfig configuration
#   lib/gdk-pixbuf-2.0/  the image loaders, with their cache
#   share/glib-2.0/schemas/  the GTK settings schemas (without them, GTK
#                            stops when a file chooser opens)
#   share/icons/             the icon themes (Adwaita and hicolor)
#   share/locale/            the translations of GTK's own texts
#   share/licenses/          the licenses of the shipped libraries
stage_windows() {
    local root="$1" gui="$2" console="$3" loaders=() loader dll name lang catalog
    rm -rf "${root:?}"
    install -d "$root/bin" "$root/etc" "$root/share/licenses"
    install -m 0755 "$gui" "$root/bin/$GUI_EXE"
    install -m 0755 "$console" "$root/bin/$CONSOLE_EXE"

    step "Image loaders (gdk-pixbuf)..."
    local loader_src="$MINGW_PREFIX/lib/gdk-pixbuf-2.0/2.10.0/loaders"
    local loader_dst="$root/lib/gdk-pixbuf-2.0/2.10.0/loaders"
    for name in "${PIXBUF_REQUIRED[@]}" "${PIXBUF_OPTIONAL[@]}"; do
        if [ -f "$loader_src/libpixbufloader-$name.dll" ]; then
            install -D -m 0755 "$loader_src/libpixbufloader-$name.dll" "$loader_dst/libpixbufloader-$name.dll"
            loaders+=("$loader_src/libpixbufloader-$name.dll")
        elif [[ " ${PIXBUF_REQUIRED[*]} " == *" $name "* ]]; then
            die "the gdk-pixbuf image loader '$name' is missing ($loader_src): the program would not be able to open this format. Install the MSYS2 package mingw-w64-ucrt-x86_64-gdk-pixbuf2."
        else
            echo "    (no '$name' loader in this MSYS2 installation: this format will not be read)"
        fi
    done

    step "Libraries the program depends on..."
    local dlls=()
    while IFS= read -r dll; do dlls+=("$dll"); done < <(dll_closure "$gui" "$console" "${loaders[@]}")
    [ ${#dlls[@]} -gt 0 ] || die "no DLL found for $gui: ldd does not see the MSYS2 libraries (UCRT64 terminal?)."
    for dll in "${dlls[@]}"; do
        install -m 0755 "$dll" "$root/bin/"
    done
    echo "    ${#dlls[@]} DLLs copied into bin/"

    # The cache lists the loaders with their path relative to the prefix: the
    # folder can thus be installed anywhere.
    if [ ${#loaders[@]} -gt 0 ]; then
        local cache="$root/lib/gdk-pixbuf-2.0/2.10.0/loaders.cache"
        local staged=()
        for loader in "${loaders[@]}"; do staged+=("$(cygpath -w "$loader_dst/$(basename "$loader")")"); done
        gdk-pixbuf-query-loaders "${staged[@]}" \
            | sed -E 's#^"[^"]*[/\\]+(lib[/\\]+gdk-pixbuf-2\.0)#"\1#; /^"lib/ s#/#\\\\#g' \
            > "$cache"
        if grep -qE '^"[A-Za-z]:|^"/' "$cache"; then
            die "the image loader cache contains an absolute path: $(grep -E '^"[A-Za-z]:|^"/' "$cache" | head -1)"
        fi
        echo "    image loader cache: $(grep -c '^"lib' "$cache") loader(s)"
    fi

    step "Configuring fontconfig..."
    cp -rL "$MINGW_PREFIX/etc/fonts" "$root/etc/fonts"

    step "GTK settings, icons and translations..."
    local schemas="$MINGW_PREFIX/share/glib-2.0/schemas/gschemas.compiled"
    [ -f "$schemas" ] || die "the GTK schemas were not found: $schemas"
    install -D -m 0644 "$schemas" "$root/share/glib-2.0/schemas/gschemas.compiled"
    local theme
    for theme in Adwaita hicolor; do
        [ -d "$MINGW_PREFIX/share/icons/$theme" ] || die "the $theme icon theme was not found in $MINGW_PREFIX/share/icons."
        mkdir -p "$root/share/icons"
        cp -rL "$MINGW_PREFIX/share/icons/$theme" "$root/share/icons/$theme"
    done
    for lang in "${GTK_LOCALES[@]}"; do
        for catalog in gtk40 glib20; do
            if [ -f "$MINGW_PREFIX/share/locale/$lang/LC_MESSAGES/$catalog.mo" ]; then
                install -D -m 0644 "$MINGW_PREFIX/share/locale/$lang/LC_MESSAGES/$catalog.mo" \
                    "$root/share/locale/$lang/LC_MESSAGES/$catalog.mo"
            else
                echo "    (no '$lang' translation of $catalog in this MSYS2 installation)"
            fi
        done
    done

    step "Licenses of the shipped libraries..."
    write_licenses "$root" "${dlls[@]}" "${loaders[@]}" "$MINGW_PREFIX/share/icons/Adwaita/index.theme"

    install -m 0644 "$PROJECT_DIR/LICENSE" "$root/LICENSE"
    install -m 0644 "$PROJECT_DIR/README.md" "$root/README.md"
    install -m 0644 "$PROJECT_DIR/README.fr.md" "$root/README.fr.md"
    install -m 0644 "$PROJECT_DIR/CHANGELOG.md" "$root/CHANGELOG.md"
    install -m 0644 "$PROJECT_DIR/CHANGELOG.fr.md" "$root/CHANGELOG.fr.md"
    install -m 0644 "$PROJECT_DIR/CODE_OF_CONDUCT.md" "$root/CODE_OF_CONDUCT.md"
    install -m 0644 "$PROJECT_DIR/CODE_OF_CONDUCT.fr.md" "$root/CODE_OF_CONDUCT.fr.md"
}

# write_licenses ROOT FILE...: for each MSYS2 package that one of the files comes from,
# copies its license texts and lists it in COMPONENTS.txt.
write_licenses() {
    local root="$1"; shift
    local owners pkg short list="$root/share/licenses/COMPONENTS.txt"
    # pacman fails if one of the files has no owner: harmless.
    owners="$(pacman -Qo "$@" 2>/dev/null | sed -E 's/.* is owned by ([^ ]+) .*/\1/' | sort -u || true)"
    {
        # The byte order mark (BOM) lets all the Windows tools (old Notepad,
        # PowerShell 5.1) read the file as UTF-8.
        printf '\xef\xbb\xbf'
        echo "Free libraries shipped with Image Print Layout"
        echo "=============================================="
        echo ""
        echo "Each library remains subject to its own license, the text of which is"
        echo "in the folder of the same name. Their source code is available from the"
        echo "MSYS2 project: https://packages.msys2.org/ (\"Sources\" section)."
        echo ""
    } > "$list"
    for pkg in $owners; do
        short="${pkg#mingw-w64-ucrt-x86_64-}"
        if [ -d "$MINGW_PREFIX/share/licenses/$short" ]; then
            mkdir -p "$root/share/licenses/$short"
            cp -rL "$MINGW_PREFIX/share/licenses/$short/." "$root/share/licenses/$short/"
        fi
        # "Licenses : spdx:LGPL-2.1-or-later": we keep everything after the first ":".
        printf '%-26s %-34s %s\n' "$short" \
            "$(pacman -Qi "$pkg" | awk '/^Version/ { sub(/^[^:]*: */, ""); print; exit }')" \
            "$(pacman -Qi "$pkg" | awk '/^Licenses/ { sub(/^[^:]*: */, ""); print; exit }')" >> "$list"
    done
    echo "    $(echo "$owners" | wc -w) libraries described in share/licenses/COMPONENTS.txt"
}

# write_installer_includes FOLDER ROOT OUTPUT writes the two files that
# installer.nsi includes: the values of this build, and the list of the
# files to delete at uninstall. This list prevents an uninstaller from
# erasing a whole folder, in case the user installed the program in a
# folder that contains other files.
write_installer_includes() {
    local dir="$1" root="$2" output="$3" win_root rel
    win_root="$(cygpath -w "$root")"
    mkdir -p "$dir"
    {
        echo "!define APP_VERSION \"$VERSION\""
        echo "!define APP_VERSION_NUMERIC \"$VERSION_BASE.0\""
        echo "!define STAGE_DIR \"$win_root\""
        echo "!define OUTPUT_FILE \"$(cygpath -w "$output")\""
        echo "!define APP_ICON \"$(cygpath -w "$WINDOWS_DIR/imageprintlayout.ico")\""
    } > "$dir/build-vars.nsh"
    {
        ( cd "$root" && find . -type f | LC_ALL=C sort ) | while IFS= read -r rel; do
            rel="${rel#./}"
            echo "Delete \"\$INSTDIR\\${rel//\//\\}\""
        done
        ( cd "$root" && find . -mindepth 1 -type d | LC_ALL=C sort -r ) | while IFS= read -r rel; do
            rel="${rel#./}"
            echo "RMDir \"\$INSTDIR\\${rel//\//\\}\""
        done
    } > "$dir/uninstall-files.nsh"
}

build_installer() {
    local makensis="$1" root="$2" output="$3" dir="$WORK/installer"
    write_installer_includes "$dir" "$root" "$output"
    cp "$WINDOWS_DIR/installer.nsi" "$dir/installer.nsi"
    rm -f "${output:?}"
    # "-V2" and not "/V2": MSYS2 would take "/V2" for a POSIX path.
    ( cd "$dir" && "$makensis" -V2 installer.nsi )
    [ -f "$output" ] || die "NSIS did not produce $output."
}

main() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --version)
                [ $# -ge 2 ] || die "--version expects a version number (e.g. --version 1.2.3)."
                VERSION="$2"
                shift 2
                ;;
            --install-deps) INSTALL_DEPS=true; shift ;;
            --check-deps)   CHECK_DEPS=true; shift ;;
            --help|-h)      show_help; exit 0 ;;
            *)
                echo "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done

    [ "${MSYSTEM:-}" = "UCRT64" ] || die "this script must run in an MSYS2 UCRT64 terminal (MSYSTEM is '${MSYSTEM:-empty}').
Run .\\build-windows.ps1 from PowerShell instead, at the root of the repository."
    [ "$(uname -m)" = "x86_64" ] || die "only 64-bit Windows (x64) is supported."
    WORK="$(cygpath -u "${SVGPL_BUILD_DIR:-$HOME/imageprintlayout-build}")"

    ensure_packages
    [ "$CHECK_DEPS" = true ] && exit 0

    # MSYS2's Go package is built with -trimpath: its program cannot find
    # its installation folder by itself. We give it, unless
    # the user has already chosen.
    if [ -z "${GOROOT:-}" ] && [ -d "$MINGW_PREFIX/lib/go" ]; then
        GOROOT="$(cygpath -m "$MINGW_PREFIX/lib/go")"
        export GOROOT
    fi

    read_version
    set_source_date_epoch
    check_go_prerequisites
    step "Go: $(go version)"
    require_tool windres "It is part of the MSYS2 package mingw-w64-ucrt-x86_64-binutils (installed with gcc)."
    require_tool ldd "It is part of MSYS2 (package msys2-runtime)."
    require_tool zip "pacman -S zip"

    local pkgdir="${PACKAGE_NAME}-${VERSION}-windows-x64"
    local gui="$WORK/$GUI_EXE" console="$WORK/$CONSOLE_EXE"
    local root="$WORK/$pkgdir"
    local zip_out="$PROJECT_DIR/${PACKAGE_NAME}_${VERSION}_windows-portable.zip"
    local setup_out="$PROJECT_DIR/${PACKAGE_NAME}_${VERSION}_windows-setup.exe"

    echo ""
    echo "========================================"
    echo " Building $PACKAGE_NAME $VERSION (Windows x64)"
    echo "========================================"

    rm -rf "${WORK:?}"
    mkdir -p "$WORK"
    make_resources "$PROJECT_DIR/cmd/$PACKAGE_NAME"
    build_binary "$gui" windows
    build_binary "$console" console
    rm -f "$SYSO"

    step "Preparing the package tree..."
    stage_windows "$root" "$gui" "$console"
    normalize_tree "$root"

    # The package must work on a machine where MSYS2 is not installed: we
    # run it with a PATH reduced to that of Windows, its DLLs must be enough.
    step "Checking the package (self-test with the Windows PATH only)..."
    local winroot
    winroot="$(cygpath -u "${SYSTEMROOT:-${SystemRoot:-C:\\Windows}}")"
    env PATH="$winroot/System32:$winroot" "$root/bin/$CONSOLE_EXE" -selftest \
        || die "the program's self-test fails outside MSYS2: the package is incomplete (missing DLLs, fonts or image loaders)."

    step "Creating the portable version (.zip)..."
    rm -f "${zip_out:?}"
    ( cd "$WORK" && find "$pkgdir" | LC_ALL=C sort | TZ=UTC zip -X -9 -q "$zip_out" -@ )

    local built="$zip_out"
    local makensis
    makensis="$(find_makensis)"
    if [ -n "$makensis" ]; then
        step "Creating the installer (.exe) with NSIS..."
        build_installer "$makensis" "$root" "$setup_out"
        built="$built
   $setup_out"
    else
        echo ""
        echo "NSIS not found: only the portable version (.zip) is produced."
        echo "To get the installer too: winget install NSIS.NSIS, then run again."
    fi

    echo ""
    echo "✅ Package(s) generated:"
    echo "   $built"
}

# The script can be "sourced" (to test its functions) without running anything.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi

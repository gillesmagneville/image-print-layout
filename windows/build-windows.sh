#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
set -euo pipefail

# Construit le paquet Windows de SVG Print Layout : une version portable (.zip)
# et, si NSIS est installé, un installeur (.exe), à la racine du dépôt.
#
# Ce script s'exécute dans un terminal MSYS2 UCRT64 (c'est là que se trouvent
# Go, le compilateur C, GTK 4, librsvg et cairo pour Windows). On le lance de
# préférence par build-windows.ps1, à la racine du dépôt : celui-ci gère les
# options et le fichier VERSION comme build-deb.sh, prépare MSYS2 et NSIS, puis
# appelle ce script avec le numéro de version choisi. Ce script ne modifie
# jamais le fichier VERSION.

# ====================== CONFIGURATION ======================
PACKAGE_NAME="svgprintlayout"
WINDOWS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$WINDOWS_DIR")"
# Dossier de travail : celui que build-windows.ps1 choisit (variable
# SVGPL_BUILD_DIR, en chemin Windows ou MSYS2), à défaut un dossier du
# répertoire personnel. Fixé par main().
WORK=""
# ===========================================================

step() { echo ">>> $*"; }

die() {
    echo "" >&2
    echo "Erreur : $*" >&2
    exit 1
}

# require_tool COMMANDE "conseil d'installation"
require_tool() {
    command -v "$1" >/dev/null 2>&1 || die "$1 est introuvable. $2"
}

# Paquets MSYS2 nécessaires à la construction (les bibliothèques d'exécution
# viennent avec GTK 4, librsvg et cairo). Les thèmes d'icônes sont livrés avec
# le programme : sans eux, les boutons de la fenêtre n'ont pas d'image.
MSYS_PACKAGES=(
    mingw-w64-ucrt-x86_64-go
    mingw-w64-ucrt-x86_64-gcc
    mingw-w64-ucrt-x86_64-pkgconf
    mingw-w64-ucrt-x86_64-librsvg
    mingw-w64-ucrt-x86_64-cairo
    mingw-w64-ucrt-x86_64-fontconfig
    mingw-w64-ucrt-x86_64-gtk4
    mingw-w64-ucrt-x86_64-gobject-introspection
    mingw-w64-ucrt-x86_64-adwaita-icon-theme
    mingw-w64-ucrt-x86_64-hicolor-icon-theme
    zip
    git
)

# Deux exécutables : svgprintlayout.exe, celui de la fenêtre (sous-système
# Windows : pas de console qui s'ouvre derrière), et svgprintlayout-console.exe,
# le même programme en sous-système console, pour -version, -selftest et les
# messages d'erreur dans un terminal (un programme fenêtré n'écrit nulle part
# dans un terminal).
GUI_EXE="$PACKAGE_NAME.exe"
CONSOLE_EXE="$PACKAGE_NAME-console.exe"

# Langues de GTK livrées avec le programme (le texte de ses propres boutons) ;
# l'anglais est dans le code de GTK.
GTK_LOCALES=(fr es de)

# Formats d'image que les chargeurs gdk-pixbuf livrés savent lire : ceux des
# images incorporées aux SVG (Inkscape produit surtout du PNG et du JPEG).
PIXBUF_LOADERS=(png jpeg gif bmp webp svg)

show_help() {
    cat << EOF2
Usage : windows/build-windows.sh [options]

Construit, à la racine du dépôt :
  svgprintlayout_<version>_windows-portable.zip   version portable
  svgprintlayout_<version>_windows-setup.exe      installeur (si NSIS est là)

OPTIONS :
  --version X.Y.Z   Numéro de version à construire (par défaut : le contenu du
                    fichier VERSION). Ce script ne modifie jamais VERSION.
  --install-deps    Installe avec pacman les paquets MSYS2 manquants
  --check-deps      Vérifie seulement que les paquets MSYS2 sont présents
  --help, -h        Affiche cette aide

À lancer dans un terminal « MSYS2 UCRT64 » (pas MSYS ni MINGW64). Plus simple :
.\\build-windows.ps1 à la racine du dépôt, depuis PowerShell : il accepte
-Major, -Minor, -Patch et -NoVersionChange, comme build-deb.sh.

Le dossier de travail est la variable d'environnement SVGPL_BUILD_DIR
(par défaut ~/svgprintlayout-build).
EOF2
}

INSTALL_DEPS=false
CHECK_DEPS=false
VERSION=""

# ensure_packages vérifie les paquets MSYS2 ; avec --install-deps, il installe
# ceux qui manquent, avec --check-deps il se contente de les lister.
ensure_packages() {
    local missing=() p
    for p in "${MSYS_PACKAGES[@]}"; do
        pacman -Q "$p" >/dev/null 2>&1 || missing+=("$p")
    done
    [ ${#missing[@]} -eq 0 ] && return 0
    if [ "$CHECK_DEPS" = true ]; then
        echo "Paquets MSYS2 manquants :" >&2
        printf '  %s\n' "${missing[@]}" >&2
        return 1
    fi
    if [ "$INSTALL_DEPS" != true ]; then
        die "paquets MSYS2 manquants :
$(printf '  %s\n' "${missing[@]}")
Installez-les avec : pacman -S --needed ${missing[*]}
ou relancez ce script avec --install-deps."
    fi
    step "Installation des paquets MSYS2 manquants..."
    pacman -S --needed --noconfirm "${missing[@]}"
}

# Date des fichiers du paquet : celle du dernier commit, pour que deux
# constructions du même code donnent le même paquet.
set_source_date_epoch() {
    if [ -z "${SOURCE_DATE_EPOCH:-}" ]; then
        SOURCE_DATE_EPOCH="$(git -c safe.directory="$PROJECT_DIR" -C "$PROJECT_DIR" log -1 --format=%ct 2>/dev/null || true)"
        SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(date +%s)}"
    fi
    export SOURCE_DATE_EPOCH
}

# read_version fixe VERSION (option --version, à défaut fichier VERSION) et
# VERSION_BASE, sa partie numérique : une version d'avant-publication
# (0.1.0-dev) garde son suffixe dans les textes, mais les numéros de version
# de Windows n'ont que quatre nombres.
read_version() {
    if [ -z "$VERSION" ]; then
        [ -f "$PROJECT_DIR/VERSION" ] || die "fichier VERSION introuvable : $PROJECT_DIR/VERSION"
        VERSION="$(tr -d '[:space:]' < "$PROJECT_DIR/VERSION")"
    fi
    local semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'
    [[ "$VERSION" =~ $semver ]] || die "« $VERSION » n'est pas une version X.Y.Z ou X.Y.Z-suffixe."
    VERSION_BASE="${VERSION%%-*}"
}

check_go_prerequisites() {
    require_tool go "Installez le paquet MSYS2 mingw-w64-ucrt-x86_64-go."
    require_tool gcc "Installez le paquet MSYS2 mingw-w64-ucrt-x86_64-gcc."
    require_tool pkg-config "Installez le paquet MSYS2 mingw-w64-ucrt-x86_64-pkgconf."
    pkg-config --exists librsvg-2.0 cairo fontconfig || die "les bibliothèques de développement de librsvg, de cairo et de fontconfig sont absentes (paquets MSYS2 mingw-w64-ucrt-x86_64-librsvg, -cairo et -fontconfig)."
    pkg-config --exists gtk4 gobject-introspection-1.0 || die "les bibliothèques de développement de GTK 4 sont absentes (paquets MSYS2 mingw-w64-ucrt-x86_64-gtk4 et -gobject-introspection)."
}

# build_binary DESTINATION SOUS-SYSTÈME compile le programme, en sous-système
# « windows » (la fenêtre) ou « console ». -trimpath, un identifiant de
# construction vide et l'absence d'informations VCS rendent le binaire
# indépendant du dossier et de l'heure de la construction. Le numéro de
# version est inscrit dans le programme (svgprintlayout -version).
build_binary() {
    local destination="$1" subsystem="$2" flags="-s -w -buildid= -X main.version=$VERSION" expected
    if [ "$subsystem" = "windows" ]; then
        flags="$flags -H windowsgui"
        expected="Windows GUI"
    else
        expected="Windows CUI"
    fi
    step "Compilation de $(basename "$destination") $VERSION (cgo, GTK4, librsvg, cairo ; sous-système $subsystem)..."
    if [ "$subsystem" = "windows" ]; then
        echo "    La première compilation de GTK (gotk4) dure plusieurs minutes et n'affiche rien ;"
        echo "    les suivantes sont presque instantanées (cache de Go)."
    fi
    mkdir -p "$(dirname "$destination")"
    # gotk4 déclare « free » autrement que <stdlib.h> : gcc le signale à chaque
    # compilation. Ce n'est pas un défaut de ce dépôt ; on masque cet
    # avertissement seulement, et on garde les options par défaut de cgo.
    ( cd "$PROJECT_DIR" && CGO_ENABLED=1 CGO_CFLAGS="${CGO_CFLAGS:--O2 -g} -Wno-builtin-declaration-mismatch" go build -trimpath -buildvcs=false \
        -buildmode=exe -ldflags "$flags" -o "$destination" ./cmd/svgprintlayout )
    # Un exécutable fenêtré lancé depuis un terminal n'écrit nulle part, un
    # exécutable console ouvre une fenêtre noire derrière la fenêtre : on
    # vérifie que chacun a le sous-système voulu.
    objdump -p "$destination" | grep -E '^Subsystem' | grep -q "$expected" \
        || die "$(basename "$destination") n'a pas le sous-système « $subsystem » : $(objdump -p "$destination" | grep -E '^Subsystem')"
}

# normalize_tree RACINE fixe la date de tout le contenu.
normalize_tree() {
    find "${1:?normalize_tree : dossier manquant}" -exec touch -h -d "@$SOURCE_DATE_EPOCH" {} +
}

# find_makensis affiche le chemin de makensis (NSIS), s'il est installé.
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

# make_resources DOSSIER écrit dans le dossier du programme la ressource qui
# lui donne son icône et ses propriétés (clic droit > Propriétés), en anglais
# et sans accent : windres lit ce fichier dans la page de code de Windows, pas
# en UTF-8. Go la lie
# toute seule : tout fichier .syso du dossier d'un paquet est ajouté à l'édition
# de liens. Elle est supprimée à la sortie du script.
make_resources() {
    local cmd_dir="$1"
    local rc_dir="$WORK/resources"
    local file_version="${VERSION_BASE//./,},0"
    mkdir -p "$rc_dir"
    cp "$WINDOWS_DIR/svgprintlayout.ico" "$rc_dir/"
    cat > "$rc_dir/svgprintlayout.rc" << EOF2
1 ICON "svgprintlayout.ico"
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
      VALUE "ProductName", "SVG Print Layout"
      VALUE "FileDescription", "Layout and printing of SVG drawings"
      VALUE "FileVersion", "$VERSION"
      VALUE "ProductVersion", "$VERSION"
      VALUE "CompanyName", "Gilles MAGNEVILLE"
      VALUE "LegalCopyright", "Gilles MAGNEVILLE - GNU GPL v3"
      VALUE "OriginalFilename", "svgprintlayout.exe"
    END
  END
  BLOCK "VarFileInfo"
  BEGIN
    VALUE "Translation", 0x0409, 1200
  END
END
EOF2
    SYSO="$cmd_dir/rsrc_windows_amd64.syso"
    ( cd "$rc_dir" && windres -O coff -i svgprintlayout.rc -o "$SYSO" )
    trap 'rm -f "$SYSO"' EXIT
}

# dll_closure BINAIRE... affiche, une par ligne, les DLL de MSYS2 dont les
# binaires dépendent, directement ou non. Les DLL de Windows (kernel32,
# ucrtbase...) n'en font pas partie : elles sont sur toutes les machines.
# Chaque DLL trouvée est examinée à son tour, jusqu'à ce qu'il n'y ait plus
# rien de nouveau : on ne suppose pas que ldd donne les dépendances indirectes.
dll_closure() {
    local -A seen=()
    local queue=("$@") item listing dll
    while [ ${#queue[@]} -gt 0 ]; do
        item="${queue[0]}"
        queue=("${queue[@]:1}")
        listing="$(ldd "$item")"
        if grep -q "not found" <<< "$listing"; then
            die "bibliothèques introuvables pour $item :
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

# stage_windows RACINE EXE_FENÊTRE EXE_CONSOLE prépare l'arborescence « à la
# manière d'un préfixe » que le zip et l'installeur reproduisent à l'identique
# (GLib et GTK retrouvent leurs fichiers à partir de l'emplacement de leurs DLL,
# qui doivent donc être dans un dossier bin/ à côté de share/) :
#   bin/        les deux exécutables et les DLL
#   etc/fonts/  la configuration de fontconfig
#   lib/gdk-pixbuf-2.0/  les chargeurs d'images, avec leur cache
#   share/glib-2.0/schemas/  les schémas de réglages de GTK (sans eux, GTK
#                            s'arrête à l'ouverture d'un sélecteur de fichier)
#   share/icons/             les thèmes d'icônes (Adwaita et hicolor)
#   share/locale/            les traductions des textes propres à GTK
#   share/licenses/          les licences des bibliothèques livrées
stage_windows() {
    local root="$1" gui="$2" console="$3" loaders=() loader dll name lang catalog
    rm -rf "${root:?}"
    install -d "$root/bin" "$root/etc" "$root/share/licenses"
    install -m 0755 "$gui" "$root/bin/$GUI_EXE"
    install -m 0755 "$console" "$root/bin/$CONSOLE_EXE"

    step "Chargeurs d'images (gdk-pixbuf)..."
    local loader_src="$MINGW_PREFIX/lib/gdk-pixbuf-2.0/2.10.0/loaders"
    local loader_dst="$root/lib/gdk-pixbuf-2.0/2.10.0/loaders"
    for name in "${PIXBUF_LOADERS[@]}"; do
        if [ -f "$loader_src/libpixbufloader-$name.dll" ]; then
            install -D -m 0755 "$loader_src/libpixbufloader-$name.dll" "$loader_dst/libpixbufloader-$name.dll"
            loaders+=("$loader_src/libpixbufloader-$name.dll")
        else
            echo "    (pas de chargeur « $name » dans cette installation de MSYS2)"
        fi
    done

    step "Bibliothèques dont le programme dépend..."
    local dlls=()
    while IFS= read -r dll; do dlls+=("$dll"); done < <(dll_closure "$gui" "$console" "${loaders[@]}")
    [ ${#dlls[@]} -gt 0 ] || die "aucune DLL trouvée pour $gui : ldd ne voit pas les bibliothèques de MSYS2 (terminal UCRT64 ?)."
    for dll in "${dlls[@]}"; do
        install -m 0755 "$dll" "$root/bin/"
    done
    echo "    ${#dlls[@]} DLL copiées dans bin/"

    # Le cache liste les chargeurs avec leur chemin relatif au préfixe : le
    # dossier peut ainsi être installé n'importe où.
    if [ ${#loaders[@]} -gt 0 ]; then
        local cache="$root/lib/gdk-pixbuf-2.0/2.10.0/loaders.cache"
        local staged=()
        for loader in "${loaders[@]}"; do staged+=("$(cygpath -w "$loader_dst/$(basename "$loader")")"); done
        gdk-pixbuf-query-loaders "${staged[@]}" \
            | sed -E 's#^"[^"]*[/\\]+(lib[/\\]+gdk-pixbuf-2\.0)#"\1#; /^"lib/ s#/#\\\\#g' \
            > "$cache"
        if grep -qE '^"[A-Za-z]:|^"/' "$cache"; then
            die "le cache des chargeurs d'images contient un chemin absolu : $(grep -E '^"[A-Za-z]:|^"/' "$cache" | head -1)"
        fi
        echo "    cache des chargeurs d'images : $(grep -c '^"lib' "$cache") chargeur(s)"
    fi

    step "Configuration de fontconfig..."
    cp -rL "$MINGW_PREFIX/etc/fonts" "$root/etc/fonts"

    step "Réglages, icônes et traductions de GTK..."
    local schemas="$MINGW_PREFIX/share/glib-2.0/schemas/gschemas.compiled"
    [ -f "$schemas" ] || die "les schémas de GTK sont introuvables : $schemas"
    install -D -m 0644 "$schemas" "$root/share/glib-2.0/schemas/gschemas.compiled"
    local theme
    for theme in Adwaita hicolor; do
        [ -d "$MINGW_PREFIX/share/icons/$theme" ] || die "le thème d'icônes $theme est introuvable dans $MINGW_PREFIX/share/icons."
        mkdir -p "$root/share/icons"
        cp -rL "$MINGW_PREFIX/share/icons/$theme" "$root/share/icons/$theme"
    done
    for lang in "${GTK_LOCALES[@]}"; do
        for catalog in gtk40 glib20; do
            if [ -f "$MINGW_PREFIX/share/locale/$lang/LC_MESSAGES/$catalog.mo" ]; then
                install -D -m 0644 "$MINGW_PREFIX/share/locale/$lang/LC_MESSAGES/$catalog.mo" \
                    "$root/share/locale/$lang/LC_MESSAGES/$catalog.mo"
            else
                echo "    (pas de traduction « $lang » de $catalog dans cette installation de MSYS2)"
            fi
        done
    done

    step "Licences des bibliothèques livrées..."
    write_licenses "$root" "${dlls[@]}" "${loaders[@]}" "$MINGW_PREFIX/share/icons/Adwaita/index.theme"

    install -m 0644 "$PROJECT_DIR/LICENSE" "$root/LICENSE"
    install -m 0644 "$PROJECT_DIR/README.md" "$root/README.md"
    install -m 0644 "$PROJECT_DIR/CHANGELOG.md" "$root/CHANGELOG.md"
}

# write_licenses RACINE FICHIER... : pour chaque paquet MSYS2 dont vient un des
# fichiers, copie ses textes de licence et le range dans COMPOSANTS.txt.
write_licenses() {
    local root="$1"; shift
    local owners pkg short list="$root/share/licenses/COMPOSANTS.txt"
    # pacman sort en erreur si l'un des fichiers n'a pas de propriétaire : sans gravité.
    owners="$(pacman -Qo "$@" 2>/dev/null | sed -E 's/.* is owned by ([^ ]+) .*/\1/' | sort -u || true)"
    {
        # La marque d'ordre des octets (BOM) permet à tous les outils de Windows
        # (Bloc-notes ancien, PowerShell 5.1) de lire les accents.
        printf '\xef\xbb\xbf'
        echo "Bibliothèques libres livrées avec SVG Print Layout"
        echo "=================================================="
        echo ""
        echo "Chaque bibliothèque reste soumise à sa propre licence, dont le texte est"
        echo "dans le dossier de même nom. Leur code source est disponible auprès du"
        echo "projet MSYS2 : https://packages.msys2.org/ (rubrique « Sources »)."
        echo ""
    } > "$list"
    for pkg in $owners; do
        short="${pkg#mingw-w64-ucrt-x86_64-}"
        if [ -d "$MINGW_PREFIX/share/licenses/$short" ]; then
            mkdir -p "$root/share/licenses/$short"
            cp -rL "$MINGW_PREFIX/share/licenses/$short/." "$root/share/licenses/$short/"
        fi
        # « Licenses : spdx:LGPL-2.1-or-later » : on garde tout ce qui suit le premier « : ».
        printf '%-26s %-34s %s\n' "$short" \
            "$(pacman -Qi "$pkg" | awk '/^Version/ { sub(/^[^:]*: */, ""); print; exit }')" \
            "$(pacman -Qi "$pkg" | awk '/^Licenses/ { sub(/^[^:]*: */, ""); print; exit }')" >> "$list"
    done
    echo "    $(echo "$owners" | wc -w) bibliothèques décrites dans share/licenses/COMPOSANTS.txt"
}

# write_installer_includes DOSSIER RACINE SORTIE écrit les deux fichiers que
# installer.nsi inclut : les valeurs de cette construction, et la liste des
# fichiers à supprimer à la désinstallation. Cette liste évite qu'un
# désinstalleur n'efface un dossier entier, au cas où l'utilisateur aurait
# installé le programme dans un dossier qui contient d'autres fichiers.
write_installer_includes() {
    local dir="$1" root="$2" output="$3" win_root rel
    win_root="$(cygpath -w "$root")"
    mkdir -p "$dir"
    {
        echo "!define APP_VERSION \"$VERSION\""
        echo "!define APP_VERSION_NUMERIC \"$VERSION_BASE.0\""
        echo "!define STAGE_DIR \"$win_root\""
        echo "!define OUTPUT_FILE \"$(cygpath -w "$output")\""
        echo "!define APP_ICON \"$(cygpath -w "$WINDOWS_DIR/svgprintlayout.ico")\""
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
    # « -V2 » et non « /V2 » : MSYS2 prendrait « /V2 » pour un chemin POSIX.
    ( cd "$dir" && "$makensis" -V2 installer.nsi )
    [ -f "$output" ] || die "NSIS n'a pas produit $output."
}

main() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --version)
                [ $# -ge 2 ] || die "--version attend un numéro de version (ex : --version 1.2.3)."
                VERSION="$2"
                shift 2
                ;;
            --install-deps) INSTALL_DEPS=true; shift ;;
            --check-deps)   CHECK_DEPS=true; shift ;;
            --help|-h)      show_help; exit 0 ;;
            *)
                echo "Option inconnue : $1"
                show_help
                exit 1
                ;;
        esac
    done

    [ "${MSYSTEM:-}" = "UCRT64" ] || die "ce script doit s'exécuter dans un terminal MSYS2 UCRT64 (MSYSTEM vaut « ${MSYSTEM:-vide} »).
Lancez plutôt .\\build-windows.ps1 depuis PowerShell, à la racine du dépôt."
    [ "$(uname -m)" = "x86_64" ] || die "seul Windows 64 bits (x64) est pris en charge."
    WORK="$(cygpath -u "${SVGPL_BUILD_DIR:-$HOME/svgprintlayout-build}")"

    ensure_packages
    [ "$CHECK_DEPS" = true ] && exit 0

    # Le paquet Go de MSYS2 est compilé avec -trimpath : son programme ne sait
    # pas retrouver tout seul son dossier d'installation. On le lui donne, sauf
    # si l'utilisateur a déjà choisi.
    if [ -z "${GOROOT:-}" ] && [ -d "$MINGW_PREFIX/lib/go" ]; then
        GOROOT="$(cygpath -m "$MINGW_PREFIX/lib/go")"
        export GOROOT
    fi

    read_version
    set_source_date_epoch
    check_go_prerequisites
    step "Go : $(go version)"
    require_tool windres "Il fait partie du paquet MSYS2 mingw-w64-ucrt-x86_64-binutils (installé avec gcc)."
    require_tool ldd "Il fait partie de MSYS2 (paquet msys2-runtime)."
    require_tool zip "pacman -S zip"

    local pkgdir="${PACKAGE_NAME}-${VERSION}-windows-x64"
    local gui="$WORK/$GUI_EXE" console="$WORK/$CONSOLE_EXE"
    local root="$WORK/$pkgdir"
    local zip_out="$PROJECT_DIR/${PACKAGE_NAME}_${VERSION}_windows-portable.zip"
    local setup_out="$PROJECT_DIR/${PACKAGE_NAME}_${VERSION}_windows-setup.exe"

    echo ""
    echo "========================================"
    echo " Construction de $PACKAGE_NAME $VERSION (Windows x64)"
    echo "========================================"

    rm -rf "${WORK:?}"
    mkdir -p "$WORK"
    make_resources "$PROJECT_DIR/cmd/$PACKAGE_NAME"
    build_binary "$gui" windows
    build_binary "$console" console
    rm -f "$SYSO"

    step "Préparation de l'arborescence du paquet..."
    stage_windows "$root" "$gui" "$console"
    normalize_tree "$root"

    # Le paquet doit marcher sur une machine où MSYS2 n'est pas installé : on
    # le lance avec un PATH réduit à celui de Windows, ses DLL doivent suffire.
    step "Vérification du paquet (auto-test avec le seul PATH de Windows)..."
    local winroot
    winroot="$(cygpath -u "${SYSTEMROOT:-${SystemRoot:-C:\\Windows}}")"
    env PATH="$winroot/System32:$winroot" "$root/bin/$CONSOLE_EXE" -selftest \
        || die "l'auto-test du programme échoue hors de MSYS2 : le paquet est incomplet (DLL, polices ou chargeurs d'images manquants)."

    step "Création de la version portable (.zip)..."
    rm -f "${zip_out:?}"
    ( cd "$WORK" && find "$pkgdir" | LC_ALL=C sort | TZ=UTC zip -X -9 -q "$zip_out" -@ )

    local built="$zip_out"
    local makensis
    makensis="$(find_makensis)"
    if [ -n "$makensis" ]; then
        step "Création de l'installeur (.exe) avec NSIS..."
        build_installer "$makensis" "$root" "$setup_out"
        built="$built
   $setup_out"
    else
        echo ""
        echo "NSIS est introuvable : seule la version portable (.zip) est produite."
        echo "Pour obtenir aussi l'installeur : winget install NSIS.NSIS, puis relancez."
    fi

    echo ""
    echo "✅ Paquet(s) créé(s) avec succès :"
    echo "   $built"
}

# Le script peut être « sourcé » (pour en tester les fonctions) sans rien lancer.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi

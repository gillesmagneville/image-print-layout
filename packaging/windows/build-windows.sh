#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
set -euo pipefail

# Construit le paquet Windows de SVG Print Layout : une version portable (.zip)
# et, si NSIS est installé, un installeur (.exe).
#
# Ce script s'exécute dans un terminal MSYS2 UCRT64 (c'est là que se trouvent
# Go, le compilateur C, librsvg et cairo pour Windows). On le lance de
# préférence par build-windows.ps1, à la racine du dépôt, qui prépare MSYS2 et
# NSIS puis l'appelle ; GitHub Actions l'utilise aussi directement.

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

# Paquets MSYS2 nécessaires à la construction (les bibliothèques d'exécution
# viennent avec librsvg et cairo).
MSYS_PACKAGES=(
    mingw-w64-ucrt-x86_64-go
    mingw-w64-ucrt-x86_64-gcc
    mingw-w64-ucrt-x86_64-pkgconf
    mingw-w64-ucrt-x86_64-librsvg
    mingw-w64-ucrt-x86_64-cairo
    zip
    git
)

# Formats d'image que les chargeurs gdk-pixbuf livrés savent lire : ceux des
# images incorporées aux SVG (Inkscape produit surtout du PNG et du JPEG).
PIXBUF_LOADERS=(png jpeg gif bmp webp)

show_help() {
    cat << EOF2
Usage : packaging/windows/build-windows.sh [options]

Construit, à partir du code du dépôt et du numéro de version du fichier
VERSION (ce script ne le modifie jamais : une version se change par une pull
request « Release »), dans dist/ :
  svgprintlayout_<version>_windows-portable.zip   version portable
  svgprintlayout_<version>_windows-setup.exe      installeur (si NSIS est là)

OPTIONS :
  --install-deps    Installe avec pacman les paquets MSYS2 manquants
  --check-deps      Vérifie seulement que les paquets MSYS2 sont présents
  --no-installer    Ne produit pas l'installeur, même si NSIS est présent
  --clean           Supprime le dossier de travail et les paquets Windows de dist/
  --help, -h        Affiche cette aide

À lancer dans un terminal « MSYS2 UCRT64 » (pas MSYS ni MINGW64). Plus simple :
.\\build-windows.ps1 à la racine du dépôt, depuis PowerShell.
EOF2
}

INSTALL_DEPS=false
CHECK_DEPS=false
NO_INSTALLER=false
CLEAN=false

WORK="$DIST_DIR/build/windows"

# missing_packages affiche les paquets MSYS2 absents, un par ligne.
missing_packages() {
    local p
    for p in "${MSYS_PACKAGES[@]}"; do
        pacman -Q "$p" >/dev/null 2>&1 || echo "$p"
    done
}

ensure_packages() {
    local missing
    missing="$(missing_packages)"
    [ -z "$missing" ] && return 0
    if [ "$CHECK_DEPS" = true ]; then
        echo "Paquets MSYS2 manquants :" >&2
        echo "$missing" | sed 's/^/  /' >&2
        return 1
    fi
    if [ "$INSTALL_DEPS" != true ]; then
        die "paquets MSYS2 manquants :
$(echo "$missing" | sed 's/^/  /')
Installez-les avec : pacman -S --needed $(echo $missing)
ou relancez ce script avec --install-deps."
    fi
    step "Installation des paquets MSYS2 manquants..."
    # shellcheck disable=SC2086  # un argument par paquet
    pacman -S --needed --noconfirm $missing
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
# lui donne son icône et ses propriétés (clic droit > Propriétés). Go la lie
# toute seule : tout fichier .syso du dossier d'un paquet est ajouté à l'édition
# de liens. Elle est supprimée à la sortie du script.
make_resources() {
    local cmd_dir="$1"
    local rc_dir="$WORK/resources"
    local file_version="${VERSION_BASE//./,},0"
    mkdir -p "$rc_dir"
    cp "$DATA_DIR/svgprintlayout.ico" "$rc_dir/"
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
    BLOCK "040c04b0"
    BEGIN
      VALUE "ProductName", "SVG Print Layout"
      VALUE "FileDescription", "Mise en page et impression de dessins SVG"
      VALUE "FileVersion", "$VERSION"
      VALUE "ProductVersion", "$VERSION"
      VALUE "CompanyName", "Gilles MAGNEVILLE"
      VALUE "LegalCopyright", "GNU GPL v3"
      VALUE "OriginalFilename", "svgprintlayout.exe"
    END
  END
  BLOCK "VarFileInfo"
  BEGIN
    VALUE "Translation", 0x040c, 1200
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

# stage_windows RACINE BINAIRE prépare l'arborescence « à la manière d'un
# préfixe » que le zip et l'installeur reproduisent à l'identique :
#   bin/        le programme et ses DLL
#   etc/fonts/  la configuration de fontconfig
#   lib/gdk-pixbuf-2.0/  les chargeurs d'images, avec leur cache
#   share/licenses/      les licences des bibliothèques livrées
stage_windows() {
    local root="$1" binary="$2" loaders=() loader dll name
    rm -rf "${root:?}"
    install -d "$root/bin" "$root/etc" "$root/share/licenses"
    install -m 0755 "$binary" "$root/bin/$PACKAGE_NAME.exe"

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
    while IFS= read -r dll; do dlls+=("$dll"); done < <(dll_closure "$binary" "${loaders[@]}")
    [ ${#dlls[@]} -gt 0 ] || die "aucune DLL trouvée pour $binary : ldd ne voit pas les bibliothèques de MSYS2 (terminal UCRT64 ?)."
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

    step "Licences des bibliothèques livrées..."
    write_licenses "$root" "${dlls[@]}" "${loaders[@]}"

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
        echo "!define APP_ICON \"$(cygpath -w "$DATA_DIR/svgprintlayout.ico")\""
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
    cp "$PROJECT_DIR/packaging/windows/installer.nsi" "$dir/installer.nsi"
    rm -f "${output:?}"
    # « -V2 » et non « /V2 » : MSYS2 prendrait « /V2 » pour un chemin POSIX.
    ( cd "$dir" && "$makensis" -V2 installer.nsi )
    [ -f "$output" ] || die "NSIS n'a pas produit $output."
}

main() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --install-deps) INSTALL_DEPS=true; shift ;;
            --check-deps)   CHECK_DEPS=true; shift ;;
            --no-installer) NO_INSTALLER=true; shift ;;
            --clean)        CLEAN=true; shift ;;
            --help|-h)      show_help; exit 0 ;;
            *)
                echo "Option inconnue : $1"
                show_help
                exit 1
                ;;
        esac
    done

    if [ "$CLEAN" = true ]; then
        step "Nettoyage de ${DIST_DIR:?}..."
        rm -rf "${WORK:?}"
        find "${DIST_DIR:?}" -maxdepth 1 -name "${PACKAGE_NAME}_*_windows-*" -type f -delete 2>/dev/null || true
        step "Nettoyage terminé."
        exit 0
    fi

    [ "${MSYSTEM:-}" = "UCRT64" ] || die "ce script doit s'exécuter dans un terminal MSYS2 UCRT64 (MSYSTEM vaut « ${MSYSTEM:-vide} »).
Lancez plutôt .\\build-windows.ps1 depuis PowerShell, à la racine du dépôt."
    [ "$(uname -m)" = "x86_64" ] || die "seul Windows 64 bits (x64) est pris en charge."

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
    local binary="$WORK/$PACKAGE_NAME.exe"
    local root="$WORK/$pkgdir"
    local zip_out="$DIST_DIR/${PACKAGE_NAME}_${VERSION}_windows-portable.zip"
    local setup_out="$DIST_DIR/${PACKAGE_NAME}_${VERSION}_windows-setup.exe"

    echo ""
    echo "========================================"
    echo " Construction de $PACKAGE_NAME $VERSION (Windows x64)"
    echo "========================================"

    rm -rf "${WORK:?}"
    mkdir -p "$WORK" "$DIST_DIR"
    make_resources "$PROJECT_DIR/cmd/$PACKAGE_NAME"
    GO_BUILDMODE=exe build_binary "$binary"
    rm -f "$SYSO"

    step "Préparation de l'arborescence du paquet..."
    stage_windows "$root" "$binary"
    normalize_tree "$root"

    # Le paquet doit marcher sur une machine où MSYS2 n'est pas installé : on
    # le lance avec un PATH réduit à celui de Windows, ses DLL doivent suffire.
    step "Vérification du paquet (auto-test avec le seul PATH de Windows)..."
    local winroot
    winroot="$(cygpath -u "${SYSTEMROOT:-${SystemRoot:-C:\\Windows}}")"
    env PATH="$winroot/System32:$winroot" "$root/bin/$PACKAGE_NAME.exe" -selftest \
        || die "l'auto-test du programme échoue hors de MSYS2 : le paquet est incomplet (DLL, polices ou chargeurs d'images manquants)."

    step "Création de la version portable (.zip)..."
    rm -f "${zip_out:?}"
    ( cd "$WORK" && find "$pkgdir" | LC_ALL=C sort | TZ=UTC zip -X -9 -q "$zip_out" -@ )

    local built="$zip_out"
    if [ "$NO_INSTALLER" = true ]; then
        step "Installeur ignoré (--no-installer)."
    else
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
    fi

    echo ""
    echo "✅ Paquet(s) créé(s) avec succès :"
    echo "   $built"
    echo "   Version (fichier VERSION, inchangée) : $VERSION"
}

# Le script peut être « sourcé » (pour en tester les fonctions) sans rien lancer.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi

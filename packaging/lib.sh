#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
#
# Fonctions communes à build-deb.sh et build-rpm.sh. Ce fichier se source, il
# ne s'exécute pas.
#
# Les deux scripts construisent le paquet avec les outils natifs de la famille
# (dpkg-deb, rpmbuild) et lisent la version dans le fichier VERSION sans
# jamais le modifier : une version se change par une pull request « Release »
# (voir CONTRIBUTING.md).

# shellcheck disable=SC2034  # variables lues par les scripts qui sourcent ce fichier
# ====================== CONFIGURATION ======================
PACKAGE_NAME="svgprintlayout"
APP_ID="io.github.gillesmagneville.svgprintlayout"
MAINTAINER="Gilles MAGNEVILLE <gilles@magneville.fr>"
SUMMARY="Met en page un dessin SVG sur plusieurs feuilles et l'exporte en PDF"
URL="https://github.com/gillesmagneville/svg-print-layout"
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$PROJECT_DIR/dist"
DATA_DIR="$PROJECT_DIR/packaging/data"
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

# read_version lit VERSION et en déduit les numéros des deux formats. Une
# version d'avant-publication (0.1.0-dev) doit se classer avant la version
# finale (0.1.0) : « ~ » chez Debian, une release « 0.1.dev » chez RPM.
read_version() {
    local file="$PROJECT_DIR/VERSION"
    [ -f "$file" ] || die "fichier VERSION introuvable : $file"
    VERSION="$(tr -d '[:space:]' < "$file")"
    local semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'
    [[ "$VERSION" =~ $semver ]] || die "VERSION contient « $VERSION », qui n'est pas une version SemVer (X.Y.Z ou X.Y.Z-suffixe)."

    VERSION_BASE="${VERSION%%-*}"
    local pre=""
    if [[ "$VERSION" == *-* ]]; then pre="${VERSION#*-}"; pre="${pre//-/.}"; fi
    DEB_VERSION="${VERSION_BASE}${pre:+~$pre}-1"
    RPM_VERSION="$VERSION_BASE"
    RPM_RELEASE="${pre:+0.1.$pre}"
    RPM_RELEASE="${RPM_RELEASE:-1}"
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

# Architecture de la machine de construction : le programme est compilé ici.
set_architecture() {
    case "$(uname -m)" in
        x86_64)  DEB_ARCH="amd64"; RPM_ARCH="x86_64" ;;
        aarch64) DEB_ARCH="arm64"; RPM_ARCH="aarch64" ;;
        *) die "architecture $(uname -m) non prise en charge (x86_64 et aarch64 seulement)." ;;
    esac
}

check_go_prerequisites() {
    require_tool go "Installez Go (voir go.mod pour la version) : https://go.dev/doc/install"
    require_tool gcc "Installez un compilateur C (Debian/Ubuntu : sudo apt install build-essential ; Fedora : sudo dnf install gcc)."
    require_tool pkg-config "Installez pkg-config (Debian/Ubuntu : sudo apt install pkg-config ; Fedora : sudo dnf install pkgconf)."
    pkg-config --exists librsvg-2.0 cairo || die "les bibliothèques de développement de librsvg et de cairo sont absentes.
  Debian/Ubuntu : sudo apt install librsvg2-dev
  Fedora        : sudo dnf install librsvg2-devel cairo-devel"
}

# build_binary DESTINATION compile le programme. -trimpath, un identifiant de
# construction vide et l'absence d'informations VCS rendent le binaire
# indépendant du dossier et de l'heure de la construction ; -buildmode=pie
# permet à Linux de placer le programme à une adresse aléatoire (durcissement) ;
# le script Windows demande « exe » avec GO_BUILDMODE (Windows fait déjà l'équivalent).
build_binary() {
    step "Compilation de $PACKAGE_NAME $VERSION (cgo, librsvg, cairo)..."
    mkdir -p "$(dirname "$1")"
    ( cd "$PROJECT_DIR" && CGO_ENABLED=1 go build -trimpath -buildvcs=false \
        -buildmode="${GO_BUILDMODE:-pie}" -ldflags "-s -w -buildid= -X main.version=$VERSION" \
        -o "$1" ./cmd/svgprintlayout )
}

# stage_files RACINE BINAIRE copie dans RACINE l'arborescence commune aux deux
# formats : programme, entrée de menu, métadonnées AppStream, icônes, page de
# manuel, notes.
stage_files() {
    local root="${1:?stage_files : dossier de destination manquant}" binary="$2"
    step "Préparation de l'arborescence du paquet..."
    rm -rf "${root:?}"
    install -D -m 0755 "$binary" "$root/usr/bin/$PACKAGE_NAME"
    install -D -m 0644 "$DATA_DIR/$APP_ID.desktop" "$root/usr/share/applications/$APP_ID.desktop"
    install -D -m 0644 "$DATA_DIR/$APP_ID.metainfo.xml" "$root/usr/share/metainfo/$APP_ID.metainfo.xml"
    install -D -m 0644 "$DATA_DIR/svgprintlayout.svg" "$root/usr/share/icons/hicolor/scalable/apps/svgprintlayout.svg"
    install -D -m 0644 "$DATA_DIR/svgprintlayout-256.png" "$root/usr/share/icons/hicolor/256x256/apps/svgprintlayout.png"
    install -D -m 0644 "$PROJECT_DIR/README.md" "$root/usr/share/doc/$PACKAGE_NAME/README.md"
    install -D -m 0644 "$PROJECT_DIR/CHANGELOG.md" "$root/usr/share/doc/$PACKAGE_NAME/CHANGELOG.md"
    install -d -m 0755 "$root/usr/share/man/man1"
    gzip -9n -c "$DATA_DIR/svgprintlayout.1" > "$root/usr/share/man/man1/$PACKAGE_NAME.1.gz"
    chmod 0644 "$root/usr/share/man/man1/$PACKAGE_NAME.1.gz"
}

# normalize_tree RACINE fixe la date de tout le contenu. install -m fixe déjà
# les permissions, quel que soit l'umask de la machine de construction (un
# umask restrictif rendrait sinon des fichiers illisibles après installation).
normalize_tree() {
    find "${1:?normalize_tree : dossier manquant}" -exec touch -h -d "@$SOURCE_DATE_EPOCH" {} +
}

# clean_dist EXTENSION supprime les paquets déjà construits et le dossier de travail.
clean_dist() {
    local ext="${1:?clean_dist : extension manquante}"
    step "Nettoyage de ${DIST_DIR:?}..."
    rm -rf "${DIST_DIR:?}/build"
    find "${DIST_DIR:?}" -maxdepth 1 -name "*.$ext" -type f -delete 2>/dev/null || true
    step "Nettoyage terminé."
}

# fill_template MODÈLE CLÉ=valeur... écrit le modèle sur la sortie standard, où
# chaque @CLÉ@ est remplacé par sa valeur.
fill_template() {
    local content kv
    content="$(cat "$1")"
    shift
    for kv in "$@"; do
        content="${content//"@${kv%%=*}@"/"${kv#*=}"}"
    done
    printf '%s\n' "$content"
}

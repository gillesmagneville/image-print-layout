#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
set -euo pipefail

# Construit le paquet Debian (.deb) de SVG Print Layout, pour Debian, Ubuntu et
# leurs dérivés. Mêmes options et même gestion du fichier VERSION que le
# build-deb.sh de Planche-Contact. Le programme est compilé en Go (cgo,
# librsvg, cairo) et empaqueté avec dpkg-deb : ni fpm, ni environnement
# Python embarqué.

# ====================== CONFIGURATION ======================
PACKAGE_NAME="svgprintlayout"
APP_ID="io.github.gillesmagneville.svgprintlayout"
MAINTAINER="Gilles MAGNEVILLE <gilles@magneville.fr>"
DESCRIPTION="Met en page un dessin SVG sur plusieurs feuilles et l'exporte en PDF"
URL="https://github.com/gillesmagneville/svg-print-layout"
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="$HOME/deb-build"
BUILD_DIR="$WORK_DIR/build"
VERSION_FILE="$PROJECT_DIR/VERSION"
# ===========================================================

show_help() {
    cat << EOF
Usage: ./build-deb.sh [options]

Script de construction du paquet Debian pour SVG Print Layout.

OPTIONS :
  --major               Incrémente le numéro majeur   (ex: 1.2.3 → 2.0.0)
  --minor               Incrémente le numéro mineur   (ex: 1.2.3 → 1.3.0)
  --patch               Incrémente le numéro de patch (ex: 1.2.3 → 1.2.4)
  --no-version-change   Reconstruit le paquet sans changer la version
  --clean               Nettoie le dossier de build et supprime tous les .deb
  --help, -h            Affiche cette aide

Sans argument → mode interactif (demande major / minor / patch / no-version-change)

Le fichier VERSION n'est mis à jour qu'une fois le paquet construit. Une
version d'avant-publication (0.1.0-dev) est publiée par --minor ou --patch
(→ 0.1.0) ; --major donne 1.0.0.

Le paquet est créé à la racine du dépôt : svgprintlayout_<version>-1_<arch>.deb

Prérequis (Debian, Ubuntu) :
  sudo apt install golang-go build-essential pkg-config librsvg2-dev libgtk-4-dev libgirepository1.0-dev dpkg-dev

Pour une construction identique d'une fois sur l'autre, la date des fichiers
est celle du dernier commit ; SOURCE_DATE_EPOCH la remplace si elle est définie.

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
            echo "Option inconnue : $1"
            show_help
            exit 1
            ;;
    esac
done

cd "$PROJECT_DIR"

# === Mode nettoyage ===
if [ "$CLEAN" = true ]; then
    echo ">>> Nettoyage du dossier de build..."
    rm -rf "${WORK_DIR:?}"
    echo ">>> Suppression des paquets .deb existants..."
    rm -f ./*.deb
    echo ">>> Nettoyage terminé."
    exit 0
fi

# === Vérification des outils ===
erreur() {
    echo "" >&2
    echo "Erreur : $*" >&2
    exit 1
}

command -v dpkg-deb >/dev/null 2>&1 || erreur "dpkg-deb n'est pas installé. Installe-le avec : sudo apt install dpkg"
command -v dpkg-shlibdeps >/dev/null 2>&1 || erreur "dpkg-shlibdeps n'est pas installé. Installe-le avec : sudo apt install dpkg-dev"
command -v go >/dev/null 2>&1 || erreur "Go n'est pas installé (voir go.mod pour la version) : https://go.dev/doc/install"
command -v gcc >/dev/null 2>&1 || erreur "gcc n'est pas installé. Installe-le avec : sudo apt install build-essential"
command -v pkg-config >/dev/null 2>&1 || erreur "pkg-config n'est pas installé. Installe-le avec : sudo apt install pkg-config"
pkg-config --exists librsvg-2.0 cairo || erreur "les bibliothèques de développement de librsvg et de cairo sont absentes.
  Installe-les avec : sudo apt install librsvg2-dev"
pkg-config --exists gtk4 gobject-introspection-1.0 || erreur "les bibliothèques de développement de GTK 4 sont absentes (GTK 4.14 ou plus récent).
  Installe-les avec : sudo apt install libgtk-4-dev libgirepository1.0-dev"

case "$(uname -m)" in
    x86_64)  DEB_ARCH="amd64" ;;
    aarch64) DEB_ARCH="arm64" ;;
    *) erreur "architecture $(uname -m) non prise en charge (x86_64 et aarch64 seulement)." ;;
esac

# === Lecture de la version actuelle ===
[ -f "$VERSION_FILE" ] || erreur "fichier VERSION introuvable : $VERSION_FILE"
CURRENT_VERSION=$(tr -d '[:space:]' < "$VERSION_FILE")

# Une version est X.Y.Z, éventuellement suivie d'un suffixe d'avant-publication
# (X.Y.Z-dev).
SEMVER='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'
[[ "$CURRENT_VERSION" =~ $SEMVER ]] || erreur "VERSION contient « $CURRENT_VERSION », qui n'est pas une version X.Y.Z ou X.Y.Z-suffixe."

# next_version VERSION major|minor|patch affiche la version suivante. Sans
# suffixe, c'est l'incrément habituel (1.2.3 → 1.2.4). Avec un suffixe, la
# version d'avant-publication est d'abord publiée : 0.1.0-dev → 0.1.0 par
# --minor ou --patch, puisque 0.1.0 n'existe pas encore ; --major donne 1.0.0.
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

# === Mode interactif si aucun argument ===
if [ -z "$INCREMENT" ] && [ "$NO_VERSION_CHANGE" = false ]; then
    echo ""
    echo "Version actuelle : $CURRENT_VERSION"
    echo ""
    echo "Que souhaitez-vous faire ?"
    echo "  1) major              (→ $(next_version "$CURRENT_VERSION" major))"
    echo "  2) minor              (→ $(next_version "$CURRENT_VERSION" minor))"
    echo "  3) patch              (→ $(next_version "$CURRENT_VERSION" patch))"
    echo "  4) no-version-change  (reconstruire en $CURRENT_VERSION)"
    echo ""
    read -r -p "Votre choix [1/2/3/4] : " CHOICE || CHOICE=""

    case $CHOICE in
        1) INCREMENT="major" ;;
        2) INCREMENT="minor" ;;
        3) INCREMENT="patch" ;;
        4) NO_VERSION_CHANGE=true ;;
        *)
            echo "Choix invalide. Annulation."
            exit 1
            ;;
    esac
fi

# === Calcul de la nouvelle version ===
if [ "$NO_VERSION_CHANGE" = true ]; then
    NEW_VERSION="$CURRENT_VERSION"
else
    NEW_VERSION=$(next_version "$CURRENT_VERSION" "$INCREMENT")
fi

# Numéro de version Debian. Une version d'avant-publication (0.1.0-dev) doit se
# classer avant la version finale (0.1.0) : c'est le rôle du « ~ ».
VERSION_BASE="${NEW_VERSION%%-*}"
PRERELEASE=""
if [[ "$NEW_VERSION" == *-* ]]; then
    PRERELEASE="${NEW_VERSION#*-}"
    PRERELEASE="${PRERELEASE//-/.}"
fi
DEB_VERSION="${VERSION_BASE}${PRERELEASE:+~$PRERELEASE}-1"

# === Demande de confirmation ===
echo ""
echo "========================================"
if [ "$NO_VERSION_CHANGE" = true ]; then
    echo "  Mode           : Reconstruction sans changement de version"
    echo "  Version        : $NEW_VERSION"
else
    echo "  Version actuelle : $CURRENT_VERSION"
    echo "  Nouvelle version : $NEW_VERSION"
fi
echo "========================================"
echo ""
read -r -p "Construire le paquet en version $NEW_VERSION ? [o/N] " CONFIRM || CONFIRM=""

if [[ ! "$CONFIRM" =~ ^[oOyY]$ ]]; then
    echo "Construction annulée. Aucun fichier n'a été modifié."
    exit 0
fi

# === À partir d'ici, la construction est confirmée ===

echo ">>> Suppression des paquets .deb existants..."
rm -f ./*.deb

DEB_FILE="./${PACKAGE_NAME}_${DEB_VERSION}_${DEB_ARCH}.deb"

# Date des fichiers du paquet : celle du dernier commit, pour que deux
# constructions du même code donnent le même paquet.
if [ -z "${SOURCE_DATE_EPOCH:-}" ]; then
    SOURCE_DATE_EPOCH="$(git -c safe.directory="$PROJECT_DIR" -C "$PROJECT_DIR" log -1 --format=%ct 2>/dev/null || true)"
    SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(date +%s)}"
fi
export SOURCE_DATE_EPOCH

echo ""
echo "========================================"
echo " Construction du paquet $PACKAGE_NAME v$NEW_VERSION ($DEB_ARCH)"
echo "========================================"

rm -rf "${WORK_DIR:?}"
mkdir -p "$BUILD_DIR"

# -trimpath, un identifiant de construction vide et l'absence d'informations
# VCS rendent le binaire indépendant du dossier et de l'heure de la
# construction ; -buildmode=pie permet à Linux de placer le programme à une
# adresse aléatoire (durcissement). Le numéro de version du paquet est inscrit
# dans le programme (svgprintlayout -version), même si le fichier VERSION du
# projet n'est mis à jour qu'après un build réussi (voir plus bas).
echo ">>> Compilation (cgo, GTK4, librsvg, cairo)..."
BINARY="$WORK_DIR/$PACKAGE_NAME"
( cd "$PROJECT_DIR" && CGO_ENABLED=1 go build -trimpath -buildvcs=false -buildmode=pie \
    -ldflags "-s -w -buildid= -X main.version=$NEW_VERSION" -o "$BINARY" ./cmd/svgprintlayout )

echo ">>> Création de l'arborescence..."
install -D -m 0755 "$BINARY" "$BUILD_DIR/usr/bin/$PACKAGE_NAME"
install -D -m 0644 "$PROJECT_DIR/metainfo/$APP_ID.desktop" "$BUILD_DIR/usr/share/applications/$APP_ID.desktop"
# Métadonnées AppStream : elles permettent aux logithèques (GNOME Logiciels,
# App Center) d'afficher le paquet avec sa description complète.
install -D -m 0644 "$PROJECT_DIR/metainfo/$APP_ID.metainfo.xml" "$BUILD_DIR/usr/share/metainfo/$APP_ID.metainfo.xml"
install -D -m 0644 "$PROJECT_DIR/icons/svgprintlayout.svg" "$BUILD_DIR/usr/share/icons/hicolor/scalable/apps/svgprintlayout.svg"
install -D -m 0644 "$PROJECT_DIR/icons/svgprintlayout-256.png" "$BUILD_DIR/usr/share/icons/hicolor/256x256/apps/svgprintlayout.png"
# Page de manuel : en anglais dans man1 (langue de repli), traduite dans le
# dossier de chaque langue (man-db choisit selon la langue de l'utilisateur).
install -d -m 0755 "$BUILD_DIR/usr/share/man/man1"
gzip -9n -c "$PROJECT_DIR/docs/svgprintlayout.1" > "$BUILD_DIR/usr/share/man/man1/$PACKAGE_NAME.1.gz"
for LANGUE in fr es de; do
    install -d -m 0755 "$BUILD_DIR/usr/share/man/$LANGUE/man1"
    gzip -9n -c "$PROJECT_DIR/docs/$LANGUE/svgprintlayout.1" > "$BUILD_DIR/usr/share/man/$LANGUE/man1/$PACKAGE_NAME.1.gz"
done

DOC_DIR="$BUILD_DIR/usr/share/doc/$PACKAGE_NAME"
install -D -m 0644 "$PROJECT_DIR/README.md" "$DOC_DIR/README.md"
install -D -m 0644 "$PROJECT_DIR/debian/copyright" "$DOC_DIR/copyright"
# Noms que Debian attend pour les journaux de modifications : celui du projet
# (changelog.gz) et celui du paquet (changelog.Debian.gz, au format Debian).
gzip -9n -c "$PROJECT_DIR/CHANGELOG.md" > "$DOC_DIR/changelog.gz"
{
    echo "$PACKAGE_NAME ($DEB_VERSION) unstable; urgency=low"
    echo ""
    echo "  * Paquet construit à partir du dépôt ; l'historique est dans changelog.gz."
    echo ""
    echo " -- $MAINTAINER  $(date -u -d "@$SOURCE_DATE_EPOCH" -R)"
} | gzip -9n -c > "$DOC_DIR/changelog.Debian.gz"
install -D -m 0644 "$PROJECT_DIR/debian/lintian-overrides" "$BUILD_DIR/usr/share/lintian/overrides/$PACKAGE_NAME"

# === Normalisation des permissions --------------------------------------
# install -m fixe déjà les permissions voulues, quel que soit l'umask de la
# machine de build (un umask restrictif rendrait sinon des fichiers illisibles
# pour un utilisateur normal une fois le paquet installé). On normalise ici les
# dossiers et les dates, pour que le paquet ne dépende pas non plus du moment
# de la construction.
echo ">>> Normalisation des permissions..."
chmod 0644 "$DOC_DIR/changelog.gz" "$DOC_DIR/changelog.Debian.gz" "$BUILD_DIR"/usr/share/man/man1/*.gz "$BUILD_DIR"/usr/share/man/*/man1/*.gz
find "$BUILD_DIR" -type d -exec chmod 755 {} +

# Dépendances : dpkg-shlibdeps lit les bibliothèques que le binaire charge et
# en déduit les paquets (avec les versions minimales) : librsvg, cairo et GTK 4
# (libgtk-4-1) pour la fenêtre.
echo ">>> Calcul des dépendances (dpkg-shlibdeps)..."
SHLIBS_TMP="$(mktemp -d)"
trap 'rm -rf "${SHLIBS_TMP:?}"' EXIT
# dpkg-shlibdeps attend un dossier debian/ avec un fichier control. Il avertit
# que le binaire n'est pas encore à sa place dans un paquet : c'est normal,
# on l'examine avant de construire le paquet, et on ne garde pas cet avertissement.
mkdir "$SHLIBS_TMP/debian"
printf 'Source: %s\n\nPackage: %s\nArchitecture: any\n' "$PACKAGE_NAME" "$PACKAGE_NAME" > "$SHLIBS_TMP/debian/control"
DEPENDS="$(cd "$SHLIBS_TMP" && dpkg-shlibdeps -O "-e$BINARY" 2> "$SHLIBS_TMP/avertissements" | sed -n 's/^shlibs:Depends=//p')"
grep -v 'should already be installed' "$SHLIBS_TMP/avertissements" >&2 || true
[ -n "$DEPENDS" ] || erreur "dpkg-shlibdeps n'a trouvé aucune dépendance : le binaire n'est pas lié aux bibliothèques attendues."
echo "    Depends: $DEPENDS"

echo ">>> Création des fichiers de contrôle..."
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
Homepage: $URL
Description: $DESCRIPTION
 SVG Print Layout prépare l'impression d'un document vectoriel SVG de
 dimensions quelconques sur plusieurs feuilles de papier standard : on choisit
 le format, les marges et la largeur du dessin, on place les coupures à la
 souris, puis on exporte un seul PDF multipage vectoriel.
 .
 Le dessin garde une échelle unique : aucune page n'est réduite en silence
 pour faire tenir une portion trop haute. Le fichier SVG d'origine n'est
 jamais modifié.
EOF
# Scripts de maintenance : dpkg-deb les traite nativement quand ils sont dans DEBIAN/.
install -m 0755 "$PROJECT_DIR/debian/postinst" "$BUILD_DIR/DEBIAN/postinst"
install -m 0755 "$PROJECT_DIR/debian/postrm" "$BUILD_DIR/DEBIAN/postrm"
( cd "$BUILD_DIR" && find usr -type f -print0 | sort -z | xargs -0 md5sum ) > "$BUILD_DIR/DEBIAN/md5sums"
chmod 0644 "$BUILD_DIR/DEBIAN/md5sums"
find "$BUILD_DIR" -exec touch -h -d "@$SOURCE_DATE_EPOCH" {} +

echo ">>> Création du paquet .deb..."
dpkg-deb --root-owner-group -Zxz --build "$BUILD_DIR" "$DEB_FILE" > /dev/null

echo ""
echo "✅ Paquet créé avec succès :"
echo "   $DEB_FILE"
echo "   $(du -h "$DEB_FILE" | cut -f1) — sha256 $(sha256sum "$DEB_FILE" | cut -c1-16)…"

# Le fichier VERSION du projet n'est mis à jour qu'ici, une fois le .deb
# effectivement construit. Ainsi, un échec de build (compilation,
# dpkg-shlibdeps, dpkg-deb...) ne laisse jamais le projet dans un état
# incohérent (VERSION incrémenté sans .deb correspondant).
if [ "$NO_VERSION_CHANGE" = true ]; then
    echo "   Version inchangée : $NEW_VERSION"
else
    echo "$NEW_VERSION" > "$VERSION_FILE"
    echo "   Version enregistrée : $NEW_VERSION"
    if [[ "$NEW_VERSION" != *-* ]]; then
        echo "   Pensez à ajouter « ## [$NEW_VERSION] » à CHANGELOG.md (voir CONTRIBUTING.md)."
    fi
fi
echo ""
echo "Pour l'essayer : sudo apt install \"$DEB_FILE\" && $PACKAGE_NAME -selftest"

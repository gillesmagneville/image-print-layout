#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
set -euo pipefail

# Construit le paquet RPM de SVG Print Layout, pour Fedora, RHEL et leurs
# dérivés. Mêmes options et même gestion du fichier VERSION que build-deb.sh.
# rpmbuild empaquète un dossier préparé par le script ; il ne compile rien
# lui-même.

# ====================== CONFIGURATION ======================
PACKAGE_NAME="svgprintlayout"
APP_ID="io.github.gillesmagneville.svgprintlayout"
MAINTAINER="Gilles MAGNEVILLE <gilles@magneville.fr>"
DESCRIPTION="Met en page un dessin SVG sur plusieurs feuilles et l'exporte en PDF"
URL="https://github.com/gillesmagneville/svg-print-layout"
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="$HOME/rpm-build"
BUILD_DIR="$WORK_DIR/build"
VERSION_FILE="$PROJECT_DIR/VERSION"
# ===========================================================

show_help() {
    cat << EOF
Usage: ./build-rpm.sh [options]

Script de construction du paquet RPM pour SVG Print Layout.

OPTIONS :
  --major               Incrémente le numéro majeur   (ex: 1.2.3 → 2.0.0)
  --minor               Incrémente le numéro mineur   (ex: 1.2.3 → 1.3.0)
  --patch               Incrémente le numéro de patch (ex: 1.2.3 → 1.2.4)
  --no-version-change   Reconstruit le paquet sans changer la version
  --clean               Nettoie le dossier de build et supprime tous les .rpm
  --help, -h            Affiche cette aide

Sans argument → mode interactif (demande major / minor / patch / no-version-change)

Le fichier VERSION n'est mis à jour qu'une fois le paquet construit. Une
version d'avant-publication (0.1.0-dev) est publiée par --minor ou --patch
(→ 0.1.0) ; --major donne 1.0.0.

Le paquet est créé à la racine du dépôt :
svgprintlayout-<version>-<release>.<arch>.rpm. Une version d'avant-publication
(0.1.0-dev) devient la release « 0.1.dev » et se classe avant la 0.1.0.

Prérequis :
  Fedora        : sudo dnf install golang gcc pkgconf librsvg2-devel cairo-devel rpm-build
  Debian/Ubuntu : sudo apt install golang-go build-essential pkg-config librsvg2-dev rpm

Les dépendances du paquet (bibliothèques partagées) sont déduites du binaire
par rpmbuild. Le paquet est lié aux versions des bibliothèques de la machine
de construction : pour viser une distribution, construisez-le dessus (ou dans
un conteneur de cette distribution).

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
    echo ">>> Suppression des paquets .rpm existants..."
    rm -f ./*.rpm
    echo ">>> Nettoyage terminé."
    exit 0
fi

# === Vérification des outils ===
erreur() {
    echo "" >&2
    echo "Erreur : $*" >&2
    exit 1
}

command -v rpmbuild >/dev/null 2>&1 || erreur "rpmbuild n'est pas installé. Installe-le avec : sudo dnf install rpm-build (Debian/Ubuntu : sudo apt install rpm)"
command -v go >/dev/null 2>&1 || erreur "Go n'est pas installé (voir go.mod pour la version) : https://go.dev/doc/install"
command -v gcc >/dev/null 2>&1 || erreur "gcc n'est pas installé. Installe-le avec : sudo dnf install gcc (Debian/Ubuntu : sudo apt install build-essential)"
command -v pkg-config >/dev/null 2>&1 || erreur "pkg-config n'est pas installé. Installe-le avec : sudo dnf install pkgconf (Debian/Ubuntu : sudo apt install pkg-config)"
pkg-config --exists librsvg-2.0 cairo || erreur "les bibliothèques de développement de librsvg et de cairo sont absentes.
  Fedora        : sudo dnf install librsvg2-devel cairo-devel
  Debian/Ubuntu : sudo apt install librsvg2-dev"

case "$(uname -m)" in
    x86_64)  RPM_ARCH="x86_64" ;;
    aarch64) RPM_ARCH="aarch64" ;;
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

# Numéros de version RPM. Une version d'avant-publication (0.1.0-dev) doit se
# classer avant la version finale (0.1.0) : sa release est « 0.1.dev », celle
# d'une version finale est « 1 ».
RPM_VERSION="${NEW_VERSION%%-*}"
PRERELEASE=""
if [[ "$NEW_VERSION" == *-* ]]; then
    PRERELEASE="${NEW_VERSION#*-}"
    PRERELEASE="${PRERELEASE//-/.}"
fi
RPM_RELEASE="${PRERELEASE:+0.1.$PRERELEASE}"
RPM_RELEASE="${RPM_RELEASE:-1}"

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

echo ">>> Suppression des paquets .rpm existants..."
rm -f ./*.rpm

RPM_FILE="./${PACKAGE_NAME}-${RPM_VERSION}-${RPM_RELEASE}.${RPM_ARCH}.rpm"

# Date des fichiers du paquet : celle du dernier commit, pour que deux
# constructions du même code donnent le même paquet.
if [ -z "${SOURCE_DATE_EPOCH:-}" ]; then
    SOURCE_DATE_EPOCH="$(git -c safe.directory="$PROJECT_DIR" -C "$PROJECT_DIR" log -1 --format=%ct 2>/dev/null || true)"
    SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(date +%s)}"
fi
export SOURCE_DATE_EPOCH

echo ""
echo "========================================"
echo " Construction du paquet $PACKAGE_NAME v$NEW_VERSION ($RPM_ARCH)"
echo "========================================"

rm -rf "${WORK_DIR:?}"
mkdir -p "$BUILD_DIR"

# -trimpath, un identifiant de construction vide et l'absence d'informations
# VCS rendent le binaire indépendant du dossier et de l'heure de la
# construction ; -buildmode=pie permet à Linux de placer le programme à une
# adresse aléatoire (durcissement). Le numéro de version du paquet est inscrit
# dans le programme (svgprintlayout -version), même si le fichier VERSION du
# projet n'est mis à jour qu'après un build réussi (voir plus bas).
echo ">>> Compilation (cgo, librsvg, cairo)..."
BINARY="$WORK_DIR/$PACKAGE_NAME"
( cd "$PROJECT_DIR" && CGO_ENABLED=1 go build -trimpath -buildvcs=false -buildmode=pie \
    -ldflags "-s -w -buildid= -X main.version=$NEW_VERSION" -o "$BINARY" ./cmd/svgprintlayout )

echo ">>> Création de l'arborescence..."
install -D -m 0755 "$BINARY" "$BUILD_DIR/usr/bin/$PACKAGE_NAME"
install -D -m 0644 "$PROJECT_DIR/metainfo/$APP_ID.desktop" "$BUILD_DIR/usr/share/applications/$APP_ID.desktop"
# Métadonnées AppStream : elles permettent aux logithèques (GNOME Logiciels,
# KDE Discover) d'afficher le paquet avec sa description complète.
install -D -m 0644 "$PROJECT_DIR/metainfo/$APP_ID.metainfo.xml" "$BUILD_DIR/usr/share/metainfo/$APP_ID.metainfo.xml"
install -D -m 0644 "$PROJECT_DIR/icons/svgprintlayout.svg" "$BUILD_DIR/usr/share/icons/hicolor/scalable/apps/svgprintlayout.svg"
install -D -m 0644 "$PROJECT_DIR/icons/svgprintlayout-256.png" "$BUILD_DIR/usr/share/icons/hicolor/256x256/apps/svgprintlayout.png"
install -d -m 0755 "$BUILD_DIR/usr/share/man/man1"
gzip -9n -c "$PROJECT_DIR/docs/svgprintlayout.1" > "$BUILD_DIR/usr/share/man/man1/$PACKAGE_NAME.1.gz"
chmod 0644 "$BUILD_DIR/usr/share/man/man1/$PACKAGE_NAME.1.gz"
install -D -m 0644 "$PROJECT_DIR/README.md" "$BUILD_DIR/usr/share/doc/$PACKAGE_NAME/README.md"
install -D -m 0644 "$PROJECT_DIR/CHANGELOG.md" "$BUILD_DIR/usr/share/doc/$PACKAGE_NAME/CHANGELOG.md"
install -D -m 0644 "$PROJECT_DIR/LICENSE" "$BUILD_DIR/usr/share/licenses/$PACKAGE_NAME/LICENSE"

# install -m fixe les permissions voulues, quel que soit l'umask de la machine
# de build ; on fixe ici les dates, pour que le paquet ne dépende pas du moment
# de la construction.
echo ">>> Normalisation des permissions..."
find "$BUILD_DIR" -type d -exec chmod 755 {} +
find "$BUILD_DIR" -exec touch -h -d "@$SOURCE_DATE_EPOCH" {} +

echo ">>> Écriture du fichier .spec..."
TOP="$WORK_DIR/rpmbuild"
SPEC="$TOP/SPECS/$PACKAGE_NAME.spec"
mkdir -p "$TOP/SPECS"
{
    # Il n'y a ni %prep, ni %build, ni %install : le programme est déjà
    # compilé et installé dans BUILD_DIR, rpmbuild ne fait que l'empaqueter.
    cat << EOF
# Le binaire est déjà dépouillé à l'édition de liens ; pas de paquet debuginfo.
%global debug_package %{nil}
%global __strip /bin/true
%define _build_id_links none

# Un même contenu donne un même paquet : dates fixées par SOURCE_DATE_EPOCH.
%define use_source_date_epoch_as_buildtime 1
%define clamp_mtime_to_source_date_epoch 1
%define _buildhost reproducible

Name:           $PACKAGE_NAME
Version:        $RPM_VERSION
Release:        $RPM_RELEASE
Summary:        $DESCRIPTION
License:        GPL-3.0-only
URL:            $URL
Packager:       $MAINTAINER

%description
SVG Print Layout prépare l'impression d'un document vectoriel SVG de
dimensions quelconques sur plusieurs feuilles de papier standard : on choisit
le format, les marges et la largeur du dessin, on place les coupures à la
souris, puis on exporte un seul PDF multipage vectoriel.

Le dessin garde une échelle unique : aucune page n'est réduite en silence
pour faire tenir une portion trop haute. Le fichier SVG d'origine n'est
jamais modifié.

# Met à jour les caches de l'icône et du menu ; un échec n'est pas une erreur.
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
    # Les notes et la licence reçoivent leurs marques (%doc, %license) : dnf
    # peut alors les exclure ou les lister à part.
    ( cd "$BUILD_DIR" && find usr -type f ! -path "usr/bin/*" | sort | sed \
        -e 's|^usr/share/doc/|%doc /usr/share/doc/|' \
        -e 's|^usr/share/licenses/|%license /usr/share/licenses/|' \
        -e 's|^usr/share/man/|%doc /usr/share/man/|' \
        -e 's|^usr/|/usr/|' )
    echo ""
    echo "%changelog"
    echo "* $(LC_ALL=C date -u -d "@$SOURCE_DATE_EPOCH" '+%a %b %d %Y') $MAINTAINER - $RPM_VERSION-$RPM_RELEASE"
    echo "- Paquet construit à partir du dépôt ; l'historique est dans CHANGELOG.md."
} > "$SPEC"

echo ">>> Création du paquet .rpm..."
rpmbuild -bb "$SPEC" \
    --quiet \
    --buildroot "$BUILD_DIR" \
    --target "${RPM_ARCH}-linux" \
    --define "_topdir $TOP" \
    --define "_rpmdir $PROJECT_DIR" \
    --define "_rpmfilename %%{NAME}-%%{VERSION}-%%{RELEASE}.%%{ARCH}.rpm"
[ -f "$RPM_FILE" ] || erreur "rpmbuild n'a pas produit $RPM_FILE."

echo ""
echo "✅ Paquet créé avec succès :"
echo "   $RPM_FILE"
echo "   $(du -h "$RPM_FILE" | cut -f1) — sha256 $(sha256sum "$RPM_FILE" | cut -c1-16)…"

# Le fichier VERSION du projet n'est mis à jour qu'ici, une fois le .rpm
# effectivement construit. Ainsi, un échec de build (compilation, rpmbuild...)
# ne laisse jamais le projet dans un état incohérent (VERSION incrémenté sans
# .rpm correspondant).
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
echo "Pour l'essayer : sudo dnf install \"$RPM_FILE\" && $PACKAGE_NAME -selftest"

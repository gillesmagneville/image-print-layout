#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
set -euo pipefail

# Construit le paquet RPM de SVG Print Layout, pour Fedora, RHEL et leurs
# dérivés. Même organisation que build-deb.sh ; rpmbuild empaquète un dossier
# préparé par le script, il ne compile rien lui-même.

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/packaging/lib.sh"

show_help() {
    cat << EOF2
Usage: ./build-rpm.sh [options]

Construit dist/svgprintlayout-<version>-<release>.<architecture>.rpm à partir
du code du dépôt et du numéro de version du fichier VERSION (ce script ne le
modifie jamais : une version se change par une pull request « Release »).

OPTIONS :
  --clean       Supprime le dossier de travail et les .rpm de dist/
  --help, -h    Affiche cette aide

Prérequis :
  Fedora        : sudo dnf install golang gcc pkgconf librsvg2-devel cairo-devel rpm-build
  Debian/Ubuntu : sudo apt install golang-go build-essential pkg-config librsvg2-dev rpm

Les dépendances du paquet (bibliothèques partagées) sont déduites du binaire
par rpmbuild. Le paquet est lié aux versions des bibliothèques de la machine
de construction : pour viser une distribution, construisez-le dessus (ou dans
un conteneur de cette distribution). Une version d'avant-publication
(0.1.0-dev) devient la release « 0.1.dev » et se classe avant la 0.1.0.
EOF2
}

CLEAN=false
while [[ $# -gt 0 ]]; do
    case $1 in
        --clean)    CLEAN=true; shift ;;
        --help|-h)  show_help; exit 0 ;;
        *)
            echo "Option inconnue : $1"
            show_help
            exit 1
            ;;
    esac
done

if [ "$CLEAN" = true ]; then
    clean_dist rpm
    exit 0
fi

require_tool rpmbuild "Fedora : sudo dnf install rpm-build ; Debian/Ubuntu : sudo apt install rpm"
read_version
set_architecture
set_source_date_epoch
check_go_prerequisites

WORK="$DIST_DIR/build/rpm"
BINARY="$WORK/$PACKAGE_NAME"
ROOT="$WORK/root"
TOP="$WORK/rpmbuild"
SPEC="$TOP/SPECS/$PACKAGE_NAME.spec"
OUT="$DIST_DIR/${PACKAGE_NAME}-${RPM_VERSION}-${RPM_RELEASE}.${RPM_ARCH}.rpm"

echo ""
echo "========================================"
echo " Construction du paquet $PACKAGE_NAME $RPM_VERSION-$RPM_RELEASE ($RPM_ARCH)"
echo "========================================"

mkdir -p "$WORK"
build_binary "$BINARY"
stage_files "$ROOT" "$BINARY"
install -D -m 0644 "$PROJECT_DIR/LICENSE" "$ROOT/usr/share/licenses/$PACKAGE_NAME/LICENSE"
normalize_tree "$ROOT"

step "Écriture du fichier .spec..."
mkdir -p "$TOP/SPECS"
{
    fill_template "$PROJECT_DIR/packaging/rpm/svgprintlayout.spec.in" \
        "NAME=$PACKAGE_NAME" "VERSION=$RPM_VERSION" "RELEASE=$RPM_RELEASE" \
        "SUMMARY=$SUMMARY" "URL=$URL" "MAINTAINER=$MAINTAINER"
    echo "%files"
    echo "%defattr(-,root,root,-)"
    echo "%attr(0755,root,root) /usr/bin/$PACKAGE_NAME"
    echo "%dir /usr/share/doc/$PACKAGE_NAME"
    echo "%dir /usr/share/licenses/$PACKAGE_NAME"
    # Les notes et la licence reçoivent leurs marques (%doc, %license) : dnf
    # peut alors les exclure ou les lister à part.
    ( cd "$ROOT" && find usr -type f ! -path "usr/bin/*" | sort | sed \
        -e 's|^usr/share/doc/|%doc /usr/share/doc/|' \
        -e 's|^usr/share/licenses/|%license /usr/share/licenses/|' \
        -e 's|^usr/share/man/|%doc /usr/share/man/|' \
        -e 's|^usr/|/usr/|' )
    echo ""
    echo "%changelog"
    echo "* $(LC_ALL=C date -u -d "@$SOURCE_DATE_EPOCH" '+%a %b %d %Y') $MAINTAINER - $RPM_VERSION-$RPM_RELEASE"
    echo "- Paquet construit à partir du dépôt ; l'historique est dans CHANGELOG.md."
} > "$SPEC"

step "Création du paquet .rpm..."
rm -f "${OUT:?}"
mkdir -p "$DIST_DIR"
rpmbuild -bb "$SPEC" \
    --quiet \
    --buildroot "$ROOT" \
    --target "${RPM_ARCH}-linux" \
    --define "_topdir $TOP" \
    --define "_rpmdir $DIST_DIR" \
    --define "_rpmfilename %%{NAME}-%%{VERSION}-%%{RELEASE}.%%{ARCH}.rpm"
[ -f "$OUT" ] || die "rpmbuild n'a pas produit $OUT."

echo ""
echo "✅ Paquet créé avec succès :"
echo "   $OUT"
echo "   $(du -h "$OUT" | cut -f1) — sha256 $(sha256sum "$OUT" | cut -c1-16)…"
echo "   Version (fichier VERSION, inchangé) : $VERSION"
echo ""
echo "Pour l'essayer : sudo dnf install \"$OUT\" && svgprintlayout -selftest"

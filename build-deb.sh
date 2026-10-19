#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
set -euo pipefail

# Construit le paquet Debian (.deb) de SVG Print Layout, pour Debian, Ubuntu et
# leurs dérivés. Équivalent de build-deb.sh de Planche-Contact, avec dpkg-deb :
# le programme est compilé en Go (cgo, librsvg, cairo), pas embarqué dans un
# environnement Python.

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/packaging/lib.sh"

show_help() {
    cat << EOF2
Usage: ./build-deb.sh [options]

Construit dist/svgprintlayout_<version>_<architecture>.deb à partir du code
du dépôt et du numéro de version du fichier VERSION (ce script ne le modifie
jamais : une version se change par une pull request « Release »).

OPTIONS :
  --clean       Supprime le dossier de travail et les .deb de dist/
  --help, -h    Affiche cette aide

Prérequis (Debian, Ubuntu) :
  sudo apt install golang-go build-essential pkg-config librsvg2-dev dpkg-dev

Pour une construction identique d'une fois sur l'autre, la date des fichiers
est celle du dernier commit ; SOURCE_DATE_EPOCH la remplace si elle est définie.
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
    clean_dist deb
    exit 0
fi

require_tool dpkg-deb "Installez-le avec : sudo apt install dpkg"
require_tool dpkg-shlibdeps "Installez-le avec : sudo apt install dpkg-dev"
read_version
set_architecture
set_source_date_epoch
check_go_prerequisites

WORK="$DIST_DIR/build/deb"
BINARY="$WORK/$PACKAGE_NAME"
ROOT="$WORK/root"
OUT="$DIST_DIR/${PACKAGE_NAME}_${DEB_VERSION}_${DEB_ARCH}.deb"

echo ""
echo "========================================"
echo " Construction du paquet $PACKAGE_NAME $DEB_VERSION ($DEB_ARCH)"
echo "========================================"

mkdir -p "$WORK"
build_binary "$BINARY"
stage_files "$ROOT" "$BINARY"
DOC="$ROOT/usr/share/doc/$PACKAGE_NAME"
install -D -m 0644 "$PROJECT_DIR/packaging/debian/copyright" "$DOC/copyright"

# Noms que Debian attend pour les journaux de modifications : celui du paquet
# (changelog.Debian.gz, au format Debian) et celui du projet (changelog.gz).
rm -f "${DOC:?}/CHANGELOG.md"
gzip -9n -c "$PROJECT_DIR/CHANGELOG.md" > "$DOC/changelog.gz"
{
    echo "$PACKAGE_NAME ($DEB_VERSION) unstable; urgency=low"
    echo ""
    echo "  * Paquet construit à partir du dépôt ; l'historique est dans changelog.gz."
    echo ""
    echo " -- $MAINTAINER  $(date -u -d "@$SOURCE_DATE_EPOCH" -R)"
} | gzip -9n -c > "$DOC/changelog.Debian.gz"
chmod 0644 "$DOC/changelog.gz" "$DOC/changelog.Debian.gz"
install -D -m 0644 "$PROJECT_DIR/packaging/debian/lintian-overrides" \
    "$ROOT/usr/share/lintian/overrides/$PACKAGE_NAME"

# Dépendances : dpkg-shlibdeps lit les bibliothèques que le binaire charge et
# en déduit les paquets (avec les versions minimales). Quand l'interface GTK4
# sera liée au programme, libgtk-4-1 apparaîtra ici sans rien changer.
step "Calcul des dépendances (dpkg-shlibdeps)..."
SHLIBS_TMP="$(mktemp -d)"
trap 'rm -rf "${SHLIBS_TMP:?}"' EXIT
# dpkg-shlibdeps attend un dossier debian/ avec un fichier control. Il avertit
# que le binaire n'est pas encore à sa place dans un paquet : c'est normal,
# on l'examine avant de construire le paquet, et on ne garde pas cet avertissement.
mkdir "$SHLIBS_TMP/debian"
printf 'Source: %s\n\nPackage: %s\nArchitecture: any\n' "$PACKAGE_NAME" "$PACKAGE_NAME" > "$SHLIBS_TMP/debian/control"
DEPENDS="$(cd "$SHLIBS_TMP" && dpkg-shlibdeps -O "-e$BINARY" 2> "$SHLIBS_TMP/avertissements" | sed -n 's/^shlibs:Depends=//p')"
grep -v 'should already be installed' "$SHLIBS_TMP/avertissements" >&2 || true
[ -n "$DEPENDS" ] || die "dpkg-shlibdeps n'a trouvé aucune dépendance : le binaire n'est pas lié aux bibliothèques attendues."
echo "    Depends: $DEPENDS"

step "Création des fichiers de contrôle..."
INSTALLED_SIZE="$(du -sk "$ROOT" | cut -f1)"
mkdir -p "$ROOT/DEBIAN"
fill_template "$PROJECT_DIR/packaging/debian/control.in" \
    "NAME=$PACKAGE_NAME" "VERSION=$DEB_VERSION" "ARCH=$DEB_ARCH" "MAINTAINER=$MAINTAINER" \
    "SIZE=$INSTALLED_SIZE" "DEPENDS=$DEPENDS" "URL=$URL" "SUMMARY=$SUMMARY" \
    | grep -v '^#' > "$ROOT/DEBIAN/control"   # dpkg-deb refuse les commentaires d'un control binaire
install -m 0755 "$PROJECT_DIR/packaging/debian/postinst" "$ROOT/DEBIAN/postinst"
install -m 0755 "$PROJECT_DIR/packaging/debian/postrm" "$ROOT/DEBIAN/postrm"
( cd "$ROOT" && find usr -type f -print0 | sort -z | xargs -0 md5sum ) > "$ROOT/DEBIAN/md5sums"
chmod 0644 "$ROOT/DEBIAN/md5sums"
find "$ROOT" -type d -exec chmod 0755 {} +

step "Création du paquet .deb..."
normalize_tree "$ROOT"
rm -f "${OUT:?}"
dpkg-deb --root-owner-group -Zxz --build "$ROOT" "$OUT" > /dev/null

echo ""
echo "✅ Paquet créé avec succès :"
echo "   $OUT"
echo "   $(du -h "$OUT" | cut -f1) — sha256 $(sha256sum "$OUT" | cut -c1-16)…"
echo "   Version (fichier VERSION, inchangé) : $VERSION"
echo ""
echo "Pour l'essayer : sudo apt install \"$OUT\" && svgprintlayout -selftest"

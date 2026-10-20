#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
#
# Étapes du job « distributions » de la CI (.github/workflows/ci.yml), qui
# s'exécute dans un conteneur de la distribution à essayer. Le script installe
# les outils de construction avec le gestionnaire de paquets de la distribution,
# construit le paquet de sa famille (.deb ou .rpm) avec le script du dépôt,
# l'installe comme le ferait une personne (apt ou dnf, qui tirent GTK 4 et les
# autres dépendances), puis lance le programme installé : -version, -selftest et
# la fenêtre sur un écran virtuel.
#
# Un paquet est lié aux bibliothèques de la machine qui l'a construit : on le
# construit donc sur la distribution où on l'installe.
#
# La fin du journal est recopiée dans une annotation de l'exécution, que l'on
# lit sans télécharger les journaux : en erreur si une étape échoue, en simple
# note sinon (versions des bibliothèques, dépendances du paquet).
#
# Usage : test-distribution.sh debian|redhat

set -euo pipefail

family="${1:-}"
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
log="${RUNNER_TEMP:-/tmp}/distribution.log"
screenshots="$PWD/screenshots"

# Écrit la fin du journal dans une annotation GitHub (%, retours chariot et
# sauts de ligne y sont codés).
annotate() {
    local text
    text="$(tail -n 70 "$log")"
    text="${text//'%'/'%25'}"
    text="${text//$'\r'/'%0D'}"
    text="${text//$'\n'/'%0A'}"
    echo "::$1 title=$2::$text"
}

install_debian() {
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y --no-install-recommends build-essential pkg-config dpkg-dev git ca-certificates \
        librsvg2-dev libgtk-4-dev libgirepository1.0-dev \
        fonts-ibm-plex adwaita-icon-theme xvfb xauth imagemagick
}

build_install_debian() {
    echo o | ./build-deb.sh --no-version-change
    echo "--- Depends du paquet"
    dpkg-deb -f ./svgprintlayout_*.deb Depends
    apt-get install -y ./svgprintlayout_*.deb
}

install_redhat() {
    # Les paquets de développement de GTK et de GObject Introspection sont dans
    # le dépôt CRB (« PowerTools » avant la version 9) des clones de RHEL ; ni
    # CRB ni EPEL n'existent chez Fedora, d'où les « || true ».
    dnf -y install dnf-plugins-core epel-release || true
    dnf config-manager --set-enabled crb || dnf config-manager --set-enabled powertools || true
    dnf -y install gcc make pkgconf-pkg-config git tar gzip findutils which rpm-build \
        librsvg2-devel cairo-devel gtk4-devel gobject-introspection-devel \
        dejavu-sans-fonts adwaita-icon-theme
    # L'écran virtuel et l'outil de capture sont facultatifs : sans eux, la
    # fenêtre n'est pas essayée, et le journal le dit.
    dnf -y install xorg-x11-server-Xvfb xorg-x11-xauth ImageMagick || echo "(Xvfb ou ImageMagick introuvable dans les dépôts)"
}

build_install_redhat() {
    echo o | ./build-rpm.sh --no-version-change
    echo "--- Requires du paquet"
    rpm -qpR ./svgprintlayout-*.rpm
    dnf -y install ./svgprintlayout-*.rpm
}

versions() {
    echo "--- $(. /etc/os-release && echo "$PRETTY_NAME")"
    echo "go $(go version | awk '{print $3}'), gcc $(gcc -dumpversion)"
    echo "gtk4 $(pkg-config --modversion gtk4), glib $(pkg-config --modversion glib-2.0), librsvg $(pkg-config --modversion librsvg-2.0), cairo $(pkg-config --modversion cairo)"
}

# Le programme installé, comme le lancerait une personne.
run_installed() {
    echo "--- programme installé"
    svgprintlayout -version
    svgprintlayout -selftest
    if command -v xvfb-run > /dev/null && command -v import > /dev/null; then
        mkdir -p "$screenshots"
        for lang in en fr es de; do
            .github/scripts/screenshot.sh /usr/bin/svgprintlayout "$lang" "$screenshots/$lang.png"
        done
        echo "FENÊTRE : ouverte dans les quatre langues"
    else
        echo "FENÊTRE : non essayée (pas d'écran virtuel ou d'outil de capture sur cette distribution)"
    fi
}

main() {
    # Le sous-shell du tube hérite de « set +e » ci-dessous : on remet l'arrêt à
    # la première erreur ici.
    set -euo pipefail
    case "$family" in
        debian)
            install_debian
            versions
            build_install_debian
            ;;
        redhat)
            install_redhat
            versions
            build_install_redhat
            ;;
        *)
            echo "usage : $0 debian|redhat" >&2
            exit 2
            ;;
    esac
    run_installed
}

set +e
main 2>&1 | tee "$log"
status="${PIPESTATUS[0]}"
set -e

if [ "$status" -eq 0 ]; then
    annotate notice "Distribution essayée"
else
    annotate error "Distribution en échec"
fi
exit "$status"

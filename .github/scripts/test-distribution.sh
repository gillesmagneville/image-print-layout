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
# sauts de ligne y sont codés). GitHub n'affiche qu'environ 4 000 caractères
# d'une annotation : on garde la fin du journal, en erreur, et le résumé seul
# quand tout a réussi.
annotate() {
    local text
    if [ "$1" = notice ]; then
        text="$(cat "$summary")"
    else
        text="$(tail -c 3500 "$log")"
    fi
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
        fonts-dejavu-core adwaita-icon-theme xvfb xauth imagemagick
    # IBM Plex est la police du jeu d'essai, mais Debian ne la fournit pas
    # partout ; le test n'en dépend pas (une police de remplacement suffit).
    apt-get install -y --no-install-recommends fonts-ibm-plex || echo "(fonts-ibm-plex absent de cette distribution)"
}

build_install_debian() {
    echo o | ./build-deb.sh --no-version-change
    note "Depends : $(dpkg-deb -f ./svgprintlayout_*.deb Depends)"
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
    # L'écran virtuel est nécessaire pour essayer la fenêtre ; l'outil de
    # capture est facultatif. Un par un, pour que le journal dise lequel manque.
    local package
    for package in xorg-x11-server-Xvfb xorg-x11-xauth ImageMagick; do
        dnf -y install "$package" || note "paquet introuvable dans les dépôts : $package"
    done
}

build_install_redhat() {
    echo o | ./build-rpm.sh --no-version-change
    note "Requires : $(rpm -qpR ./svgprintlayout-*.rpm | grep -v 'GLIBC\|^rpmlib\|^rtld\|^/bin/sh' | sed 's/()(64bit)//' | sort -u | tr '\n' ' ')"
    dnf -y install ./svgprintlayout-*.rpm
}

# Les lignes du résumé, relues à la fin pour que l'annotation les montre.
summary="${RUNNER_TEMP:-/tmp}/distribution-summary.txt"
: > "$summary"
note() { echo "$*" | tee -a "$summary"; }

versions() {
    note "$(. /etc/os-release && echo "$PRETTY_NAME")"
    note "go $(go version | awk '{print $3}'), gcc $(gcc -dumpversion)"
    note "gtk4 $(pkg-config --modversion gtk4), glib $(pkg-config --modversion glib-2.0), pango $(pkg-config --modversion pango), librsvg $(pkg-config --modversion librsvg-2.0), cairo $(pkg-config --modversion cairo)"
}

# Sans Xvfb (RHEL 10 et ses clones n'ont plus de serveur X), GTK 4 sait aussi
# afficher ses fenêtres avec son pilote Broadway : il n'a besoin ni d'écran ni
# de navigateur pour les ouvrir. On y essaie la fenêtre, sans capture.
window_broadway() {
    local daemon lang pid
    export GDK_BACKEND=broadway BROADWAY_DISPLAY=:5
    gtk4-broadwayd :5 > /dev/null 2>&1 &
    daemon=$!
    sleep 1
    for lang in en fr es de; do
        rm -f "$PWD/broadway-$lang.ready"
        HOME="$(mktemp -d)" svgprintlayout -lang "$lang" -ready-file "$PWD/broadway-$lang.ready" \
            2> "$PWD/broadway-$lang.log" &
        pid=$!
        for _ in $(seq 1 100); do
            [ -e "$PWD/broadway-$lang.ready" ] && break
            kill -0 "$pid" 2> /dev/null || break
            sleep 0.2
        done
        if [ ! -e "$PWD/broadway-$lang.ready" ]; then
            echo "la fenêtre ne s'est pas affichée en $lang ; journal du programme :"
            cat "$PWD/broadway-$lang.log"
            kill "$pid" "$daemon" 2> /dev/null || true
            return 1
        fi
        kill "$pid" 2> /dev/null || true
        wait "$pid" 2> /dev/null || true
    done
    kill "$daemon" 2> /dev/null || true
    unset GDK_BACKEND BROADWAY_DISPLAY
}

# Le programme installé, comme le lancerait une personne.
run_installed() {
    echo "--- programme installé"
    svgprintlayout -version
    svgprintlayout -selftest
    note "-selftest : réussi"
    if command -v xvfb-run > /dev/null; then
        mkdir -p "$screenshots"
        for lang in en fr es de; do
            .github/scripts/screenshot.sh /usr/bin/svgprintlayout "$lang" "$screenshots/$lang.png"
        done
        note "fenêtre : ouverte dans les quatre langues ($(command -v import > /dev/null && echo "avec captures" || echo "sans capture : ImageMagick absent"))"
    elif command -v gtk4-broadwayd > /dev/null; then
        window_broadway
        note "fenêtre : ouverte dans les quatre langues (pilote Broadway de GTK : pas de xvfb-run sur cette distribution, pas de capture)"
    else
        note "fenêtre : NON ESSAYÉE (ni xvfb-run ni gtk4-broadwayd sur cette distribution)"
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
    echo "=== RÉSUMÉ ==="
    cat "$summary"
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

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
# Avec « refus » en second argument, la distribution est connue pour être trop
# ancienne (GTK < 4.14, GLib < 2.80) : le test vérifie que le script de
# construction refuse de compiler, avec un message qui dit pourquoi, au lieu
# des centaines d'erreurs de cgo.
#
# Usage : test-distribution.sh debian|redhat [refus]

set -euo pipefail

family="${1:-}"
expectation="${2:-}"

# Après setup-go, le lanceur de la CI donne à certains conteneurs (Rocky Linux
# 10) un PATH sans les dossiers système : on les remet, derrière le reste.
PATH="$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export PATH

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
        librsvg2-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev \
        fonts-dejavu-core adwaita-icon-theme xvfb xauth imagemagick poppler-utils
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
        librsvg2-devel cairo-devel fontconfig-devel gtk4-devel gobject-introspection-devel \
        dejavu-sans-fonts adwaita-icon-theme poppler-utils
    # L'écran virtuel est nécessaire pour essayer la fenêtre ; l'outil de
    # capture est facultatif. Un par un, pour que le journal dise lequel manque.
    local package
    for package in xorg-x11-server-Xvfb xorg-x11-xauth ImageMagick; do
        dnf -y install "$package" || note "paquet introuvable dans les dépôts : $package"
    done
    # RHEL 10 et ses clones n'ont plus Xvfb : un compositeur Wayland sans écran
    # (weston, lancé par wlheadless-run) en tient lieu, sans capture.
    command -v xvfb-run > /dev/null || dnf -y install xwayland-run weston || note "xwayland-run ou weston introuvable dans les dépôts"
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

# Distribution trop ancienne : le script de construction doit refuser.
expect_refusal() {
    local script out status=0
    case "$family" in
        debian) script=./build-deb.sh ;;
        *) script=./build-rpm.sh ;;
    esac
    out="$(echo o | "$script" --no-version-change 2>&1)" || status=$?
    echo "$out"
    if [ "$status" -ne 0 ] && grep -q "trop anciennes" <<< "$out"; then
        note "construction refusée, comme prévu :"
        note "$(grep -A2 "trop anciennes" <<< "$out")"
    else
        note "ERREUR : la construction devait être refusée pour bibliothèques trop anciennes (code $status)"
        return 1
    fi
}

# Le programme installé, comme le lancerait une personne.
run_installed() {
    echo "--- programme installé"
    svgprintlayout -version
    svgprintlayout -selftest
    note "-selftest : réussi"
    if command -v xvfb-run > /dev/null || command -v wlheadless-run > /dev/null; then
        mkdir -p "$screenshots"
        for lang in en fr es de; do
            .github/scripts/screenshot.sh /usr/bin/svgprintlayout "$lang" "$screenshots/$lang.png"
        done
        if command -v xvfb-run > /dev/null; then
            note "fenêtre : ouverte dans les quatre langues, sur Xvfb ($(command -v import > /dev/null && echo "avec captures" || echo "sans capture : ImageMagick absent"))"
        else
            note "fenêtre : ouverte dans les quatre langues, sur un compositeur Wayland sans écran (weston) : pas de capture"
        fi
    else
        note "fenêtre : NON ESSAYÉE (ni xvfb-run ni wlheadless-run sur cette distribution)"
    fi
}

# Les 14 tests d'acceptation du cahier des charges (§13), sur cette
# distribution : le Mini-CAD synthétique, A4 portrait, 200 mm, des coupures, un
# fond blanc, le PDF rastérisé par poppler, puis le projet rouvert.
run_acceptance() {
    echo "--- tests d'acceptation"
    go test -count=1 ./internal/acceptance
    note "tests d'acceptation 1 à 14 : réussis"
}

main() {
    case "$family" in
        debian)
            install_debian
            versions
            if [ "$expectation" = refus ]; then
                expect_refusal
                return
            fi
            build_install_debian
            ;;
        redhat)
            install_redhat
            versions
            if [ "$expectation" = refus ]; then
                expect_refusal
                return
            fi
            build_install_redhat
            ;;
        *)
            echo "usage : $0 debian|redhat" >&2
            exit 2
            ;;
    esac
    run_installed
    run_acceptance
}

everything() {
    # Le sous-shell du tube hérite de « set +e » ci-dessous : on remet l'arrêt à
    # la première erreur ici. « main » est appelée seule (pas dans un « && »,
    # qui désactiverait « set -e » dans toute la fonction).
    set -euo pipefail
    main
    echo "=== RÉSUMÉ ==="
    cat "$summary"
}

set +e
everything 2>&1 | tee "$log"
status="${PIPESTATUS[0]}"
set -e

if [ "$status" -eq 0 ]; then
    annotate notice "Distribution essayée"
else
    annotate error "Distribution en échec"
fi
exit "$status"

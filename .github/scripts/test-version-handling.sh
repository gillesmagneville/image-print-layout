#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
set -euo pipefail

# Vérifie, pour build-deb.sh et build-rpm.sh, la gestion du fichier VERSION :
#   - le calcul de la version suivante (le même que build-windows.ps1) ;
#   - une confirmation refusée ne change rien ;
#   - le menu interactif et les options --minor / --patch écrivent la nouvelle
#     version dans VERSION, une fois le paquet construit, et nommé d'après elle.
# Les paquets et VERSION sont remis dans leur état d'origine à la fin.
#
# À lancer à la racine du dépôt, après les autres vérifications : le script
# reconstruit et supprime les paquets du dépôt.

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

before="$(tr -d '[:space:]' < VERSION)"
restore() {
    printf '%s\n' "$before" > VERSION
    ./build-deb.sh --clean > /dev/null
    ./build-rpm.sh --clean > /dev/null
}
trap restore EXIT

fail() { echo "ÉCHEC : $*" >&2; exit 1; }

# --- 1. Calcul de la version suivante -------------------------------------
# La fonction est lue dans build-deb.sh : un test de repocheck garantit
# qu'elle est identique dans build-rpm.sh.
eval "$(sed -n '/^next_version() {/,/^}/p' build-deb.sh)"
check() {
    local got
    got="$(next_version "$1" "$2")"
    [ "$got" = "$3" ] || fail "next_version $1 $2 donne $got, attendu $3"
}
check 1.2.3 major 2.0.0
check 1.2.3 minor 1.3.0
check 1.2.3 patch 1.2.4
check 0.1.0 major 1.0.0
check 0.1.0 minor 0.2.0
check 0.1.0 patch 0.1.1
check 0.1.0-dev major 1.0.0
check 0.1.0-dev minor 0.1.0
check 0.1.0-dev patch 0.1.0
check 0.1.2-dev patch 0.1.2
check 0.1.2-dev minor 0.2.0
check 1.0.0-rc.1 major 1.0.0
check 1.0.0-rc.1 minor 1.0.0
check 10.9.99 patch 10.9.100
echo "ok  calcul des versions"

# --- 2. Confirmation refusée : rien ne change -----------------------------
packages_before="$(cat ./*.deb ./*.rpm | sha256sum)"
echo n | ./build-deb.sh --minor > /dev/null
echo n | ./build-rpm.sh --patch > /dev/null
[ "$(tr -d '[:space:]' < VERSION)" = "$before" ] || fail "VERSION a changé malgré le refus"
[ "$(cat ./*.deb ./*.rpm | sha256sum)" = "$packages_before" ] || fail "les paquets ont changé malgré le refus"
echo "ok  confirmation refusée"

# --- 3. Option --patch : VERSION écrite après la construction du .deb ------
expected="$(next_version "$before" patch)"
echo o | ./build-deb.sh --patch > /dev/null
[ "$(cat VERSION)" = "$expected" ] || fail "VERSION vaut « $(cat VERSION) » après --patch, attendu « $expected »"
[ "$(wc -l < VERSION)" -eq 1 ] || fail "VERSION doit tenir sur une ligne"
ls ./svgprintlayout_"$expected"_*.deb > /dev/null || fail "paquet .deb de la version $expected introuvable"
echo "ok  --patch (deb) : $before -> $expected"

# --- 4. Menu interactif : choix 2 (minor) puis confirmation ---------------
after_deb="$expected"
expected="$(next_version "$after_deb" minor)"
printf '2\no\n' | ./build-rpm.sh > /dev/null
[ "$(cat VERSION)" = "$expected" ] || fail "VERSION vaut « $(cat VERSION) » après le choix 2, attendu « $expected »"
ls ./svgprintlayout-"${expected%%-*}"-*.rpm > /dev/null || fail "paquet .rpm de la version $expected introuvable"
echo "ok  menu interactif (rpm) : $after_deb -> $expected"

# --- 5. --no-version-change : VERSION inchangée ---------------------------
echo o | ./build-rpm.sh --no-version-change > /dev/null
[ "$(cat VERSION)" = "$expected" ] || fail "VERSION a changé avec --no-version-change"
echo "ok  --no-version-change"

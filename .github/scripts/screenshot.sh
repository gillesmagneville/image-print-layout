#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
#
# Lance le programme sur un écran virtuel, attend que la fenêtre soit affichée
# (option -ready-file du programme), enregistre une capture de l'écran et ferme
# le programme. Échoue, avec le journal du programme, si la fenêtre n'apparaît
# pas en vingt secondes.
#
# L'écran virtuel est Xvfb quand xvfb-run existe ; sinon un compositeur Wayland
# sans écran (wlheadless-run et weston : RHEL 10 et ses clones n'ont plus de
# serveur X), où il n'y a pas de capture.
#
# Usage : screenshot.sh PROGRAMME LANGUE CAPTURE.png [FICHIER.svg]

set -euo pipefail

[ $# -ge 3 ] || { echo "usage : $0 PROGRAMME LANGUE CAPTURE.png [FICHIER.svg]" >&2; exit 2; }

program=$1
lang=$2
out=$3
svg=${4:-}

# Un dossier personnel neuf : les préférences de la personne qui lance le
# script ne changent pas la capture, et le script ne touche pas à ses fichiers.
HOME="$(mktemp -d)"
export HOME
export NO_AT_BRIDGE=1 GSK_RENDERER=cairo GTK_A11Y=none LIBGL_ALWAYS_SOFTWARE=1

rm -f "$out" "$out.ready" "$out.log"

# La capture demande un serveur X et ImageMagick (import).
capture=non
if command -v xvfb-run > /dev/null; then
    display=(xvfb-run -a -s "-screen 0 1300x820x24")
    command -v import > /dev/null && capture=oui
else
    display=(wlheadless-run -c weston --)
    export GDK_BACKEND=wayland
fi

"${display[@]}" bash -c '
    program=$1; lang=$2; out=$3; svg=$4; capture=$5
    if [ -n "$svg" ]; then
        "$program" -lang "$lang" -ready-file "$out.ready" "$svg" 2> "$out.log" &
    else
        "$program" -lang "$lang" -ready-file "$out.ready" 2> "$out.log" &
    fi
    pid=$!
    for _ in $(seq 1 100); do
        [ -e "$out.ready" ] && break
        kill -0 "$pid" 2>/dev/null || break
        sleep 0.2
    done
    status=0
    if [ -e "$out.ready" ]; then
        # La fenêtre est affichée, ce qui est déjà le but du test.
        if [ "$capture" = oui ]; then
            sleep 0.5
            import -window root "$out" || status=$?
        else
            : > "$out"
        fi
    else
        echo "la fenêtre ne s'"'"'est pas affichée ; journal du programme :" >&2
        cat "$out.log" >&2
        status=1
    fi
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    exit "$status"
' _ "$program" "$lang" "$out" "$svg" "$capture"

if [ "$capture" = oui ]; then
    [ -s "$out" ] || { echo "capture vide : $out" >&2; exit 1; }
    echo "capture : $out"
else
    rm -f "$out"
    echo "fenêtre affichée (sans capture)"
fi

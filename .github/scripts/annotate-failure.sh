#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
#
# Lance une commande ; si elle échoue, montre l'essentiel de sa sortie dans des
# annotations de la vérification (les journaux complets ne sont pas lisibles
# hors de GitHub). Une annotation ne montre qu'environ 4 000 caractères : un
# bloc pour les échecs, un pour les lignes DEBUGINFO.
#
# Usage : annotate-failure.sh commande [arguments...]

out="$(mktemp)"
"$@" 2>&1 | tee "$out"
status="${PIPESTATUS[0]}"

encode() {
    local text="$1"
    text="${text//'%'/'%25'}"
    text="${text//$'\r'/'%0D'}"
    text="${text//$'\n'/'%0A'}"
    printf '%s' "$text"
}

if [ "$status" -ne 0 ]; then
    failed="$(grep -E -- '--- FAIL|^FAIL|^panic|_test\.go:[0-9]+:|Failed:' "$out" | grep -v 'DEBUGINFO' | cut -c1-300 | head -c 3500)"
    echo "::error title=Tests en échec ($1 ...)::$(encode "$failed")"
    debug="$(grep 'DEBUGINFO' "$out" | cut -c1-400 | head -c 3500)"
    if [ -n "$debug" ]; then
        echo "::warning title=Sortie de débogage::$(encode "$debug")"
    fi
    echo "::notice title=Bibliothèque d'images::$(encode "$(pkg-config --modversion gdk-pixbuf-2.0 2>/dev/null) ; $(ls "$(pkg-config --variable=gdk_pixbuf_moduledir gdk-pixbuf-2.0 2>/dev/null)" 2>/dev/null | tr '\n' ' ')")"
fi
rm -f "$out"
exit "$status"

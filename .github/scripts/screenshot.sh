#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
#
# Runs the program on a virtual screen, waits for the window to be displayed
# (the program's -ready-file option), saves a capture of the screen and closes
# the program. Fails, with the program's log, if the window does not appear
# within twenty seconds.
#
# The virtual screen is Xvfb when xvfb-run exists; otherwise a headless Wayland
# compositor (wlheadless-run and weston: RHEL 10 and its clones no longer have
# an X server), where there is no capture.
#
# Usage: screenshot.sh PROGRAM LANGUAGE CAPTURE.png [FILE.svg]

set -euo pipefail

[ $# -ge 3 ] || { echo "usage: $0 PROGRAM LANGUAGE CAPTURE.png [FILE.svg]" >&2; exit 2; }

program=$1
lang=$2
out=$3
svg=${4:-}

# A brand-new home folder: the preferences of the person who runs the
# script do not change the capture, and the script does not touch their files.
HOME="$(mktemp -d)"
export HOME
export NO_AT_BRIDGE=1 GSK_RENDERER=cairo GTK_A11Y=none LIBGL_ALWAYS_SOFTWARE=1

rm -f "$out" "$out.ready" "$out.log"

# The capture needs an X server and ImageMagick (import).
capture=no
if command -v xvfb-run > /dev/null; then
    display=(xvfb-run -a -s "-screen 0 1300x820x24")
    command -v import > /dev/null && capture=yes
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
        # The window is displayed, which is already the goal of the test.
        if [ "$capture" = yes ]; then
            sleep 0.5
            import -window root "$out" || status=$?
        else
            : > "$out"
        fi
    else
        echo "the window did not appear; program log:" >&2
        cat "$out.log" >&2
        status=1
    fi
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    exit "$status"
' _ "$program" "$lang" "$out" "$svg" "$capture"

if [ "$capture" = yes ]; then
    [ -s "$out" ] || { echo "empty capture: $out" >&2; exit 1; }
    echo "capture: $out"
else
    rm -f "$out"
    echo "window displayed (no capture)"
fi

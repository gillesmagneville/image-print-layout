#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
#
# Steps of the "distributions" job of the CI (.github/workflows/ci.yml), which
# runs in a container of the distribution to try. The script installs
# the build tools with the distribution's package manager,
# builds the package of its family (.deb or .rpm) with the script of the repository,
# installs it as a person would (apt or dnf, which pull in GTK 4 and the
# other dependencies), then runs the installed program: -version, -selftest and
# the window on a virtual screen.
#
# A package is tied to the libraries of the machine that built it: it is
# therefore built on the distribution where it is installed.
#
# The end of the log is copied into an annotation of the run, which can be
# read without downloading the logs: as an error if a step fails, as a plain
# note otherwise (library versions, package dependencies).
#
# With "refusal" as second argument, the distribution is known to be too
# old (GTK < 4.14, GLib < 2.80): the test checks that the build script
# refuses to compile, with a message saying why, instead of the hundreds
# of cgo errors.
#
# Usage: test-distribution.sh debian|redhat [refusal]

set -euo pipefail

family="${1:-}"
expectation="${2:-}"

# After setup-go, the CI runner gives some containers (Rocky Linux
# 10) a PATH without the system folders: we put them back, after the rest.
PATH="$PATH:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export PATH

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
log="${RUNNER_TEMP:-/tmp}/distribution.log"
screenshots="$PWD/screenshots"

# Writes the end of the log into a GitHub annotation (%, carriage returns and
# line feeds are encoded in it). GitHub only shows about 4,000 characters
# of an annotation: we keep the end of the log, on error, and the summary alone
# when everything succeeded.
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
        librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev \
        fonts-dejavu-core adwaita-icon-theme xvfb xauth imagemagick poppler-utils
    # IBM Plex is the font of the test set, but Debian does not provide it
    # everywhere; the test does not depend on it (a replacement font is enough).
    apt-get install -y --no-install-recommends fonts-ibm-plex || echo "(fonts-ibm-plex not available in this distribution)"
}

build_install_debian() {
    echo y | ./build-deb.sh --no-version-change
    note "Depends : $(dpkg-deb -f ./imageprintlayout_*.deb Depends)"
    apt-get install -y ./imageprintlayout_*.deb
}

# A faulty EPEL mirror (wrong certificate or checksum) makes every dnf
# transaction fail, and it is not the program we are trying: if the failure
# comes from downloading the metadata, we start again from fresh metadata,
# so that another mirror is chosen, before concluding. A package that is
# simply absent from the repositories fails right away, without a new attempt.
dnf_retry() {
    local try log status
    log=$(mktemp)
    for try in 1 2 3; do
        status=0
        dnf -y "$@" > "$log" 2>&1 || status=$?
        cat "$log"
        if [ "$status" -eq 0 ]; then
            rm -f "$log"
            return 0
        fi
        grep -q 'downloading metadata\|download metadata\|Cannot download' "$log" || break
        note "dnf $1: metadata unavailable (attempt $try of 3), trying again"
        dnf clean metadata > /dev/null 2>&1 || true
        sleep 5
    done
    rm -f "$log"
    return 1
}

install_redhat() {
    # The GTK and GObject Introspection development packages are in the CRB
    # repository ("PowerTools" before version 9) of the RHEL clones; neither
    # CRB nor EPEL exists on Fedora, hence the "|| true".
    dnf_retry install dnf-plugins-core epel-release || true
    dnf config-manager --set-enabled crb || dnf config-manager --set-enabled powertools || true
    dnf_retry install gcc make pkgconf-pkg-config git tar gzip findutils which rpm-build \
        librsvg2-devel gdk-pixbuf2-devel cairo-devel fontconfig-devel gtk4-devel gobject-introspection-devel \
        dejavu-sans-fonts adwaita-icon-theme poppler-utils
    # The virtual screen is needed to try the window; the capture tool is
    # optional. One by one, so that the log says which one is missing.
    local package
    for package in xorg-x11-server-Xvfb xorg-x11-xauth ImageMagick; do
        dnf_retry install "$package" || note "package not found in the repositories: $package"
    done
    # RHEL 10 and its clones no longer have Xvfb: a headless Wayland compositor
    # (weston, started by wlheadless-run) replaces it, without a capture.
    command -v xvfb-run > /dev/null || dnf_retry install xwayland-run weston || note "xwayland-run or weston not found in the repositories"
}

build_install_redhat() {
    echo y | ./build-rpm.sh --no-version-change
    note "Requires : $(rpm -qpR ./imageprintlayout-*.rpm | grep -v 'GLIBC\|^rpmlib\|^rtld\|^/bin/sh' | sed 's/()(64bit)//' | sort -u | tr '\n' ' ')"
    dnf -y install ./imageprintlayout-*.rpm
}

# The summary lines, read again at the end so that the annotation shows them.
summary="${RUNNER_TEMP:-/tmp}/distribution-summary.txt"
: > "$summary"
note() { echo "$*" | tee -a "$summary"; }

versions() {
    note "$(. /etc/os-release && echo "$PRETTY_NAME")"
    note "go $(go version | awk '{print $3}'), gcc $(gcc -dumpversion)"
    note "gtk4 $(pkg-config --modversion gtk4), glib $(pkg-config --modversion glib-2.0), pango $(pkg-config --modversion pango), librsvg $(pkg-config --modversion librsvg-2.0), cairo $(pkg-config --modversion cairo)"
}

# Distribution too old: the build script must refuse.
expect_refusal() {
    local script out status=0
    case "$family" in
        debian) script=./build-deb.sh ;;
        *) script=./build-rpm.sh ;;
    esac
    out="$(echo y | "$script" --no-version-change 2>&1)" || status=$?
    echo "$out"
    if [ "$status" -ne 0 ] && grep -q "too old" <<< "$out"; then
        note "build refused, as expected:"
        note "$(grep -A2 "too old" <<< "$out")"
    else
        note "ERROR: the build should have been refused because of libraries that are too old (code $status)"
        return 1
    fi
}

# The installed program, as a person would run it.
run_installed() {
    echo "--- installed program"
    imageprintlayout -version
    imageprintlayout -selftest
    note "-selftest: passed"
    if command -v xvfb-run > /dev/null || command -v wlheadless-run > /dev/null; then
        mkdir -p "$screenshots"
        for lang in en fr es de; do
            .github/scripts/screenshot.sh /usr/bin/imageprintlayout "$lang" "$screenshots/$lang.png"
        done
        if command -v xvfb-run > /dev/null; then
            note "window: opened in the four languages, on Xvfb ($(command -v import > /dev/null && echo "with captures" || echo "without capture: ImageMagick missing"))"
        else
            note "window: opened in the four languages, on a headless Wayland compositor (weston): no capture"
        fi
    else
        note "window: NOT TRIED (neither xvfb-run nor wlheadless-run on this distribution)"
    fi
}

# The 14 acceptance tests of the specification (§13), on this
# distribution: the synthetic Mini-CAD, A4 portrait, 200 mm, cuts, a
# white background, the PDF rasterised by poppler, then the project reopened.
run_acceptance() {
    echo "--- acceptance tests"
    go test -count=1 ./internal/acceptance
    note "acceptance tests 1 to 14: passed"
}

main() {
    case "$family" in
        debian)
            install_debian
            versions
            if [ "$expectation" = refusal ]; then
                expect_refusal
                return
            fi
            build_install_debian
            ;;
        redhat)
            install_redhat
            versions
            if [ "$expectation" = refusal ]; then
                expect_refusal
                return
            fi
            build_install_redhat
            ;;
        *)
            echo "usage: $0 debian|redhat" >&2
            exit 2
            ;;
    esac
    run_installed
    run_acceptance
}

everything() {
    # The subshell of the pipe inherits the "set +e" below: we restore stopping at
    # the first error here. "main" is called alone (not in a "&&",
    # which would disable "set -e" in the whole function).
    set -euo pipefail
    main
    echo "=== SUMMARY ==="
    cat "$summary"
}

set +e
everything 2>&1 | tee "$log"
status="${PIPESTATUS[0]}"
set -e

if [ "$status" -eq 0 ]; then
    annotate notice "Distribution tried"
else
    annotate error "Distribution failed"
fi
exit "$status"

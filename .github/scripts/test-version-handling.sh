#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-only
set -euo pipefail

# Checks, for build-deb.sh and build-rpm.sh, the handling of the VERSION file:
#   - the computation of the next version (the same as build-windows.ps1);
#   - a refused confirmation changes nothing;
#   - the interactive menu and the --minor / --patch options write the new
#     version into VERSION, once the package is built, and named after it.
# The packages and VERSION are put back in their original state at the end.
#
# To be run at the root of the repository, after the other checks: the script
# rebuilds and deletes the packages of the repository.

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

before="$(tr -d '[:space:]' < VERSION)"
restore() {
    printf '%s\n' "$before" > VERSION
    ./build-deb.sh --clean > /dev/null
    ./build-rpm.sh --clean > /dev/null
}
trap restore EXIT

fail() { echo "FAILED: $*" >&2; exit 1; }

# --- 1. Computation of the next version -----------------------------------
# The function is read from build-deb.sh: a repocheck test guarantees that
# it is identical in build-rpm.sh.
eval "$(sed -n '/^next_version() {/,/^}/p' build-deb.sh)"
check() {
    local got
    got="$(next_version "$1" "$2")"
    [ "$got" = "$3" ] || fail "next_version $1 $2 gives $got, expected $3"
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
echo "ok  version computation"

# --- 2. Refused confirmation: nothing changes -----------------------------
packages_before="$(cat ./*.deb ./*.rpm | sha256sum)"
echo n | ./build-deb.sh --minor > /dev/null
echo n | ./build-rpm.sh --patch > /dev/null
[ "$(tr -d '[:space:]' < VERSION)" = "$before" ] || fail "VERSION changed despite the refusal"
[ "$(cat ./*.deb ./*.rpm | sha256sum)" = "$packages_before" ] || fail "the packages changed despite the refusal"
echo "ok  confirmation refused"

# --- 3. --patch option: VERSION written after the .deb is built -----------
expected="$(next_version "$before" patch)"
echo y | ./build-deb.sh --patch > /dev/null
[ "$(cat VERSION)" = "$expected" ] || fail "VERSION is '$(cat VERSION)' after --patch, expected '$expected'"
[ "$(wc -l < VERSION)" -eq 1 ] || fail "VERSION must be on one line"
ls ./imageprintlayout_"$expected"_*.deb > /dev/null || fail ".deb package of version $expected not found"
echo "ok  --patch (deb) : $before -> $expected"

# --- 4. Interactive menu: choice 2 (minor) then confirmation --------------
after_deb="$expected"
expected="$(next_version "$after_deb" minor)"
printf '2\ny\n' | ./build-rpm.sh > /dev/null
[ "$(cat VERSION)" = "$expected" ] || fail "VERSION is '$(cat VERSION)' after choice 2, expected '$expected'"
ls ./imageprintlayout-"${expected%%-*}"-*.rpm > /dev/null || fail ".rpm package of version $expected not found"
echo "ok  interactive menu (rpm): $after_deb -> $expected"

# --- 5. --no-version-change: VERSION unchanged ----------------------------
echo y | ./build-rpm.sh --no-version-change > /dev/null
[ "$(cat VERSION)" = "$expected" ] || fail "VERSION changed with --no-version-change"
echo "ok  --no-version-change"

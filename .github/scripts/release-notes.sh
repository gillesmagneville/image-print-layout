#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
#
# Writes the text of a GitHub release, in English then in French: for each
# language, the section of the CHANGELOG for the version (CHANGELOG.md for
# English, CHANGELOG.fr.md for French), then the list of the attached files
# with what they are. Used by the release.yml workflow.
#
# Usage: release-notes.sh VERSION [FILES_FOLDER]
#
# Without a folder, only the sections of the two CHANGELOGs are written (the
# workflow checks them before building). The text is written to standard output.

set -euo pipefail

version="${1:?usage: release-notes.sh VERSION [FILES_FOLDER]}"
files="${2:-}"
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

# The "## [X.Y.Z] - YYYY-MM-DD" section of CHANGELOG $1 up to the next one,
# without its title or its blank lines at the start and the end. Fails if it is missing.
changelog_section() {
    local file="$1" section
    section="$(awk -v version="$version" '
        $0 ~ "^## \\[" version "\\] - " { found = 1; next }
        found && /^## \[/ { exit }
        found { print }
    ' "$file")"

    if [ -z "$(tr -d '[:space:]' <<< "$section")" ]; then
        echo "release-notes.sh: $file has no '## [$version] - YYYY-MM-DD' section with text" >&2
        return 1
    fi

    # Remove the blank lines at the start and at the end.
    printf '%s\n' "$section" | sed -e '/./,$!d' | sed -e ':a' -e '/^\n*$/{$d;N;ba' -e '}'
}

# The list of the attached files, in language $1 (en or fr).
files_list() {
    local lang="$1" path name what checksum
    for path in "$files"/imageprintlayout*; do
        [ -f "$path" ] || continue
        name="$(basename "$path")"
        # The checksums are told with the package they verify.
        case "$name" in *.sha256) continue ;; esac
        case "$lang:$name" in
            en:*.deb)                  what="Ubuntu 24.04 and later, Debian 13: \`sudo apt install ./$name\`" ;;
            fr:*.deb)                  what="Ubuntu 24.04 et suivants, Debian 13 : \`sudo apt install ./$name\`" ;;
            en:*.rpm)                  what="RHEL 10 and its clones, Fedora: \`sudo dnf install ./$name\`" ;;
            fr:*.rpm)                  what="RHEL 10 et ses clones, Fedora : \`sudo dnf install ./$name\`" ;;
            en:*_windows-setup.exe)    what="Windows 10 and 11, 64-bit: installer. It is not signed: SmartScreen shows \"Windows protected your PC\", click \"More info\" then \"Run anyway\"" ;;
            fr:*_windows-setup.exe)    what="Windows 10 et 11, 64 bits : installeur. Il n'est pas signé : SmartScreen affiche « Windows a protégé votre PC », cliquez sur « Informations complémentaires » puis « Exécuter quand même »" ;;
            en:*_windows-portable.zip) what="Windows 10 and 11, 64-bit: portable version, to be unzipped anywhere" ;;
            fr:*_windows-portable.zip) what="Windows 10 et 11, 64 bits : version portable, à décompresser n'importe où" ;;
            *)                         what="" ;;
        esac
        checksum=""
        if [ -f "$path.sha256" ]; then
            case "$lang" in
                en) checksum=" (checksum: \`$name.sha256\`)" ;;
                fr) checksum=" (empreinte : \`$name.sha256\`)" ;;
            esac
        fi
        if [ -n "$what" ]; then
            case "$lang" in
                en) echo "- \`$name\`: $what$checksum" ;;
                fr) echo "- \`$name\` : $what$checksum" ;;
            esac
        else
            echo "- \`$name\`$checksum"
        fi
    done
}

# The two sections are read before anything is written: if one is missing,
# nothing is written.
english="$(changelog_section CHANGELOG.md)"
french="$(changelog_section CHANGELOG.fr.md)"

printf '%s\n' "$english"

if [ -n "$files" ]; then
    echo
    echo "## Files"
    echo
    files_list en
    echo
    echo "Each package has its \`.sha256\` file. To verify a downloaded package, put it in the same folder as its \`.sha256\` file and run \`sha256sum -c <package>.sha256\` (Linux), or compare the result of \`Get-FileHash -Algorithm SHA256 <package>\` (PowerShell) with the content of the file."
fi

echo
echo "---"
echo
echo "## Version française"
echo
printf '%s\n' "$french"

if [ -n "$files" ]; then
    echo
    echo "## Fichiers"
    echo
    files_list fr
    echo
    echo "Chaque paquet a son fichier \`.sha256\`. Pour vérifier un paquet téléchargé, mettez-le dans le même dossier que son fichier \`.sha256\` et lancez \`sha256sum -c <paquet>.sha256\` (Linux) ou comparez le résultat de \`Get-FileHash -Algorithm SHA256 <paquet>\` (PowerShell) avec le contenu du fichier."
fi

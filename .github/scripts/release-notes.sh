#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
#
# Écrit le texte d'une release GitHub, en anglais puis en français : pour
# chaque langue, la section du CHANGELOG de la version (CHANGELOG.md pour
# l'anglais, CHANGELOG.fr.md pour le français), puis la liste des fichiers
# joints avec ce qu'ils sont. Utilisé par le workflow release.yml.
#
# Usage : release-notes.sh VERSION [DOSSIER_DES_FICHIERS]
#
# Sans dossier, seules les sections des deux CHANGELOG sont écrites (le workflow
# les vérifie avant de construire). Le texte est écrit sur la sortie standard.

set -euo pipefail

version="${1:?usage : release-notes.sh VERSION [DOSSIER_DES_FICHIERS]}"
files="${2:-}"
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

# La section « ## [X.Y.Z] - AAAA-MM-JJ » du CHANGELOG $1 jusqu'à la suivante,
# sans son titre ni ses lignes vides du début et de la fin. Échoue si elle manque.
changelog_section() {
    local file="$1" section
    section="$(awk -v version="$version" '
        $0 ~ "^## \\[" version "\\] - " { found = 1; next }
        found && /^## \[/ { exit }
        found { print }
    ' "$file")"

    if [ -z "$(tr -d '[:space:]' <<< "$section")" ]; then
        echo "release-notes.sh : $file n'a pas de section « ## [$version] - AAAA-MM-JJ » avec du texte" >&2
        return 1
    fi

    # Retire les lignes vides du début et de la fin.
    printf '%s\n' "$section" | sed -e '/./,$!d' | sed -e ':a' -e '/^\n*$/{$d;N;ba' -e '}'
}

# La liste des fichiers joints, dans la langue $1 (en ou fr).
files_list() {
    local lang="$1" path name what checksum
    for path in "$files"/imageprintlayout*; do
        [ -f "$path" ] || continue
        name="$(basename "$path")"
        # Les empreintes sont dites avec le paquet qu'elles vérifient.
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

# Les deux sections sont lues avant d'écrire quoi que ce soit : il en manque une,
# rien n'est écrit.
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

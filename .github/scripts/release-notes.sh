#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
#
# Écrit le texte d'une release GitHub : la section du CHANGELOG de la version,
# puis la liste des fichiers joints avec ce qu'ils sont. Utilisé par le
# workflow release.yml.
#
# Usage : release-notes.sh VERSION [DOSSIER_DES_FICHIERS]
#
# Sans dossier, seule la section du CHANGELOG est écrite (le workflow la
# vérifie avant de construire). Le texte est écrit sur la sortie standard.

set -euo pipefail

version="${1:?usage : release-notes.sh VERSION [DOSSIER_DES_FICHIERS]}"
files="${2:-}"
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

# La section « ## [X.Y.Z] - AAAA-MM-JJ » jusqu'à la suivante, sans son titre.
section="$(awk -v version="$version" '
    $0 ~ "^## \\[" version "\\] - " { found = 1; next }
    found && /^## \[/ { exit }
    found { print }
' CHANGELOG.md)"

if [ -z "$(tr -d '[:space:]' <<< "$section")" ]; then
    echo "release-notes.sh : CHANGELOG.md n'a pas de section « ## [$version] - AAAA-MM-JJ » avec du texte" >&2
    exit 1
fi

# Retire les lignes vides du début et de la fin.
printf '%s\n' "$section" | sed -e '/./,$!d' | sed -e ':a' -e '/^\n*$/{$d;N;ba' -e '}'

[ -n "$files" ] || exit 0

echo
echo "## Fichiers"
echo
for path in "$files"/svgprintlayout*; do
    [ -f "$path" ] || continue
    name="$(basename "$path")"
    # Les empreintes sont dites avec le paquet qu'elles vérifient.
    case "$name" in *.sha256) continue ;; esac
    case "$name" in
        *.deb)                  what="Ubuntu 24.04 et suivants, Debian 13 : \`sudo apt install ./$name\`" ;;
        *.rpm)                  what="RHEL 10 et ses clones, Fedora : \`sudo dnf install ./$name\`" ;;
        *_windows-setup.exe)    what="Windows 10 et 11, 64 bits : installeur. Il n'est pas signé : SmartScreen affiche « Windows a protégé votre PC », cliquez sur « Informations complémentaires » puis « Exécuter quand même »" ;;
        *_windows-portable.zip) what="Windows 10 et 11, 64 bits : version portable, à décompresser n'importe où" ;;
        *)                      what="" ;;
    esac
    echo "- \`$name\`${what:+ : $what}$([ -f "$path.sha256" ] && echo " (empreinte : \`$name.sha256\`)")"
done
echo
echo "Chaque paquet a son fichier \`.sha256\`. Pour vérifier un paquet téléchargé, mettez-le dans le même dossier que son fichier \`.sha256\` et lancez \`sha256sum -c <paquet>.sha256\` (Linux) ou comparez le résultat de \`Get-FileHash -Algorithm SHA256 <paquet>\` (PowerShell) avec le contenu du fichier."

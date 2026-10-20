# SPDX-License-Identifier: GPL-3.0-only
#Requires -Version 5.1
<#
.SYNOPSIS
    Relais vers windows\build-windows.ps1, pour un usage symétrique à
    build-deb.sh et build-rpm.sh (qui se lancent depuis la racine du dépôt).

.DESCRIPTION
    Ce script ne fait que transmettre ses arguments à windows\build-windows.ps1,
    où vit la logique de construction (voir windows\README.md : prérequis,
    options, dépannage). Rien n'est dupliqué : ce fichier reste à jour tout
    seul, puisqu'il ne fait qu'appeler l'autre script.

.EXAMPLE
    .\build-windows.ps1
    .\build-windows.ps1 -Major
    .\build-windows.ps1 -NoVersionChange
    .\build-windows.ps1 -Clean
    .\build-windows.ps1 -Help
#>

& "$PSScriptRoot\windows\build-windows.ps1" @args
exit $LASTEXITCODE

# SPDX-License-Identifier: GPL-3.0-only
#Requires -Version 5.1
<#
.SYNOPSIS
    Relais vers packaging\windows\build-windows.ps1, pour un usage symétrique à
    build-deb.sh et build-rpm.sh (qui se lancent depuis la racine du dépôt).

.DESCRIPTION
    Ce script ne fait que transmettre ses arguments à
    packaging\windows\build-windows.ps1, où vit la logique de construction
    (voir packaging\windows\README.md : prérequis, options, dépannage).

.EXAMPLE
    .\build-windows.ps1
    .\build-windows.ps1 -SansInstalleur
    .\build-windows.ps1 -Clean
    .\build-windows.ps1 -Help
#>

& "$PSScriptRoot\packaging\windows\build-windows.ps1" @args
exit $LASTEXITCODE

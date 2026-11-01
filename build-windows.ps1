# SPDX-License-Identifier: GPL-3.0-only
#Requires -Version 5.1
<#
.SYNOPSIS
    Relay to windows\build-windows.ps1, for a usage symmetrical to
    build-deb.sh and build-rpm.sh (which are run from the root of the repository).

.DESCRIPTION
    This script only passes its arguments on to windows\build-windows.ps1,
    where the build logic lives (see windows\README.md: prerequisites,
    options, troubleshooting). Nothing is duplicated: this file stays up to date
    on its own, since it only calls the other script.

.EXAMPLE
    .\build-windows.ps1
    .\build-windows.ps1 -Major
    .\build-windows.ps1 -NoVersionChange
    .\build-windows.ps1 -Clean
    .\build-windows.ps1 -Help
#>

& "$PSScriptRoot\windows\build-windows.ps1" @args
exit $LASTEXITCODE

# SPDX-License-Identifier: GPL-3.0-only
#Requires -Version 5.1
<#
.SYNOPSIS
    Builds the Windows package of Image Print Layout: a portable version
    (.zip) and, if NSIS is available, an installer (.exe).

.DESCRIPTION
    Windows equivalent of build-deb.sh and build-rpm.sh, with the same options
    and the same handling of the VERSION file as the build-windows.ps1 of
    Planche-Contact. This script checks the prerequisites (MSYS2 and its
    packages, NSIS), offering to install what is missing, then runs
    windows\build-windows.sh in an MSYS2 UCRT64 terminal: that is where Go,
    the C compiler, GTK 4, librsvg and cairo for Windows are found.

    The VERSION file is only updated once the packages have been built.

    This file is UTF-8 with a byte order mark (BOM), which Windows
    PowerShell 5.1 requires to read accents; do not save it without a BOM.

.PARAMETER Major
    Increments the major version number (e.g. 1.2.3 -> 2.0.0)
.PARAMETER Minor
    Increments the minor version number (e.g. 1.2.3 -> 1.3.0)
.PARAMETER Patch
    Increments the patch version number (e.g. 1.2.3 -> 1.2.4)
.PARAMETER NoVersionChange
    Rebuilds the package without changing the version
.PARAMETER Clean
    Cleans the build folder and deletes the generated .zip/.exe files
.PARAMETER Yes
    Answers "yes" to the confirmation and to the offers to install
    MSYS2, its packages and NSIS: for unattended use (the CI)
.PARAMETER Help
    Shows the help
#>

param(
    [switch]$Major,
    [switch]$Minor,
    [switch]$Patch,
    [switch]$NoVersionChange,
    [switch]$Clean,
    [switch]$Yes,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

# ====================== CONFIGURATION ======================
$PackageName = "imageprintlayout"

# Location of the script: $MyInvocation.MyCommand.Path (falling back on
# $PSCommandPath) rather than $PSScriptRoot, which turned out to be unreliable
# when this script is called indirectly from the small relay script placed
# at the root of the repository (build-windows.ps1 -> windows\build-
# windows.ps1). This folder lives in windows\: the project is one level
# above.
$ScriptPath = $MyInvocation.MyCommand.Path
if (-not $ScriptPath) { $ScriptPath = $PSCommandPath }
$WindowsDir  = Split-Path -Parent $ScriptPath
$ProjectDir  = Split-Path -Parent $WindowsDir

$BuildRoot   = "$env:LOCALAPPDATA\imageprintlayout-build"
$VersionFile = "$ProjectDir\VERSION"
$BuildScript = "./windows/build-windows.sh"
# =============================================================

function Show-Help {
    @'
Usage: .\build-windows.ps1 [options]

Script that builds the Windows package for Image Print Layout.

OPTIONS:
  -Major              Increment the major number    (e.g. 1.2.3 -> 2.0.0)
  -Minor              Increment the minor number    (e.g. 1.2.3 -> 1.3.0)
  -Patch              Increment the patch number    (e.g. 1.2.3 -> 1.2.4)
  -NoVersionChange    Rebuild the package without changing the version
  -Clean              Clean the build folder and delete the generated packages
  -Yes                Answer "yes" to every question (unattended use)
  -Help               Show this help

Without an option -> interactive mode (asks major / minor / patch / no-version-change)

The VERSION file is only updated once the packages have been built. A
pre-release version (0.1.0-dev) is released by -Minor or -Patch
(-> 0.1.0); -Major gives 1.0.0.

The packages are created at the root of the repository:
  imageprintlayout_<version>_windows-portable.zip   portable version
  imageprintlayout_<version>_windows-setup.exe      installer (if NSIS is there)

Prerequisites, checked automatically (with an offer to install them if missing):
  - Windows 10 or 11, 64-bit
  - MSYS2 (https://www.msys2.org), in C:\msys64, and its Go, gcc, GTK 4,
    librsvg and cairo packages for the UCRT64 environment
  - NSIS (optional: only needed to generate the .exe installer)
'@
}

# A version is X.Y.Z, optionally followed by a pre-release suffix
# (X.Y.Z-dev).
$SemverPattern = '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'

# Get-NextVersion gives the next version. Without a suffix, it is the usual
# increment (1.2.3 -> 1.2.4). With a suffix, the pre-release version
# is released first: 0.1.0-dev -> 0.1.0 with -Minor or -Patch, since 0.1.0
# does not exist yet; -Major gives 1.0.0. Same rule as build-deb.sh.
function Get-NextVersion([string]$Version, [string]$Part) {
    $pre = ""
    $core = $Version
    $dash = $Version.IndexOf("-")
    if ($dash -ge 0) {
        $pre = $Version.Substring($dash + 1)
        $core = $Version.Substring(0, $dash)
    }
    $numbers = $core.Split(".")
    $maj = [int]$numbers[0]
    $min = [int]$numbers[1]
    $pat = [int]$numbers[2]
    switch ($Part) {
        "major" {
            if ($min -ne 0 -or $pat -ne 0 -or $pre -eq "") { $maj++ }
            $min = 0
            $pat = 0
        }
        "minor" {
            if ($pat -ne 0 -or $pre -eq "") { $min++ }
            $pat = 0
        }
        "patch" {
            if ($pre -eq "") { $pat++ }
        }
    }
    return "$maj.$min.$pat"
}

function Confirm-Action([string]$Question) {
    if ($Yes) {
        Write-Host "$Question [y/N] y (-Yes)"
        return $true
    }
    $answer = Read-Host "$Question [y/N]"
    return ($answer -match '^[yY]$')
}

function Find-Msys2 {
    $candidates = @()
    if ($env:MSYS2_ROOT) { $candidates += $env:MSYS2_ROOT }
    $candidates += "C:\msys64"
    if ($env:SystemDrive) { $candidates += (Join-Path $env:SystemDrive "msys64") }
    foreach ($candidate in $candidates) {
        if (Test-Path (Join-Path $candidate "usr\bin\bash.exe")) { return $candidate }
    }
    return $null
}

function Find-Nsis {
    $candidates = @(
        "${env:ProgramFiles(x86)}\NSIS\makensis.exe",
        "$env:ProgramFiles\NSIS\makensis.exe"
    )
    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) { return $candidate }
    }
    $onPath = Get-Command makensis -ErrorAction SilentlyContinue
    if ($onPath) { return $onPath.Source }
    return $null
}

# Runs a command in an MSYS2 UCRT64 terminal, from the repository folder,
# and returns its exit code. The output (errors included) is written line by
# line with Write-Host: it appears on screen and can be redirected to a file,
# without getting mixed into the returned value. The error lines of bash
# must not stop the script (Windows PowerShell 5.1 would make them a fatal
# error with $ErrorActionPreference = "Stop"): only the exit code matters.
# de sortie compte.
function Invoke-Msys([string]$Command) {
    $env:MSYSTEM = "UCRT64"
    $env:CHERE_INVOKING = "1"
    $previous = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & (Join-Path $script:Msys2Root "usr\bin\bash.exe") -lc $Command 2>&1 | ForEach-Object { Write-Host "$_" }
        return $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previous
    }
}

# Loaded with a dot (. .\build-windows.ps1), the script only defines its
# functions: that is how the CI checks its version computation.
if ($MyInvocation.InvocationName -eq ".") { return }

if ($Help) { Show-Help; exit 0 }

# === Clean mode ===
if ($Clean) {
    Write-Host ">>> Cleaning the build folder..."
    if (Test-Path $BuildRoot) { Remove-Item -Recurse -Force $BuildRoot }
    Write-Host ">>> Deleting the generated packages..."
    Get-ChildItem -Path $ProjectDir -Filter "${PackageName}_*_windows-*" -File -ErrorAction SilentlyContinue | Remove-Item -Force
    Write-Host ">>> Clean-up done."
    exit 0
}

# The messages of the bash script are in UTF-8.
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }

# === Reading the current version ===
Write-Host ">>> Project folder detected: $ProjectDir"
if (-not (Test-Path $VersionFile)) {
    Write-Host ""
    Write-Host "Error: VERSION file not found at the expected location:" -ForegroundColor Red
    Write-Host "    $VersionFile"
    exit 1
}
$CurrentVersion = (Get-Content $VersionFile -Raw).Trim()
Write-Host ">>> VERSION file found ($VersionFile): $CurrentVersion"
if ($CurrentVersion -notmatch $SemverPattern) {
    Write-Host ""
    Write-Host "Error: VERSION contains '$CurrentVersion', which is not an X.Y.Z or X.Y.Z-suffix version." -ForegroundColor Red
    exit 1
}

$Increment = $null
if ($Major) { $Increment = "major" }
elseif ($Minor) { $Increment = "minor" }
elseif ($Patch) { $Increment = "patch" }

# === Interactive mode when there is no option ===
if (-not $Increment -and -not $NoVersionChange) {
    Write-Host ""
    Write-Host "Current version: $CurrentVersion"
    Write-Host ""
    Write-Host "What do you want to do?"
    Write-Host "  1) major              (-> $(Get-NextVersion $CurrentVersion 'major'))"
    Write-Host "  2) minor              (-> $(Get-NextVersion $CurrentVersion 'minor'))"
    Write-Host "  3) patch              (-> $(Get-NextVersion $CurrentVersion 'patch'))"
    Write-Host "  4) no-version-change  (rebuild as $CurrentVersion)"
    Write-Host ""
    $choice = Read-Host "Your choice [1/2/3/4]"
    switch ($choice) {
        "1" { $Increment = "major" }
        "2" { $Increment = "minor" }
        "3" { $Increment = "patch" }
        "4" { $NoVersionChange = $true }
        default { Write-Host "Invalid choice. Cancelled."; exit 1 }
    }
}

# === Computing the new version ===
if ($NoVersionChange) {
    $NewVersion = $CurrentVersion
} else {
    $NewVersion = Get-NextVersion $CurrentVersion $Increment
}

# === Confirmation ===
Write-Host ""
Write-Host "========================================"
if ($NoVersionChange) {
    Write-Host "  Mode           : Rebuild without changing the version"
    Write-Host "  Version        : $NewVersion"
} else {
    Write-Host "  Current version : $CurrentVersion"
    Write-Host "  New version     : $NewVersion"
}
Write-Host "========================================"
Write-Host ""
if (-not (Confirm-Action "Build the package as version $NewVersion?")) {
    Write-Host "Build cancelled. No file was modified."
    exit 0
}

# ====================== CHECKING THE PREREQUISITES ======================

Write-Host ""
Write-Host ">>> Checking the prerequisites..."

# --- 64-bit Windows ---
if (-not [Environment]::Is64BitOperatingSystem) {
    Write-Host ""
    Write-Host "Error: this script only supports 64-bit Windows (x64)." -ForegroundColor Red
    exit 1
}
Write-Host "  64-bit Windows (OK)"

# --- MSYS2 ---
$script:Msys2Root = Find-Msys2
if (-not $script:Msys2Root) {
    Write-Host ""
    Write-Host "MSYS2 not found (looked in C:\msys64 and in the MSYS2_ROOT variable)."
    Write-Host "It provides Go, the C compiler, GTK 4, librsvg and cairo for Windows."
    if (Confirm-Action "Install it with winget now?") {
        try {
            winget install --id MSYS2.MSYS2 -e --accept-source-agreements --accept-package-agreements
        } catch {
            Write-Host "Automatic installation of MSYS2 through winget failed." -ForegroundColor Yellow
        }
        $script:Msys2Root = Find-Msys2
    }
    if (-not $script:Msys2Root) {
        Write-Host ""
        Write-Host "Build cancelled: MSYS2 is required." -ForegroundColor Red
        Write-Host ""
        Write-Host "Manual installation:"
        Write-Host "    1) Download the installer from https://www.msys2.org and run it"
        Write-Host "       (default folder: C:\msys64)."
        Write-Host "    2) Run this script again."
        exit 1
    }
}
Write-Host "  MSYS2 found: $script:Msys2Root"

# ====================== BUILD ======================

Push-Location $ProjectDir
try {
    # --- MSYS2 packages ---
    $installDeps = ""
    $rc = Invoke-Msys "bash $BuildScript --check-deps"
    if ($rc -ne 0) {
        Write-Host ""
        Write-Host "Some MSYS2 packages are missing (list above)."
        if (-not (Confirm-Action "Update MSYS2 and install them now?")) {
            Write-Host ""
            Write-Host "Build cancelled: these packages are required." -ForegroundColor Red
            exit 1
        }
        # MSYS2 sometimes asks to run the update again after that of its own
        # core: two passes, whose exit code does not matter
        # (the build script checks the packages again).
        Write-Host ">>> Updating MSYS2 (first pass)..."
        $null = Invoke-Msys "pacman -Syu --noconfirm"
        Write-Host ">>> Updating MSYS2 (second pass)..."
        $null = Invoke-Msys "pacman -Syu --noconfirm"
        $installDeps = " --install-deps"
    } else {
        Write-Host "  MSYS2 packages present"
    }

    # --- NSIS (optional: only for the .exe installer) ---
    $nsis = Find-Nsis
    if (-not $nsis) {
        Write-Host ""
        Write-Host "NSIS not found (only needed to generate the .exe installer -"
        Write-Host "the portable .zip version will be produced in any case)."
        if (Confirm-Action "Install NSIS with winget now?") {
            try {
                winget install --id NSIS.NSIS -e --accept-source-agreements --accept-package-agreements
            } catch {
                Write-Host "Automatic installation of NSIS through winget failed." -ForegroundColor Yellow
            }
            $nsis = Find-Nsis
        }
    }
    if ($nsis) {
        Write-Host "  NSIS found: $nsis"
    } else {
        Write-Host ""
        Write-Host "NSIS not available: only the portable version (.zip) will be generated." -ForegroundColor Yellow
        Write-Host "To generate the .exe installer too, install NSIS from:"
        Write-Host "    https://nsis.sourceforge.io/Download"
        Write-Host "then run this script again."
    }

    Write-Host ""
    Write-Host "========================================"
    Write-Host " Building $PackageName v$NewVersion (Windows)"
    Write-Host "========================================"

    # The working folder, like that of Planche-Contact, is in
    # %LOCALAPPDATA%, outside the repository. The bash script receives it through the
    # SVGPL_BUILD_DIR variable and recreates it at every build.
    $env:SVGPL_BUILD_DIR = $BuildRoot
    $rc = Invoke-Msys "bash $BuildScript$installDeps --version $NewVersion"
    if ($rc -ne 0) {
        Write-Host ""
        Write-Host "The build failed (code $rc). See windows\README.md, Troubleshooting section." -ForegroundColor Red
        exit $rc
    }
} finally {
    Pop-Location
}

# ====================== END ======================

$portableZip = Join-Path $ProjectDir "${PackageName}_${NewVersion}_windows-portable.zip"
$setupExe = Join-Path $ProjectDir "${PackageName}_${NewVersion}_windows-setup.exe"
if (-not (Test-Path $portableZip)) {
    Write-Host ""
    Write-Host "Error: the portable version was not found after the build ($portableZip)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Package(s) created successfully:"
Write-Host "   $portableZip"
if (Test-Path $setupExe) {
    Write-Host "   $setupExe"
}

# The project's VERSION file is only updated here, once the build is really
# finished - a failure along the way never leaves the project in an
# inconsistent state (VERSION incremented without a matching package). It is
# written as UTF-8 without a BOM, with a Unix line ending, as build-deb.sh
# and build-rpm.sh do.
if ($NoVersionChange) {
    Write-Host "   Version unchanged: $NewVersion"
} else {
    [System.IO.File]::WriteAllText($VersionFile, "$NewVersion`n")
    Write-Host "   Version recorded: $NewVersion"
    if ($NewVersion -notmatch '-') {
        Write-Host "   Remember to add '## [$NewVersion]' to CHANGELOG.md and CHANGELOG.fr.md (see CONTRIBUTING.md)."
    }
}
exit 0

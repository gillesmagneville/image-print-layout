# SPDX-License-Identifier: GPL-3.0-only
#Requires -Version 5.1
<#
.SYNOPSIS
    Construit le paquet Windows de SVG Print Layout : une version portable
    (.zip) et, si NSIS est disponible, un installeur (.exe).

.DESCRIPTION
    Équivalent Windows de build-deb.sh et build-rpm.sh. Ce script prépare
    l'environnement (MSYS2, paquets MSYS2, NSIS), en proposant d'installer ce
    qui manque, puis lance packaging\windows\build-windows.sh dans un terminal
    MSYS2 UCRT64 : c'est là que se trouvent Go, le compilateur C, librsvg et
    cairo pour Windows.

    Le numéro de version est celui du fichier VERSION ; ce script ne le
    modifie jamais (une version se change par une pull request « Release »).

    Ce fichier est en UTF-8 avec marque d'ordre des octets (BOM), que Windows
    PowerShell 5.1 exige pour lire les accents ; ne pas l'enregistrer sans BOM.

.PARAMETER Clean
    Supprime le dossier de travail et les paquets Windows de dist\
.PARAMETER SansInstalleur
    Ne produit que la version portable, même si NSIS est installé
.PARAMETER Oui
    Répond « oui » à toutes les questions (installation de MSYS2, de ses
    paquets et de NSIS) : pour un usage sans surveillance
.PARAMETER Help
    Affiche l'aide
#>

param(
    [switch]$Clean,
    [switch]$SansInstalleur,
    [switch]$Oui,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

# Dossier du script : $MyInvocation.MyCommand.Path (avec repli sur
# $PSCommandPath) plutôt que $PSScriptRoot, peu fiable quand le script est
# appelé par le petit relais de la racine du dépôt. Ce fichier est dans
# packaging\windows : le dépôt est deux niveaux au-dessus.
$ScriptPath = $MyInvocation.MyCommand.Path
if (-not $ScriptPath) { $ScriptPath = $PSCommandPath }
$WindowsDir = Split-Path -Parent $ScriptPath
$ProjectDir = Split-Path -Parent (Split-Path -Parent $WindowsDir)
$BuildScript = "./packaging/windows/build-windows.sh"

function Show-Help {
    @'
Usage : .\build-windows.ps1 [options]

Construit le paquet Windows de SVG Print Layout dans dist\ :
  svgprintlayout_<version>_windows-portable.zip   version portable
  svgprintlayout_<version>_windows-setup.exe      installeur (si NSIS est là)

OPTIONS :
  -SansInstalleur   Ne produit que la version portable
  -Oui              Répond « oui » à toutes les questions
  -Clean            Supprime le dossier de travail et les paquets Windows
  -Help             Affiche cette aide

Prérequis, vérifiés au lancement (avec proposition de les installer) :
  - Windows 10 ou 11, 64 bits
  - MSYS2 (https://www.msys2.org), dans C:\msys64, et ses paquets Go, gcc,
    librsvg et cairo pour l'environnement UCRT64
  - NSIS, pour l'installeur seulement (sans lui, seul le .zip est produit)

Le numéro de version est lu dans le fichier VERSION et n'est jamais modifié.
'@
}

function Confirm-Action([string]$Question) {
    if ($Oui) {
        Write-Host "$Question [o/N] o (-Oui)"
        return $true
    }
    $answer = Read-Host "$Question [o/N]"
    return ($answer -match '^[oOyY]')
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

# Exécute une commande dans un terminal MSYS2 UCRT64, depuis le dossier du
# dépôt, et renvoie son code de sortie. Sa sortie va à l'écran.
function Invoke-Msys([string]$Command) {
    $env:MSYSTEM = "UCRT64"
    $env:CHERE_INVOKING = "1"
    & (Join-Path $script:Msys2Root "usr\bin\bash.exe") -lc $Command | Out-Host
    return $LASTEXITCODE
}

if ($Help) { Show-Help; exit 0 }

# === Mode nettoyage ===
if ($Clean) {
    Write-Host ">>> Nettoyage du dossier de travail..."
    $work = Join-Path $ProjectDir "dist\build\windows"
    if (Test-Path $work) { Remove-Item -Recurse -Force $work }
    Write-Host ">>> Suppression des paquets Windows de dist\..."
    $dist = Join-Path $ProjectDir "dist"
    if (Test-Path $dist) {
        Get-ChildItem -Path $dist -Filter "svgprintlayout_*_windows-*" -File -ErrorAction SilentlyContinue | Remove-Item -Force
    }
    Write-Host ">>> Nettoyage terminé."
    exit 0
}

# Les messages du script bash sont en UTF-8.
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }

Write-Host ">>> Dossier du projet : $ProjectDir"
Write-Host ""
Write-Host ">>> Vérification des prérequis..."

# --- Windows 64 bits ---
if (-not [Environment]::Is64BitOperatingSystem) {
    Write-Host ""
    Write-Host "Erreur : ce script ne prend en charge que Windows 64 bits (x64)." -ForegroundColor Red
    exit 1
}
Write-Host "  Windows 64 bits (OK)"

# --- MSYS2 ---
$script:Msys2Root = Find-Msys2
if (-not $script:Msys2Root) {
    Write-Host ""
    Write-Host "MSYS2 est introuvable (cherché dans C:\msys64 et dans la variable MSYS2_ROOT)."
    Write-Host "Il fournit Go, le compilateur C, librsvg et cairo pour Windows."
    if (Confirm-Action "L'installer avec winget maintenant ?") {
        try {
            winget install --id MSYS2.MSYS2 -e --accept-source-agreements --accept-package-agreements
        } catch {
            Write-Host "Échec de l'installation automatique de MSYS2 via winget." -ForegroundColor Yellow
        }
        $script:Msys2Root = Find-Msys2
    }
    if (-not $script:Msys2Root) {
        Write-Host ""
        Write-Host "Construction annulée : MSYS2 est indispensable." -ForegroundColor Red
        Write-Host ""
        Write-Host "Installation manuelle :"
        Write-Host "    1) Téléchargez l'installeur depuis https://www.msys2.org et lancez-le"
        Write-Host "       (dossier par défaut : C:\msys64)."
        Write-Host "    2) Relancez ce script."
        exit 1
    }
}
Write-Host "  MSYS2 trouvé : $script:Msys2Root"

# --- Paquets MSYS2 ---
Push-Location $ProjectDir
try {
    $installDeps = ""
    $rc = Invoke-Msys "bash $BuildScript --check-deps"
    if ($rc -ne 0) {
        Write-Host ""
        Write-Host "Des paquets MSYS2 sont manquants (liste ci-dessus)."
        if (-not (Confirm-Action "Mettre à jour MSYS2 et les installer maintenant ?")) {
            Write-Host ""
            Write-Host "Construction annulée : ces paquets sont indispensables." -ForegroundColor Red
            exit 1
        }
        # MSYS2 demande parfois de relancer la mise à jour après celle de son
        # propre noyau : deux passes, dont le code de sortie n'a pas
        # d'importance (le script de construction revérifie les paquets).
        Write-Host ">>> Mise à jour de MSYS2 (première passe)..."
        $null = Invoke-Msys "pacman -Syu --noconfirm"
        Write-Host ">>> Mise à jour de MSYS2 (seconde passe)..."
        $null = Invoke-Msys "pacman -Syu --noconfirm"
        $installDeps = " --install-deps"
    } else {
        Write-Host "  Paquets MSYS2 présents"
    }

    # --- NSIS (facultatif : uniquement pour l'installeur .exe) ---
    $noInstaller = ""
    if ($SansInstalleur) {
        $noInstaller = " --no-installer"
    } else {
        $nsis = Find-Nsis
        if (-not $nsis) {
            Write-Host ""
            Write-Host "NSIS est introuvable (nécessaire uniquement pour l'installeur .exe ;"
            Write-Host "la version portable .zip sera produite dans tous les cas)."
            if (Confirm-Action "Installer NSIS avec winget maintenant ?") {
                try {
                    winget install --id NSIS.NSIS -e --accept-source-agreements --accept-package-agreements
                } catch {
                    Write-Host "Échec de l'installation automatique de NSIS via winget." -ForegroundColor Yellow
                }
                $nsis = Find-Nsis
            }
        }
        if ($nsis) {
            Write-Host "  NSIS trouvé : $nsis"
        } else {
            Write-Host "  NSIS absent : seule la version portable (.zip) sera produite." -ForegroundColor Yellow
        }
    }

    # --- Construction ---
    $rc = Invoke-Msys "bash $BuildScript$installDeps$noInstaller"
    if ($rc -ne 0) {
        Write-Host ""
        Write-Host "La construction a échoué (code $rc). Voir packaging\windows\README.md, section Dépannage." -ForegroundColor Red
        exit $rc
    }
} finally {
    Pop-Location
}
exit 0

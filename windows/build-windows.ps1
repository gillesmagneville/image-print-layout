# SPDX-License-Identifier: GPL-3.0-only
#Requires -Version 5.1
<#
.SYNOPSIS
    Construit le paquet Windows d'Image Print Layout : une version portable
    (.zip) et, si NSIS est disponible, un installeur (.exe).

.DESCRIPTION
    Équivalent Windows de build-deb.sh et build-rpm.sh, avec les mêmes options
    et la même gestion du fichier VERSION que le build-windows.ps1 de
    Planche-Contact. Ce script vérifie les prérequis (MSYS2 et ses paquets,
    NSIS) en proposant d'installer ce qui manque, puis lance
    windows\build-windows.sh dans un terminal MSYS2 UCRT64 : c'est là que se
    trouvent Go, le compilateur C, GTK 4, librsvg et cairo pour Windows.

    Le fichier VERSION n'est mis à jour qu'une fois les paquets construits.

    Ce fichier est en UTF-8 avec marque d'ordre des octets (BOM), que Windows
    PowerShell 5.1 exige pour lire les accents ; ne pas l'enregistrer sans BOM.

.PARAMETER Major
    Incrémente le numéro de version majeur (ex : 1.2.3 -> 2.0.0)
.PARAMETER Minor
    Incrémente le numéro de version mineur (ex : 1.2.3 -> 1.3.0)
.PARAMETER Patch
    Incrémente le numéro de version de patch (ex : 1.2.3 -> 1.2.4)
.PARAMETER NoVersionChange
    Reconstruit le paquet sans changer la version
.PARAMETER Clean
    Nettoie le dossier de build et supprime les .zip/.exe générés
.PARAMETER Oui
    Répond « oui » à la confirmation et aux propositions d'installation de
    MSYS2, de ses paquets et de NSIS : pour un usage sans surveillance (la CI)
.PARAMETER Help
    Affiche l'aide
#>

param(
    [switch]$Major,
    [switch]$Minor,
    [switch]$Patch,
    [switch]$NoVersionChange,
    [switch]$Clean,
    [switch]$Oui,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

# ====================== CONFIGURATION ======================
$PackageName = "imageprintlayout"

# Localisation du script : $MyInvocation.MyCommand.Path (avec repli sur
# $PSCommandPath) plutôt que $PSScriptRoot, ce dernier s'étant avéré peu
# fiable lorsque ce script est invoqué indirectement depuis le petit script
# relais placé à la racine du dépôt (build-windows.ps1 -> windows\build-
# windows.ps1). Ce dossier vit dans windows\ : le projet est un niveau
# au-dessus.
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
Usage : .\build-windows.ps1 [options]

Script de construction du paquet Windows pour Image Print Layout.

OPTIONS :
  -Major              Incrémente le numéro majeur   (ex: 1.2.3 -> 2.0.0)
  -Minor              Incrémente le numéro mineur   (ex: 1.2.3 -> 1.3.0)
  -Patch              Incrémente le numéro de patch (ex: 1.2.3 -> 1.2.4)
  -NoVersionChange    Reconstruit le paquet sans changer la version
  -Clean              Nettoie le dossier de build et supprime les paquets générés
  -Oui                Répond « oui » à toutes les questions (usage sans surveillance)
  -Help               Affiche cette aide

Sans argument -> mode interactif (demande major / minor / patch / no-version-change)

Le fichier VERSION n'est mis à jour qu'une fois les paquets construits. Une
version d'avant-publication (0.1.0-dev) est publiée par -Minor ou -Patch
(-> 0.1.0) ; -Major donne 1.0.0.

Les paquets sont créés à la racine du dépôt :
  imageprintlayout_<version>_windows-portable.zip   version portable
  imageprintlayout_<version>_windows-setup.exe      installeur (si NSIS est là)

Prérequis vérifiés automatiquement (avec proposition d'installation si absents) :
  - Windows 10 ou 11, 64 bits
  - MSYS2 (https://www.msys2.org), dans C:\msys64, et ses paquets Go, gcc, GTK 4,
    librsvg et cairo pour l'environnement UCRT64
  - NSIS (facultatif : nécessaire uniquement pour générer l'installeur .exe)
'@
}

# Une version est X.Y.Z, éventuellement suivie d'un suffixe d'avant-publication
# (X.Y.Z-dev).
$SemverPattern = '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'

# Get-NextVersion donne la version suivante. Sans suffixe, c'est l'incrément
# habituel (1.2.3 -> 1.2.4). Avec un suffixe, la version d'avant-publication
# est d'abord publiée : 0.1.0-dev -> 0.1.0 par -Minor ou -Patch, puisque 0.1.0
# n'existe pas encore ; -Major donne 1.0.0. Même règle que build-deb.sh.
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
    if ($Oui) {
        Write-Host "$Question [o/N] o (-Oui)"
        return $true
    }
    $answer = Read-Host "$Question [o/N]"
    return ($answer -match '^[oOyY]$')
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
# dépôt, et renvoie son code de sortie. La sortie (erreurs comprises) est
# écrite ligne à ligne avec Write-Host : elle s'affiche à l'écran et peut être
# redirigée vers un fichier, sans se mêler à la valeur renvoyée. Les lignes
# d'erreur de bash ne doivent pas arrêter le script (Windows PowerShell 5.1 en
# ferait une erreur fatale avec $ErrorActionPreference = "Stop") : seul le code
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

# Chargé par un point (. .\build-windows.ps1), le script ne fait que définir ses
# fonctions : c'est ainsi que la CI en vérifie le calcul des versions.
if ($MyInvocation.InvocationName -eq ".") { return }

if ($Help) { Show-Help; exit 0 }

# === Mode nettoyage ===
if ($Clean) {
    Write-Host ">>> Nettoyage du dossier de build..."
    if (Test-Path $BuildRoot) { Remove-Item -Recurse -Force $BuildRoot }
    Write-Host ">>> Suppression des paquets générés..."
    Get-ChildItem -Path $ProjectDir -Filter "${PackageName}_*_windows-*" -File -ErrorAction SilentlyContinue | Remove-Item -Force
    Write-Host ">>> Nettoyage terminé."
    exit 0
}

# Les messages du script bash sont en UTF-8.
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }

# === Lecture de la version actuelle ===
Write-Host ">>> Dossier du projet détecté : $ProjectDir"
if (-not (Test-Path $VersionFile)) {
    Write-Host ""
    Write-Host "Erreur : fichier VERSION introuvable à l'emplacement attendu :" -ForegroundColor Red
    Write-Host "    $VersionFile"
    exit 1
}
$CurrentVersion = (Get-Content $VersionFile -Raw).Trim()
Write-Host ">>> Fichier VERSION trouvé ($VersionFile) : $CurrentVersion"
if ($CurrentVersion -notmatch $SemverPattern) {
    Write-Host ""
    Write-Host "Erreur : VERSION contient « $CurrentVersion », qui n'est pas une version X.Y.Z ou X.Y.Z-suffixe." -ForegroundColor Red
    exit 1
}

$Increment = $null
if ($Major) { $Increment = "major" }
elseif ($Minor) { $Increment = "minor" }
elseif ($Patch) { $Increment = "patch" }

# === Mode interactif si aucun argument ===
if (-not $Increment -and -not $NoVersionChange) {
    Write-Host ""
    Write-Host "Version actuelle : $CurrentVersion"
    Write-Host ""
    Write-Host "Que souhaitez-vous faire ?"
    Write-Host "  1) major              (-> $(Get-NextVersion $CurrentVersion 'major'))"
    Write-Host "  2) minor              (-> $(Get-NextVersion $CurrentVersion 'minor'))"
    Write-Host "  3) patch              (-> $(Get-NextVersion $CurrentVersion 'patch'))"
    Write-Host "  4) no-version-change  (reconstruire en $CurrentVersion)"
    Write-Host ""
    $choice = Read-Host "Votre choix [1/2/3/4]"
    switch ($choice) {
        "1" { $Increment = "major" }
        "2" { $Increment = "minor" }
        "3" { $Increment = "patch" }
        "4" { $NoVersionChange = $true }
        default { Write-Host "Choix invalide. Annulation."; exit 1 }
    }
}

# === Calcul de la nouvelle version ===
if ($NoVersionChange) {
    $NewVersion = $CurrentVersion
} else {
    $NewVersion = Get-NextVersion $CurrentVersion $Increment
}

# === Confirmation ===
Write-Host ""
Write-Host "========================================"
if ($NoVersionChange) {
    Write-Host "  Mode           : Reconstruction sans changement de version"
    Write-Host "  Version        : $NewVersion"
} else {
    Write-Host "  Version actuelle : $CurrentVersion"
    Write-Host "  Nouvelle version : $NewVersion"
}
Write-Host "========================================"
Write-Host ""
if (-not (Confirm-Action "Construire le paquet en version $NewVersion ?")) {
    Write-Host "Construction annulée. Aucun fichier n'a été modifié."
    exit 0
}

# ====================== VÉRIFICATION DES PRÉREQUIS ======================

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
    Write-Host "Il fournit Go, le compilateur C, GTK 4, librsvg et cairo pour Windows."
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

# ====================== CONSTRUCTION ======================

Push-Location $ProjectDir
try {
    # --- Paquets MSYS2 ---
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
    $nsis = Find-Nsis
    if (-not $nsis) {
        Write-Host ""
        Write-Host "NSIS est introuvable (nécessaire uniquement pour générer l'installeur .exe -"
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
        Write-Host ""
        Write-Host "NSIS non disponible : seule la version portable (.zip) sera générée." -ForegroundColor Yellow
        Write-Host "Pour générer aussi l'installeur .exe, installez NSIS depuis :"
        Write-Host "    https://nsis.sourceforge.io/Download"
        Write-Host "puis relancez ce script."
    }

    Write-Host ""
    Write-Host "========================================"
    Write-Host " Construction de $PackageName v$NewVersion (Windows)"
    Write-Host "========================================"

    # Le dossier de travail, comme celui de Planche-Contact, est dans
    # %LOCALAPPDATA%, hors du dépôt. Le script bash le reçoit par la variable
    # SVGPL_BUILD_DIR et le recrée à chaque construction.
    $env:SVGPL_BUILD_DIR = $BuildRoot
    $rc = Invoke-Msys "bash $BuildScript$installDeps --version $NewVersion"
    if ($rc -ne 0) {
        Write-Host ""
        Write-Host "La construction a échoué (code $rc). Voir windows\README.md, section Dépannage." -ForegroundColor Red
        exit $rc
    }
} finally {
    Pop-Location
}

# ====================== FIN ======================

$portableZip = Join-Path $ProjectDir "${PackageName}_${NewVersion}_windows-portable.zip"
$setupExe = Join-Path $ProjectDir "${PackageName}_${NewVersion}_windows-setup.exe"
if (-not (Test-Path $portableZip)) {
    Write-Host ""
    Write-Host "Erreur : la version portable est introuvable après la construction ($portableZip)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Paquet(s) créé(s) avec succès :"
Write-Host "   $portableZip"
if (Test-Path $setupExe) {
    Write-Host "   $setupExe"
}

# Le fichier VERSION du projet n'est mis à jour qu'ici, une fois la
# construction réellement terminée - un échec en cours de route ne laisse
# jamais le projet dans un état incohérent (VERSION incrémenté sans paquet
# correspondant). Il est écrit en UTF-8 sans BOM, avec une fin de ligne Unix,
# comme le font build-deb.sh et build-rpm.sh.
if ($NoVersionChange) {
    Write-Host "   Version inchangée : $NewVersion"
} else {
    [System.IO.File]::WriteAllText($VersionFile, "$NewVersion`n")
    Write-Host "   Version enregistrée : $NewVersion"
    if ($NewVersion -notmatch '-') {
        Write-Host "   Pensez à ajouter « ## [$NewVersion] » à CHANGELOG.md (voir CONTRIBUTING.md)."
    }
}
exit 0

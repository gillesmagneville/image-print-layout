# SPDX-License-Identifier: GPL-3.0-only
#
# Étapes du job « windows-package » de la CI (.github/workflows/ci.yml), écrites
# dans un script pour que l'échec d'une étape laisse une trace lisible : la fin
# de sa sortie est recopiée dans une annotation d'erreur, que l'on retrouve
# dans la page de l'exécution (et par l'API) sans télécharger les journaux.
#
#   -Step build      construit les paquets avec build-windows.ps1 -Patch, comme un
#                    utilisateur, et vérifie la gestion du fichier VERSION
#   -Step portable   lance la version portable avec le seul PATH de Windows
#   -Step installer  installe, lance, désinstalle en silence
#   -Step clean      vérifie que build-windows.ps1 -Clean supprime les paquets

param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("build", "portable", "installer", "clean")]
    [string]$Step
)

# Une commande native qui écrit sur sa sortie d'erreur ne doit pas arrêter le
# script : on vérifie les codes de sortie nous-mêmes.
$ErrorActionPreference = "Continue"

$log = Join-Path $env:RUNNER_TEMP "$Step.log"
$cleanPath = "$env:SystemRoot\System32;$env:SystemRoot"
$uninstallKey = "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SVGPrintLayout"

function Assert-Success([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What a échoué (code $LASTEXITCODE)" }
}

# La version du fichier VERSION, lue à chaque fois : la construction la change.
function Get-Version { (Get-Content VERSION -Raw).Trim() }

function Invoke-Build {
    # Chargé par un point, build-windows.ps1 ne fait que définir ses fonctions.
    . .\windows\build-windows.ps1
    $ErrorActionPreference = "Continue"
    $table = @(
        @("1.2.3", "major", "2.0.0"), @("1.2.3", "minor", "1.3.0"), @("1.2.3", "patch", "1.2.4"),
        @("0.1.0", "major", "1.0.0"), @("0.1.0", "minor", "0.2.0"), @("0.1.0", "patch", "0.1.1"),
        @("0.1.0-dev", "major", "1.0.0"), @("0.1.0-dev", "minor", "0.1.0"), @("0.1.0-dev", "patch", "0.1.0"),
        @("0.1.2-dev", "patch", "0.1.2"), @("0.1.2-dev", "minor", "0.2.0"),
        @("1.0.0-rc.1", "major", "1.0.0"), @("1.0.0-rc.1", "minor", "1.0.0"), @("10.9.99", "patch", "10.9.100")
    )
    foreach ($row in $table) {
        $got = Get-NextVersion $row[0] $row[1]
        if ($got -ne $row[2]) { throw "Get-NextVersion $($row[0]) $($row[1]) donne $got, attendu $($row[2])" }
    }
    Write-Host "ok  calcul des versions"

    # Pas de variable $help : le script chargé par un point a déjà créé $Help, un
    # [switch] qui refuse une chaîne.
    $helpText = (.\build-windows.ps1 -Help) -join "`n"
    Assert-Success "build-windows.ps1 -Help"
    foreach ($option in "-Major", "-Minor", "-Patch", "-NoVersionChange", "-Clean", "-Help") {
        if ($helpText -notmatch [regex]::Escape($option)) { throw "l'aide ne mentionne pas $option" }
    }
    Write-Host "ok  aide"

    # -Patch : la version change, une fois les paquets construits ; -Oui répond
    # à la confirmation et aux propositions d'installation.
    $before = Get-Version
    $expected = Get-NextVersion $before "patch"
    .\build-windows.ps1 -Patch -Oui
    Assert-Success "build-windows.ps1 -Patch -Oui"

    $bytes = [System.IO.File]::ReadAllBytes((Join-Path (Get-Location) "VERSION"))
    $text = [System.Text.Encoding]::UTF8.GetString($bytes)
    if ($text -ne "$expected`n") { throw "VERSION vaut « $($text.TrimEnd()) » après -Patch, attendu « $expected » (sans BOM, avec une fin de ligne Unix)" }
    Write-Host "ok  VERSION : $before -> $expected"

    Write-Host "--- paquets"
    Get-ChildItem -File -Filter "svgprintlayout_*" | ForEach-Object { Write-Host ("{0,12:N0} octets  {1}" -f $_.Length, $_.Name) }
    foreach ($name in "svgprintlayout_${expected}_windows-portable.zip", "svgprintlayout_${expected}_windows-setup.exe") {
        if (-not (Test-Path $name)) { throw "$name manque à la racine du dépôt" }
    }
}

function Test-Clean {
    .\build-windows.ps1 -Clean
    Assert-Success "build-windows.ps1 -Clean"
    $left = @(Get-ChildItem -File -Filter "svgprintlayout_*_windows-*")
    if ($left.Count -gt 0) { throw "-Clean a laissé : $($left.Name -join ', ')" }
    if (Test-Path "$env:LOCALAPPDATA\svgprintlayout-build") { throw "-Clean a laissé le dossier de travail" }
}

function Test-Portable {
    $version = Get-Version
    $dest = Join-Path $env:RUNNER_TEMP "portable"
    Expand-Archive "svgprintlayout_${version}_windows-portable.zip" -DestinationPath $dest
    $root = (Get-ChildItem $dest -Directory | Select-Object -First 1).FullName
    Write-Host "--- contenu de $root"
    Get-ChildItem $root | ForEach-Object { Write-Host $_.Name }
    Write-Host ("--- bin : {0} DLL, {1:N0} octets au total" -f (Get-ChildItem "$root\bin" -Filter *.dll).Count, (Get-ChildItem $root -Recurse -File | Measure-Object Length -Sum).Sum)
    Write-Host "--- loaders.cache"
    Get-Content "$root\lib\gdk-pixbuf-2.0\2.10.0\loaders.cache" -ErrorAction SilentlyContinue | Select-Object -First 12 | ForEach-Object { Write-Host $_ }
    Write-Host "--- COMPOSANTS.txt"
    Get-Content "$root\share\licenses\COMPOSANTS.txt" | Select-Object -First 40 | ForEach-Object { Write-Host $_ }
    $env:Path = $cleanPath
    $versionLine = (& "$root\bin\svgprintlayout.exe" -version) -join " "
    Assert-Success "svgprintlayout.exe -version"
    Write-Host $versionLine
    # Les versions des bibliothèques doivent ressembler à des versions (la lecture
    # de celle de librsvg a donné des nombres absurdes sous Windows).
    if ($versionLine -notmatch 'librsvg [0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}, cairo [0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$') {
        throw "la ligne de version est invalide : $versionLine"
    }
    & "$root\bin\svgprintlayout.exe" -selftest
    Assert-Success "svgprintlayout.exe -selftest"
}

function Test-Installer {
    $setup = Join-Path (Get-Location) "svgprintlayout_$(Get-Version)_windows-setup.exe"
    $target = Join-Path $env:RUNNER_TEMP "installed"
    Start-Process -FilePath $setup -ArgumentList "/S", "/D=$target" -Wait
    if (-not (Test-Path "$target\bin\svgprintlayout.exe")) { throw "l'installeur n'a pas installé le programme" }
    if (-not (Test-Path $uninstallKey)) { throw "l'entrée de désinstallation manque" }
    if (-not (Test-Path "$env:ProgramData\Microsoft\Windows\Start Menu\Programs\SVG Print Layout\SVG Print Layout.lnk")) {
        throw "le raccourci du menu Démarrer manque"
    }
    $env:Path = $cleanPath
    & "$target\bin\svgprintlayout.exe" -selftest
    Assert-Success "svgprintlayout.exe -selftest (programme installé)"
    # _?= fait tourner le désinstalleur sur place et attendre, au lieu de se
    # copier dans un dossier temporaire et de rendre la main aussitôt.
    Start-Process -FilePath "$target\Uninstall.exe" -ArgumentList "/S", "_?=$target" -Wait
    if (Test-Path "$target\bin") { throw "le désinstalleur a laissé le programme" }
    if (Test-Path $uninstallKey) { throw "l'entrée de désinstallation est toujours là" }
}

try {
    & {
        switch ($Step) {
            "build"     { Invoke-Build }
            "portable"  { Test-Portable }
            "installer" { Test-Installer }
            "clean"     { Test-Clean }
        }
    } *>&1 | Tee-Object -FilePath $log
    # Succès : la fin de la sortie reste lisible dans une annotation, comme en cas d'échec.
    $summary = ((Get-Content $log -Tail 60 -ErrorAction SilentlyContinue) -join "`n")
    $summary = $summary.Replace("%", "%25").Replace("`r", "%0D").Replace("`n", "%0A")
    Write-Host "::notice title=Étape $Step terminée::$summary"
} catch {
    $tail = ((Get-Content $log -Tail 60 -ErrorAction SilentlyContinue) -join "`n") + "`n" + $_.Exception.Message
    $tail = $tail.Replace("%", "%25").Replace("`r", "%0D").Replace("`n", "%0A")
    Write-Host "::error title=Étape $Step en échec::$tail"
    exit 1
}

# SPDX-License-Identifier: GPL-3.0-only
#
# Étapes du job « windows-package » de la CI (.github/workflows/ci.yml), écrites
# dans un script pour que l'échec d'une étape laisse une trace lisible : la fin
# de sa sortie est recopiée dans une annotation d'erreur, que l'on retrouve
# dans la page de l'exécution (et par l'API) sans télécharger les journaux.
#
#   -Step build      construit les paquets avec build-windows.ps1, comme un utilisateur
#   -Step portable   lance la version portable avec le seul PATH de Windows
#   -Step installer  installe, lance, désinstalle en silence

param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("build", "portable", "installer")]
    [string]$Step
)

# Une commande native qui écrit sur sa sortie d'erreur ne doit pas arrêter le
# script : on vérifie les codes de sortie nous-mêmes.
$ErrorActionPreference = "Continue"

$version = (Get-Content VERSION -Raw).Trim()
$log = Join-Path $env:RUNNER_TEMP "$Step.log"
$cleanPath = "$env:SystemRoot\System32;$env:SystemRoot"
$uninstallKey = "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SVGPrintLayout"

function Assert-Success([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What a échoué (code $LASTEXITCODE)" }
}

function Invoke-Build {
    .\build-windows.ps1 -Oui
    Assert-Success "build-windows.ps1"
}

function Test-Portable {
    $dest = Join-Path $env:RUNNER_TEMP "portable"
    Expand-Archive "dist\svgprintlayout_${version}_windows-portable.zip" -DestinationPath $dest
    $root = (Get-ChildItem $dest -Directory | Select-Object -First 1).FullName
    Write-Host "--- contenu de $root"
    Get-ChildItem $root | ForEach-Object { Write-Host $_.Name }
    Write-Host "--- loaders.cache"
    Get-Content "$root\lib\gdk-pixbuf-2.0\2.10.0\loaders.cache" -ErrorAction SilentlyContinue | Select-Object -First 12 | ForEach-Object { Write-Host $_ }
    Write-Host "--- COMPOSANTS.txt"
    Get-Content "$root\share\licenses\COMPOSANTS.txt" | Select-Object -First 40 | ForEach-Object { Write-Host $_ }
    $env:Path = $cleanPath
    & "$root\bin\svgprintlayout.exe" -version
    Assert-Success "svgprintlayout.exe -version"
    & "$root\bin\svgprintlayout.exe" -selftest
    Assert-Success "svgprintlayout.exe -selftest"
}

function Test-Installer {
    $setup = "dist\svgprintlayout_${version}_windows-setup.exe"
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
        }
    } *>&1 | Tee-Object -FilePath $log
} catch {
    $tail = ((Get-Content $log -Tail 60 -ErrorAction SilentlyContinue) -join "`n") + "`n" + $_.Exception.Message
    $tail = $tail.Replace("%", "%25").Replace("`r", "%0D").Replace("`n", "%0A")
    Write-Host "::error title=Étape $Step en échec::$tail"
    exit 1
}

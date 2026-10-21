# SPDX-License-Identifier: GPL-3.0-only
#
# Étapes du job « windows-package » de la CI (.github/workflows/ci.yml), écrites
# dans un script pour que l'échec d'une étape laisse une trace lisible : la fin
# de sa sortie est recopiée dans une annotation d'erreur, que l'on retrouve
# dans la page de l'exécution (et par l'API) sans télécharger les journaux.
#
#   -Step build      construit les paquets avec build-windows.ps1 -Patch, comme un
#                    utilisateur, et vérifie la gestion du fichier VERSION
#   -Step release    construit les paquets de la version de VERSION, sans la
#                    changer (build-windows.ps1 -NoVersionChange) : ceux que le
#                    workflow release.yml joint à la release
#   -Step acceptance exécute les 14 tests d'acceptation du cahier des charges
#                    (go test ./internal/acceptance, sous MSYS2 UCRT64, poppler
#                    rastérise les PDF)
#   -Step portable   lance la version portable avec le seul PATH de Windows :
#                    l'auto-test, puis la fenêtre dans les quatre langues
#   -Step installer  installe, lance (auto-test et fenêtre), désinstalle en
#                    silence
#   -Step clean      vérifie que build-windows.ps1 -Clean supprime les paquets

param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("build", "release", "acceptance", "portable", "installer", "clean")]
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

# Les deux exécutables du programme : la fenêtre (sous-système Windows, rien
# n'est écrit dans un terminal) et la console (-version, -selftest).
$sampleSvg = @'
<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="5479" viewBox="0 0 1280 5479">
  <rect width="1280" height="5479" fill="#f4f5f8"/>
  <rect width="1280" height="84" fill="#2b3a55"/>
  <rect x="40" y="150" width="1200" height="60" fill="#ffffff" stroke="#8790a3" stroke-width="1.2"/>
  <rect x="40" y="2600" width="1200" height="60" fill="#ffffff" stroke="#8790a3" stroke-width="1.2"/>
  <rect x="40" y="5300" width="1200" height="60" fill="#ffffff" stroke="#8790a3" stroke-width="1.2"/>
</svg>
'@

# Enregistre une capture de l'écran. Elle sert à regarder le résultat : un
# échec ici ne fait pas échouer l'étape.
function Save-Screenshot([string]$Path) {
    try {
        Add-Type -AssemblyName System.Windows.Forms, System.Drawing
        $bounds = [System.Windows.Forms.SystemInformation]::VirtualScreen
        $bitmap = New-Object System.Drawing.Bitmap $bounds.Width, $bounds.Height
        $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
        $graphics.CopyFromScreen($bounds.Left, $bounds.Top, 0, 0, $bitmap.Size)
        $bitmap.Save($Path, [System.Drawing.Imaging.ImageFormat]::Png)
        $graphics.Dispose()
        $bitmap.Dispose()
        Write-Host "capture : $Path"
    } catch {
        Write-Host "capture impossible ($($_.Exception.Message))"
    }
}

# Lance la fenêtre du programme (avec le seul PATH de Windows), attend qu'elle
# soit affichée avec le document dessiné (-ready-file), enregistre une capture,
# puis ferme le programme. Le rendu logiciel de GTK est demandé : une machine
# virtuelle n'a pas de carte graphique.
function Test-Window([string]$Exe, [string]$Lang, [string]$Name) {
    $dir = Join-Path $env:RUNNER_TEMP "window"
    New-Item -ItemType Directory -Force -Path $dir, "screenshots" | Out-Null
    $svg = Join-Path $dir "plan.svg"
    [System.IO.File]::WriteAllText($svg, $sampleSvg, (New-Object System.Text.UTF8Encoding $false))
    $ready = Join-Path $dir "$Name.ready"
    $stderr = Join-Path $dir "$Name.err"
    $stdout = Join-Path $dir "$Name.out"
    Remove-Item $ready, $stderr, $stdout -ErrorAction SilentlyContinue
    $env:Path = $cleanPath
    $env:GSK_RENDERER = "cairo"
    $process = Start-Process -FilePath $Exe -PassThru `
        -ArgumentList "-lang", $Lang, "-ready-file", "`"$ready`"", "`"$svg`"" `
        -RedirectStandardError $stderr -RedirectStandardOutput $stdout
    # Sans cette lecture, Windows PowerShell ne garde pas le descripteur du
    # processus et ne sait plus donner son code de sortie.
    $null = $process.Handle
    $deadline = (Get-Date).AddSeconds(60)
    while (-not (Test-Path $ready) -and -not $process.HasExited -and (Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 300
    }
    $shown = Test-Path $ready
    if ($shown) {
        Start-Sleep -Seconds 1
        Save-Screenshot (Join-Path (Get-Location) "screenshots\$Name.png")
    }
    if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force }
    if (-not $shown) {
        $code = if ($process.HasExited) { $process.ExitCode } else { "toujours en cours" }
        $messages = (Get-Content $stderr, $stdout -ErrorAction SilentlyContinue) -join "`n"
        throw "la fenêtre ($Lang) ne s'est pas affichée en 60 secondes (code de sortie : $code)`n$messages"
    }
    Write-Host "ok  fenêtre ($Lang) affichée"
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
    foreach ($file in "bin\svgprintlayout.exe", "bin\svgprintlayout-console.exe", "share\glib-2.0\schemas\gschemas.compiled", "share\icons\Adwaita\index.theme", "share\icons\hicolor\index.theme") {
        if (-not (Test-Path "$root\$file")) { throw "$file manque dans la version portable" }
    }
    Write-Host ("--- traductions de GTK : {0}" -f ((Get-ChildItem "$root\share\locale" -Recurse -Filter *.mo | ForEach-Object { $_.FullName.Substring($root.Length + 1) }) -join ", "))
    $env:Path = $cleanPath
    $versionLine = (& "$root\bin\svgprintlayout-console.exe" -version) -join " "
    Assert-Success "svgprintlayout-console.exe -version"
    Write-Host $versionLine
    # Les versions des bibliothèques doivent ressembler à des versions (la lecture
    # de celle de librsvg a donné des nombres absurdes sous Windows).
    if ($versionLine -notmatch 'librsvg [0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}, cairo [0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$') {
        throw "la ligne de version est invalide : $versionLine"
    }
    & "$root\bin\svgprintlayout-console.exe" -selftest
    Assert-Success "svgprintlayout-console.exe -selftest"
    foreach ($lang in "fr", "en", "es", "de") {
        Test-Window "$root\bin\svgprintlayout.exe" $lang "portable-$lang"
    }
}

# Les tests d'acceptation (cahier des charges, §13) : le moteur, l'export et le
# projet sur ce Windows. poppler fournit pdftoppm et pdfinfo, qui rastérisent
# les PDF écrits ; avec CI définie, un outil manquant fait échouer le test au
# lieu de le sauter.
function Test-Acceptance {
    $root = if ($env:MSYS2_ROOT) { $env:MSYS2_ROOT } else { "C:\msys64" }
    $bash = Join-Path $root "usr\bin\bash.exe"
    if (-not (Test-Path $bash)) { throw "MSYS2 est introuvable ($root)" }
    $env:MSYSTEM = "UCRT64"
    $env:CHERE_INVOKING = "1"
    & $bash -lc "pacman -S --needed --noconfirm mingw-w64-ucrt-x86_64-poppler" 2>&1 | ForEach-Object { Write-Host "$_" }
    Assert-Success "l'installation de poppler"
    & $bash -lc "CGO_ENABLED=1 go test -count=1 -v ./internal/acceptance" 2>&1 | ForEach-Object { Write-Host "$_" }
    Assert-Success "go test ./internal/acceptance"
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
    & "$target\bin\svgprintlayout-console.exe" -selftest
    Assert-Success "svgprintlayout-console.exe -selftest (programme installé)"
    Test-Window "$target\bin\svgprintlayout.exe" "en" "installed-en"
    # _?= fait tourner le désinstalleur sur place et attendre, au lieu de se
    # copier dans un dossier temporaire et de rendre la main aussitôt.
    Start-Process -FilePath "$target\Uninstall.exe" -ArgumentList "/S", "_?=$target" -Wait
    if (Test-Path "$target\bin") { throw "le désinstalleur a laissé le programme" }
    if (Test-Path $uninstallKey) { throw "l'entrée de désinstallation est toujours là" }
}

# Les paquets de la release : la version est celle de VERSION, qui ne bouge pas.
function Invoke-Release {
    $version = Get-Version
    .\build-windows.ps1 -NoVersionChange -Oui
    Assert-Success "build-windows.ps1 -NoVersionChange -Oui"
    $after = Get-Version
    if ($after -ne $version) { throw "VERSION vaut « $after » après -NoVersionChange, attendu « $version »" }
    Get-ChildItem -File -Filter "svgprintlayout_*" | ForEach-Object { Write-Host ("{0,12:N0} octets  {1}" -f $_.Length, $_.Name) }
    foreach ($name in "svgprintlayout_${version}_windows-portable.zip", "svgprintlayout_${version}_windows-setup.exe") {
        if (-not (Test-Path $name)) { throw "$name manque à la racine du dépôt" }
    }
}

try {
    & {
        switch ($Step) {
            "build"     { Invoke-Build }
            "release"   { Invoke-Release }
            "acceptance" { Test-Acceptance }
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

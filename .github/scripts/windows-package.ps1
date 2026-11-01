# SPDX-License-Identifier: GPL-3.0-only
#
# Steps of the "windows-package" job of the CI (.github/workflows/ci.yml), written
# in a script so that the failure of a step leaves a readable trace: the end
# of its output is copied into an error annotation, found on the page of the
# run (and through the API) without downloading the logs.
#
#   -Step build      builds the packages with build-windows.ps1 -Patch, like a
#                    user, and checks the handling of the VERSION file
#   -Step release    builds the packages of the version in VERSION, without
#                    changing it (build-windows.ps1 -NoVersionChange): the ones
#                    that the release.yml workflow attaches to the release
#   -Step acceptance runs the 14 acceptance tests of the specification
#                    (go test ./internal/acceptance, under MSYS2 UCRT64, poppler
#                    rasterises the PDFs)
#   -Step portable   runs the portable version with only the Windows PATH:
#                    the self-test, then the window in the four languages
#   -Step installer  installs, runs (self-test and window), uninstalls
#                    silently
#   -Step clean      checks that build-windows.ps1 -Clean removes the packages

param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("build", "release", "acceptance", "portable", "installer", "clean")]
    [string]$Step
)

# A native command that writes to its error output must not stop the
# script: we check the exit codes ourselves.
$ErrorActionPreference = "Continue"

$log = Join-Path $env:RUNNER_TEMP "$Step.log"
$cleanPath = "$env:SystemRoot\System32;$env:SystemRoot"
$uninstallKey = "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\ImagePrintLayout"

function Assert-Success([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (code $LASTEXITCODE)" }
}

# The version in the VERSION file, read every time: the build changes it.
function Get-Version { (Get-Content VERSION -Raw).Trim() }

function Invoke-Build {
    # Loaded with a dot, build-windows.ps1 only defines its functions.
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
        if ($got -ne $row[2]) { throw "Get-NextVersion $($row[0]) $($row[1]) gives $got, expected $($row[2])" }
    }
    Write-Host "ok  version computation"

    # No $help variable: the script loaded with a dot has already created $Help, a
    # [switch] that refuses a string.
    $helpText = (.\build-windows.ps1 -Help) -join "`n"
    Assert-Success "build-windows.ps1 -Help"
    foreach ($option in "-Major", "-Minor", "-Patch", "-NoVersionChange", "-Clean", "-Help") {
        if ($helpText -notmatch [regex]::Escape($option)) { throw "the help does not mention $option" }
    }
    Write-Host "ok  help"

    # -Patch: the version changes, once the packages are built; -Yes answers
    # the confirmation and the offers to install.
    $before = Get-Version
    $expected = Get-NextVersion $before "patch"
    .\build-windows.ps1 -Patch -Yes
    Assert-Success "build-windows.ps1 -Patch -Yes"

    $bytes = [System.IO.File]::ReadAllBytes((Join-Path (Get-Location) "VERSION"))
    $text = [System.Text.Encoding]::UTF8.GetString($bytes)
    if ($text -ne "$expected`n") { throw "VERSION is '$($text.TrimEnd())' after -Patch, expected '$expected' (no BOM, with a Unix line ending)" }
    Write-Host "ok  VERSION : $before -> $expected"

    Write-Host "--- packages"
    Get-ChildItem -File -Filter "imageprintlayout_*" | ForEach-Object { Write-Host ("{0,12:N0} bytes  {1}" -f $_.Length, $_.Name) }
    foreach ($name in "imageprintlayout_${expected}_windows-portable.zip", "imageprintlayout_${expected}_windows-setup.exe") {
        if (-not (Test-Path $name)) { throw "$name is missing at the root of the repository" }
    }
}

# The two executables of the program: the window (Windows subsystem, nothing
# is written in a terminal) and the console (-version, -selftest).
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

# Saves a capture of the screen. It is used to look at the result: a
# failure here does not make the step fail.
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
        Write-Host "capture: $Path"
    } catch {
        Write-Host "capture impossible ($($_.Exception.Message))"
    }
}

# Starts the program's window (with only the Windows PATH), waits for it to be
# displayed with the document drawn (-ready-file), saves a capture,
# then closes the program. GTK's software rendering is requested: a
# virtual machine has no graphics card.
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
    # Without this read, Windows PowerShell does not keep the process handle
    # and can no longer give its exit code.
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
        $code = if ($process.HasExited) { $process.ExitCode } else { "still running" }
        $messages = (Get-Content $stderr, $stdout -ErrorAction SilentlyContinue) -join "`n"
        throw "the window ($Lang) was not displayed within 60 seconds (exit code: $code)`n$messages"
    }
    Write-Host "ok  window ($Lang) displayed"
}

function Test-Clean {
    .\build-windows.ps1 -Clean
    Assert-Success "build-windows.ps1 -Clean"
    $left = @(Get-ChildItem -File -Filter "imageprintlayout_*_windows-*")
    if ($left.Count -gt 0) { throw "-Clean left behind: $($left.Name -join ', ')" }
    if (Test-Path "$env:LOCALAPPDATA\imageprintlayout-build") { throw "-Clean left the working folder behind" }
}

function Test-Portable {
    $version = Get-Version
    $dest = Join-Path $env:RUNNER_TEMP "portable"
    Expand-Archive "imageprintlayout_${version}_windows-portable.zip" -DestinationPath $dest
    $root = (Get-ChildItem $dest -Directory | Select-Object -First 1).FullName
    Write-Host "--- contents of $root"
    Get-ChildItem $root | ForEach-Object { Write-Host $_.Name }
    Write-Host ("--- bin: {0} DLLs, {1:N0} bytes in total" -f (Get-ChildItem "$root\bin" -Filter *.dll).Count, (Get-ChildItem $root -Recurse -File | Measure-Object Length -Sum).Sum)
    Write-Host "--- loaders.cache"
    Get-Content "$root\lib\gdk-pixbuf-2.0\2.10.0\loaders.cache" -ErrorAction SilentlyContinue | Select-Object -First 12 | ForEach-Object { Write-Host $_ }
    Write-Host "--- COMPONENTS.txt"
    Get-Content "$root\share\licenses\COMPONENTS.txt" | Select-Object -First 40 | ForEach-Object { Write-Host $_ }
    foreach ($file in "bin\imageprintlayout.exe", "bin\imageprintlayout-console.exe", "share\glib-2.0\schemas\gschemas.compiled", "share\icons\Adwaita\index.theme", "share\icons\hicolor\index.theme") {
        if (-not (Test-Path "$root\$file")) { throw "$file is missing from the portable version" }
    }
    Write-Host ("--- GTK translations: {0}" -f ((Get-ChildItem "$root\share\locale" -Recurse -Filter *.mo | ForEach-Object { $_.FullName.Substring($root.Length + 1) }) -join ", "))
    $env:Path = $cleanPath
    $versionLine = (& "$root\bin\imageprintlayout-console.exe" -version) -join " "
    Assert-Success "imageprintlayout-console.exe -version"
    Write-Host $versionLine
    # The library versions must look like versions (reading
    # that of librsvg once gave absurd numbers under Windows).
    if ($versionLine -notmatch 'librsvg [0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}, cairo [0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$') {
        throw "the version line is invalid: $versionLine"
    }
    & "$root\bin\imageprintlayout-console.exe" -selftest
    Assert-Success "imageprintlayout-console.exe -selftest"
    foreach ($lang in "fr", "en", "es", "de") {
        Test-Window "$root\bin\imageprintlayout.exe" $lang "portable-$lang"
    }
}

# The acceptance tests (specification, §13): the engine, the export and the
# project on this Windows. poppler provides pdftoppm and pdfinfo, which rasterise
# the PDFs written; with CI set, a missing tool makes the test fail
# instead of skipping it.
function Test-Acceptance {
    $root = if ($env:MSYS2_ROOT) { $env:MSYS2_ROOT } else { "C:\msys64" }
    $bash = Join-Path $root "usr\bin\bash.exe"
    if (-not (Test-Path $bash)) { throw "MSYS2 not found ($root)" }
    $env:MSYSTEM = "UCRT64"
    $env:CHERE_INVOKING = "1"
    & $bash -lc "pacman -S --needed --noconfirm mingw-w64-ucrt-x86_64-poppler" 2>&1 | ForEach-Object { Write-Host "$_" }
    Assert-Success "installing poppler"
    & $bash -lc "CGO_ENABLED=1 go test -count=1 -v ./internal/acceptance" 2>&1 | ForEach-Object { Write-Host "$_" }
    Assert-Success "go test ./internal/acceptance"
}

function Test-Installer {
    $setup = Join-Path (Get-Location) "imageprintlayout_$(Get-Version)_windows-setup.exe"
    $target = Join-Path $env:RUNNER_TEMP "installed"
    Start-Process -FilePath $setup -ArgumentList "/S", "/D=$target" -Wait
    if (-not (Test-Path "$target\bin\imageprintlayout.exe")) { throw "the installer did not install the program" }
    if (-not (Test-Path $uninstallKey)) { throw "the uninstall entry is missing" }
    if (-not (Test-Path "$env:ProgramData\Microsoft\Windows\Start Menu\Programs\Image Print Layout\Image Print Layout.lnk")) {
        throw "the Start menu shortcut is missing"
    }
    $env:Path = $cleanPath
    & "$target\bin\imageprintlayout-console.exe" -selftest
    Assert-Success "imageprintlayout-console.exe -selftest (installed program)"
    Test-Window "$target\bin\imageprintlayout.exe" "en" "installed-en"
    # _?= makes the uninstaller run in place and wait, instead of copying
    # itself to a temporary folder and returning right away.
    Start-Process -FilePath "$target\Uninstall.exe" -ArgumentList "/S", "_?=$target" -Wait
    if (Test-Path "$target\bin") { throw "the uninstaller left the program behind" }
    if (Test-Path $uninstallKey) { throw "the uninstall entry is still there" }
}

# The packages of the release: the version is that of VERSION, which does not move.
function Invoke-Release {
    $version = Get-Version
    .\build-windows.ps1 -NoVersionChange -Yes
    Assert-Success "build-windows.ps1 -NoVersionChange -Yes"
    $after = Get-Version
    if ($after -ne $version) { throw "VERSION is '$after' after -NoVersionChange, expected '$version'" }
    Get-ChildItem -File -Filter "imageprintlayout_*" | ForEach-Object { Write-Host ("{0,12:N0} bytes  {1}" -f $_.Length, $_.Name) }
    foreach ($name in "imageprintlayout_${version}_windows-portable.zip", "imageprintlayout_${version}_windows-setup.exe") {
        if (-not (Test-Path $name)) { throw "$name is missing at the root of the repository" }
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
    # Success: the end of the output stays readable in an annotation, as on failure.
    $summary = ((Get-Content $log -Tail 60 -ErrorAction SilentlyContinue) -join "`n")
    $summary = $summary.Replace("%", "%25").Replace("`r", "%0D").Replace("`n", "%0A")
    Write-Host "::notice title=Step $Step done::$summary"
} catch {
    $tail = ((Get-Content $log -Tail 60 -ErrorAction SilentlyContinue) -join "`n") + "`n" + $_.Exception.Message
    $tail = $tail.Replace("%", "%25").Replace("`r", "%0D").Replace("`n", "%0A")
    Write-Host "::error title=Step $Step failed::$tail"
    exit 1
}

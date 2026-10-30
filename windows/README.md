<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Image Print Layout: Windows package

🇫🇷 [Version française](README.fr.md)

This folder contains what is needed to build the Windows package: a **portable
version** (`.zip`) and an **installer** (`.exe`), from the code of the repository.

## Usage

Windows blocks local PowerShell scripts by default (error such as *"running scripts
is disabled on this system"*). Allow them for the current session
only, which changes nothing permanently:

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
```

Then, from PowerShell, **at the root of the repository**:

```powershell
.\build-windows.ps1
```

The script has the same options as `build-deb.sh` and `build-rpm.sh` (and as the
`build-windows.ps1` of Planche-Contact):

```powershell
.\build-windows.ps1 -Major             # increments the major number   (1.2.3 -> 2.0.0)
.\build-windows.ps1 -Minor             # increments the minor number   (1.2.3 -> 1.3.0)
.\build-windows.ps1 -Patch             # increments the patch number   (1.2.3 -> 1.2.4)
.\build-windows.ps1 -NoVersionChange   # rebuilds without changing the version
.\build-windows.ps1 -Clean             # deletes the build folder and the Windows packages
.\build-windows.ps1 -Help              # help
.\build-windows.ps1 -Oui               # answers "yes" to all the questions (unattended)
```

Without a version option, the script shows the current version (`VERSION` file),
offers to move to major, minor or patch or to change nothing, then asks for
confirmation before building. **`VERSION` is only updated once the packages are
built**: a failure does not leave a version without a package. A pre-release
version such as `0.1.0-dev` is published first: `-Minor` and `-Patch` give
`0.1.0`, `-Major` gives `1.0.0`. If you have already built the Linux packages of this
version, use `-NoVersionChange`, so as not to increment it a second time.

The script checks its prerequisites and offers to install what is missing (see below).
At the end, the root of the repository contains:

- `imageprintlayout_X.Y.Z_windows-portable.zip`: a standalone folder, to unzip
  anywhere and launch without installation;
- `imageprintlayout_X.Y.Z_windows-setup.exe`: the installer (Start menu, uninstaller),
  if NSIS is available.

The working folder is `%LOCALAPPDATA%\imageprintlayout-build`, outside the repository; `-Clean`
deletes it.

## SmartScreen warning at installation

⚠️ **To warn users about**: on the first launch of the installer (or of the
portable program), Windows will very likely display a blue **"Windows protected your
PC"** screen (SmartScreen). This is expected: the installer is not digitally
signed. It is not a sign of malicious software.

To continue: click **More info**, then **Run
anyway**.

The warning fades as the installer gains reputation with
SmartScreen, or disappears immediately with a paid code-signing certificate.
No change to the code can remove it.

## Prerequisites

⚠️ **64-bit Windows (x64) only.**

| Tool | Role | Installation offered by the script |
| --- | --- | --- |
| [MSYS2](https://www.msys2.org) | Provides, for Windows, Go, the C compiler, GTK 4, librsvg and cairo (UCRT64 environment) | Yes, with `winget` (in `C:\msys64`) |
| MSYS2 packages: `go`, `gcc`, `pkgconf`, `librsvg`, `gdk-pixbuf2`, `cairo`, `gtk4`, `gobject-introspection`, `adwaita-icon-theme`, `hicolor-icon-theme`, `zip`, `git` | The build chain, and what the window bundles | Yes, with `pacman`, after an MSYS2 update |
| [NSIS](https://nsis.sourceforge.io/) | Produces the `.exe` installer (optional) | Yes, with `winget` |

Without NSIS, only the portable version is produced; this is not an error.

The script touches nothing outside MSYS2, its working folder and the root of the
repository, where it writes the packages. If you prefer, run the build by hand in an
**MSYS2 UCRT64** terminal (and not MSYS nor MINGW64); it takes the version from the
`VERSION` file, or from `--version`, and never modifies it:

```sh
pacman -Syu
./windows/build-windows.sh --install-deps
./windows/build-windows.sh --version 1.2.3
```

## Contents of the package

The delivered folder is organized like a prefix, which is what the libraries expect:

```
imageprintlayout-X.Y.Z-windows-x64/
├── bin/
│   ├── imageprintlayout.exe          the window (Windows subsystem: no console behind it)
│   ├── imageprintlayout-console.exe  the same program as a console one: -version, -selftest, error messages
│   └── *.dll                       GTK 4, cairo, librsvg, pango, glib, fontconfig...
├── etc/fonts/                      fontconfig configuration
├── lib/gdk-pixbuf-2.0/2.10.0/      image loaders (JPEG, PNG, TIFF, GIF, BMP, WebP, SVG) and their cache
├── share/glib-2.0/schemas/         GTK settings schemas (without them, GTK stops when a file chooser opens)
├── share/icons/                    Adwaita and hicolor icon themes (button images)
├── share/locale/                   translations of GTK's own texts (French, Spanish, German)
├── share/licenses/                 license of each library, and COMPOSANTS.txt (list, versions)
├── LICENSE, README.md, README.fr.md, CHANGELOG.md, CHANGELOG.fr.md
```

A Windows program is not installed under a fixed prefix: fontconfig and gdk-pixbuf
therefore do not know where to look for their data. At startup, the program tells them
(`engine.ConfigureRuntime`, which only acts if this folder is there). GLib and GTK
find their schemas, icons and translations from the location of
their DLLs, which requires the `bin/` folder next to `share/`. The program also asks
pango to use fontconfig, as on Linux, so that the fonts of a document
are found, or replaced, in the same way everywhere.

There are two executables because a Windows program with a window writes nothing to
a terminal, and a console program opens a black window behind its windows.
`imageprintlayout.exe` is the one for shortcuts; in a terminal, use
`imageprintlayout-console.exe -version` to attach the version to a problem report.

The packages weigh several tens of MB (GTK 4 and its dependencies); the CI gives the
exact size of each build.

**Languages.** The window follows the language of Windows (French, English, Spanish or
German, English otherwise); the window's menu or `-lang` picks another one, and
the choice is saved in `%AppData%\imageprintlayout\settings.json`. The installer
also follows the language of Windows.

The fonts come from Windows (`C:\Windows\Fonts`, and the fonts installed by the
user): **the package bundles none**. A drawing that asks for IBM Plex
will have it if it is installed; otherwise, a replacement font is chosen (see
`ARCHITECTURE.md`, §12).

## How it works

`build-windows.ps1` (at the root, a simple relay) calls `windows\build-windows.ps1`,
which reads `VERSION`, computes the version to build, asks for confirmation, prepares MSYS2 and
NSIS, then runs `windows/build-windows.sh --version X.Y.Z` in an MSYS2 UCRT64 terminal;
when everything has succeeded, it writes the new version to `VERSION`. The bash script:

1. checks the MSYS2 packages (and installs them with `--install-deps`);
2. makes the Windows resource (icon, file properties) with `windres`;
3. compiles the program with Go and cgo (GTK 4, librsvg and cairo from MSYS2), twice: as a
   Windows subsystem program (the window) and as a console subsystem program, and checks the
   subsystem of each;
4. prepares the package folder: the two programs, the DLLs they depend on
   (found with `ldd`, step by step: the DLLs of Windows itself are excluded),
   the image loaders and their cache, the fontconfig configuration, the schemas,
   the icons and the translations of GTK, the licenses;
5. **runs the console program with `-selftest` and only the Windows PATH**: if a DLL, a
   font or an image loader is missing, the build stops there, without producing a
   broken package;
6. creates the `.zip` (file order and dates fixed), then the installer with NSIS.

The installer (`installer.nsi`) copies this same folder into `C:\Program Files\SVG Print
Layout`, adds the entry in *Settings > Apps* and the shortcuts, and
uninstalls by deleting exactly the files it installed: no folder
is erased wholesale, in case the user chose a folder that contains
something else. An update first removes the installed version. An installation of "SVG Print Layout" (the program's name up to 1.0.x) is removed in the same way by the Image Print Layout installer, with its own uninstaller; the settings and the projects remain.

Silent installation, for a deployment:

```powershell
.\imageprintlayout_X.Y.Z_windows-setup.exe /S /D=C:\Tools\Image Print Layout   # /D= last, without quotes
C:\Tools\Image Print Layout\Uninstall.exe /S
```

## Files in this folder

| File | Role |
| --- | --- |
| `build-windows.ps1` | Options and version, prerequisites (MSYS2, NSIS, offering to install them), runs the build |
| `build-windows.sh` | The build, in an MSYS2 UCRT64 terminal |
| `installer.nsi` | The NSIS script of the installer |
| `imageprintlayout.ico` | The icon of the program and of the installer |

The two `.ps1` files and `installer.nsi` are in **UTF-8 with a byte order mark
(BOM)**: Windows PowerShell 5.1 and NSIS need it to read accents.
Do not save them without a BOM.

## What is verified, and by whom

The CI (`windows-package` job, on `windows-2022` and `windows-latest`) does exactly what
a user does: it runs `.\build-windows.ps1 -Patch -Oui` in Windows PowerShell 5.1 (and checks that
`VERSION` is updated after the build), then
unzips the `.zip`, runs `imageprintlayout-console.exe -selftest` with only the PATH of
Windows, then **opens the window** in the four languages (the program creates a file
with `-ready-file` when the window is displayed and the document drawn) and takes
a screenshot of it, deposited as a `windows-screenshots-<system>` artifact; it then installs silently
with the installer, runs `-selftest` and the window from the installed folder,
and uninstalls, checking that nothing remains; finally `-Clean` must remove the
packages. GTK's software rendering is requested for these trials (`GSK_RENDERER=cairo`):
a virtual machine has no graphics card.

The installer has also been compiled and run under Wine, on Linux, with a test
tree: installation, update over an existing installation, uninstallation.

**Windows 10 and 11.** The packages target both, in 64-bit. The CI has no Windows 10 nor
Windows 11, only Windows Server: Windows Server 2022 (the oldest offered) serves
as the reference for Windows 10, Windows Server 2025 for Windows 11. Both were built and
tried, and the CI packages were tried by hand on a real Windows machine
(M5 acceptance run). These are the same packages for both versions of Windows: the installer
only checks for 64-bit.

**Not yet seen on a real machine**: the build by the user with their own
MSYS2 installations (the CI does it on a fresh MSYS2). Adjustments may be
needed on the first trial.

## Troubleshooting

**"ce script doit s'exécuter dans un terminal MSYS2 UCRT64"** (the script's message, in French: "this script must run in an MSYS2 UCRT64 terminal")
`build-windows.sh` was run in another terminal (MSYS, MINGW64, Git Bash).
Run `.\build-windows.ps1`, or open "MSYS2 UCRT64" from the Start menu.

**`pacman`: "target not found" or 404 errors on the packages**
The MSYS2 package database is out of date. In an MSYS2 UCRT64 terminal, `pacman -Syu`
(twice if MSYS2 asks to restart the update), then rerun the build.

**"bibliothèques introuvables pour …"** (the script's message, in French: "libraries not found for …")
`ldd` does not see an MSYS2 DLL: this is a sign that the script was not run in
a UCRT64 terminal, or that a package is half installed (`pacman -S --needed` repairs
it).

**The window does not open, or a command line prints nothing**
`imageprintlayout.exe` is the program with a window: launched from a terminal it writes nothing.
Use `imageprintlayout-console.exe` to see the messages (`-version`, `-selftest`, or
the message of a startup error).

**"l'auto-test du programme échoue hors de MSYS2"** (the script's message, in French: "the program's self-test fails outside MSYS2")
The package is incomplete. The message of `-selftest`, displayed just above, says what is
missing: a font ("aucune police n'a été incorporée", i.e. "no font was embedded"), a PNG image that does not appear
(image loaders or their cache), or a DLL (the Windows error window
names the DLL).

**NSIS not found**
`winget install NSIS.NSIS`, then rerun. The script looks for `makensis.exe` in
`C:\Program Files (x86)\NSIS` and `C:\Program Files\NSIS`, or in the folder of the
`NSIS_HOME` variable.

**Accents are unreadable in the console**
A Windows console with an old code page: `chcp 65001`, or use Windows
Terminal.

**An antivirus quarantines the installer**
An unsigned, little-used executable can trigger an alert. This is not an
indication of infection; the code is public, and you can build the package yourself with this
script.

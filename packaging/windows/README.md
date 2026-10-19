<!-- SPDX-License-Identifier: GPL-3.0-only -->

# SVG Print Layout : paquet Windows

Ce dossier contient ce qu'il faut pour construire le paquet Windows : une **version
portable** (`.zip`) et un **installeur** (`.exe`), à partir du code du dépôt.

## Utilisation

Windows bloque par défaut les scripts PowerShell locaux (erreur du type *« l'exécution
de scripts est désactivée sur ce système »*). Autorisez-les pour la session en cours
seulement, ce qui ne change rien de façon permanente :

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
```

Puis, depuis PowerShell, **à la racine du dépôt** :

```powershell
.\build-windows.ps1
```

Options :

```powershell
.\build-windows.ps1 -SansInstalleur    # ne produit que la version portable
.\build-windows.ps1 -Oui               # répond « oui » à toutes les questions
.\build-windows.ps1 -Clean             # supprime le dossier de travail et les paquets Windows
.\build-windows.ps1 -Help              # aide
```

Le script vérifie ses prérequis et propose d'installer ce qui manque (voir ci-dessous).
À la fin, `dist\` contient :

- `svgprintlayout_X.Y.Z_windows-portable.zip` : un dossier autonome, à décompresser
  n'importe où et à lancer sans installation ;
- `svgprintlayout_X.Y.Z_windows-setup.exe` : l'installeur (menu Démarrer, désinstalleur),
  si NSIS est disponible.

Comme pour les paquets Linux, **le numéro de version vient du fichier `VERSION` et n'est
jamais modifié par le script**.

## Avertissement SmartScreen à l'installation

⚠️ **À prévenir auprès des utilisateurs** : au premier lancement de l'installeur (ou du
programme portable), Windows affiche très probablement un écran bleu **« Windows a
protégé votre PC »** (SmartScreen). C'est attendu : l'installeur n'est pas signé
numériquement. Ce n'est pas le signe d'un logiciel malveillant.

Pour continuer : cliquer sur **Informations complémentaires**, puis sur **Exécuter
quand même**.

L'avertissement s'atténue à mesure que l'installeur gagne en réputation auprès de
SmartScreen, ou disparaît tout de suite avec un certificat de signature de code payant.
Aucun changement du code ne peut le supprimer.

## Prérequis

⚠️ **Windows 64 bits (x64) uniquement.**

| Outil | Rôle | Installation proposée par le script |
| --- | --- | --- |
| [MSYS2](https://www.msys2.org) | Fournit, pour Windows, Go, le compilateur C, librsvg et cairo (environnement UCRT64) | Oui, avec `winget` (dans `C:\msys64`) |
| Paquets MSYS2 : `go`, `gcc`, `pkgconf`, `librsvg`, `cairo`, `zip`, `git` | La chaîne de construction | Oui, avec `pacman`, après une mise à jour de MSYS2 |
| [NSIS](https://nsis.sourceforge.io/) | Produit l'installeur `.exe` (facultatif) | Oui, avec `winget` |

Sans NSIS, seule la version portable est produite ; ce n'est pas une erreur.

Le script ne touche à rien hors de MSYS2 et de son dossier de travail
(`dist\build\windows`). Si vous préférez, lancez-le à la main dans un terminal
**MSYS2 UCRT64** (et pas MSYS ni MINGW64) :

```sh
pacman -Syu
./packaging/windows/build-windows.sh --install-deps
```

## Contenu du paquet

Le dossier livré est organisé comme un préfixe, ce que les bibliothèques attendent :

```
svgprintlayout-X.Y.Z-windows-x64/
├── bin/
│   ├── svgprintlayout.exe          le programme (avec son icône et ses propriétés)
│   └── *.dll                       cairo, librsvg, pango, glib, fontconfig... (43 à la dernière construction de la CI)
├── etc/fonts/                      configuration de fontconfig
├── lib/gdk-pixbuf-2.0/2.10.0/      chargeurs d'images (PNG, JPEG, GIF, BMP) et leur cache
├── share/licenses/                 licence de chaque bibliothèque, et COMPOSANTS.txt (liste, versions)
├── LICENSE, README.md, CHANGELOG.md
```

Un programme Windows n'est pas installé sous un préfixe fixe : fontconfig et gdk-pixbuf
ne savent donc pas où chercher leurs données. Au démarrage, le programme les leur
indique (`engine.ConfigureRuntime`, qui n'agit que si ce dossier est là). Il demande aussi
à pango d'utiliser fontconfig, comme sous Linux, pour que les polices d'un document
soient trouvées, ou remplacées, de la même façon partout.

Le `.zip` pèse une quinzaine de Mo et l'installeur une douzaine (compression solide).

Les polices viennent de Windows (`C:\Windows\Fonts`, et les polices installées par
l'utilisateur) : **le paquet n'en embarque aucune**. Un dessin qui demande IBM Plex
l'aura si elle est installée ; sinon, une police de remplacement est choisie (voir
`ARCHITECTURE.md`, §12).

## Comment ça marche

`build-windows.ps1` (à la racine, un simple relais) appelle
`packaging\windows\build-windows.ps1`, qui prépare MSYS2 et NSIS puis lance
`packaging/windows/build-windows.sh` dans un terminal MSYS2 UCRT64. Ce dernier :

1. vérifie les paquets MSYS2 (et les installe avec `--install-deps`) ;
2. fabrique la ressource Windows (icône, propriétés du fichier) avec `windres` ;
3. compile le programme avec Go et cgo (librsvg et cairo de MSYS2) ;
4. prépare le dossier du paquet : le programme, les DLL dont il dépend (trouvées avec
   `ldd`, de proche en proche : les DLL de Windows lui-même sont exclues), les chargeurs
   d'images et leur cache, la configuration de fontconfig, les licences ;
5. **lance le programme avec `-selftest` et le seul PATH de Windows** : si une DLL, une
   police ou un chargeur d'images manque, la construction s'arrête là, sans produire de
   paquet cassé ;
6. crée le `.zip` (ordre des fichiers et dates fixés), puis l'installeur avec NSIS.

L'installeur (`installer.nsi`) copie ce même dossier dans `C:\Program Files\SVG Print
Layout`, ajoute l'entrée de *Paramètres > Applications* et les raccourcis, et se
désinstalle en supprimant exactement les fichiers qu'il a installés : aucun dossier
n'est effacé en bloc, au cas où l'utilisateur aurait choisi un dossier qui contient
autre chose. Une mise à jour retire d'abord la version installée.

Installation silencieuse, pour un déploiement :

```powershell
.\svgprintlayout_X.Y.Z_windows-setup.exe /S /D=C:\Outils\SVGPrintLayout   # /D= en dernier, sans guillemets
C:\Outils\SVGPrintLayout\Uninstall.exe /S
```

## Fichiers de ce dossier

| Fichier | Rôle |
| --- | --- |
| `build-windows.ps1` | Prépare MSYS2 et NSIS (en proposant de les installer), lance la construction |
| `build-windows.sh` | La construction, dans un terminal MSYS2 UCRT64 ; GitHub Actions l'utilise aussi |
| `installer.nsi` | Le script NSIS de l'installeur |

Les deux fichiers `.ps1` et `installer.nsi` sont en **UTF-8 avec marque d'ordre des
octets (BOM)** : Windows PowerShell 5.1 et NSIS en ont besoin pour lire les accents.
Ne les enregistrez pas sans BOM.

## Ce qui est vérifié, et par qui

La CI (job `windows-package`, sur `windows-latest`) fait exactement ce que fait un
utilisateur : elle exécute `.\build-windows.ps1 -Oui` dans Windows PowerShell 5.1, puis
décompresse le `.zip` et lance `svgprintlayout.exe -selftest` avec le seul PATH de
Windows, puis installe en silence avec l'installeur, lance `-selftest` depuis le dossier
installé, et désinstalle en vérifiant que rien ne reste.

L'installeur a aussi été compilé et exécuté sous Wine, sur Linux, avec une arborescence
d'essai : installation, mise à jour par-dessus une installation, désinstallation.

**Pas d'essai sur une machine Windows réelle** : ni la construction par l'utilisateur
avec ses propres installations de MSYS2, ni le comportement de SmartScreen, ni
l'affichage dans *Paramètres > Applications* n'ont été vus de ses yeux. Des ajustements
seront probablement nécessaires au premier essai réel.

## Dépannage

**« ce script doit s'exécuter dans un terminal MSYS2 UCRT64 »**
`build-windows.sh` a été lancé dans un autre terminal (MSYS, MINGW64, Git Bash).
Lancez `.\build-windows.ps1`, ou ouvrez « MSYS2 UCRT64 » dans le menu Démarrer.

**`pacman` : « target not found » ou erreurs 404 sur les paquets**
La base de paquets de MSYS2 est périmée. Dans un terminal MSYS2 UCRT64, `pacman -Syu`
(deux fois si MSYS2 demande de relancer la mise à jour), puis relancez la construction.

**« bibliothèques introuvables pour … »**
`ldd` ne voit pas une DLL de MSYS2 : c'est le signe que le script n'a pas été lancé dans
un terminal UCRT64, ou qu'un paquet est à moitié installé (`pacman -S --needed` le
répare).

**« l'auto-test du programme échoue hors de MSYS2 »**
Le paquet est incomplet. Le message d'`-selftest`, affiché juste au-dessus, dit ce qui
manque : une police (« aucune police n'a été incorporée »), une image PNG qui n'apparaît
pas (chargeurs d'images ou leur cache), ou une DLL (la fenêtre d'erreur de Windows
nomme la DLL).

**NSIS introuvable**
`winget install NSIS.NSIS`, puis relancez. Le script cherche `makensis.exe` dans
`C:\Program Files (x86)\NSIS` et `C:\Program Files\NSIS`, ou dans le dossier de la
variable `NSIS_HOME`.

**Les accents sont illisibles dans la console**
Une console Windows en page de codes ancienne : `chcp 65001`, ou utilisez Windows
Terminal.

**Un antivirus met l'installeur en quarantaine**
Un exécutable non signé et peu répandu peut déclencher une alerte. Ce n'est pas un
indice d'infection ; le code est public, et le paquet se construit soi-même avec ce
script.

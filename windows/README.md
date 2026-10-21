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

Le script a les mêmes options que `build-deb.sh` et `build-rpm.sh` (et que le
`build-windows.ps1` de Planche-Contact) :

```powershell
.\build-windows.ps1 -Major             # incrémente le numéro majeur   (1.2.3 -> 2.0.0)
.\build-windows.ps1 -Minor             # incrémente le numéro mineur   (1.2.3 -> 1.3.0)
.\build-windows.ps1 -Patch             # incrémente le numéro de patch (1.2.3 -> 1.2.4)
.\build-windows.ps1 -NoVersionChange   # reconstruit sans changer la version
.\build-windows.ps1 -Clean             # supprime le dossier de build et les paquets Windows
.\build-windows.ps1 -Help              # aide
.\build-windows.ps1 -Oui               # répond « oui » à toutes les questions (sans surveillance)
```

Sans option de version, le script affiche la version actuelle (fichier `VERSION`),
propose de passer en major, minor ou patch ou de ne rien changer, puis demande
confirmation avant de construire. **`VERSION` n'est mis à jour qu'une fois les paquets
construits** : un échec ne laisse pas une version sans paquet. Une version
d'avant-publication comme `0.1.0-dev` est d'abord publiée : `-Minor` et `-Patch` donnent
`0.1.0`, `-Major` donne `1.0.0`. Si vous avez déjà construit les paquets Linux de cette
version, utilisez `-NoVersionChange`, pour ne pas l'incrémenter une seconde fois.

Le script vérifie ses prérequis et propose d'installer ce qui manque (voir ci-dessous).
À la fin, la racine du dépôt contient :

- `svgprintlayout_X.Y.Z_windows-portable.zip` : un dossier autonome, à décompresser
  n'importe où et à lancer sans installation ;
- `svgprintlayout_X.Y.Z_windows-setup.exe` : l'installeur (menu Démarrer, désinstalleur),
  si NSIS est disponible.

Le dossier de travail est `%LOCALAPPDATA%\svgprintlayout-build`, hors du dépôt ; `-Clean`
le supprime.

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
| [MSYS2](https://www.msys2.org) | Fournit, pour Windows, Go, le compilateur C, GTK 4, librsvg et cairo (environnement UCRT64) | Oui, avec `winget` (dans `C:\msys64`) |
| Paquets MSYS2 : `go`, `gcc`, `pkgconf`, `librsvg`, `cairo`, `gtk4`, `gobject-introspection`, `adwaita-icon-theme`, `hicolor-icon-theme`, `zip`, `git` | La chaîne de construction, et ce que la fenêtre embarque | Oui, avec `pacman`, après une mise à jour de MSYS2 |
| [NSIS](https://nsis.sourceforge.io/) | Produit l'installeur `.exe` (facultatif) | Oui, avec `winget` |

Sans NSIS, seule la version portable est produite ; ce n'est pas une erreur.

Le script ne touche à rien hors de MSYS2, de son dossier de travail et de la racine du
dépôt, où il écrit les paquets. Si vous préférez, lancez la construction à la main dans un
terminal **MSYS2 UCRT64** (et pas MSYS ni MINGW64) ; elle prend la version du fichier
`VERSION`, ou celle de `--version`, et ne le modifie jamais :

```sh
pacman -Syu
./windows/build-windows.sh --install-deps
./windows/build-windows.sh --version 1.2.3
```

## Contenu du paquet

Le dossier livré est organisé comme un préfixe, ce que les bibliothèques attendent :

```
svgprintlayout-X.Y.Z-windows-x64/
├── bin/
│   ├── svgprintlayout.exe          la fenêtre (sous-système Windows : pas de console derrière)
│   ├── svgprintlayout-console.exe  le même programme en console : -version, -selftest, messages d'erreur
│   └── *.dll                       GTK 4, cairo, librsvg, pango, glib, fontconfig...
├── etc/fonts/                      configuration de fontconfig
├── lib/gdk-pixbuf-2.0/2.10.0/      chargeurs d'images (PNG, JPEG, GIF, BMP, WebP, SVG) et leur cache
├── share/glib-2.0/schemas/         schémas de réglages de GTK (sans eux, GTK s'arrête à l'ouverture d'un sélecteur de fichier)
├── share/icons/                    thèmes d'icônes Adwaita et hicolor (images des boutons)
├── share/locale/                   traductions des textes propres à GTK (français, espagnol, allemand)
├── share/licenses/                 licence de chaque bibliothèque, et COMPOSANTS.txt (liste, versions)
├── LICENSE, README.md, CHANGELOG.md
```

Un programme Windows n'est pas installé sous un préfixe fixe : fontconfig et gdk-pixbuf
ne savent donc pas où chercher leurs données. Au démarrage, le programme les leur
indique (`engine.ConfigureRuntime`, qui n'agit que si ce dossier est là). GLib et GTK
retrouvent leurs schémas, leurs icônes et leurs traductions à partir de l'emplacement de
leurs DLL, ce qui impose le dossier `bin/` à côté de `share/`. Le programme demande aussi
à pango d'utiliser fontconfig, comme sous Linux, pour que les polices d'un document
soient trouvées, ou remplacées, de la même façon partout.

Il y a deux exécutables parce qu'un programme Windows à fenêtre n'écrit nulle part dans
un terminal, et qu'un programme console ouvre une fenêtre noire derrière ses fenêtres.
`svgprintlayout.exe` est celui des raccourcis ; dans un terminal, utilisez
`svgprintlayout-console.exe -version` pour joindre la version à un signalement de problème.

Les paquets pèsent plusieurs dizaines de Mo (GTK 4 et ses dépendances) ; la CI donne la
taille exacte de chaque construction.

**Langues.** La fenêtre suit la langue de Windows (français, anglais, espagnol ou
allemand, l'anglais sinon) ; le menu de la fenêtre ou `-lang` en choisit une autre, et
le choix est enregistré dans `%AppData%\svgprintlayout\settings.json`. L'installeur
suit lui aussi la langue de Windows.

Les polices viennent de Windows (`C:\Windows\Fonts`, et les polices installées par
l'utilisateur) : **le paquet n'en embarque aucune**. Un dessin qui demande IBM Plex
l'aura si elle est installée ; sinon, une police de remplacement est choisie (voir
`ARCHITECTURE.md`, §12).

## Comment ça marche

`build-windows.ps1` (à la racine, un simple relais) appelle `windows\build-windows.ps1`,
qui lit `VERSION`, calcule la version à construire, demande confirmation, prépare MSYS2 et
NSIS, puis lance `windows/build-windows.sh --version X.Y.Z` dans un terminal MSYS2 UCRT64 ;
quand tout a réussi, il écrit la nouvelle version dans `VERSION`. Le script bash :

1. vérifie les paquets MSYS2 (et les installe avec `--install-deps`) ;
2. fabrique la ressource Windows (icône, propriétés du fichier) avec `windres` ;
3. compile le programme avec Go et cgo (GTK 4, librsvg et cairo de MSYS2), deux fois : en
   sous-système Windows (la fenêtre) et en sous-système console, et vérifie le
   sous-système de chacun ;
4. prépare le dossier du paquet : les deux programmes, les DLL dont ils dépendent
   (trouvées avec `ldd`, de proche en proche : les DLL de Windows lui-même sont exclues),
   les chargeurs d'images et leur cache, la configuration de fontconfig, les schémas,
   les icônes et les traductions de GTK, les licences ;
5. **lance le programme console avec `-selftest` et le seul PATH de Windows** : si une DLL, une
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
| `build-windows.ps1` | Options et version, prérequis (MSYS2, NSIS, en proposant de les installer), lance la construction |
| `build-windows.sh` | La construction, dans un terminal MSYS2 UCRT64 |
| `installer.nsi` | Le script NSIS de l'installeur |
| `svgprintlayout.ico` | L'icône du programme et de l'installeur |

Les deux fichiers `.ps1` et `installer.nsi` sont en **UTF-8 avec marque d'ordre des
octets (BOM)** : Windows PowerShell 5.1 et NSIS en ont besoin pour lire les accents.
Ne les enregistrez pas sans BOM.

## Ce qui est vérifié, et par qui

La CI (job `windows-package`, sur `windows-2022` et `windows-latest`) fait exactement ce que
fait un utilisateur : elle exécute `.\build-windows.ps1 -Patch -Oui` dans Windows PowerShell 5.1 (et vérifie que
`VERSION` est mis à jour après la construction), puis
décompresse le `.zip`, lance `svgprintlayout-console.exe -selftest` avec le seul PATH de
Windows, puis **ouvre la fenêtre** dans les quatre langues (le programme crée un fichier
avec `-ready-file` quand la fenêtre est affichée et le document dessiné) et en enregistre
une capture d'écran, déposée en artefact `windows-screenshots-<système>` ; elle installe ensuite en
silence avec l'installeur, lance `-selftest` et la fenêtre depuis le dossier installé,
et désinstalle en vérifiant que rien ne reste ; enfin `-Clean` doit retirer les
paquets. Le rendu logiciel de GTK est demandé pour ces essais (`GSK_RENDERER=cairo`) :
une machine virtuelle n'a pas de carte graphique.

L'installeur a aussi été compilé et exécuté sous Wine, sur Linux, avec une arborescence
d'essai : installation, mise à jour par-dessus une installation, désinstallation.

**Windows 10 et 11.** Les paquets visent les deux, en 64 bits. La CI n'a pas de Windows 10 ni
de Windows 11, seulement des Windows Server : Windows Server 2022 (le plus ancien proposé) sert
de repère pour Windows 10, Windows Server 2025 pour Windows 11. Les deux ont été construits et
essayés, et les paquets de la CI ont été essayés à la main sur une machine Windows réelle
(recette de M5). Ce sont les mêmes paquets pour les deux versions de Windows : l'installeur
ne vérifie que le 64 bits.

**Pas encore vu sur une machine réelle** : la construction par l'utilisateur avec ses propres
installations de MSYS2 (la CI la fait sur un MSYS2 neuf). Des ajustements seront peut-être
nécessaires au premier essai.

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

**La fenêtre ne s'ouvre pas, ou une ligne de commande n'affiche rien**
`svgprintlayout.exe` est le programme à fenêtre : lancé depuis un terminal il n'écrit rien.
Utilisez `svgprintlayout-console.exe` pour voir les messages (`-version`, `-selftest`, ou
le message d'une erreur de démarrage).

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

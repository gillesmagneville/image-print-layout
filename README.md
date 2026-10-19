<!-- SPDX-License-Identifier: GPL-3.0-only -->

# svg-print-layout

Application de bureau (Linux/Ubuntu et Windows 10/11, écrite en Go) pour **mettre en page, découper et imprimer un document vectoriel SVG sur plusieurs feuilles de papier standard**, puis l'exporter en **PDF multipage vectoriel**.

> **État : prototype du moteur de rendu (M0a).** Rien n'est encore utilisable : le dépôt contient l'organisation des paquets, la bibliothèque de conversions d'unités et de formats papier, un prototype du moteur (librsvg + cairo : rendu d'une fenêtre et export PDF multipage, validé sur le Mini-CAD, voir [`docs/M0.md`](docs/M0.md)), les scripts qui construisent les paquets `.deb`, `.rpm` et Windows (qui n'installent pour l'instant qu'un programme sans interface, voir [Paquets](#paquets)) et la CI. Le prototype de l'interface GTK4 est la prochaine étape.

## Principe

1. On ouvre un SVG de dimensions quelconques (par exemple un schéma de 1 280 × 5 479 unités).
2. On choisit le format papier, l'orientation, les marges et la largeur du dessin (ex. 200 mm sur A4 portrait).
3. On place les coupures à la souris sur l'aperçu, ou on accepte une proposition automatique.
4. On vérifie le résultat page par page, puis on exporte un seul PDF.

Règles fondamentales, reprises du cahier des charges :

- **Une seule échelle pour tout le dessin** : les proportions ne sont jamais modifiées, et aucune page n'est réduite en silence pour « faire tenir » une portion trop haute — l'utilisateur est averti et choisit (ajouter une coupure, déplacer une coupure, changer l'échelle ou le papier).
- **Le nombre de pages découle des dimensions et des coupures** ; il n'est jamais imposé.
- **Le SVG d'origine n'est jamais modifié** : toutes les transformations (fond blanc, suppression du fond…) s'appliquent à une copie de travail.
- **Le PDF est fidèle à l'aperçu** et déterministe : mêmes paramètres, même résultat.
- **Aucun service en ligne**.

## Organisation du code

```
cmd/svgprintlayout/   point d'entrée (-version, -selftest, -lang)
build-deb.sh, build-rpm.sh, build-windows.ps1   construisent les paquets (voir Paquets)
debian/, metainfo/, icons/, windows/   fichiers qu'ils empaquettent ; docs/svgprintlayout.1 : page de manuel
internal/
  units/     conversions SVG / px / pt / mm, formats papier, marges      (Go pur, testé)
  model/     types de données partagés                                    (Go pur)
  layout/    calcul des pages, bandes, états, coupures automatiques       (Go pur)
  project/   fichier projet .svgpl (JSON versionné, écriture déterministe) (Go pur)
  svgdoc/    lecture du SVG : dimensions, unités, fonctions non prises en charge, fond
  engine/    moteur de rendu : librsvg + cairo par cgo (prototype M0a)
  export/    export PDF multipage et vérifications avant/après
  app/       état de l'application et cas d'usage, indépendants de l'interface
  diag/      avertissements et erreurs destinés à l'utilisateur (un code et des valeurs)
  i18n/      traduction : français, anglais, espagnol, allemand ; nombres  (Go pur, testé)
  ui/        interface graphique (GTK4 via gotk4 à valider en M0b)
  testsvg/   générateur de SVG d'essai déterministe                      (Go pur)
  repocheck/ tests sur le dépôt lui-même (en-têtes de licence, versions, documentation)
docs/        rapports de jalon (M0.md)
testdata/    SVG de référence
```

Le sens des dépendances va de `ui` vers `app`, puis vers `layout`, `project`, `svgdoc` et `export` ; `svgdoc` et `export` n'appellent `engine` que par son interface. Seuls `ui` et `engine` utilisent cgo : tous les autres paquets se testent partout, sans bibliothèque graphique installée.

## Documentation

- [`PROJECT_CONTEXT.md`](PROJECT_CONTEXT.md) : pourquoi le projet existe, ses principes et son périmètre.
- [`ARCHITECTURE.md`](ARCHITECTURE.md) : le contrat technique (paquets, interfaces, formules, format de projet, vérifications, plan de livraison). Il fait foi pour tout ce qui est concret.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) : signaler un bug, proposer une amélioration, contribuer du code.
- [`docs/M0.md`](docs/M0.md) : rapport du prototype du moteur de rendu (mesures, découvertes, ce qui reste à valider).
- [`CHANGELOG.md`](CHANGELOG.md) : l'historique des changements.

## Développement

Prérequis pour l'instant : Go 1.24 ou plus récent.

```sh
gofmt -l .          # ne doit rien afficher
go vet ./...
go test ./...
go run ./cmd/svgprintlayout -version
```

La CI (GitHub Actions) exécute ces commandes sous Ubuntu et Windows, avec `CGO_ENABLED=0`.

Le moteur de rendu (`internal/engine`) demande cgo, librsvg et cairo, et ses tests lisent des PDF avec poppler. Sous Ubuntu 24.04 :

```sh
sudo apt-get install librsvg2-dev poppler-utils fonts-ibm-plex
CGO_ENABLED=1 go test ./internal/engine ./internal/testsvg
```

Sans `CGO_ENABLED=1`, le paquet `engine` est vide et le reste se compile et se teste sans bibliothèque graphique. Un outil de poppler absent fait sauter le test concerné, sauf quand la variable `CI` est définie.

Les dépendances de l'interface (GTK4) et la procédure pour Windows (MSYS2) seront documentées ici une fois le volet M0b validé.

## Paquets

Trois scripts, à la racine, construisent les paquets (comme dans Planche-Contact) :

```sh
./build-deb.sh              # Debian, Ubuntu : svgprintlayout_<version>-1_amd64.deb
./build-rpm.sh              # Fedora, RHEL : svgprintlayout-<version>-<release>.x86_64.rpm
```

```powershell
.\build-windows.ps1         # Windows 10 et 11 : un .zip portable et un installeur .exe
```

Les paquets sont écrits à la racine du dépôt (non versionnés). Les trois scripts ont les
mêmes options et gèrent le fichier `VERSION` de la même façon :

| Option (PowerShell) | Effet |
| --- | --- |
| `--major` (`-Major`) | incrémente le numéro majeur (1.2.3 → 2.0.0) |
| `--minor` (`-Minor`) | incrémente le numéro mineur (1.2.3 → 1.3.0) |
| `--patch` (`-Patch`) | incrémente le numéro de patch (1.2.3 → 1.2.4) |
| `--no-version-change` (`-NoVersionChange`) | reconstruit sans changer la version |
| `--clean` (`-Clean`) | supprime le dossier de build et les paquets déjà construits |
| `--help`, `-h` (`-Help`) | affiche l'aide |

Sans option, le script affiche la version actuelle, propose de passer en major, minor ou
patch ou de ne rien changer, puis demande confirmation (`o`) avant de construire. **Le
fichier `VERSION` n'est mis à jour qu'une fois le paquet construit** : un échec ne laisse
jamais une version sans paquet. Sous PowerShell, `-Oui` répond « oui » à toutes les
questions (pour un usage sans surveillance).

Une version d'avant-publication comme `0.1.0-dev` est d'abord publiée : `--minor` et
`--patch` la font passer à `0.1.0`, `--major` à `1.0.0`. Les versions ordinaires
s'incrémentent comme d'habitude. Quand on construit les trois formats d'une même version,
on incrémente avec le premier script et on relance les deux autres avec
`--no-version-change` (`-NoVersionChange`) :

```sh
./build-deb.sh --minor
./build-rpm.sh --no-version-change
```

Après une version sans suffixe, ajoutez sa section `## [X.Y.Z]` à `CHANGELOG.md`
(voir `CONTRIBUTING.md`).

Chaque script vérifie ses prérequis. Les paquets Linux s'installent avec `apt install ./…deb`
ou `dnf install ./…rpm`. Sous Debian et Ubuntu, les prérequis de construction sont :

```sh
sudo apt install golang-go build-essential pkg-config librsvg2-dev dpkg-dev rpm
```

L'organisation des paquets (contenu, numéros de version `0.1.0~dev-1` / `0.1.dev`,
construction reproductible, dépendances) est décrite dans `ARCHITECTURE.md`, §4 « Paquets » ;
la construction Windows (MSYS2, NSIS, SmartScreen, dépannage) dans
[`windows/README.md`](windows/README.md).

Le programme installé sait s'auto-vérifier : `svgprintlayout -selftest` contrôle le
moteur de rendu (bibliothèques, polices, images incorporées, export PDF) sur la machine
où il tourne. C'est la première commande à lancer en cas de problème.

## Licence

Ce projet est distribué sous la licence [GNU General Public License v3.0](LICENSE) (GPL-3.0-only).

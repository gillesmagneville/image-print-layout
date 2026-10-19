<!-- SPDX-License-Identifier: GPL-3.0-only -->

# svg-print-layout

Application de bureau (Linux/Ubuntu et Windows 10/11, écrite en Go) pour **mettre en page, découper et imprimer un document vectoriel SVG sur plusieurs feuilles de papier standard**, puis l'exporter en **PDF multipage vectoriel**.

> **État : prototype du moteur de rendu (M0a).** Rien n'est encore utilisable : le dépôt contient l'organisation des paquets, la bibliothèque de conversions d'unités et de formats papier, un prototype du moteur (librsvg + cairo : rendu d'une fenêtre et export PDF multipage, validé sur le Mini-CAD, voir [`docs/M0.md`](docs/M0.md)), les scripts qui construisent les paquets `.deb`, `.rpm` et Windows (qui n'installent pour l'instant qu'un programme sans interface, voir [`packaging/README.md`](packaging/README.md)) et la CI. Le prototype de l'interface GTK4 est la prochaine étape.

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
cmd/svgprintlayout/   point d'entrée (-version, -selftest)
packaging/   données et modèles des paquets .deb, .rpm et Windows
build-deb.sh, build-rpm.sh, build-windows.ps1   construisent les paquets (voir Paquets)
internal/
  units/     conversions SVG / px / pt / mm, formats papier, marges      (Go pur, testé)
  model/     types de données partagés                                    (Go pur)
  layout/    calcul des pages, bandes, états, coupures automatiques       (Go pur)
  project/   fichier projet .svgpl (JSON versionné, écriture déterministe) (Go pur)
  svgdoc/    lecture du SVG : dimensions, unités, fonctions non prises en charge, fond
  engine/    moteur de rendu : librsvg + cairo par cgo (prototype M0a)
  export/    export PDF multipage et vérifications avant/après
  app/       état de l'application et cas d'usage, indépendants de l'interface
  diag/      messages et avertissements destinés à l'utilisateur
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

Trois scripts, à la racine, construisent les paquets dans `dist/` à partir du code et du
numéro de `VERSION` (qu'ils ne modifient jamais) :

```sh
./build-deb.sh              # Debian, Ubuntu : dist/svgprintlayout_<version>_amd64.deb
./build-rpm.sh              # Fedora, RHEL : dist/svgprintlayout-<version>-<release>.x86_64.rpm
```

```powershell
.\build-windows.ps1         # Windows 10 et 11 : un .zip portable et un installeur .exe
```

Chaque script répond à `--help` (`-Help` sous PowerShell) et vérifie ses prérequis. Les
détails (contenu des paquets, numéros de version, construction reproductible,
dépannage) sont dans [`packaging/README.md`](packaging/README.md) et
[`packaging/windows/README.md`](packaging/windows/README.md).

Le programme installé sait s'auto-vérifier : `svgprintlayout -selftest` contrôle le
moteur de rendu (bibliothèques, polices, images incorporées, export PDF) sur la machine
où il tourne. C'est la première commande à lancer en cas de problème.

## Licence

Ce projet est distribué sous la licence [GNU General Public License v3.0](LICENSE) (GPL-3.0-only).

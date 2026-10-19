<!-- SPDX-License-Identifier: GPL-3.0-only -->

# svg-print-layout

Application de bureau (Linux/Ubuntu et Windows 10/11, écrite en Go) pour **mettre en page, découper et imprimer un document vectoriel SVG sur plusieurs feuilles de papier standard**, puis l'exporter en **PDF multipage vectoriel**.

> **État : squelette du dépôt (étape M0 à venir).** Rien n'est encore utilisable : le dépôt contient l'organisation des paquets, la bibliothèque de conversions d'unités et de formats papier (testée), et la CI. Le prototype M0, qui valide le moteur de rendu et la bibliothèque graphique, est la prochaine étape.

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
cmd/svgprintlayout/   point d'entrée
internal/
  units/     conversions SVG / px / pt / mm, formats papier, marges      (Go pur, testé)
  model/     types de données partagés                                    (Go pur)
  layout/    calcul des pages, bandes, états, coupures automatiques       (Go pur)
  project/   fichier projet .svgpl (JSON versionné, écriture déterministe) (Go pur)
  svgdoc/    lecture du SVG : dimensions, unités, fonctions non prises en charge, fond
  engine/    moteur de rendu derrière une interface (librsvg + cairo prévus)
  export/    export PDF multipage et vérifications avant/après
  app/       état de l'application et cas d'usage, indépendants de l'interface
  diag/      messages et avertissements destinés à l'utilisateur
  ui/        interface graphique (GTK4 via gotk4 à valider en M0)
  repocheck/ tests sur le dépôt lui-même (en-têtes de licence, versions, documentation)
testdata/    SVG de référence (Mini-CAD à ajouter)
```

Le sens des dépendances va de `ui` vers `app`, puis vers `layout`, `project`, `svgdoc` et `export` ; `svgdoc` et `export` n'appellent `engine` que par son interface. Seuls `ui` et `engine` utilisent cgo : tous les autres paquets se testent partout, sans bibliothèque graphique installée.

## Documentation

- [`PROJECT_CONTEXT.md`](PROJECT_CONTEXT.md) : pourquoi le projet existe, ses principes et son périmètre.
- [`ARCHITECTURE.md`](ARCHITECTURE.md) : le contrat technique (paquets, interfaces, formules, format de projet, vérifications, plan de livraison). Il fait foi pour tout ce qui est concret.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) : signaler un bug, proposer une amélioration, contribuer du code.
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

Les dépendances natives (GTK4, cairo, librsvg, polices système) seront documentées ici une fois le prototype M0 validé, avec la procédure pour Ubuntu et pour Windows (MSYS2).

## Licence

Ce projet est distribué sous la licence [GNU General Public License v3.0](LICENSE) (GPL-3.0-only).

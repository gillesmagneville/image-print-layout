<!-- SPDX-License-Identifier: GPL-3.0-only -->

# svg-print-layout

Application de bureau (Linux — Debian, Ubuntu, Red Hat et leurs dérivés —, Windows 10 et 11, écrite en Go) pour **mettre en page, découper et imprimer un document vectoriel SVG sur plusieurs feuilles de papier standard**, puis l'exporter en **PDF multipage vectoriel**.

> **État : en construction (M2 fait, M3 à M5 à venir).** La fenêtre GTK4, en **français, anglais, espagnol et allemand**, ouvre un SVG, l'analyse (fonctions non prises en charge, polices absentes, images introuvables), calcule ses pages avec les réglages de papier, de marges, d'échelle et de positionnement, et montre l'aperçu avec les limites et les numéros de page, la liste des pages et les corrections proposées quand une page dépasse. Les coupures à la souris, la gestion du fond, l'export PDF et l'enregistrement du projet dans la fenêtre sont la suite (M3 à M5). Le dépôt contient aussi le prototype du moteur (librsvg + cairo : rendu d'une fenêtre et export PDF multipage, validé sur le Mini-CAD, voir [`docs/M0.md`](docs/M0.md)), les scripts qui construisent les paquets `.deb`, `.rpm` et Windows (voir [Paquets](#paquets)) et la CI.

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
cmd/svgprintlayout/   point d'entrée : la fenêtre, ou -version, -selftest, -lang
build-deb.sh, build-rpm.sh, build-windows.ps1   construisent les paquets (voir Paquets)
debian/, metainfo/, icons/, windows/   fichiers qu'ils empaquettent ; docs/svgprintlayout.1 (+ fr/, es/, de/) : pages de manuel
internal/
  units/     conversions SVG / px / pt / mm, formats papier, marges      (Go pur, testé)
  model/     types de données partagés                                    (Go pur)
  layout/    calcul des pages, bandes, états, coupures automatiques       (Go pur)
  project/   fichier projet .svgpl (JSON versionné, écriture déterministe) (Go pur)
  svgdoc/    lecture du SVG : constats, polices, blocs, fond, copie de travail   (Go pur)
  engine/    moteur de rendu : interface Document, librsvg + cairo + fontconfig par cgo
  export/    export PDF multipage et vérifications avant/après
  app/       session (projet, plan, historique), ouverture d'un document     (Go pur)
  viewport/  arithmétique de la vue : zoom, déplacement, fenêtre à dessiner   (Go pur)
  diag/      avertissements et erreurs destinés à l'utilisateur (un code et des valeurs)
  i18n/      traduction : français, anglais, espagnol, allemand ; nombres  (Go pur, testé)
  ui/        interface graphique : GTK4 via gotk4 (bouchon sans cgo)
  testsvg/   générateur de SVG d'essai déterministe                      (Go pur)
  repocheck/ tests sur le dépôt lui-même (en-têtes de licence, versions, documentation)
docs/        rapports de jalon (M0.md)
testdata/    SVG de référence
```

Le sens des dépendances va de `ui` vers `app` et `viewport`, puis de `app` vers `layout`, `project`, `svgdoc` et `export` ; `app` n'appelle `engine` que par son interface `Document`. Seuls `ui` et `engine` utilisent cgo : tous les autres paquets se testent partout, sans bibliothèque graphique installée.

## Documentation

- [`PROJECT_CONTEXT.md`](PROJECT_CONTEXT.md) : pourquoi le projet existe, ses principes et son périmètre.
- [`ARCHITECTURE.md`](ARCHITECTURE.md) : le contrat technique (paquets, interfaces, formules, format de projet, vérifications, plan de livraison). Il fait foi pour tout ce qui est concret.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) : signaler un bug, proposer une amélioration, contribuer du code.
- [`docs/M0.md`](docs/M0.md) : rapport du prototype du moteur de rendu (mesures, découvertes, ce qui reste à valider).
- [`CHANGELOG.md`](CHANGELOG.md) : l'historique des changements.

## Développement

Prérequis : Go 1.24 ou plus récent.

```sh
gofmt -l .          # ne doit rien afficher
go vet ./...
go test ./...
go run ./cmd/svgprintlayout -version
```

La CI (GitHub Actions) exécute ces commandes sous Ubuntu et Windows, avec `CGO_ENABLED=0`.

Le moteur de rendu (`internal/engine`) demande cgo, librsvg et cairo, et ses tests lisent des PDF avec poppler. Sous Ubuntu 24.04 :

```sh
sudo apt-get install librsvg2-dev libfontconfig-dev poppler-utils fonts-ibm-plex
CGO_ENABLED=1 go test ./internal/engine ./internal/testsvg
```

Sans `CGO_ENABLED=1`, le paquet `engine` est vide et le reste se compile et se teste sans bibliothèque graphique. Un outil de poppler absent fait sauter le test concerné, sauf quand la variable `CI` est définie.

La fenêtre (`internal/ui`) demande en plus GTK 4.14 ou plus récent (gotk4 v0.3.1) : sans cgo, `ui` est un bouchon et le programme dit qu'il a été construit sans interface graphique. Sous Ubuntu 24.04 :

```sh
sudo apt-get install librsvg2-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev poppler-utils fonts-ibm-plex
go run ./cmd/svgprintlayout                       # la fenêtre
go run ./cmd/svgprintlayout -lang de plan.svg     # dans une langue, avec un document
```

Les tests de la fenêtre ouvrent de vraies fenêtres : sans écran (terminal, CI), ils se lancent sur un écran virtuel, avec `sudo apt-get install xvfb xauth` :

```sh
xvfb-run -a go test ./internal/ui ./cmd/svgprintlayout
```

La première compilation de gotk4 dure plusieurs minutes ; les suivantes utilisent le cache de Go. À chaque compilation de gotk4, gcc affiche des avertissements sur `free` (`-Wbuiltin-declaration-mismatch`) : ils viennent de la bibliothèque, et les scripts de paquets les masquent. Pour les masquer aussi en développement : `export CGO_CFLAGS="-O2 -g -Wno-builtin-declaration-mismatch"`. Pour Windows (MSYS2 UCRT64), voir [`windows/README.md`](windows/README.md).

## Systèmes pris en charge

| Famille | Systèmes | Paquet |
| --- | --- | --- |
| Debian et dérivés | Debian 13 et suivantes, Ubuntu 24.04 LTS et suivantes, et leurs dérivés (Linux Mint 22, Pop!_OS 24.04, elementary OS 8…) | `.deb` |
| Red Hat et dérivés | RHEL 10 et ses clones (CentOS Stream 10, AlmaLinux 10, Rocky Linux 10…), Fedora | `.rpm` |
| Windows | Windows 10 et 11, 64 bits | `.zip` portable ou installeur `.exe` |

Sous Linux, ce qui compte est la version des bibliothèques : GTK 4.14, GLib 2.80 et librsvg 2.52
au moins. **Debian 12, Ubuntu 22.04 (et Linux Mint 21) et RHEL 9 (et ses clones) sont trop
anciens** : les scripts de construction le disent au lieu de compiler. La CI construit et
installe le paquet dans un conteneur de chacun des systèmes pris en charge et lance le
programme installé (détails dans [`ARCHITECTURE.md`](ARCHITECTURE.md), §4 « Plateformes prises
en charge »).

## Paquets

Trois scripts, à la racine, construisent les paquets (comme dans Planche-Contact) :

```sh
./build-deb.sh              # Debian, Ubuntu et dérivés : svgprintlayout_<version>_amd64.deb
./build-rpm.sh              # Fedora, RHEL et dérivés : svgprintlayout-<version>-<release>.x86_64.rpm
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

Après une version sans suffixe, mettez à jour tous les fichiers qui la portent : section
`## [X.Y.Z]` de `CHANGELOG.md`, `<release>` de la fiche AppStream, ligne `.TH` des quatre
pages de manuel, états du README et d'`ARCHITECTURE.md` (voir `CONTRIBUTING.md` ; un test
de `repocheck` échoue tant que l'un d'eux est en retard).

Chaque script vérifie ses prérequis, dont la version des bibliothèques. Un paquet est lié aux
bibliothèques de la machine qui l'a construit : on le construit sur le système où il sera
installé. Les paquets Linux s'installent avec `apt install ./…deb` ou `dnf install ./…rpm`.
Sous Debian et Ubuntu, les prérequis de construction sont :

```sh
sudo apt install golang-go build-essential pkg-config librsvg2-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev dpkg-dev rpm
```

Sous Fedora et la famille RHEL 10 (`golang` est dans AppStream ; `-devel` est dans CRB, à
activer avec `sudo dnf config-manager --set-enabled crb`) :

```sh
sudo dnf install golang gcc make pkgconf-pkg-config librsvg2-devel cairo-devel fontconfig-devel gtk4-devel gobject-introspection-devel rpm-build
```

Go 1.24 ou plus récent est nécessaire (`go.mod`) : si la version de la distribution est plus
ancienne, installez celle de <https://go.dev/dl/>.

L'organisation des paquets (contenu, numéros de version `0.1.0~dev` / `0.1.dev`,
construction reproductible, dépendances) est décrite dans `ARCHITECTURE.md`, §4 « Paquets » ;
la construction Windows (MSYS2, NSIS, SmartScreen, dépannage) dans
[`windows/README.md`](windows/README.md).

Le programme installé sait s'auto-vérifier : `svgprintlayout -selftest` contrôle le
moteur de rendu (bibliothèques, polices, images incorporées, export PDF) sur la machine
où il tourne. C'est la première commande à lancer en cas de problème.

## Licence

Ce projet est distribué sous la licence [GNU General Public License v3.0](LICENSE) (GPL-3.0-only).

<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Image Print Layout

🇬🇧 [English version](README.md)

Application de bureau (Linux — Debian, Ubuntu, Red Hat et leurs dérivés —, Windows 10 et 11, écrite en Go) pour **mettre en page, découper et imprimer un document SVG ou une image (JPEG, PNG, TIFF…) sur plusieurs feuilles de papier standard**, puis l'exporter en **PDF multipage** (vectoriel pour un SVG) ou en **page HTML unique**, image entière ou découpée aux coupures.

Développé par Gilles MAGNEVILLE, conçu avec l'aide de [claude.ai](https://claude.ai).

> **Image Print Layout s'appelait « SVG Print Layout » jusqu'à la version 1.0.x.** Le nom a changé parce que le programme ouvre aussi des images (JPEG, PNG, TIFF…), ce qu'il fait depuis la 1.2.0. Les projets `.svgpl` de la 1.0.x s'ouvrent toujours, la langue choisie est reprise et installer un paquet remplace l'ancien (voir le [CHANGELOG](CHANGELOG.fr.md)).

> **État : version 1.3.1, toutes les fonctions du cahier des charges sont là (jalons M1 à M5), et la recette a été faite à la main sous Ubuntu et sous Windows.** La fenêtre GTK4, en **français, anglais, espagnol et allemand**, ouvre un SVG ou une image (JPEG, PNG, TIFF, WebP, BMP, GIF), l'analyse (fonctions non prises en charge, polices absentes, images introuvables ; pour une image, sa résolution, un fichier tronqué ou de plusieurs pages), calcule ses pages avec les réglages de papier, de marges, d'échelle et de positionnement, et montre l'aperçu avec les limites et les numéros de page, la liste des pages et les corrections proposées quand une page dépasse. On y ajoute, déplace et supprime les coupures à la souris (double-clic, glisser, aimantation aux vides, clavier) ou on demande des coupures automatiques, sans jamais changer l'échelle. On y choisit le fond du document (le garder, le remplacer par du blanc, le supprimer, avec l'aperçu immédiat) et celui de la page (blanc opaque ou transparent), on voit chaque feuille telle qu'elle sera imprimée, puis on exporte un PDF multipage que le programme relit et vérifie avant de le livrer, ou une page HTML unique (Ctrl+Maj+E), relue elle aussi. On enregistre le projet (réglages, coupures, fond, positions) et on le rouvre plus tard : le fichier d'origine n'est jamais modifié, un document déplacé se localise, et une astérisque dans le titre signale les changements non enregistrés. Les 14 tests d'acceptation du cahier des charges sont automatisés (`internal/acceptance`). Le dépôt contient aussi le prototype du moteur (librsvg + cairo : rendu d'une fenêtre et export PDF multipage, validé sur le Mini-CAD, voir [`docs/M0.fr.md`](docs/M0.fr.md)), les scripts qui construisent les paquets `.deb`, `.rpm` et Windows (voir [Paquets](#paquets)) et la CI.

## Principe

1. On ouvre un SVG de dimensions quelconques (par exemple un schéma de 1 280 × 5 479 unités) ou une image (une photo, un scan, une longue capture).
2. On choisit le format papier, l'orientation, les marges et la largeur du dessin (ex. 200 mm sur A4 portrait).
3. On place les coupures à la souris sur l'aperçu, ou on accepte une proposition automatique.
4. On vérifie le résultat page par page, puis on exporte un seul PDF, ou une seule page HTML (voir [Page HTML](#page-html)).

Règles fondamentales, reprises du cahier des charges :

- **Une seule échelle pour tout le dessin** : les proportions ne sont jamais modifiées, et aucune page n'est réduite en silence pour « faire tenir » une portion trop haute — l'utilisateur est averti et choisit (ajouter une coupure, déplacer une coupure, changer l'échelle ou le papier).
- **Le nombre de pages découle des dimensions et des coupures** ; il n'est jamais imposé.
- **Le fichier d'origine n'est jamais modifié** : toutes les transformations (fond blanc, suppression du fond d'un SVG…) s'appliquent à une copie de travail.
- **Le PDF est fidèle à l'aperçu** et déterministe : mêmes paramètres, même résultat.
- **Aucun service en ligne**.

## Formats ouverts

Le format est reconnu au contenu du fichier, jamais à son extension. Une image se découpe et s'imprime comme un SVG : même échelle unique, mêmes coupures, même aperçu, même PDF.

| Format | Ce que le programme en fait |
| --- | --- |
| SVG | Rendu vectoriel par librsvg ; le PDF garde les tracés et le texte. |
| JPEG | Résolution (JFIF, à défaut Exif) et orientation (EXIF) lues. Le JPEG entre dans le PDF avec ses octets d'origine, sans recompression, quand le PDF sait le lire (8 bits, gris ou YCbCr). |
| PNG | Résolution et transparence lues ; le fond de page se voit à travers les parties transparentes. |
| TIFF | Première page ; résolution et orientation lues ; un fichier de plusieurs pages est signalé. |
| WebP | Lu quand le chargeur gdk-pixbuf de WebP est installé (`webp-pixbuf-loader` sous Linux, que les paquets recommandent). Sinon, le fichier est refusé en le disant. |
| BMP, GIF | Lus ; pour un GIF animé, seule la première image. |

La taille d'une image sur le papier est celle que son fichier indique : ses pixels divisés par sa résolution (une photo de 3 000 pixels à 300 points par pouce fait 254 mm à 100 %). Sans résolution, elle est comptée à 96 points par pouce, comme un écran, et le programme le dit. Une image fait au plus 100 millions de pixels : au-delà, elle est refusée avant d'être décodée. Une image n'a pas de fond à part à remplacer : seul le fond de page agit, à travers ses parties transparentes. Ses coupures automatiques tombent entre ses blocs quand elle a un fond uni (une capture, un schéma, une page numérisée) et se répartissent également quand elle n'en a pas (une photographie).

## Page HTML

Le bouton « Exporter en page HTML… » de la section Export (Ctrl+Maj+E) écrit la mise en page en **un seul fichier `.html`**, qui s'ouvre dans n'importe quel navigateur, hors connexion, sans dossier d'accompagnement : l'image y est écrite une seule fois.

- **Découpée aux coupures** (case cochée par défaut) : un morceau par page du plan, les uns sous les autres, avec leur légende (« Page 2 sur 4 »). Ce sont exactement les bandes de l'aperçu. Le navigateur imprime un morceau par feuille.
- **Entière** (case décochée) : l'image d'un seul tenant, à la largeur du plan.
- Un SVG reste vectoriel dans la page (son texte prend les polices de l'ordinateur qui ouvre la page) et ses images liées y sont incorporées. Un JPEG ou un PNG est écrit tel quel ; un TIFF, un GIF, un WebP ou un BMP, tel que l'aperçu le montre, en PNG.
- Comme le PDF, la page est relue après l'écriture et comparée au plan ; si elle ne correspond pas, rien n'est livré. Au-delà de 256 Mo, l'export est refusé.

## Organisation du code

```
cmd/imageprintlayout/   point d'entrée : la fenêtre, ou -version, -selftest, -lang
build-deb.sh, build-rpm.sh, build-windows.ps1   construisent les paquets (voir Paquets)
debian/, metainfo/, icons/, windows/   fichiers qu'ils empaquettent ; docs/imageprintlayout.1 (+ fr/, es/, de/) : pages de manuel
internal/
  units/     conversions SVG / px / pt / mm, formats papier, marges      (Go pur, testé)
  model/     types de données partagés                                    (Go pur)
  layout/    calcul des pages, bandes, états, coupures automatiques       (Go pur)
  project/   fichier projet .imgpl (JSON versionné, écriture déterministe) (Go pur)
  svgdoc/    lecture du SVG : constats, polices, blocs, fond, copie de travail   (Go pur)
  engine/    moteur de rendu : interface Document, librsvg (SVG) + gdk-pixbuf (images) + cairo + fontconfig par cgo
  export/    exports PDF multipage et page HTML : contrôles avant, relecture et vérification du fichier écrit   (Go pur)
  app/       session (projet, plan, historique), ouverture d'un document     (Go pur)
  viewport/  arithmétique de la vue : zoom, déplacement, fenêtre à dessiner   (Go pur)
  diag/      avertissements et erreurs destinés à l'utilisateur (un code et des valeurs)
  i18n/      traduction : français, anglais, espagnol, allemand ; nombres  (Go pur, testé)
  ui/        interface graphique : GTK4 via gotk4 (bouchon sans cgo)
  testsvg/   générateur de SVG d'essai déterministe                      (Go pur)
  testimg/   générateur d'images d'essai déterministe (PNG, TIFF, JPEG)   (Go pur)
  acceptance/ les tests d'acceptation du cahier des charges sur toute la chaîne
  repocheck/ tests sur le dépôt lui-même (en-têtes de licence, versions, documentation)
docs/        rapports de jalon (M0.md, M0.fr.md)
testdata/    documents de référence
```

Le sens des dépendances va de `ui` vers `app` et `viewport`, puis de `app` vers `layout`, `project`, `svgdoc` et `export` ; `app` n'appelle `engine` que par son interface `Document`. Seuls `ui` et `engine` utilisent cgo : tous les autres paquets se testent partout, sans bibliothèque graphique installée.

## Documentation

Chaque document existe en deux langues : l'anglais (le fichier par défaut, `X.md`) et le français (`X.fr.md`), chacun renvoyant à l'autre en tête de fichier.

- [`PROJECT_CONTEXT.fr.md`](PROJECT_CONTEXT.fr.md) : pourquoi le projet existe, ses principes et son périmètre.
- [`ARCHITECTURE.fr.md`](ARCHITECTURE.fr.md) : le contrat technique (paquets, interfaces, formules, format de projet, vérifications, plan de livraison). Il fait foi pour tout ce qui est concret.
- [`CONTRIBUTING.fr.md`](CONTRIBUTING.fr.md) : signaler un bug, proposer une amélioration, contribuer du code.
- [`docs/M0.fr.md`](docs/M0.fr.md) : rapport du prototype du moteur de rendu (mesures, découvertes, ce qui reste à valider).
- [`CHANGELOG.fr.md`](CHANGELOG.fr.md) : l'historique des changements.

## Développement

Prérequis : Go 1.24 ou plus récent.

```sh
gofmt -l .          # ne doit rien afficher
go vet ./...
go test ./...
go run ./cmd/imageprintlayout -version
```

La CI (GitHub Actions) exécute ces commandes sous Ubuntu et Windows, avec `CGO_ENABLED=0`.

Le moteur de rendu (`internal/engine`) demande cgo, librsvg, gdk-pixbuf et cairo, et ses tests lisent des PDF avec poppler. Sous Ubuntu 24.04 :

```sh
sudo apt-get install librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev poppler-utils fonts-ibm-plex
CGO_ENABLED=1 go test ./internal/engine ./internal/testsvg ./internal/testimg
```

Sans `CGO_ENABLED=1`, le paquet `engine` est vide et le reste se compile et se teste sans bibliothèque graphique. Un outil de poppler absent fait sauter le test concerné, sauf quand la variable `CI` est définie.

La fenêtre (`internal/ui`) demande en plus GTK 4.14 ou plus récent (gotk4 v0.3.1) : sans cgo, `ui` est un bouchon et le programme dit qu'il a été construit sans interface graphique. Sous Ubuntu 24.04 :

```sh
sudo apt-get install librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev poppler-utils fonts-ibm-plex
go run ./cmd/imageprintlayout                       # la fenêtre
go run ./cmd/imageprintlayout -lang de plan.svg     # dans une langue, avec un document (SVG ou image)
```

Les tests de la fenêtre ouvrent de vraies fenêtres : sans écran (terminal, CI), ils se lancent sur un écran virtuel, avec `sudo apt-get install xvfb xauth` :

```sh
xvfb-run -a go test ./internal/ui ./cmd/imageprintlayout
```

La première compilation de gotk4 dure plusieurs minutes ; les suivantes utilisent le cache de Go. À chaque compilation de gotk4, gcc affiche des avertissements sur `free` (`-Wbuiltin-declaration-mismatch`) : ils viennent de la bibliothèque, et les scripts de paquets les masquent. Pour les masquer aussi en développement : `export CGO_CFLAGS="-O2 -g -Wno-builtin-declaration-mismatch"`. Pour Windows (MSYS2 UCRT64), voir [`windows/README.fr.md`](windows/README.fr.md).

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
programme installé (détails dans [`ARCHITECTURE.fr.md`](ARCHITECTURE.fr.md), §4 « Plateformes prises
en charge »).

## Paquets

Trois scripts, à la racine, construisent les paquets (comme dans Planche-Contact) :

```sh
./build-deb.sh              # Debian, Ubuntu et dérivés : imageprintlayout_<version>_amd64.deb
./build-rpm.sh              # Fedora, RHEL et dérivés : imageprintlayout-<version>-<release>.x86_64.rpm
```

```powershell
.\build-windows.ps1         # Windows 10 et 11 : un .zip portable et un installeur .exe
```

Les paquets de chaque version sont aussi joints à sa [release GitHub](https://github.com/gillesmagneville/image-print-layout/releases) (`.deb`, `.rpm`, installeur et version portable Windows, avec leurs empreintes SHA-256).

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
`## [X.Y.Z]` de `CHANGELOG.fr.md` (et de `CHANGELOG.md`), `<release>` de la fiche AppStream, ligne `.TH` des quatre
pages de manuel, états du README et d'`ARCHITECTURE.fr.md` (voir `CONTRIBUTING.fr.md` ; un test
de `repocheck` échoue tant que l'un d'eux est en retard).

Chaque script vérifie ses prérequis, dont la version des bibliothèques. Un paquet est lié aux
bibliothèques de la machine qui l'a construit : on le construit sur le système où il sera
installé. Les paquets Linux s'installent avec `apt install ./…deb` ou `dnf install ./…rpm`.
Sous Debian et Ubuntu, les prérequis de construction sont :

```sh
sudo apt install golang-go build-essential pkg-config librsvg2-dev libgdk-pixbuf-2.0-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev dpkg-dev rpm
```

Sous Fedora et la famille RHEL 10 (`golang` est dans AppStream ; `-devel` est dans CRB, à
activer avec `sudo dnf config-manager --set-enabled crb`) :

```sh
sudo dnf install golang gcc make pkgconf-pkg-config librsvg2-devel gdk-pixbuf2-devel cairo-devel fontconfig-devel gtk4-devel gobject-introspection-devel rpm-build
```

Go 1.24 ou plus récent est nécessaire (`go.mod`) : si la version de la distribution est plus
ancienne, installez celle de <https://go.dev/dl/>.

L'organisation des paquets (contenu, numéros de version `0.1.0~dev` / `0.1.dev`,
construction reproductible, dépendances) est décrite dans `ARCHITECTURE.fr.md`, §4 « Paquets » ;
la construction Windows (MSYS2, NSIS, SmartScreen, dépannage) dans
[`windows/README.fr.md`](windows/README.fr.md).

Le programme installé sait s'auto-vérifier : `imageprintlayout -selftest` contrôle le
moteur de rendu (bibliothèques, polices, images incorporées, lecture d'un JPEG, d'un PNG et d'un TIFF, export PDF) sur la machine
où il tourne. C'est la première commande à lancer en cas de problème.

## Auteur

Image Print Layout est développé par **Gilles MAGNEVILLE**, et conçu avec l'aide de [claude.ai](https://claude.ai). La fenêtre « À propos » du programme (menu principal) le dit aussi, dans la langue de l'interface, de même que la section AUTEUR de la page de manuel.

## Licence

Ce projet est distribué sous la licence [GNU General Public License v3.0](LICENSE) (GPL-3.0-only).

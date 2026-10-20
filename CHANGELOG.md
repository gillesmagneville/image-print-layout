<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Changelog

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/),
et ce projet adhère au [Semantic Versioning](https://semver.org/lang/fr/).

## [Non publié]

### Ajouté

- **Quatre langues** : le paquet `internal/i18n` traduit l'interface en français, en anglais,
  en espagnol et en allemand (clés, paramètres nommés, pluriels, nombres avec virgule ou
  point, lecture d'un nombre tapé dans l'une ou l'autre notation), détecte la langue du
  système (Linux : `LANGUAGE`, `LC_ALL`, `LC_MESSAGES`, `LANG` ; Windows : langue de
  l'utilisateur) et ajoute l'option `-lang`. Des tests vérifient que les quatre catalogues ont
  les mêmes clés, les mêmes paramètres et aucun texte vide, et que le code ne demande que des
  clés qui existent.
- **Première fenêtre** (`internal/ui`, GTK4 par gotk4 v0.3.1) : trois panneaux (paramètres,
  aperçu, liste des pages), barre de titre avec « Ouvrir un SVG » et menu (langue, À propos,
  Quitter). Un SVG s'ouvre sans bloquer la fenêtre (le moteur tourne dans sa propre goroutine)
  et s'affiche en entier, ajusté à la vue ; un fichier qui n'est pas un SVG est refusé avec un
  message, le document ouvert reste. La langue se change à chaud (automatique, français,
  anglais, espagnol, allemand) ; le choix est enregistré dans `settings.json` du dossier de
  configuration (`internal/app`), hors des projets. Sans cgo, `ui` est un bouchon : le reste du
  dépôt se construit et se teste comme avant. Les paramètres de page, les coupures et l'export
  ne sont pas encore dans la fenêtre.
- **Fichiers du système en quatre langues** : l'entrée de menu (`.desktop`), la fiche AppStream
  et la page de manuel, en anglais (langue de repli), français, espagnol et allemand ; les
  pages de manuel traduites sont installées dans le dossier de leur langue. Des tests de
  `repocheck` vérifient que chaque langue a ses lignes et que les quatre pages de manuel portent
  la version.
- **Option `-ready-file`** : crée un fichier quand la fenêtre est affichée et le document
  dessiné (essais et documentation).
- **Systèmes pris en charge** : la famille Debian (Debian 13 et suivantes, Ubuntu 24.04 et
  suivantes, leurs dérivés), la famille Red Hat (RHEL 10 et ses clones, Fedora) et Windows 10
  et 11. Le critère est la version des bibliothèques (GTK 4.14, GLib 2.80, librsvg 2.52 au
  moins, imposées par gotk4 v0.3.1) ; Debian 12, Ubuntu 22.04 et RHEL 9 sont trop anciens.
  `build-deb.sh` et `build-rpm.sh` vérifient ces versions avant de compiler et le disent en
  français. Tableau dans le README et dans `ARCHITECTURE.md` (§4).
- **CI des distributions** : le job `distributions` construit le paquet dans un conteneur de
  Debian 13, Ubuntu 24.04, Ubuntu 26.04, Fedora, CentOS Stream 10, AlmaLinux 10 et Rocky Linux
  10, l'installe avec `apt` ou `dnf`, lance `-selftest` puis la fenêtre dans les quatre
  langues, et vérifie sur Debian 12, Ubuntu 22.04 et AlmaLinux 9 que la construction est
  refusée avec un message clair. Le résultat de chaque système est dans l'annotation de
  l'exécution. Le paquet Windows est essayé sur `windows-2022` et `windows-latest`.
- **CI** : un job `ui` (GTK4 sous Xvfb : tests de la fenêtre, captures dans les quatre langues
  en artefact), l'ouverture de la fenêtre du programme installé dans le job `linux-packages`, et
  dans le job `windows-package` l'ouverture de la fenêtre du paquet portable dans les quatre
  langues puis de celle du programme installé, avec captures d'écran.

### Modifié

- **Paquets** : les `.deb` et `.rpm` dépendent de GTK 4 (`libgtk-4-1`) ; leurs scripts
  vérifient la présence de GTK 4 à la construction (Ubuntu 24.04 ou plus récent, GTK 4.14).
  Le paquet Windows embarque GTK 4 et ce qu'il lui faut (schémas de réglages, thèmes d'icônes
  Adwaita et hicolor, traductions de GTK) et contient deux programmes : `svgprintlayout.exe`
  (la fenêtre, sans console) et `svgprintlayout-console.exe` (`-version`, `-selftest`). Le
  script de construction vérifie le sous-système de chacun. L'installeur NSIS parle les quatre
  langues.

## [0.1.0] - 2026-10-19

Première version. Le dépôt contient le squelette, les documents de référence, la bibliothèque
d'unités et de formats papier, le prototype du moteur de rendu (M0a), les scripts qui
construisent les paquets `.deb`, `.rpm` et Windows, et la CI. Le programme n'a pas encore
d'interface graphique : il n'offre que `-version` et `-selftest`. Le prototype de l'interface
GTK4 et le build de celle-ci sous Windows (M0b) sont la prochaine étape.

### Ajouté

- **Paquets** : `./build-deb.sh`, `./build-rpm.sh` et `.\build-windows.ps1` construisent à la
  racine du dépôt un `.deb`, un `.rpm`, une version portable (`.zip`) et un installeur NSIS de
  Windows, avec les outils natifs de chaque plate-forme (`dpkg-deb`, `rpmbuild`, MSYS2 et
  NSIS). Ils ont les options et la gestion de `VERSION` de ceux de Planche-Contact :
  `--major`, `--minor`, `--patch`, `--no-version-change`, `--clean`, `--help` (`-Major`… sous
  PowerShell), menu interactif sans option, confirmation avant de construire, `VERSION`
  écrit une fois le paquet construit. Une version `0.1.0-dev` est d'abord publiée (`--patch`
  et `--minor` donnent `0.1.0`). Les paquets Linux contiennent le programme, l'entrée de
  menu, les métadonnées AppStream, l'icône et la page de manuel, avec des dépendances
  déduites du binaire ; le paquet Windows embarque aussi les DLL de cairo et de librsvg,
  fontconfig, les chargeurs d'images et la licence de chaque bibliothèque. Les constructions
  sont reproductibles. Même organisation des dossiers que Planche-Contact : `debian/`,
  `metainfo/`, `icons/`, `windows/`. Voir le README, « Paquets », et `ARCHITECTURE.md`.
- **`-version` et `-selftest`** : le programme affiche sa version et celles de librsvg et de
  cairo, et vérifie son moteur de rendu sur la machine où il tourne (chargement, mesure,
  rendu, image PNG incorporée, police incorporée au PDF, export déterministe). Les paquets
  s'en servent comme test de fumée.
- **`engine.ConfigureRuntime`** : sous Windows, oriente fontconfig, gdk-pixbuf et pango vers
  les données livrées à côté du programme.
- **CI** : un job `linux-packages` (construction, lintian, AppStream, installation et
  désinstallation du `.deb`, essai du `.rpm`, constructions identiques, gestion de `VERSION`
  par les scripts) et un job `windows-package` (construction par `build-windows.ps1 -Patch`,
  essai de la version portable avec le seul PATH de Windows, installation et désinstallation
  silencieuses, `-Clean`).
- **`repocheck`** : l'en-tête de licence est aussi exigé dans les scripts PowerShell, les
  scripts NSIS et les fichiers d'empaquetage ; les trois scripts de construction doivent avoir
  les mêmes options, le même calcul de version, et n'écrire `VERSION` qu'après la
  construction ; la version de `VERSION` doit être dans le journal des modifications, dans les
  métadonnées AppStream et dans la page de manuel ; `ARCHITECTURE.md` doit garder ses sections
  numérotées de 1 à 15 sans trou ; les fichiers sont lus avec des fins de ligne Unix, comme ils
  le sont dans une copie de travail Windows.
- **`engine` (prototype M0a)** : chargement d'un SVG par librsvg (résolution imposée à 96 dpi),
  taille du document, boîte d'un élément dans le repère du document, rendu d'une fenêtre en
  image, et export PDF multipage : chaque page redessine le document sous un clip
  rectangulaire, à une échelle unique, avec dates fixes (deux exports sont identiques à
  l'octet), polices incorporées et texte sélectionnable. Écrit en cgo (`librsvg-2.0`,
  `cairo`) ; sans cgo, le paquet est vide et le reste du dépôt se compile.
- **`testsvg`** : générateur déterministe d'un SVG d'essai de 1 280 × 5 479 unités (fond gris,
  bandeau, blocs avec texte, rectangle transformé), équivalent public du Mini-CAD.
- **Tests du moteur** : taille exacte des pages du PDF, exports identiques, texte et polices,
  taille du fichier indépendante du nombre de pages, et test de couture en deux contrôles — la
  géométrie lue dans le PDF (clip et décalage de chaque page, exacts à 1/256 de point) et les
  pixels par carrés de 16 × 16 — qui échouent tous deux quand une page est décalée. Le vrai
  Mini-CAD, qui n'est pas dans le dépôt, passe les mêmes tests avec `SVGPL_MINICAD`.
- **`docs/M0.md`** : rapport du volet M0a. Verdict : le moteur librsvg + cairo est retenu ;
  la surface d'enregistrement cairo, qui vidait les pages 2 à 4, est abandonnée au profit
  d'un rendu direct par page ; une police absente est remplacée sans avertissement, ce qui
  rend la détection du §12 d'`ARCHITECTURE.md` indispensable.
- **Intégration continue** : un job `engine` sous Ubuntu (`librsvg2-dev`, `poppler-utils`,
  `fonts-ibm-plex`, cgo activé) pour `engine` et `testsvg`.
- **Squelette du dépôt** : module Go `github.com/gillesmagneville/svg-print-layout`, point
  d'entrée `svgprintlayout` (qui n'affiche encore que sa version avec `-version`), un paquet
  par rôle dans `internal/` (`model`, `units`, `diag`, `layout`, `project`, `svgdoc`,
  `engine`, `export`, `app`, `ui`).
- **`units`** : conversions millimètres, points et pixels CSS ; lecture des longueurs SVG
  (`mm`, `cm`, `in`, `pt`, `pc`, `px`, sans unité), avec une erreur explicite pour `%`, `em` et
  `ex` ; formats A0 à A5, Letter, Legal, Tabloid et personnalisé, portrait et paysage ;
  marges indépendantes et zone utile ; échelle en millimètres par unité du document. Les
  chiffres du test d'acceptation du Mini-CAD (200 mm pour 1 280 unités : 856,09375 mm de
  haut, 4 pages A4 au minimum) sont vérifiés par un test.
- **Documents de référence** : `PROJECT_CONTEXT.md` (vision, principes, périmètre),
  `ARCHITECTURE.md` (contrat technique, tiré du document de conception), `CONTRIBUTING.md`,
  licence GPL-3.0 (`LICENSE`).
- **`repocheck`** : tests sur le dépôt lui-même — en-tête SPDX dans chaque fichier, `VERSION`
  cohérente avec ce fichier, cgo limité à `ui` et `engine`, chaque paquet de `internal/`
  décrit dans `ARCHITECTURE.md`.
- **Intégration continue** (GitHub Actions) : `gofmt`, `go vet` et `go test` sous Ubuntu et
  Windows, avec `CGO_ENABLED=0`.

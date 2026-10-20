<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Changelog

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/),
et ce projet adhère au [Semantic Versioning](https://semver.org/lang/fr/).

## [Non publié]

### Ajouté

- **Cœur de la mise en page, en Go pur (jalon M1)**, essayé par des tests sans cgo, dont des
  tests de propriétés tirés au hasard (graine fixe) :
  - `internal/model` : les réglages d'un projet (papier, marges, échelle, positionnement, fond,
    coupures, surcharges par page), valeurs immuables copiées par chaque commande ;
  - `internal/layout` : la feuille et sa zone utile (formats A0 à A5, Letter, Legal, Tabloid,
    personnalisé en mm ; portrait ou paysage ; quatre marges indépendantes), l'échelle unique en
    trois modes (largeur, hauteur, manuel par largeur ou par pourcentage), le plan des pages
    (bandes qui couvrent le document sans trou, même largeur partout, positionnement horizontal et
    vertical indépendant de l'échelle, surcharges par page, états Conforme, Dépassement de marge,
    Dépassement de feuille, page vide), les coupures (ajout, déplacement, suppression, bornes d'un
    glisser, écart minimal de 0,5 mm, identifiants stables), les vides entre blocs, la détection
    des coupures qui traversent un bloc, l'aimantation aux vides, les quatre corrections proposées
    quand une page dépasse (jamais appliquées seules) et les coupures automatiques
    (programmation dynamique : moins de blocs traversés, puis moins de pages, puis pages
    équilibrées) ;
  - `internal/project` : le fichier de projet `.svgpl` (JSON lisible, écriture déterministe et
    atomique, version avec migrations, lecture stricte, chemin relatif au projet avec repli sur
    le chemin absolu, empreinte SHA-256 et avertissements quand le SVG a changé) ;
  - `internal/diag` : les diagnostics, par code et valeurs, avec leurs textes en quatre langues ;
    un test vérifie que chaque code a un texte dans chaque langue et que le code n'en produit pas
    d'autre.
- **Tests d'acceptation 2, 3, 4, 6, 7 et 14 au niveau logique** : zone utile de 200 × 277 mm en A4
  portrait, échelle 0,15625 mm par unité pour 200 mm de large, proportions conservées, ajout et
  déplacement d'une coupure sans changement d'échelle, enregistrement et réouverture d'un projet.

- **Document, analyse et fenêtre (jalon M2)** : la fenêtre ouvre un SVG, l'analyse, calcule ses
  pages avec le cœur de M1 et en montre le plan.
  - `internal/svgdoc` (Go pur) : lecture du SVG sans le dessiner, avec la position de chaque
    élément dans le texte source ; constats traduits en quatre langues (filtre, script,
    animation, police SVG, `foreignObject`, `flowRoot`, police web, image liée introuvable ou
    distante, `use` vers un autre fichier, images incorporées, taille inconnue, XML mal formé ;
    25 constats au plus par catégorie, le reste compté) ; polices demandées ; blocs de premier
    niveau, en descendant dans les calques d'Inkscape ; rectangle de fond, rectangle ou chemin
    de quatre segments qui couvre le cadre ; copie de travail (fond blanchi ou supprimé, un
    identifiant sur chaque bloc) faite en remplaçant quelques octets du texte, sans réécrire le
    reste, et toujours identique pour les mêmes entrées. Le fichier source n'est jamais écrit.
  - `internal/engine` : l'interface `Document` (en Go pur, remplaçable par un faux moteur dans
    les tests), le chargement par chemin pour que les images liées soient trouvées à côté du
    SVG, et la résolution des polices par fontconfig (`MatchFont`) : une police absente est
    signalée avec celle qui la remplace, au lieu de l'être en silence.
  - `internal/app` : la session (projet, plan, historique). Chaque réglage est une commande qui
    donne un nouvel état ; le plan est calculé avant d'accepter un changement, si bien qu'un
    changement refusé ne laisse aucune trace ; annuler et rétablir ; les frappes successives
    dans un même champ ne font qu'une entrée d'historique ; un glisser n'en fait qu'une ;
    l'indicateur de modification se déduit de l'encodage du projet.
  - `internal/viewport` (Go pur) : zoom autour d'un point, déplacement, bornes, fenêtre du
    dessin à demander au moteur, besoin de redessiner ; testé sans écran.
  - Fenêtre : panneau gauche (Fichier source avec le rapport d'analyse, Papier, Marges liables,
    Échelle en trois modes, Positionnement), liste des pages à droite (largeur et hauteur du
    contenu, échelle, début et fin dans le document, état, corrections proposées sur le bouton
    même), aperçu au centre (zoom, déplacement, limites et numéros de page, page teintée selon
    son état, rendu de la seule partie visible à l'échelle de l'écran, HiDPI compris), boutons
    Annuler et Rétablir. Les panneaux Fond et Export, les coupures à la souris et l'export
    restent annoncés comme à venir.
  - **Documents lourds** : au-delà de 3 000 blocs, les vides sont lus sur une seule image du
    dessin et non bloc par bloc, car mesurer un élément oblige librsvg à parcourir tout le
    dessin (100 000 éléments : plus de six minutes mesurés un à un, quatre secondes ainsi).
    Un constat le dit à l'utilisateur.
- **Coupures à la souris et coupures automatiques (jalon M3)** :
  - `internal/app` : les commandes de la session sur les coupures (ajouter, glisser, saisir une
    position, pousser d'un pas, supprimer, coupures automatiques). Chacune est une entrée
    d'historique (un glisser n'en fait qu'une, les pas successifs sur une même coupure non plus),
    aucune ne lit ni n'écrit l'échelle (tests d'acceptation 6 et 7), et le plan est calculé avant
    d'accepter un changement. Un glisser s'arrête contre la coupure voisine au lieu d'échouer ;
    une position saisie qui n'est pas permise est refusée avec son motif, jamais corrigée en
    silence. Pour chaque coupure : son rang, sa position, le nombre de blocs qu'elle traverse et
    leurs noms ; les vides entre blocs servent à l'aimantation.
  - Aperçu : double-clic pour ajouter une coupure, glisser une ligne (poignée, pastille en mm et
    en unités, aimantation aux vides, Alt pour la désactiver), flèches (1 unité, Maj 10, Ctrl
    0,1 mm), Suppr, Échap, clic droit (saisir une position précise ou supprimer), clic sur une
    page pour la sélectionner. Pendant un glisser, seul le calque des coupures est redessiné.
  - Panneau de droite : boutons « Ajouter une coupure » et « Coupures automatiques » (la
    proposition remplace les coupures, en une entrée d'historique qu'on peut annuler), liste des
    coupures avec leurs blocs traversés, sélection partagée entre l'aperçu et les listes.
  - Tests : commandes de `app` (dont ceux de l'échelle), scénarios de la fenêtre sous Xvfb qui
    pilotent les gestes (ajout, glisser, aimantation, clavier, menu, sélection) en quatre langues.
- **Fond, export PDF vérifié et aperçu des feuilles (jalon M4)** :
  - `internal/export` (Go pur) : les pages du plan données au moteur ; les contrôles avant
    l'export (coupures en ordre, bandes qui couvrent le document, largeur identique partout,
    contenu dans la feuille, dépassement de marge, fond de page demandé comparé au fond
    détecté) ; la relecture du PDF écrit, sans bibliothèque PDF, qui compare le nombre de pages,
    leur taille (0,02 mm), le rectangle de clip et le décalage du dessin de chaque page au plan,
    à 1/256 de point près ; l'écriture atomique (fichier temporaire synchronisé, relu, puis
    renommé : un export refusé, annulé ou raté ne laisse rien). Treize diagnostics `export.*`
    traduits en quatre langues.
  - `internal/engine` : une seule fonction dessine une page, pour le PDF comme pour l'aperçu
    des feuilles (`RenderPage`) ; progression page par page et annulation.
  - `internal/app` : `SetDocBackground`, `SetPageBackground` (une entrée d'historique chacun),
    `DocumentBackground`, `CheckExport`, et `Rebuild`, qui recharge la copie de travail avec le
    fond choisi.
  - Fenêtre : sections Fond (trois fonds distincts, choix grisés sans rectangle de fond
    détecté) et Export (résumé, état prêt, à confirmer ou bloqué, progression annulable), vue
    Feuilles à côté de la vue Document, bouton « Aperçu » sur chaque carte de page, Ctrl+E et
    bouton de la barre de titre. Le fond se change hors du fil de l'interface, l'ancienne image
    reste affichée jusqu'à la nouvelle ; annuler et rétablir suivent le projet. Une page
    transparente est peinte sur un damier. Une erreur de contrôle ouvre un compte rendu par
    page, un avertissement demande une confirmation.
  - `internal/acceptance` : les tests d'acceptation 8 à 13 sur la chaîne entière (Mini-CAD de
    1 280 × 5 479 unités à 200 mm, pages à 10 mm du haut, fond gris remplacé par du blanc, PDF
    rastérisé par poppler), le déterminisme de l'export et l'intégrité du SVG source.
  - Tests : scénario de la fenêtre sous Xvfb (fond, annulation, feuilles, export refusé puis
    écrit puis annulé, textes en quatre langues).
- **Dépendance de construction** : `libfontconfig-dev` (Debian et Ubuntu), `fontconfig-devel`
  (Fedora, RHEL), `mingw-w64-ucrt-x86_64-fontconfig` (MSYS2), ajoutés aux scripts, à la CI et à
  la documentation.

### Modifié

- **Nom du `.deb`** : `svgprintlayout_<version>_amd64.deb`, sans révision Debian (`-1`),
  comme les paquets de Planche-Contact et de LinuxDoctor ; la version du paquet (`Version:`
  du `control`) est celle de `VERSION`. La 0.2.0 s'appelait `svgprintlayout_0.2.0-1_amd64.deb`.
  Les versions suivantes (0.2.1, 0.3.0…) se classent après elle, mais un `.deb` 0.2.0
  reconstruit avec ce script se classe avant celui qui est déjà installé : `sudo dpkg -i`
  l'installe quand même, `apt install` demande `--allow-downgrades`. Le `.rpm` garde sa
  release : `svgprintlayout-<version>-1.x86_64.rpm`.

- **Unité du document** : c'est le pixel CSS de la taille déclarée du SVG, celle que lit et
  dessine le moteur (ARCHITECTURE §5). Mode B : la portion ajustée à la hauteur d'une feuille est
  un réglage, pour que déplacer une coupure ne change jamais l'échelle.

## [0.2.0] - 2026-10-20

Première version avec une interface graphique : une fenêtre GTK4 à trois panneaux, en
français, en anglais, en espagnol et en allemand, qui ouvre un SVG et l'affiche en entier.
Les paramètres de page, les coupures et l'export ne sont pas encore dans la fenêtre. Le
programme est pris en charge, et essayé par la CI, sur Debian 13, Ubuntu 24.04 et suivantes,
RHEL 10 et ses clones, Fedora, Windows 10 et 11 (voir « Systèmes pris en charge » dans le
README).

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

### Corrigé

- **Avertissements à la construction** : les trois avertissements de gcc sur `free`, que
  gotk4 déclare autrement que `<stdlib.h>`, ne s'affichent plus pendant la construction des
  paquets (les autres avertissements restent visibles) ; `dpkg-shlibdeps` n'affiche plus
  « les binaires à analyser devraient déjà être présents… » sur un système qui n'est pas en
  anglais (le filtre cherchait le message anglais). Les scripts préviennent aussi que la
  première compilation de GTK dure plusieurs minutes sans rien afficher.

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

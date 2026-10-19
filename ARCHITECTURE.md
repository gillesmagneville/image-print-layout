<!-- SPDX-License-Identifier: GPL-3.0-only -->

# SVG Print Layout — Architecture

Ce document est le contrat technique canonique de SVG Print Layout. Là où
`PROJECT_CONTEXT.md` décrit la vision et les principes, celui-ci définit le concret : pile
technique, paquets, interfaces, formules, format de projet, vérifications et plan de
livraison. Si les deux semblent se contredire, ce document l'emporte pour tout ce qui est
concret (interfaces, formules, formats, valeurs par défaut).

## 1. Objet et périmètre

Ce document dit comment réaliser SVG Print Layout v1.0 tel que défini par le cahier des charges. Il ne répète pas les exigences ; il dit comment les satisfaire et où se concentre le risque.

Hypothèses retenues pour la v1, à confirmer :

- **Plateformes** : Linux (Ubuntu) et Windows 10 et 11.
- **Coupures horizontales uniquement.** Une coupure verticale crée des colonnes de pages, donc un assemblage en mosaïque, hors périmètre (§14 du cahier) et en tension avec le test 12 (largeur identique sur toutes les pages). Le modèle de données garde un champ d'axe pour ne pas fermer la porte.
- **Mode B (ajustement à la hauteur)** : l'échelle est calculée pour que la portion sélectionnée, par défaut la première page, occupe exactement la hauteur utile. Si la largeur obtenue dépasse la largeur utile, la page passe en état dépassement ; rien n'est réduit en silence.
- **Marges par défaut** : 10 mm en haut et en bas, 5 mm à gauche et à droite. Elles donnent 200 mm de largeur utile sur A4, ce qui rend le test d'acceptation réalisable sans avertissement.
- **Un seul SVG par projet.** Le projet le référence par chemin relatif, avec repli sur le chemin absolu.
- **Hors v1** : tout le §14 du cahier (mosaïque, repères de raccord, pieds de page, rotation automatique, imports PDF et matriciels).

Ordre de grandeur du test d'acceptation : 200 mm de largeur pour un dessin de 1 280 × 5 479 unités donnent 0,15625 mm par unité, soit 856,1 mm de hauteur. Avec 277 mm utiles par page A4 portrait, il faut au moins 4 pages.

## 2. Décisions structurantes

Sept décisions portent la conception ; les quatre premières garantissent les exigences les plus strictes du cahier : aucun redimensionnement silencieux et un aperçu fidèle au PDF.

1. **Un seul plan de pages, deux surfaces.** Un module pur calcule le plan (échelle, bandes, positions). L'aperçu et le PDF le dessinent avec la même fonction, sur deux surfaces différentes, donc ils ne peuvent pas diverger.
2. **Le dessin n'est jamais découpé, seulement fenêtré.** Chaque page est le même dessin, translaté puis recadré par un clip rectangulaire aux lignes de coupure. Un trait ou un texte qui traverse une coupure est recadré exactement, sans calcul géométrique sur les chemins.
3. **Une échelle globale unique**, en mm par unité SVG, stockée à un seul endroit. Aucune page ne peut avoir une largeur différente, et ajouter ou déplacer une coupure ne la touche pas.
4. **Coupures en coordonnées du document** (unités SVG), pas en millimètres. Changer l'échelle ou le papier laisse chaque coupure au même endroit du dessin ; seuls les états des pages sont recalculés.
5. **Rendu SVG délégué à cairo et librsvg**, par une couche cgo mince derrière une interface `Engine`. On n'écrit pas d'interpréteur SVG en Go : on obtient la fidélité (texte, dégradés, masques) et un PDF vectoriel avec clip natif.
6. **Interface GTK4 via gotk4.** Elle partage les bibliothèques graphiques du moteur (cairo, pango) : un seul ensemble à embarquer sous Windows.
7. **Logique en Go pur**, testable sans cgo : modèle, unités, mise en page, analyse du SVG, projet. Les tests de calcul tournent partout, sans bibliothèque graphique.

La décision 5 est le principal pari technique. Le jalon M0 la valide sur le fichier réel avant tout développement d'interface ; le §3 décrit le repli si elle échoue.

## 3. Choix technologiques

**Retenu : Go avec GTK4 (gotk4) pour l'interface, cairo avec librsvg pour le rendu et l'export PDF.** Le moteur de rendu est le choix le plus risqué ; le jalon M0 le valide sur le fichier réel avant le reste.

### Moteur de rendu SVG et export PDF

| Option | Fidélité SVG | Découpe exacte par page | Aperçu identique au PDF | Verdict |
| --- | --- | --- | --- | --- |
| A. Interpréteur SVG écrit en Go, PDF écrit à la main | À construire : texte, dégradés, masques, CSS | Oui, clip écrit à la main | Oui, même arbre de dessin | Repli lointain, effort maximal |
| B. [tdewolff/canvas](https://pkg.go.dev/github.com/tdewolff/canvas/renderers/rasterizer) comme cible de dessin | Cible de dessin, pas un interpréteur SVG | Non : le moteur ne dessine que chemins, texte et images | Oui | Écarté : pas de clip, module sans version stable |
| C. cairo + librsvg par cgo | Élevée : de nombreuses fonctions de SVG 1.1 et de SVG 2 | Oui, clip cairo | Oui, même fonction de dessin sur deux surfaces | **Retenu** |
| D. [svg2pdf](https://pypi.org/project/svg2pdf-py/) (Rust) en processus séparé | Élevée, texte réel en option | Possible via un objet PDF réutilisable | Non : l'aperçu demande un second moteur | Repli si C échoue |

Pourquoi C :

- **Fidélité.** [librsvg](https://gnome.pages.gitlab.gnome.org/librsvg/Rsvg-2.0/overview.html) ne gère ni script, ni animation, ni polices SVG ; il s'appuie sur les polices du système via Cairo et Pango.
- **Toute surface cairo.** Le même code dessine sur une surface image pour l'aperçu et sur une surface PDF pour l'export, et un document chargé peut être rendu autant de fois que nécessaire.
- **Aucun fond par défaut.** librsvg ne peint pas de fond : le fond de page est rempli par l'application (§8).
- **Cadrage.** `rsvg_handle_render_document` met le document à l'échelle proportionnellement dans un viewport et le centre. On lui donne un viewport de même proportion que le dessin, pour ne dépendre d'aucun centrage implicite. Le moteur impose 96 dpi à librsvg, dont la résolution par défaut est de 90 : sans cela, les longueurs en `mm`, `in` ou `pt` seraient mal converties (M0, `docs/M0.md`).
- **Boîtes des éléments.** [librsvg mesure un élément par son identifiant](https://valadoc.org/librsvg-2.0/Rsvg.Handle.get_geometry_for_element.html), sans tenir compte des clips. Le moteur utilise `rsvg_handle_get_geometry_for_layer`, qui donne la boîte dans le repère du document (la variante « element » la donne relative à l'élément) ; sur le Mini-CAD, les 36 groupes, 121 rectangles et 459 textes sont mesurés, aucun hors du document. L'analyse ajoute des identifiants à la copie de travail pour mesurer chaque bloc (§7).

Coûts assumés :

- **cgo et une couche C mince** dans le paquet `engine` : librsvg n'est pas une dépendance de gotk4.
- **Distribution Windows.** L'installateur doit embarquer les binaires GTK et leurs dépendances d'exécution ([GTK](https://gtk.org/docs/installations/windows/)), et les licences GNU imposent de fournir les sources sur demande ; la vérification vaut aussi pour cairo et librsvg. Le packaging GTK4 sous Windows est déjà pratiqué sur Planche-Contact (installateur NSIS).
- **Déterminisme** garanti à versions de bibliothèques égales : les versions embarquées sous Windows sont figées (§11).
- **Polices** du système : une police absente est substituée, donc détectée et signalée (§12).

### Interface

| Option | Pour | Contre |
| --- | --- | --- |
| [GTK4 via gotk4](https://github.com/diamondburned/gotk4-libgirepository) | Même pile graphique que le moteur (cairo, pango) : un seul ensemble à embarquer. Paquets sous MPL-2.0, liaisons cairo sous MIT | Le projet signale des fuites mémoire et des plantages possibles dans certaines parties de l'API. Build Windows à valider en M0 |
| [Fyne](https://github.com/fyne-io/fyne) | Go idiomatique, licence BSD-3-Clause, aucune bibliothèque requise à l'exécution | Compilateur C requis aussi. Aucun lien avec cairo : l'aperçu passe par une copie d'image. Premier build Windows pouvant durer jusqu'à 10 minutes |

GTK4 est retenu. Comme `Engine` renvoie des images (§4), Fyne reste un repli possible pour l'interface seule.

### Repli si M0 échoue

Si la fidélité ou le packaging Windows ne passent pas, le moteur D prend le relais derrière la même interface `Engine`. Le calcul de pages, l'analyse du SVG, le projet et l'interface ne changent pas ; seuls l'aperçu et l'export changent de moteur.

Le reste s'appuie sur la bibliothèque standard (XML, JSON, SHA-256). La relecture du PDF produit, pour les vérifications du §11, n'exige pas de bibliothèque PDF : M0 montre que les flux de contenu de cairo sont du Flate, que `compress/zlib` décompresse, et que le clip et le décalage de chaque page s'y lisent directement (§11).

## 4. Architecture modulaire

Le code se répartit en sept modules sur quatre niveaux, posés sur trois paquets de fondation (`model`, `units`, `diag`). La règle : la logique de calcul ne dépend ni de l'interface ni de cgo.

```
ui        Interface GTK4 : fenêtre, panneaux, aperçu, gestes de souris         [cgo]
 └──▶ app       Commandes, annulation, état du projet ; orchestre les modules
       ├──▶ layout    Échelle, bandes, coupures                                [Go pur]
       ├──▶ project   Fichier JSON, versions, migration                        [Go pur]
       ├──▶ svgdoc    Copie de travail, analyse, tuiles ──────┐
       └──▶ export    Contrôles avant et après, orchestration ┤ par l'interface Engine
                                                              ▼
             engine   Moteur de rendu : cairo + librsvg, chargement, boîtes  [cgo]
                      des blocs, tuiles d'aperçu et PDF multipage

Fondation : model · units · diag
            types du projet, conversions d'unités, diagnostics partagés ; importés par tous
```

Les flèches vont de l'appelant vers l'appelé. (En M0, le paquet `engine` expose des types concrets, `Load`, `Doc`, `Render`, `ExportPDF` et `PageSpec` ; l'interface ci-dessous en sera extraite en M2, quand `svgdoc` et `export` en auront besoin. `ExportPDF` y prend une liste de `PageSpec`, et non un `layout.Plan` : `engine` ne dépend pas de `layout`, et c'est `export` qui traduit le plan.) Seuls `ui` et `engine` dépendent de cgo : `svgdoc` et `export` n'appellent `engine` que par l'interface `Engine`, qu'un faux moteur remplace dans les tests. Le point d'entrée `cmd/svgprintlayout` se contente de câbler ces paquets.

Le dépôt GitHub s'appelle `svg-print-layout`, le module Go `github.com/gillesmagneville/svg-print-layout` et le binaire `svgprintlayout`. La disposition des fichiers est donnée après l'interface `Engine`.

```go
type Engine interface {
    Load(svg []byte, baseDir string) (Doc, error)
    ResolveFonts(families []string) []FontResolution
}

type Doc interface {
    Size() (w, h float64)                          // unités utilisateur
    MeasureBlocks(ids []string) ([]Box, error)     // boîtes en unités
    RenderTile(req TileRequest) (*image.RGBA, error)
    ExportPDF(plan layout.Plan, out io.Writer) error
    Close()
}
```

### Disposition du dépôt

```
svg-print-layout/
├── go.mod
├── VERSION                    # version courante (SemVer), voir CONTRIBUTING.md
├── build-deb.sh               # construisent les paquets à la racine (voir « Paquets »),
├── build-rpm.sh               #   avec les options et la gestion de VERSION de
├── build-windows.ps1          #   Planche-Contact ; celui-ci relaie vers windows/
├── cmd/svgprintlayout/        # point d'entrée : câble les paquets ; -version, -selftest
├── internal/
│   ├── model/                 # types de données partagés, sans logique
│   ├── units/                 # conversions SVG / px / pt / mm, longueurs SVG,
│   │                          #   formats papier, marges, zone utile, échelle
│   ├── diag/                  # diagnostics destinés à l'utilisateur
│   ├── layout/                # plan de pages, états, coupures automatiques
│   ├── project/               # fichier projet .svgpl
│   ├── svgdoc/                # copie de travail, analyse du SVG, détection du fond
│   ├── engine/                # moteur de rendu derrière l'interface Engine (cgo)
│   ├── testsvg/               # générateur de SVG d'essai déterministe (Go pur)
│   ├── export/                # export PDF et vérifications
│   ├── app/                   # état de l'application et commandes
│   ├── ui/                    # interface GTK4 (cgo)
│   └── repocheck/             # tests sur le dépôt lui-même (en-têtes, versions, docs)
├── debian/                    # copyright, postinst, postrm, lintian-overrides du .deb
├── metainfo/                  # entrée de menu (.desktop) et métadonnées AppStream
├── icons/                     # icône de l'application (SVG et PNG)
├── windows/                   # script de construction Windows, installeur NSIS, icône .ico
├── docs/                      # rapports de jalon (M0.md), page de manuel (svgprintlayout.1)
└── testdata/                  # SVG de référence
```

Chaque paquet de `internal/` est décrit dans ce document ; un test échoue s'il en manque un.
Les paquets hors `ui` et `engine` n'utilisent pas cgo, et un test le vérifie.

### Paquets

Trois scripts, à la racine du dépôt, construisent les paquets à partir du code ; ils
reprennent les options et la gestion du fichier `VERSION` de `build-deb.sh` et de
`windows/build-windows.ps1` de Planche-Contact, et écrivent les paquets à la racine (non
versionnés : `.gitignore`).

| Paquet | Script | Outil de construction |
| --- | --- | --- |
| `.deb` (Debian, Ubuntu) | `build-deb.sh` | `dpkg-deb`, dépendances par `dpkg-shlibdeps` |
| `.rpm` (Fedora, RHEL) | `build-rpm.sh` | `rpmbuild`, dépendances déduites du binaire |
| Windows : `.zip` portable et installeur | `build-windows.ps1`, `windows/` | MSYS2 UCRT64 (Go, gcc, librsvg, cairo), NSIS |

- **Options et version.** `--major`, `--minor`, `--patch`, `--no-version-change`, `--clean`,
  `--help` (`-Major`… sous PowerShell) ; sans option, un menu propose les quatre choix et la
  version qui en résulterait, puis le script demande confirmation. Le fichier `VERSION` n'est
  écrit qu'une fois le paquet construit : un échec ne laisse jamais une version sans paquet.
  Une version d'avant-publication (`0.1.0-dev`) est d'abord publiée : `--minor` et `--patch`
  donnent `0.1.0`, `--major` donne `1.0.0` (règle de node-semver) ; sans suffixe, c'est
  l'incrément habituel. Les trois scripts n'ont aucun code en commun, comme chez
  Planche-Contact : un test de `repocheck` vérifie qu'ils ont les mêmes options et la même
  fonction de calcul (`next_version`, identique dans les deux scripts shell), que `VERSION`
  est écrit une seule fois après la construction, et la CI exécute les scénarios
  (`.github/scripts/test-version-handling.sh` pour Linux, `windows-package.ps1` pour Windows).
  Le script Windows a une option de plus, `-Oui`, qui répond « oui » à ses questions : la CI
  ne peut pas répondre à un `Read-Host`.
- **Outils natifs.** `dpkg-deb` et `rpmbuild` sont sur toute machine de la famille, sans
  Ruby ni dépôt de modules (fpm et nfpm ne sont pas utilisés, alors que Planche-Contact
  emploie fpm : ici le programme est un binaire, pas un environnement Python) ; ils
  empaquettent un dossier que les scripts préparent, sans rien compiler. Le fichier
  `control` du `.deb` et le `.spec` du `.rpm` sont écrits par les scripts, car leurs
  dépendances, leur taille et leur numéro de version se calculent à la construction.
- **Numéros.** Une version d'avant-publication se classe avant la finale : `0.1.0-dev` donne
  `0.1.0~dev-1` chez Debian et la release `0.1.dev` chez RPM ; `0.1.0` donne `0.1.0-1` et la
  release `1`. Windows garde `0.1.0-dev` (quatre nombres dans les propriétés du `.exe`).
- **Contenu des paquets Linux.** `/usr/bin/svgprintlayout`, l'entrée de menu et les
  métadonnées AppStream (identifiant `io.github.gillesmagneville.svgprintlayout`, nom de
  domaine inversé : le fichier `.desktop` et `<launchable>` doivent s'y accorder, et
  `desktop-file-validate` et `appstreamcli validate` le vérifient en CI), l'icône (SVG et PNG
  256), la page de manuel, README et journal des modifications, copyright (`.deb`) ou licence
  (`.rpm`).
- **Reproductibilité.** `-trimpath`, `-buildvcs=false`, un identifiant de construction vide,
  `SOURCE_DATE_EPOCH` (date du dernier commit), ordre, propriétaires et permissions fixés :
  deux constructions donnent le même fichier, à versions de Go, de librsvg et de cairo égales
  (la CI le vérifie pour le `.deb` et le `.rpm`).
- **Paquets liés à leur machine de construction.** Les dépendances d'un `.deb` ou d'un `.rpm`
  sont déduites du binaire : on construit sur la distribution visée (Ubuntu 24.04 et suivantes
  pour le `.deb` ; les noms en `t64` n'existent pas avant).
- **Dossier de travail.** Hors du dépôt, comme chez Planche-Contact : `~/deb-build/build`,
  `~/rpm-build/build` et `%LOCALAPPDATA%\svgprintlayout-build`. `--clean` les supprime, et
  supprime les paquets déjà construits à la racine.
- **Disposition Windows.** Le dossier livré est un préfixe (`bin/` pour le programme et ses
  DLL, `etc/fonts/`, `lib/gdk-pixbuf-2.0/`, `share/licenses/`) : le programme peut être
  installé n'importe où. Fontconfig, gdk-pixbuf et pango ne savent pas où est leur
  configuration dans ce cas ; `engine.ConfigureRuntime` la leur indique au démarrage (sous
  Windows, quand ce dossier existe). Pango utilise fontconfig, comme sous Linux : les polices
  d'un document sont résolues, et remplacées, de la même façon partout (§12). Le paquet
  n'embarque aucune police ; il embarque la licence de chaque bibliothèque. Un script
  d'installation NSIS ne supprime que les fichiers qu'il a installés.
- **Test de fumée.** `svgprintlayout -selftest` charge un SVG intégré (fond, bloc, texte
  accentué, image PNG incorporée), le mesure, le dessine et l'exporte en PDF, et vérifie
  qu'une police est incorporée et que deux exports sont identiques. Le script Windows le
  lance sur le dossier du paquet avec le seul PATH de Windows avant de produire le `.zip` ;
  la CI le lance sur chaque paquet installé.
- **Ce que les paquets ne font pas encore** : ils n'installent qu'un programme sans interface
  (`-version`, `-selftest`) ; l'interface GTK4 et ses dépendances viendront avec M0b et M2.
  Rien n'est signé, aucun dépôt de paquets n'existe, et l'architecture arm64 est reconnue
  par les scripts Linux sans avoir été essayée.

## 14. Plan de livraison

Six jalons, dont M0 est le point de décision : le calcul de pages (M1) ne dépend pas du moteur et peut avancer en parallèle, alors que M2 à M5 attendent le verdict de M0.

| Jalon | Contenu | Critère de sortie |
| --- | --- | --- |
| M0. Prototype de vérité | Chaîne minimale, d'abord sur le SVG synthétique puis sur le Mini-CAD réel : chargement par librsvg, rendu d'une fenêtre, PDF de 4 pages A4 avec clip. Build sous Linux et sous Windows. Mesures : taille du PDF, texte, résolution des effets rastérisés, déterminisme | 4 pages de 595,28 × 841,89 pt, `w` = 200 mm, test de couture réussi, deux exports de même SHA-256, taille du PDF non multipliée par le nombre de pages, build Windows qui produit le même plan. Sinon : bascule vers le moteur D |
| M1. Cœur en Go pur | Modèle, unités, mise en page (échelle, bandes, positionnement, états, corrections), format de projet, tests de propriétés | Tests verts sans cgo ; tests 2, 3, 4, 6, 7 et 14 passés au niveau logique |
| M2. Document et aperçu | Copie de travail, analyse et rapport, moteur (chargement, tuiles), fenêtre à trois panneaux, zoom, déplacement, limites de pages | Mini-CAD affiché en entier, zoom fluide, rapport d'analyse lisible |
| M3. Coupures | Interaction à la souris, liste des pages, aimantation, annulation, coupures automatiques, détection de traversée | Tests 5, 6 et 7 passés ; sur le Mini-CAD, la proposition automatique ne traverse aucun bloc quand un vide existe |
| M4. Fond et export | Détection et remplacement du fond, export PDF, vérifications, aperçu de page | Tests 8 à 13 passés |
| M5. Projet, paquets, recette | Enregistrement et réouverture dans l'interface, paquet .deb, installateur Windows, recette complète | Les 14 tests réussissent sur Ubuntu, sur Windows 10 et sur Windows 11 |

**État.** Le squelette du dépôt, le paquet `units`, le prototype du moteur (M0a, `docs/M0.md`) et les scripts qui construisent les paquets `.deb`, `.rpm` et Windows (pour un programme sans interface) existent. M0 n'est pas terminé : l'interface GTK4 et son build sous Windows (M0b) restent à faire. Le `CHANGELOG.md` tient le détail.

Après la v1, le §14 du cahier reste la liste des suites possibles. L'aperçu du PDF réel relu par un moteur PDF indépendant (le cahier demande « un aperçu du PDF final ») est le premier candidat : la v1 s'en tient à l'aperçu produit par la même fonction de dessin, ce qui est fidèle au PDF par construction.

## 15. Risques et questions ouvertes

Les cinq premiers risques se tranchent au jalon M0 ; aucun développement d'interface n'est engagé avant son verdict. M0a en a tranché un et avancé deux (voir sous le tableau).

| Risque | Effet | Parade | Tranché en |
| --- | --- | --- | --- |
| Packaging Windows de GTK4, librsvg et leurs dépendances | Installateur lourd, ou build impossible | Build sous MSYS2 UCRT64, essai sur une machine Windows propre, moteur D en repli | M0 |
| Fidélité de librsvg sur le Mini-CAD réel (texte, polices) | Rendu différent de l'original | Essai du vrai fichier, comparé à un rendu d'Inkscape | M0 |
| Taille du PDF : le dessin répété à chaque page | Fichier N fois plus gros | Rendu direct sur chaque page, sous clip : cairo n'écrit que ce qui touche la fenêtre (la surface d'enregistrement est écartée, §6) | M0a, tranché : ×1,08 |
| Résolution des éléments rastérisés (filtres) dans le PDF | Flou ou pixels visibles à l'impression | Mesure, avertissement dès l'analyse, étude du réglage de la résolution de repli | M0 |
| Déterminisme dépendant des versions de cairo et de librsvg | Octets différents d'une machine à l'autre | Versions figées dans les paquets Windows ; comparaison par rasters entre plateformes | M0 puis M5 |
| Maturité de gotk4 | Fuites mémoire, plantages | Interface mince, moteur indépendant de gotk4, scénarios sous Xvfb, repli Fyne | M2 |
| Documents très lourds | Chargement et tuiles lents, mémoire | Test à 100 000 éléments, cache LRU, chargement hors du fil de l'interface | M2 |
| Police absente sur la machine d'export | Texte substitué | Détection et avertissement avant l'export | M2 |

Après M0a (`docs/M0.md`) :

- **Taille du PDF** : tranché, ×1,08 sur le Mini-CAD.
- **Fidélité de librsvg** : avancé. Le rendu concorde avec celui de Chromium (écart moyen d'un niveau de gris) ; la comparaison avec Inkscape reste à faire.
- **Déterminisme** : avancé. Établi sur une machine, avec dates fixées ; il reste à constater entre versions et entre machines.
- **Éléments rastérisés** : non avancé, le Mini-CAD n'a aucun filtre.
- **Packaging Windows** : avancé pour le moteur seul. Le paquet (DLL de cairo et de librsvg, fontconfig, chargeurs d'images) se construit sous MSYS2 UCRT64 et passe `-selftest` avec le seul PATH de Windows en CI ; la comparaison du plan produit sous Windows et sous Linux, et GTK4, restent à faire (M0b).
- **Police absente** : le risque est plus grand qu'on ne le pensait, car la substitution est silencieuse et le PDF incorpore la police de remplacement. La détection devient obligatoire, pas seulement souhaitable.
- **Temps d'export** proportionnel au nombre de pages (environ 0,25 s par page sur le Mini-CAD), et rendu d'une fenêtre d'aperçu de 0,24 à 0,41 s : à traiter par un cache de tuiles en M2.

### Questions ouvertes

1. **Fichier réel.** Le Mini-CAD a été fourni et sert à M0a (non versionné). Un second SVG représentatif, avec filtres, images ou polices incorporées, serait utile pour les éléments rastérisés.
2. **Coupures verticales.** Sont-elles bien hors v1, comme le suggèrent le §14 du cahier et le test 12 ?
3. **Mode B.** La définition du §1 convient-elle : échelle qui fait tenir la portion choisie dans la hauteur utile ?
4. **Aperçu du PDF final (§9.2).** La v1 le fournit avec la même fonction de dessin que l'export. Faut-il en plus relire le fichier PDF avec un moteur PDF indépendant dès la v1 ?
5. **Texte dans le PDF.** Le moteur le donne sans effort : texte sélectionnable, polices incorporées en sous-ensembles. Reste à décider si c'est une garantie du produit, ou seulement l'apparence.
6. **Langue de l'interface.** Français seul en v1 ?
7. **Versions d'Ubuntu à prendre en charge.** La version de librsvg varie selon la version d'Ubuntu, ce qui touche la fidélité et le déterminisme.

## Sources

Pages consultées le 19 octobre 2026.

- [librsvg : présentation et rendu vers cairo](https://gnome.pages.gitlab.gnome.org/librsvg/Rsvg-2.0/overview.html)
- [librsvg : mesure de la géométrie d'un élément](https://valadoc.org/librsvg-2.0/Rsvg.Handle.get_geometry_for_element.html)
- [GTK : installation et distribution sous Windows](https://gtk.org/docs/installations/windows/)
- [gotk4 : liaisons GTK4 pour Go](https://github.com/diamondburned/gotk4-libgirepository)
- [Fyne : prérequis et licence](https://github.com/fyne-io/fyne)
- [tdewolff/canvas : moteur de rasterisation](https://pkg.go.dev/github.com/tdewolff/canvas/renderers/rasterizer)
- [svg2pdf-py : liaison de svg2pdf, PDF vectoriel](https://pypi.org/project/svg2pdf-py/)

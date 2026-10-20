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

- **Plateformes** : la famille Debian (Debian, Ubuntu et leurs dérivés), la famille Red Hat (RHEL, CentOS Stream et leurs dérivés, Fedora) et Windows 10 et 11. Le critère est la version des bibliothèques, pas le nom de la distribution : voir « Plateformes prises en charge » (§4).
- **Coupures horizontales uniquement.** Une coupure verticale crée des colonnes de pages, donc un assemblage en mosaïque, hors périmètre (§14 du cahier) et en tension avec le test 12 (largeur identique sur toutes les pages). Le modèle de données garde un champ d'axe pour ne pas fermer la porte.
- **Mode B (ajustement à la hauteur)** : l'échelle est calculée pour que la portion sélectionnée, par défaut la première page, occupe exactement la hauteur utile. Si la largeur obtenue dépasse la largeur utile, la page passe en état dépassement ; rien n'est réduit en silence.
- **Marges par défaut** : 10 mm en haut et en bas, 5 mm à gauche et à droite. Elles donnent 200 mm de largeur utile sur A4, ce qui rend le test d'acceptation réalisable sans avertissement.
- **Un seul SVG par projet.** Le projet le référence par chemin relatif, avec repli sur le chemin absolu.
- **Hors v1** : tout le §14 du cahier (mosaïque, repères de raccord, pieds de page, rotation automatique, imports PDF et matriciels).

Ordre de grandeur du test d'acceptation : 200 mm de largeur pour un dessin de 1 280 × 5 479 unités donnent 0,15625 mm par unité, soit 856,1 mm de hauteur. Avec 277 mm utiles par page A4 portrait, il faut au moins 4 pages.

## 2. Décisions structurantes

Huit décisions portent la conception ; les quatre premières garantissent les exigences les plus strictes du cahier : aucun redimensionnement silencieux et un aperçu fidèle au PDF.

1. **Un seul plan de pages, deux surfaces.** Un module pur calcule le plan (échelle, bandes, positions). L'aperçu et le PDF le dessinent avec la même fonction, sur deux surfaces différentes, donc ils ne peuvent pas diverger.
2. **Le dessin n'est jamais découpé, seulement fenêtré.** Chaque page est le même dessin, translaté puis recadré par un clip rectangulaire aux lignes de coupure. Un trait ou un texte qui traverse une coupure est recadré exactement, sans calcul géométrique sur les chemins.
3. **Une échelle globale unique**, en mm par unité SVG, stockée à un seul endroit. Aucune page ne peut avoir une largeur différente, et ajouter ou déplacer une coupure ne la touche pas.
4. **Coupures en coordonnées du document** (unités SVG), pas en millimètres. Changer l'échelle ou le papier laisse chaque coupure au même endroit du dessin ; seuls les états des pages sont recalculés.
5. **Rendu SVG délégué à cairo et librsvg**, par une couche cgo mince derrière une interface `Document`. On n'écrit pas d'interpréteur SVG en Go : on obtient la fidélité (texte, dégradés, masques) et un PDF vectoriel avec clip natif.
6. **Interface GTK4 via gotk4.** Elle partage les bibliothèques graphiques du moteur (cairo, pango) : un seul ensemble à embarquer sous Windows.
7. **Logique en Go pur**, testable sans cgo : modèle, unités, mise en page, analyse du SVG, projet. Les tests de calcul tournent partout, sans bibliothèque graphique.
8. **Quatre langues dès la première interface** : français, anglais, espagnol, allemand. Aucun texte destiné à l'utilisateur n'est écrit dans le code : il porte une clé, que le paquet `i18n` traduit (§4, « Langues »).

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

GTK4 est retenu. Comme `Document` renvoie des images (§4), Fyne reste un repli possible pour l'interface seule.

**Version de gotk4.** `github.com/diamondburned/gotk4/pkg` v0.3.1, qui se compile avec GLib 2.80 et GTK 4.14 (Ubuntu 24.04, RHEL 10) : la v0.4 demande GLib 2.84 ou plus, que ces systèmes n'ont pas. La version est celle de `go.mod`. Passer à une version plus récente est un changement à part, qui doit garder les paquets constructibles sur les plus anciens systèmes pris en charge (Ubuntu 24.04, RHEL 10 et ses clones). gotk4 compile lentement (plusieurs minutes la première fois, le cache de compilation ensuite) ; la CI garde ce cache.

### Repli si M0 échoue

Si la fidélité ou le packaging Windows ne passent pas, le moteur D prend le relais derrière la même interface `Document`. Le calcul de pages, l'analyse du SVG, le projet et l'interface ne changent pas ; seuls l'aperçu et l'export changent de moteur.

Le reste s'appuie sur la bibliothèque standard (XML, JSON, SHA-256). La relecture du PDF produit, pour les vérifications du §11, n'exige pas de bibliothèque PDF : M0 montre que les flux de contenu de cairo sont du Flate, que `compress/zlib` décompresse, et que le clip et le décalage de chaque page s'y lisent directement (§11).

## 4. Architecture modulaire

Le code se répartit en huit modules sur quatre niveaux, posés sur quatre paquets de fondation (`model`, `units`, `diag`, `i18n`). La règle : la logique de calcul ne dépend ni de l'interface ni de cgo.

```
ui        Interface GTK4 : fenêtre, panneaux, aperçu, gestes de souris         [cgo]
 ├──▶ viewport  Arithmétique de la vue : zoom, déplacement, fenêtre à dessiner [Go pur]
 └──▶ app       Session : projet, historique, plan ; ouverture d'un document   [Go pur]
       ├──▶ layout    Échelle, bandes, coupures                                [Go pur]
       ├──▶ project   Fichier JSON, versions, migration                        [Go pur]
       ├──▶ svgdoc    Analyse, copie de travail, blocs, fond, polices          [Go pur]
       └──▶ export    Contrôles avant et après, orchestration (M4) ──┐ par les types
                                                                     ▼ de engine
             engine   Moteur de rendu : cairo + librsvg + fontconfig, chargement, [cgo]
                      boîtes des blocs, fenêtres d'aperçu et PDF multipage

Fondation : model · units · diag · i18n
            types du projet, conversions d'unités, diagnostics partagés, traduction ; importés par tous
```

Les flèches vont de l'appelant vers l'appelé. Le paquet `engine` a deux parties : les types et l'interface `Document`, en Go pur (`engine.go`), et leur réalisation par librsvg, en cgo (`rsvg_cgo.go`, et un bouchon `engine_nocgo.go` qui refuse de charger). `app` et `svgdoc` ne connaissent le moteur que par les types de la première partie : le programme (`cmd/svgprintlayout`) et l'interface lui donnent le `Loader` réel, et un faux moteur le remplace dans les tests. `ExportPDF` prend une liste de `PageSpec`, et non un `layout.Plan` : `engine` ne dépend pas de `layout`, et c'est `export` qui traduit le plan (M4). Seuls `ui` et `engine` dépendent de cgo. Le point d'entrée `cmd/svgprintlayout` se contente de câbler ces paquets.

Le dépôt GitHub s'appelle `svg-print-layout`, le module Go `github.com/gillesmagneville/svg-print-layout` et le binaire `svgprintlayout`. La disposition des fichiers est donnée après l'interface `Document`.

```go
// Un document chargé : utilisé par un seul fil à la fois, et à fermer.
type Document interface {
    Size() (w, h float64)                                          // unités du document, à 96 dpi
    Measure(id string) (Box, error)                                // boîte d'un élément, en unités
    MeasureBlocks(ids []string) map[string]Box                     // un élément non mesurable est absent
    Render(win Box, pxPerUnit float64, bg color.Color) (*image.RGBA, error)
    ExportPDF(out io.Writer, pages []PageSpec, opt PDFOptions) error
    Close()
}

type Loader func(svg []byte, path string) (Document, error)    // path : dossier des images liées
type FontMatcher func(family string) string                    // famille installée pour une famille demandée
```

Il n'y a pas de tuiles (§9) : l'aperçu demande au moteur la seule fenêtre visible, à l'échelle de l'écran.

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
│   ├── diag/                  # diagnostics destinés à l'utilisateur (code et valeurs, sans texte)
│   ├── i18n/                  # traduction : français, anglais, espagnol, allemand ; nombres
│   ├── layout/                # plan de pages, états, coupures automatiques
│   ├── project/               # fichier projet .svgpl
│   ├── svgdoc/                # analyse du SVG, copie de travail, blocs, détection du fond, polices demandées
│   ├── engine/                # moteur de rendu derrière l'interface Document (cgo)
│   ├── testsvg/               # générateur de SVG d'essai déterministe (Go pur)
│   ├── export/                # export PDF et vérifications
│   ├── app/                   # session : projet, historique, plan ; ouverture d'un document
│   ├── viewport/              # arithmétique de la vue (zoom, déplacement, fenêtre à dessiner)
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

### Langues

L'interface est en français, en anglais, en espagnol et en allemand ; le paquet `i18n` (Go pur, rien à installer : les textes sont compilés dans le programme) les fournit.

- **Une clé par texte.** Le code demande `tr.T("panel.paper.title")` ; le texte vient du catalogue de la langue (`catalog_fr.go`, `catalog_en.go`, `catalog_es.go`, `catalog_de.go`). Le français est le catalogue de référence : une clé absente d'une langue est complétée par l'anglais, puis par le français, et une clé inconnue de tous s'affiche telle quelle. Les tests rendent ces cas impossibles en pratique : les quatre catalogues ont exactement les mêmes clés, aucun texte vide, les mêmes paramètres, et toute clé demandée dans le code existe.
- **Des paramètres nommés.** Un texte écrit `{count}`, `{path}` ou `{w}` ; une traduction place les valeurs dans l'ordre de sa langue. Les nombres décimaux s'écrivent avec la virgule en français, en espagnol et en allemand, avec le point en anglais ; `Number` fixe le nombre de décimales.
- **Les pluriels.** `tr.N("pages.count", n)` lit `pages.count.one` ou `pages.count.other` selon la langue (le français compte 0 au singulier, les trois autres non).
- **Lire un nombre.** `ParseNumber` accepte la virgule comme le point et ignore les espaces, y compris l'espace insécable des milliers : une personne qui tape « 200,5 » ou « 200.5 » dans un champ de largeur obtient la même valeur, quelle que soit la langue de l'interface.
- **Typographie française.** Le catalogue français s'écrit avec des espaces ordinaires ; au démarrage, l'espace insécable est mise avant « : », « ; », « ? », « ! » et dans les guillemets, pour qu'un libellé ne se coupe pas avant son deux-points.
- **Choix de la langue.** Le réglage est un code (`fr`, `en`, `es`, `de`) ou `auto`. `auto` suit le système : variables `LANGUAGE`, `LC_ALL`, `LC_MESSAGES`, `LANG` sous Linux, langue de l'utilisateur sous Windows (`GetUserDefaultLocaleName`). Une langue du système qui n'est pas l'une des quatre donne l'anglais. L'option `-lang` de la ligne de commande l'emporte ; le choix fait dans l'interface (menu de la fenêtre, effet immédiat) est enregistré dans les préférences de l'utilisateur, `settings.json` du dossier de configuration du système (`~/.config/svgprintlayout/` sous Linux, `%AppData%\svgprintlayout\` sous Windows), hors du fichier projet : le projet ne dépend pas de la langue de celui qui l'ouvre.
- **Messages d'erreur et diagnostics.** Un diagnostic (`diag`) porte un code et des valeurs, jamais un texte : l'interface le traduit par la clé `diag.<code>`. Ainsi une erreur de moteur ou d'export s'affiche dans la langue de l'utilisateur sans que les paquets de calcul connaissent les langues.
- **Composants de GTK.** Les boutons des boîtes de dialogue standard (« Annuler », « Ouvrir »…) sont traduits par GTK lui-même, à partir de la langue choisie : l'interface la lui donne par la variable `LANGUAGE` avant de l'initialiser (sous Windows aussi dans l'environnement de la bibliothèque C, où gettext la lit), et le paquet Windows embarque les catalogues de GTK pour le français, l'espagnol et l'allemand.
- **Fichiers lus par le système.** L'entrée de menu (`.desktop`), la fiche AppStream et la page de manuel existent dans les quatre langues, l'anglais étant le texte sans suffixe : le système le prend quand sa langue n'est pas l'une des quatre. Des tests de `repocheck` vérifient que chaque langue a ses lignes, et que les quatre pages de manuel portent la même version. Le programme d'installation Windows (NSIS) parle lui aussi les quatre langues, selon celle de Windows.
- **Ce qui reste en français.** Les sorties de diagnostic `-version` et `-selftest` (utiles au support, relues par les scripts de paquets) ; les documents du dépôt (`ARCHITECTURE.md`, `CONTRIBUTING.md`, journal des modifications) ; les identifiants et commentaires du code, en anglais.
- **Ajouter un texte ou une langue.** Un texte : ajouter la clé aux quatre catalogues (le test dit lesquels l'oublient). Une langue : un catalogue, son nom dans `Name` et `Languages`, sa règle de pluriel dans `isSingular`, son séparateur décimal dans `Number`.

### Plateformes prises en charge

Trois familles de systèmes. Ce qui décide n'est pas le nom de la distribution mais les versions
de ses bibliothèques : **GTK 4.14, GLib 2.80, Pango 1.52 et librsvg 2.52 au moins**. Ces seuils
viennent de gotk4 v0.3.1, qui lie presque toute l'API de GTK 4.14 et de GLib 2.80 : sur un
système plus ancien, la compilation s'arrête sur des centaines de symboles inconnus. Les
dépendances des paquets sont ensuite déduites du binaire (§« Paquets »).

| Famille | Pris en charge | Paquet | Essayé par la CI |
| --- | --- | --- | --- |
| Debian et dérivés | Debian 13 et suivantes ; Ubuntu 24.04 LTS et suivantes ; les dérivés fondés sur ces versions (Linux Mint 22, Pop!_OS 24.04, elementary OS 8…) | `.deb` | Debian 13, Ubuntu 24.04, Ubuntu 26.04 |
| Red Hat et dérivés | RHEL 10 et ses clones (CentOS Stream 10, AlmaLinux 10, Rocky Linux 10…) ; Fedora (versions en cours) | `.rpm` | CentOS Stream 10, AlmaLinux 10, Rocky Linux 10, Fedora |
| Windows | Windows 10 et 11, 64 bits | `.zip` portable et installeur | Windows Server 2022 et Windows Server 2025, qui servent de repères |

| Bibliothèque | Debian 13 | Ubuntu 24.04 | Ubuntu 26.04 | RHEL 10 et clones | Fedora 44 | Windows (MSYS2) |
| --- | --- | --- | --- | --- | --- | --- |
| GTK | 4.18.6 | 4.14.5 | 4.22.4 | 4.16.7 | 4.22.5 | 4.24.1 |
| GLib | 2.84.4 | 2.80.0 | 2.88.0 | 2.80.4 | 2.88.4 | 2.90.1 |
| librsvg | 2.60.0 | 2.58.0 | 2.61.3 | 2.57.1 | 2.62.4 | 2.63.2 |
| cairo | 1.18.4 | 1.18.0 | 1.18.4 | 1.18.2 | 1.18.6 | 1.18.6 |

**Non pris en charge : Debian 12, Ubuntu 22.04 (et Linux Mint 21), RHEL 9 et ses clones.** Ils ont
GTK 4.8, 4.6 et 4.12, GLib 2.74, 2.72 et 2.68, et RHEL 9 une librsvg 2.50. `build-deb.sh` et
`build-rpm.sh` le constatent avant de compiler et le disent en français ; la CI vérifie ce refus
sur ces trois systèmes (jobs « refus attendu »). Ils pourraient être servis plus tard par un
paquet qui embarque GTK (Flatpak ou AppImage), ce qui change la distribution du logiciel ;
cela reste hors de la v1 (§15, question 7).

Dans la CI, la fenêtre s'ouvre sous Xvfb sur la famille Debian et sur Fedora (avec captures
d'écran), et sur un compositeur Wayland sans écran (`weston`, lancé par `wlheadless-run`) sur
la famille RHEL 10, qui n'a plus de serveur X : il n'y a pas de capture dans ce cas.

Ce que la CI ne prouve pas :

- **Windows 10 et 11 eux-mêmes.** Elle n'a que des Windows Server : 2022 (le plus ancien proposé)
  sert de repère pour Windows 10, 2025 pour Windows 11. La recette sur de vraies machines fait
  partie de M5.
- **RHEL lui-même**, qui demande un abonnement : on essaie CentOS Stream 10 (son amont) et ses
  clones AlmaLinux et Rocky Linux.
- **Les dérivés d'Ubuntu et de Debian** (Mint, Pop!_OS, elementary OS…) : ils partagent les
  bibliothèques de leur base, donc les mêmes seuils.
- **aarch64.** Les scripts savent construire pour `arm64` et `aarch64`, mais la CI n'essaie que
  `amd64` et `x86_64`.

### Paquets

Trois scripts, à la racine du dépôt, construisent les paquets à partir du code ; ils
reprennent les options et la gestion du fichier `VERSION` de `build-deb.sh` et de
`windows/build-windows.ps1` de Planche-Contact, et écrivent les paquets à la racine (non
versionnés : `.gitignore`).

| Paquet | Script | Outil de construction |
| --- | --- | --- |
| `.deb` (Debian, Ubuntu et dérivés) | `build-deb.sh` | `dpkg-deb`, dépendances par `dpkg-shlibdeps` |
| `.rpm` (Fedora, RHEL et dérivés) | `build-rpm.sh` | `rpmbuild`, dépendances déduites du binaire |
| Windows : `.zip` portable et installeur | `build-windows.ps1`, `windows/` | MSYS2 UCRT64 (Go, gcc, GTK 4, librsvg, cairo), NSIS |

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
  `0.1.0~dev` chez Debian et la release `0.1.dev` chez RPM ; `0.1.0` donne `0.1.0` chez Debian
  (fichier `svgprintlayout_0.1.0_amd64.deb`, sans révision Debian, comme chez Planche-Contact et
  LinuxDoctor) et la release `1` chez RPM (`svgprintlayout-0.1.0-1.x86_64.rpm`). Windows garde `0.1.0-dev` (quatre nombres dans les propriétés du `.exe`).
- **Contenu des paquets Linux.** `/usr/bin/svgprintlayout`, l'entrée de menu et les
  métadonnées AppStream (identifiant `io.github.gillesmagneville.svgprintlayout`, nom de
  domaine inversé : le fichier `.desktop` et `<launchable>` doivent s'y accorder, et
  `desktop-file-validate` et `appstreamcli validate` le vérifient en CI), l'icône (SVG et PNG
  256), les pages de manuel (anglais dans `man1`, français, espagnol et allemand dans le
  dossier de leur langue), README et journal des modifications, copyright (`.deb`) ou licence
  (`.rpm`). Les dépendances comprennent GTK 4 (`libgtk-4-1`).
- **Reproductibilité.** `-trimpath`, `-buildvcs=false`, un identifiant de construction vide,
  `SOURCE_DATE_EPOCH` (date du dernier commit), ordre, propriétaires et permissions fixés :
  deux constructions donnent le même fichier, à versions de Go, de librsvg et de cairo égales
  (la CI le vérifie pour le `.deb` et le `.rpm`).
- **Paquets liés à leur machine de construction.** Les dépendances d'un `.deb` ou d'un `.rpm`
  sont déduites du binaire : on construit sur la distribution visée (§« Plateformes prises en
  charge »). Les noms en `t64` des paquets Debian n'existent pas avant Debian 13 et Ubuntu 24.04.
- **Dossier de travail.** Hors du dépôt, comme chez Planche-Contact : `~/deb-build/build`,
  `~/rpm-build/build` et `%LOCALAPPDATA%\svgprintlayout-build`. `--clean` les supprime, et
  supprime les paquets déjà construits à la racine.
- **Disposition Windows.** Le dossier livré est un préfixe (`bin/` pour les programmes et les
  DLL, `etc/fonts/`, `lib/gdk-pixbuf-2.0/`, `share/glib-2.0/schemas/`, `share/icons/`,
  `share/locale/`, `share/licenses/`) : le programme peut être installé n'importe où. GLib et
  GTK retrouvent leurs fichiers à partir de l'emplacement de leurs DLL, d'où ce préfixe ; sans
  les schémas de réglages, GTK s'arrête à l'ouverture d'un sélecteur de fichier, et sans le
  thème d'icônes les boutons de la fenêtre n'ont pas d'image. Il y a deux exécutables :
  `svgprintlayout.exe` (sous-système Windows : aucune console ne s'ouvre derrière la
  fenêtre, mais rien n'est écrit dans un terminal) et `svgprintlayout-console.exe`, le même
  programme en sous-système console, pour `-version`, `-selftest` et les messages d'erreur ;
  le script de construction vérifie le sous-système de chacun. Fontconfig, gdk-pixbuf et pango ne savent pas où est leur
  configuration dans ce cas ; `engine.ConfigureRuntime` la leur indique au démarrage (sous
  Windows, quand ce dossier existe). Pango utilise fontconfig, comme sous Linux : les polices
  d'un document sont résolues, et remplacées, de la même façon partout (§12). Le paquet
  n'embarque aucune police ; il embarque la licence de chaque bibliothèque. Un script
  d'installation NSIS ne supprime que les fichiers qu'il a installés.
- **Test de fumée.** `svgprintlayout -selftest` charge un SVG intégré (fond, bloc, texte
  accentué, image PNG incorporée), le mesure, le dessine et l'exporte en PDF, et vérifie
  qu'une police est incorporée et que deux exports sont identiques. Le script Windows le
  lance sur le dossier du paquet avec le seul PATH de Windows avant de produire le `.zip` ;
  la CI le lance sur chaque paquet installé. Elle ouvre aussi la fenêtre du programme
  installé (option `-ready-file`, qui crée un fichier quand la fenêtre est affichée et le
  document dessiné) sur un écran virtuel, en enregistre une capture, et le fait dans les
  quatre langues pour le paquet portable de Windows.
- **Ce que les paquets ne font pas encore** : le programme qu'ils installent ouvre un SVG,
  l'analyse, l'affiche avec ses pages et ses réglages (papier, marges, échelle, positionnement)
  et propose des coupures ; les coupures à la souris, le fond et l'export viennent avec M3 et M4.
  Rien n'est signé, aucun dépôt de paquets n'existe, et l'architecture arm64 est reconnue
  par les scripts Linux sans avoir été essayée.

## 5. Modèle géométrique

Tout repose sur trois repères et une échelle unique : les unités du document, les millimètres sur la feuille et les points du PDF.

| Repère | Unité | Sert à |
| --- | --- | --- |
| Document | Unité utilisateur SVG, origine en haut à gauche | Stocker les coupures et mesurer les éléments |
| Feuille | Millimètre, origine au coin haut gauche de la feuille | Marges, positions, affichage des dimensions |
| PDF et cairo | Point (1/72 pouce) | Dessiner ; 1 mm = 72 / 25,4 = 2,834646 points, valeur fixe |

### Dimensions du document

L'unité du document est le **pixel CSS de sa taille déclarée** (96 par pouce) : c'est ce que lit librsvg (`rsvg_handle_get_intrinsic_size_in_pixels`) et ce que le moteur dessine. Pour un SVG dont la `viewBox` et les `width` et `height` s'accordent (le cas du Mini-CAD, en pixels), c'est l'unité de la `viewBox`. Quand les deux diffèrent, le dessin est mis à la taille déclarée, et les positions des coupures sont comptées dans cette unité.

- `width` et `height` en unité absolue (mm, cm, pouce, pt, pc, px) : conversion standard.
- Sans unité, en pourcentage ou absents : une unité vaut un pixel CSS à 96 dpi, soit 25,4 / 96 = 0,26458 mm.
- Ni `viewBox` ni dimensions : le moteur refuse le document avec une erreur explicite (l'étendue du contenu n'est pas utilisée pour deviner une taille).

Le facteur de l'échelle native est donc toujours 25,4 / 96 mm par unité (`layout.NativeMMPerUnit`). Si le Mini-CAD est en pixels, ses 1 280 × 5 479 unités font 338,7 × 1 449,7 mm à 100 %.

### Échelle

L'échelle `s` est un nombre unique, en mm par unité, stocké à un seul endroit. Aucune page n'en garde une copie : elles la lisent.

```latex
s = \frac{L}{W_d} \qquad h_i = (y_i - y_{i-1})\, s \qquad w = W_d\, s = L
```

`W_d` est la largeur du document en unités, `L` la largeur choisie en mm, `y_i` la position de la coupure i en unités, `h_i` la hauteur de la page i en mm. `s` ne dépend que du papier, des marges, du mode et de la taille du document, jamais des coupures : `layout.ScaleOf` est le seul endroit où il est calculé.

| Mode | Calcul de `s` |
| --- | --- |
| A. Largeur | `L` = largeur utile du papier (largeur du papier moins marges gauche et droite) |
| B. Hauteur | `s` = hauteur utile divisée par la hauteur en unités de la **portion** choisie (`portionUnits`, la totalité du document si elle est nulle). La portion est un réglage, pas une page : déplacer une coupure ne change donc pas l'échelle. L'interface propose de la prendre égale à la hauteur d'une page existante. La largeur obtenue est ensuite comparée à la largeur utile |
| C. Manuel | `L` saisie en mm (mode `width`), ou facteur `f` en % (mode `factor`) : `s` = `f` × échelle native |

La largeur est affichée en mm et en cm. Le test d'acceptation donne `s` = 200 / 1 280 = 0,15625 mm par unité, soit 856,1 mm de hauteur totale.

### Feuille et zone utile

| Format | Portrait, largeur × hauteur (mm) |
| --- | --- |
| A0 | 841 × 1 189 |
| A1 | 594 × 841 |
| A2 | 420 × 594 |
| A3 | 297 × 420 |
| A4 | 210 × 297 |
| A5 | 148 × 210 |
| Letter | 215,9 × 279,4 |
| Legal | 215,9 × 355,6 |
| Tabloid | 279,4 × 431,8 |
| Personnalisé | saisi en mm |

Le paysage échange largeur et hauteur. La zone utile vaut le papier moins les quatre marges, réglées indépendamment.

### Précision numérique

Tous les calculs sont en `float64`, et l'arrondi n'a lieu qu'à l'affichage (0,1 mm). Les comparaisons de dimensions tolèrent 0,005 mm : une largeur de 200 mm dans une zone utile de 200 mm ne doit jamais être refusée à cause d'une erreur d'arrondi. Les coupures sont enregistrées telles quelles, sans arrondi.

## 6. Pipeline de rendu et d'export PDF

Le document est chargé une fois, le plan calculé une fois, puis une seule fonction dessine chaque page : sur une surface image pour l'aperçu, sur une surface PDF pour l'export.

```
SVG source              lu en lecture seule, jamais modifié
    │
    ▼
Copie de travail        fond remplacé ou retiré ici seulement
    │
    ▼
Document chargé         chargé une seule fois, dessin gardé en mémoire
    │
    ├── taille du dessin ──▶ Plan de pages ◀── Réglages
    │                        fonction pure :    papier, marges, échelle,
    │                        bandes, positions  alignements, coupures
    │                        en mm, états
    │                              │
    │ le dessin                    │ le plan
    ▼                              ▼
Dessin des pages : une page = un clip, un décalage, une échelle
    │
    ├──▶ Aperçu        même fonction, surface image
    │
    └──▶ Export PDF    même fonction, surface PDF multipage
              │
              ▼
         Vérifications : PDF relu (nombre de pages, dimensions, manifeste de clips)
```

Le plan reste la seule source des positions : l'aperçu comme l'export le lisent, et aucun des deux ne recalcule l'échelle.

### Fenêtre d'une page

Une page est le même dessin, décalé puis recadré. Les valeurs sont en mm ; elles passent en points à l'instant du dessin.

```latex
M_i = T(x_i,\, y_i) \cdot S(s) \cdot T(0,\, -y_{i-1}) \qquad \text{clip} = [x_i,\, x_i + w] \times [y_i,\, y_i + h_i]
```

Le clip est le rectangle de la bande, jamais celui de la zone utile : c'est lui qui matérialise la coupure. Un trait ou un texte qui la traverse est donc recadré exactement, et rien n'est recalculé sur les chemins.

### Partage du dessin entre les pages

Chaque page redessine le document directement sous son clip : cairo n'écrit dans le PDF que les opérations qui touchent la fenêtre, donc le fichier ne contient pas le dessin autant de fois qu'il a de pages. M0 l'a mesuré sur le Mini-CAD : 77 080 octets pour 4 pages, 71 695 pour le dessin entier sur une seule page. Le texte reste du texte, avec ses polices incorporées en sous-ensembles.

Le plan initial était de dessiner le document une fois dans une surface d'enregistrement cairo, puis de la peindre sur chaque page. Il est abandonné : avec cairo 1.18, le PDF n'écrit alors qu'un seul objet dont l'étendue est celle de son premier usage, les pages suivantes sortent vides, et le texte devient des contours (`docs/M0.md`). Le coût est un temps d'export proportionnel au nombre de pages (environ 0,25 s par page sur le Mini-CAD).

### Aperçu

En vue Document, le dessin est une image de la fenêtre visible (§9) et les limites de page sont des surimpressions. En vue Feuilles, chaque page est peinte avec sa propre matrice sur une feuille blanche. Coupures, poignées et numéros appartiennent à l'interface : ils ne sont jamais dans le PDF.

### Export

Pour chaque page : taille de page fixée, fond de page (§8), sauvegarde du contexte, clip, matrice, rendu du document, restauration, page suivante. L'exporteur lit `s` dans le plan et ne le modifie pas. Les positions de clip et de chemin sont écrites par cairo en virgule fixe 24.8, soit au 1/256 de point (0,0014 mm) : la position d'une coupure dans le PDF est exacte à cette précision, sous la tolérance de 0,005 mm du §5.

## 7. Moteur de mise en page

Le paquet `layout` est une fonction pure : mêmes paramètres, même plan. Il ne lit aucun fichier, ne touche à aucune interface et ne modifie jamais l'échelle.

**Entrées** : taille du document, échelle `s`, papier et orientation, marges, alignements, coupures `y_1 < … < y_k` en unités, surcharges de position par page. **Sortie** : un plan, c'est-à-dire pour chaque page ses bornes en unités (`y_{i-1}`, `y_i`), sa largeur et sa hauteur de contenu en mm, sa position sur la feuille, son rectangle de clip, son état et ses messages.

Les bandes sont `[0, y_1]`, `[y_1, y_2]`, … `[y_k, H_d]`. Elles couvrent le document sans trou ni chevauchement par construction, et un test le vérifie.

### Positionnement

| Réglage | Position sur la feuille |
| --- | --- |
| Horizontal : gauche | `x` = marge gauche |
| Horizontal : centré | `x` = marge gauche + (largeur utile − `w`) / 2 |
| Horizontal : droite | `x` = largeur du papier − marge droite − `w` |
| Horizontal : personnalisé | `x` = marge gauche + décalage saisi |
| Vertical : en haut | `y` = marge haute + décalage saisi |
| Vertical : centré | `y` = marge haute + (hauteur utile − `h_i`) / 2 |

Ces formules n'utilisent `s` qu'à travers `w` et `h_i`. Changer un alignement ne peut donc pas changer la largeur du dessin, et un test le garantit.

### États des pages

Le contenu d'une page occupe le rectangle `[x, x+w] × [y, y+h_i]`.

| État | Condition | Effet |
| --- | --- | --- |
| Conforme | Le rectangle est dans la zone utile (tolérance 0,005 mm) | Export autorisé |
| Dépassement de marge | Dans la feuille, hors zone utile | Avertissement, confirmation à l'export |
| Dépassement de feuille | Hors de la feuille | Export bloqué |
| Page vide | Aucun bloc mesuré dans la bande | Avertissement |

### Corrections proposées, jamais appliquées seules

Quand une page dépasse en hauteur, le plan fournit quatre propositions chiffrées, chacune derrière un bouton :

1. **Ajouter une coupure** à la position la plus favorable de la bande (voir plus bas).
2. **Déplacer la coupure voisine** à la hauteur maximale que la page accepte.
3. **Réduire l'échelle** à la valeur qui fait tenir la bande la plus haute, avec la largeur qui en résulte (par exemple « 78 %, largeur 156 mm »).
4. **Changer de format** : premier format, du plus petit au plus grand et dans les deux orientations, pour lequel toutes les pages tiennent (le plan est recalculé pour chaque essai, car en mode A l'échelle suit le papier). Rien n'est proposé si aucun ne convient.

Chaque proposition porte un indicateur « résout tout » : une fois appliquée, plus aucune page ne dépasse (une coupure ajoutée sur un document de 856 mm n'y suffit pas, la proposition suivante prend le relais). La réduction d'échelle arrondit la largeur au dixième de millimètre inférieur et s'applique en mode C, largeur.

Pour un dépassement en largeur, seules les propositions 3 et 4 existent. Aucune n'est appliquée sans un clic, ce qui garantit l'absence de réduction silencieuse.

### Coupures

Une coupure porte un identifiant, un axe (réservé, `y` seul en v1) et une position en unités. Invariants : `0 < y < H_d`, ordre strict, écart minimal de 0,5 mm physique avec les bords et les autres coupures. Ajouter, déplacer et supprimer sont des commandes pures qui renvoient un nouvel état ; une propriété testée établit que `s` est identique avant et après chacune d'elles. Le plan n'exige que la structure (ordre strict, à l'intérieur du document) : un changement d'échelle peut rapprocher deux coupures sous 0,5 mm sans rendre le projet illisible, seules les commandes appliquent l'écart. L'identifiant d'une coupure ne change pas quand elle bouge et n'est jamais réutilisé ; il n'est pas enregistré, les coupures sont renumérotées à chaque ouverture.

Pendant le glisser, la coupure s'aimante aux vides détectés dans un rayon de 8 pixels d'écran ; maintenir Alt désactive l'aimantation.

### Blocs et détection de traversée

Les **blocs** sont les groupes de plus haut niveau qui portent du contenu visible, en descendant dans les calques Inkscape. Le rectangle de fond détecté (§8) est exclu, sinon toute position le traverserait. Le moteur mesure chaque bloc ; `layout` ne manipule que ces boîtes.

Une coupure traverse un bloc si sa position est strictement à l'intérieur de la boîte du bloc (tolérance 0,05 unité). Elle reçoit alors une icône d'avertissement, et le panneau de droite nomme les blocs traversés.

**Documents à très nombreux blocs.** Mesurer un élément oblige librsvg à parcourir tout le dessin : mesurer chaque bloc coûte un temps proportionnel au carré de leur nombre (4 000 blocs : moins d'une seconde ; 100 000 : plus de six minutes, mesuré). Au-delà de 3 000 blocs (`app.MaxExactBlocks`), `app` ne les mesure plus un par un. Ce que `layout` lit des blocs est l'union de leurs intervalles verticaux, et une ligne d'image sans aucune encre est un vide : le document, sans son rectangle de fond, est donc dessiné une fois à 1 pixel par unité (au plus 16 millions de pixels), et chaque suite de lignes qui portent de l'encre devient un bloc. C'est la même information, à la résolution de l'image ; un vide plus mince qu'une ligne est perdu, ce qui est du côté sûr (la coupure n'y est pas proposée). Le constat `svg.coarse_blocks` le dit à l'utilisateur. Un document de 100 000 éléments s'ouvre en quatre secondes environ.

### Coupures automatiques

1. **Nombre de pages minimal** : `n_min` = plafond de (hauteur du document en mm / hauteur utile).
2. **Vides** : l'union des intervalles verticaux des blocs, complémentée, donne les zones où rien n'est traversé. Le coût d'une position est le nombre de blocs qu'elle traverse, nul dans un vide.
3. **Recherche** pour `n` de `n_min` à `n_min + 2` : programmation dynamique sur les positions candidates (centre de chaque vide, positions idéales `j × H_d / n` et, près de chacune d'elles, le point d'un vide le plus proche, à une unité du bord), avec comme contraintes que chaque bande tienne dans la hauteur utile et ne soit pas inférieure à 40 % de la bande idéale. Critères, dans l'ordre : moins de blocs traversés, puis moins de coupures à moins de 2 mm d'un bloc, puis moins de pages, puis bandes les plus équilibrées (somme des carrés des écarts relatifs à la bande idéale).
4. **Proximité** : le centre d'un vide maximise la distance aux blocs voisins ; une coupure à moins de 2 mm physiques d'un bloc, sans le traverser, compte comme « à proximité » et produit un avertissement.
5. **Résultat** : une proposition, appliquée par une seule commande annulable (`SetCuts`, qui abandonne les surcharges de pages devenues sans objet). Les positions idéales étant candidates et tenant toujours dans la hauteur utile dès que `n ≥ n_min`, une solution existe toujours ; quand une coupure ne peut éviter un bloc, la proposition le dit (« la coupure 2 traverse des blocs »). L'utilisateur déplace ensuite chaque coupure.

## 8. Gestion du fond

Trois fonds n'ont rien à voir entre eux : le fond d'un élément du SVG, le fond de la page PDF et la couleur de la zone de travail de l'interface. L'application ne mélange jamais les trois.

| Fond | Qui le produit | Réglage |
| --- | --- | --- |
| Document | Un élément du SVG, détecté par l'analyse | Conserver, remplacer par du blanc, supprimer |
| Page | L'application, avant de peindre le dessin | Blanc opaque par défaut, ou transparent |
| Zone de travail | L'interface, avec la couleur du thème | Aucun : jamais exportée |

librsvg ne peint aucun fond de lui-même. Le document n'a donc de fond que si un de ses éléments en fournit un.

**Fond de page.** En blanc opaque, chaque page PDF reçoit un rectangle blanc de la taille de la feuille, peint avant le dessin. En transparent, ce rectangle est omis. Un PDF n'a pas de fond propre : un visualiseur affiche le plus souvent du blanc, de sorte que « transparent » ne change le résultat que si le PDF est placé sur autre chose.

**Aperçu.** Il peint dans cet ordre la zone de travail (gris neutre), puis la feuille avec la couleur de page, puis le dessin. Un fond gris du SVG n'est donc jamais confondu avec la couleur de l'interface.

### Détection du rectangle de fond

Le candidat est un `rect`, ou un chemin à quatre segments orthogonaux, qui remplit les conditions suivantes :

- sa boîte, une fois appliquées les transformations de ses ancêtres, couvre au moins 99 % de la `viewBox` (tolérance de 0,5 unité par bord) ;
- son remplissage n'est pas `none` ;
- il est le premier peint, dans l'ordre du document, parmi les candidats.

Si rien ne correspond, le réglage affiche « Aucun fond détecté » et les choix de remplacement sont grisés.

### Opérations sur la copie de travail

| Choix | Opération |
| --- | --- |
| Conserver | Aucune |
| Remplacer par du blanc | Le remplissage de l'élément devient `#ffffff`, opacité 1 |
| Supprimer | L'élément est retiré de l'arbre |

Le choix est enregistré dans le projet comme une instruction : mode et identifiant de l'élément, ou à défaut son chemin d'indices (par exemple `/svg/g[1]/rect[1]`). Elle est rejouée à chaque chargement. Le fichier SVG source n'est jamais écrit.

L'aperçu est immédiat : la copie modifiée est rechargée dans le moteur, hors du fil de l'interface, puis redessinée. Après l'export, un pixel de la marge haute doit être blanc opaque si ce fond a été demandé (§11).

## 9. Interface graphique

Une seule fenêtre à trois zones, sans dialogue pour les opérations courantes : les réglages se font dans les panneaux et la souris agit directement sur l'aperçu. La barre de titre porte Ouvrir un SVG, Ouvrir et Enregistrer le projet, Exporter en PDF et le menu.

### Panneau gauche : paramètres

Des sections repliables, dans l'ordre du cahier : fichier source (chemin, taille native, nombre d'avertissements d'analyse), papier, marges, échelle, positionnement, fond, export. Les marges sont quatre champs indépendants avec un bouton pour les lier. Une valeur saisie s'applique après 150 ms sans frappe : il n'y a pas de bouton Appliquer.

### Zone centrale : aperçu interactif

Deux vues, choisies par onglet :

- **Document** (par défaut) : le dessin entier, avec les lignes de coupure, les limites et les numéros de page.
- **Feuilles** : les pages côte à côte telles qu'elles seront imprimées, avec marges et positionnement.

Un `GtkOverlay` empile, dans l'ordre : une zone de travail, le dessin (une `GtkPicture` posée par un `GtkFixed`), puis une zone qui peint les limites et numéros de page, les lignes de coupure et les poignées, et reçoit les gestes.

| Geste | Effet |
| --- | --- |
| Molette | Défilement vertical |
| Ctrl + molette | Zoom autour du curseur |
| Clic milieu ou Espace + glisser | Déplacement de la vue |
| Double-clic sur le dessin, ou outil « Ajouter une coupure » | Nouvelle coupure à cette position |
| Glisser une ligne (zone de saisie de 6 pixels) | Déplacement ; infobulle en unités et en mm ; aimantation aux vides |
| Suppr | Supprime la coupure sélectionnée |
| Flèches | Déplacent de 1 unité ; Maj : 10 ; Ctrl : 0,1 mm |
| Clic droit sur une ligne | Supprimer, ou saisir une position précise |

Pendant un glisser, seul le calque des coupures est redessiné : l'image du dessin reste en place et le plan est recalculé à chaque mouvement, ce qui est peu coûteux puisque `layout` est pur.

**Fenêtre visible, sans tuiles.** Un dessin vectoriel se redessine à n'importe quelle échelle : plutôt que de découper le document en tuiles de taille fixe, l'interface demande au moteur la seule partie visible, plus un quart de sa taille de chaque côté pour qu'un déplacement ne montre pas de blanc (`viewport.Window`), à l'échelle de l'écran, c'est-à-dire le zoom multiplié par le facteur d'échelle de la fenêtre (écrans HiDPI). L'image est limitée à 12 millions de pixels ; au-delà, la marge diminue puis disparaît. Elle est produite hors du fil de l'interface, avec un compteur de génération : un résultat qui n'est plus celui qu'on attend est abandonné, sauf s'il couvre encore ce qui est visible, ce qui évite un clignotement quand les demandes se suivent. Entre deux rendus, GTK met l'image à l'échelle : le dessin reste visible, un peu flou, jusqu'à l'arrivée de la nouvelle image. Un changement de zoom de moins de 2 % ne redessine rien. Toute l'arithmétique (zoom autour d'un point, déplacement, bornes, fenêtre, besoin de redessiner) est dans le paquet `viewport`, testé sans écran.

### Panneau droit : liste des pages

Une `GtkColumnView` avec : numéro, largeur × hauteur (mm), échelle, début et fin (en unités et en mm depuis le haut du document), état, bouton d'aperçu. Sélectionner une ligne la met en surbrillance dans l'aperçu, et inversement.

En tête : nombre de pages, largeur du contenu en mm et en cm, échelle. En pied : les corrections proposées (§7) quand une page dépasse.

Le bouton d'aperçu ouvre la page rendue telle qu'elle sera exportée, avec la même fonction de dessin sur une surface image. « Aperçu complet » fait de même pour toutes les pages avant l'export.

### Annulation et tâches longues

- **Annuler et rétablir** : chaque modification est une commande qui produit un nouvel état immuable du projet. Un glisser compte pour une seule entrée d'historique. Ctrl+Z et Ctrl+Maj+Z ; une astérisque signale les changements non enregistrés, avec confirmation à la fermeture.
- **Fil de l'interface** : le moteur est piloté par un acteur unique (une goroutine), si bien qu'un document chargé n'est jamais utilisé par deux fils à la fois. Les résultats reviennent au fil GTK par `glib.IdleAdd`.
- **Export** : barre de progression, annulable.
- **Erreurs** : chaque message dit ce qui a échoué, pourquoi et quoi faire, sans code technique brut.

## 10. Format de projet

Le projet est un fichier JSON lisible et versionné, d'extension `.svgpl`. Il enregistre des réglages, jamais le dessin : le SVG reste dans son fichier d'origine, et rien de dérivé (nombre de pages, états, dimensions de page) n'est stocké, tout est recalculé.

```json
{
  "format": "svgprintlayout-project",
  "version": 1,
  "source": {
    "path": "schemas/mini-cad.svg",
    "absolutePath": "/home/…/schemas/mini-cad.svg",
    "sha256": "…",
    "width": 1280,
    "height": 5479
  },
  "paper": { "format": "A4", "orientation": "portrait", "customMm": null },
  "marginsMm": { "top": 10, "bottom": 10, "left": 5, "right": 5 },
  "scale": { "mode": "width", "widthMm": 200, "percent": 0, "portionUnits": 0 },
  "placement": { "horizontal": "center", "vertical": "top", "offsetXMm": 0, "offsetYMm": 0 },
  "background": { "document": "whiten", "element": "rect4521", "page": "white" },
  "cuts": [
    { "axis": "y", "position": 1380.5 },
    { "axis": "y", "position": 2760 },
    { "axis": "y", "position": 4120.25 }
  ],
  "pageOverrides": [ { "page": 2, "vertical": "center" } ],
  "export": { "outputPath": "mini-cad.pdf" }
}
```

Règles de lecture et d'écriture :

- **Écriture déterministe.** UTF-8, clés dans un ordre fixe, indentation de 2 espaces, nombres en représentation décimale la plus courte, écriture atomique (fichier temporaire puis renommage). Un même projet donne les mêmes octets.
- **Réglages complets, lecture stricte.** Tous les réglages sont écrits, même ceux d'un mode non choisi (le pourcentage quand le mode est « largeur » : passer d'un mode à l'autre dans l'interface ne fait pas perdre la dernière valeur saisie). À la lecture, un champ inconnu ou une valeur invalide (format de papier inexistant, marge négative, coupures hors ordre ou hors du document, surcharge d'une page qui n'existe pas) refuse le fichier en nommant le champ ; l'orientation, l'alignement et le fond de page absents prennent leur valeur par défaut. Les identifiants de coupures ne sont pas écrits.
- **Versions.** `version` est un entier. Chaque évolution ajoute une fonction de migration `vN → vN+1`, testée sur des fichiers d'exemple. Un fichier d'une version plus récente est refusé avec un message clair, jamais ouvert partiellement.
- **Chemins.** Relatif au fichier projet, avec `/` comme séparateur sur toutes les plateformes, et repli sur le chemin absolu. Si le SVG est introuvable, un dialogue propose de le localiser et les réglages sont conservés.
- **Intégrité de la source.** Le SHA-256 et les dimensions sont enregistrés. Si le hash a changé à la réouverture, un avertissement non bloquant explique que les coupures sont en unités du document et peuvent ne plus tomber là où elles avaient été placées. Si les dimensions ont changé, les coupures devenues hors du document sont signalées puis retirées après confirmation.
- **Surcharges par page.** Ajouter une coupure copie la surcharge de la page coupée sur les deux pages obtenues. Supprimer une coupure garde celle de la page du dessus.
- **Source intacte.** Le SVG n'est jamais ouvert en écriture ; un test compare son SHA-256 avant et après chaque scénario.

## 11. Vérifications avant export et déterminisme

Sept contrôles portent sur le plan avant l'export, et deux sur le PDF relu après. Une erreur bloque ; un avertissement demande une confirmation. Ils couvrent un à un les points du §9.2 du cahier.

| # | Contrôle | Si échec |
| --- | --- | --- |
| 1 | Coupures en ordre strict, toutes dans `]0, H_d[`, écart minimal respecté | Erreur |
| 2 | La somme des hauteurs de bandes vaut `H_d` (tolérance 1e-9 unité) | Erreur |
| 3 | Toutes les pages ont la même largeur `w = W_d × s` | Erreur |
| 4 | Aucun contenu hors de la feuille (rien n'est rogné par le bord) | Erreur |
| 5 | Contenu dans la zone utile, sinon dépassement de marge | Avertissement |
| 6 | Fond demandé cohérent avec le fond détecté | Avertissement |
| 7 | Fonctions SVG non prises en charge, polices substituées (§12) | Avertissement listé |
| 8 | PDF relu : nombre de pages égal au nombre de bandes | Erreur |
| 9 | PDF relu : taille de chaque page égale au papier demandé, tolérance 0,02 mm | Erreur |

Le contrôle 9 compare des points : un A4 vaut 595,28 × 841,89 pt. La correspondance des coupures avec l'aperçu vient de la construction (même plan, §6) ; les contrôles 8 et 9 relisent le PDF écrit, sans bibliothèque PDF, et ce même relevé la prouve : le rectangle de clip et le décalage du dessin de chaque page se lisent dans le flux de contenu, et sont comparés au plan à 1/256 de point près (c'est la méthode des tests de `engine`, `docs/M0.md`). La vérification visuelle équivalente est faite par les tests (§13).

### Déterminisme

- **Pas d'horodatage ni d'identifiant aléatoire** dans le PDF : cairo écrit par défaut les dates de création et de modification, le moteur les fixe à `2000-01-01T00:00:00Z`. Le champ « producteur » contient la version de cairo.
- **Entrées stables** : les instructions de fond sont rejouées dans un ordre fixe, et `layout` est séquentiel, sans parallélisme qui changerait l'ordre des opérations flottantes.
- **Limite assumée** : l'égalité d'octets est garantie à versions de cairo et de librsvg, et à polices, identiques. Entre deux machines différentes, les tests comparent des rasters avec une tolérance plutôt que des octets.
- **Prouvé en M0, sur une machine** : trois exports du même document (dont un après rechargement, à une seconde d'écart) donnent des fichiers de même SHA-256, sur le SVG synthétique et sur le Mini-CAD.

## 12. Couverture SVG et signalement

librsvg rend la plupart des documents ; l'application ajoute sa propre analyse pour dire à l'utilisateur, avant l'export, ce qui sera rastérisé, remplacé ou ignoré. Elle ne tente jamais de contourner une limite du moteur : ce qu'il ne dessine pas est signalé, pas reconstitué.

L'analyse se fait en deux passes :

1. **Passe XML** (`svgdoc`) : un parcours unique de la copie de travail, guidé par un catalogue de règles (élément ou attribut, niveau, message).
2. **Passe moteur** : résolution des familles de polices demandées et vérification des images liées.

| Constat | Niveau | Message |
| --- | --- | --- |
| Élément `filter` ou attribut `filter` | Avertissement | Effet rastérisé : il peut apparaître en bitmap dans le PDF |
| `script`, `animate`, `set` | Avertissement | Ignoré : librsvg ne gère ni script ni animation |
| Polices SVG (`font`, `glyph`) | Avertissement | Non prises en charge : police du système utilisée à la place |
| Élément `foreignObject`, texte encadré `flowRoot`, `use` vers un autre fichier | Avertissement | Non dessiné : il manquera à l'aperçu et au PDF |
| Police web (`@font-face`) | Avertissement | Non chargée : une police installée la remplace |
| Famille de police absente du système | Avertissement | Police « X » absente, remplacée par « Y » |
| Image liée par chemin, fichier introuvable | Avertissement | Image liée introuvable, nom et chemin attendus |
| Image liée par URL distante | Avertissement | Non téléchargée : l'application n'accède jamais au réseau |
| Image matricielle incorporée | Information | Conservée en bitmap, à sa résolution d'origine |
| Ni dimension ni `viewBox` exploitable | Erreur | Taille du document indéterminée |

Chaque catégorie est signalée au plus 25 fois ; le reste est compté dans un seul constat (« N autres constats semblables »). Un `foreignObject`, un `flowRoot` et une police web sont des avertissements : librsvg ne les dessine pas ou ne charge pas la police, et rien n'est reconstitué. Un document dont le XML est mal formé (balise non fermée, fin de balise qui ne correspond pas) est refusé par le moteur ; l'analyse le dit avec la raison.

Le rapport s'affiche dans la section « Fichier source », avec un compteur par niveau ; chaque constat est un texte traduit (`diag.svg.<code>`). Chaque constat sur un élément porte sa référence (son identifiant, ou son chemin d'indices) : cliquer une entrée sélectionnera l'élément dans l'aperçu (à venir avec M3, qui mesure et sélectionne les éléments). Le rapport peut être copié en texte brut.

Le texte est dessiné avec les polices du système. M0 montre qu'il reste du texte dans le PDF, sélectionnable, avec des sous-ensembles TrueType incorporés. Il montre aussi qu'une police absente est remplacée **sans rien dire** (IBM Plex remplacée par DejaVu, et le PDF incorpore la police de remplacement) : le signalement ci-dessus est donc la seule protection, et il exige une résolution des polices par fontconfig dans `engine`. Elle est faite : `MatchFont` demande à fontconfig la famille que le système donnerait, et `svgdoc.CheckFonts` signale toute liste `font-family` dont la première famille n'est pas installée, avec la famille qui sera réellement dessinée (la suivante de la liste qui est installée, sinon la famille générique ou ce que donne fontconfig) ; une liste qui commence par une famille générique (`sans-serif`, `monospace`) n'est jamais signalée, le document n'ayant rien demandé de précis.

## 13. Stratégie de tests

Les 14 tests d'acceptation sont tous automatisés sauf le geste de souris, vérifié à la main sur chaque plateforme ; les commandes qu'il déclenche, elles, sont testées automatiquement.

| # | Test d'acceptation | Vérification | Type |
| --- | --- | --- | --- |
| 1 | Importer le Mini-CAD | Dimensions lues : 1 280 × 5 479, aucune erreur bloquante | Auto |
| 2 | A4 portrait | Zone utile 200 × 277 mm avec les marges par défaut | Auto |
| 3 | Largeur de 200 mm | `s` = 0,15625 et `w` = 200 mm, tolérance 0,005 mm | Auto |
| 4 | Proportions conservées | Une seule échelle dans le modèle : aucune fonction n'accepte deux facteurs | Auto |
| 5 | Coupures à la souris | Scénario sous Xvfb pour GTK ; commandes testées en Go pur | Auto, geste à la main |
| 6 | Ajouter une coupure sans changer la largeur | Propriété : `w` avant égal `w` après, pour tout ajout | Auto |
| 7 | Déplacer une coupure sans changer l'échelle | Propriété : `s` avant égal `s` après, pour tout déplacement | Auto |
| 8 | Pages à 10 mm du haut | `y` = 10 mm sur chaque page ; premier pixel non blanc du raster du PDF à 10 mm, tolérance 0,2 mm | Auto |
| 9 | Fond gris remplacé par du blanc | Fond détecté ; pixels de l'ancienne zone grise blancs dans le raster d'export | Auto |
| 10 | Export d'un PDF multipage | Nombre de pages égal au nombre de bandes | Auto |
| 11 | Dimensions des pages du PDF | Taille des pages relue : 595,28 × 841,89 pt, tolérance 0,02 mm | Auto |
| 12 | Largeur identique sur toutes les pages | Plan, puis mesure raster d'un élément pleine largeur sur chaque page | Auto |
| 13 | Coupures conformes à l'aperçu | Test de couture (ci-dessous) : clip et décalage lus dans le PDF égaux au plan, et pixels par carrés | Auto |
| 14 | Réouverture du projet | Enregistrer, recharger, égalité profonde des réglages ; JSON de référence | Auto |

### Test de couture

Deux contrôles complémentaires, car aucun n'est à la fois exact et robuste (mesures et raisons dans `docs/M0.md`) :

- **La géométrie écrite dans le PDF.** On lit dans le flux de contenu de chaque page le rectangle de clip et l'endroit où le dessin est peint, et on les compare au plan, à 1/256 de point près. C'est le contrôle exact, valable pour toute position de coupure ; il établit aussi l'égalité de largeur.
- **Les pixels.** On rastérise le PDF produit, on découpe chaque page à sa fenêtre de contenu, et on la compare au rendu du document entier à la même échelle, par carrés de 16 × 16 pixels dont on compare la moyenne de gris (24 niveaux tolérés ; 14 mesurés au plus). Ce contrôle voit un contenu perdu, répété ou déplacé de quelques millimètres. Il ne compare pas pixel par pixel : un rastériseur place les glyphes sur une grille d'un quart de pixel et cale les traits fins sur la grille de pixels, si bien qu'un écart d'un centième de pixel à l'origine d'une page déplace un trait d'un pixel entier.

Un test vérifie que chacun des deux contrôles échoue quand une page est décalée. Ensemble, ils répondent au critère final du cahier : un PDF fidèle à l'aperçu, sans redimensionnement silencieux.

### Organisation

- **Trois niveaux.** Go pur (modèle, unités, mise en page, analyse, projet) avec `go test` partout sans cgo ; intégration du moteur (chargement, fenêtres d'aperçu, export) avec librsvg installé ; interface sous Xvfb sur Linux, plus une liste de contrôle manuelle sur Windows.
- **Tests de propriétés** sur `layout` : papiers, marges, échelles et coupures tirés au hasard, avec pour invariants la couverture exacte, `w` constant, `s` inchangé par les commandes et des états cohérents.
- **Jeu d'essai.** Le Mini-CAD réel n'est pas dans le dépôt : il décrit une infrastructure réelle. Les tests le prennent quand `SVGPL_MINICAD` donne son chemin. Le générateur déterministe `internal/testsvg` produit un équivalent public de 1 280 × 5 479 unités (fond gris, blocs avec texte, bandeau pleine largeur, rectangle transformé) ; avec `Filler`, il sert aussi de test de charge à 100 000 éléments (`internal/app/load_cgo_test.go`, ouverture en 4 s sur la machine de développement).
- **Intégration continue** (tous en place) : un job `engine` sur Ubuntu avec `librsvg2-dev`, `poppler-utils` et `fonts-ibm-plex` ; un job `ui` (GTK4 sous Xvfb : tests de la fenêtre, puis captures dans les quatre langues, déposées en artefact) ; un job `linux-packages` (construction du `.deb` et du `.rpm`, lintian, AppStream, installation, `-selftest`, fenêtre du programme installé, constructions identiques) un job `distributions` (le paquet de la famille est construit dans un conteneur de chaque système pris en charge, installé par `apt` ou `dnf`, puis le programme installé lance `-selftest` et la fenêtre dans les quatre langues ; sur Debian 12, Ubuntu 22.04 et AlmaLinux 9, le test vérifie que la construction est refusée avec un message clair) et un job `windows-package`, sur `windows-2022` et `windows-latest`, sous MSYS2 UCRT64 (construction par `build-windows.ps1`, `-selftest` puis fenêtre dans les quatre langues de la version portable, programme installé, désinstallation). Les autres paquets se construisent et se testent sans cgo (`CGO_ENABLED=0`), où `ui` est un bouchon.
- **Références.** Les rasters de référence sont stockés par plateforme, avec tolérance, et jamais comparés d'une plateforme à l'autre.

## 14. Plan de livraison

Six jalons, dont M0 est le point de décision : le calcul de pages (M1) ne dépend pas du moteur et peut avancer en parallèle, alors que M2 à M5 attendent le verdict de M0.

| Jalon | Contenu | Critère de sortie |
| --- | --- | --- |
| M0. Prototype de vérité | Chaîne minimale, d'abord sur le SVG synthétique puis sur le Mini-CAD réel : chargement par librsvg, rendu d'une fenêtre, PDF de 4 pages A4 avec clip. Build sous Linux et sous Windows. Mesures : taille du PDF, texte, résolution des effets rastérisés, déterminisme | 4 pages de 595,28 × 841,89 pt, `w` = 200 mm, test de couture réussi, deux exports de même SHA-256, taille du PDF non multipliée par le nombre de pages, build Windows qui produit le même plan. Sinon : bascule vers le moteur D |
| M1. Cœur en Go pur | Modèle, unités, mise en page (échelle, bandes, positionnement, états, corrections), format de projet, tests de propriétés | Tests verts sans cgo ; tests 2, 3, 4, 6, 7 et 14 passés au niveau logique |
| M2. Document et aperçu | Copie de travail, analyse et rapport, moteur (chargement, fenêtres d'aperçu), fenêtre à trois panneaux, zoom, déplacement, limites de pages | Mini-CAD affiché en entier, zoom fluide, rapport d'analyse lisible |
| M3. Coupures | Interaction à la souris, liste des pages, aimantation, annulation, coupures automatiques, détection de traversée | Tests 5, 6 et 7 passés ; sur le Mini-CAD, la proposition automatique ne traverse aucun bloc quand un vide existe |
| M4. Fond et export | Détection et remplacement du fond, export PDF, vérifications, aperçu de page | Tests 8 à 13 passés |
| M5. Projet, paquets, recette | Enregistrement et réouverture dans l'interface, paquet .deb, installateur Windows, recette complète | Les 14 tests réussissent sur chaque système pris en charge : Debian 13 et Ubuntu 24.04 (famille Debian), RHEL 10 ou un clone (famille Red Hat), Windows 10 et Windows 11 |

**État.** Le squelette du dépôt, le paquet `units`, le prototype du moteur (M0a, `docs/M0.md`), les scripts qui construisent les paquets `.deb`, `.rpm` et Windows, le paquet de traduction `i18n` et une première fenêtre GTK4 (trois panneaux, quatre langues, ouverture d'un SVG affiché en entier ; M0b) existent, avec leur CI, qui construit et essaie le paquet sur sept distributions Linux (familles Debian et Red Hat) et sur Windows. M0 n'est pas terminé : la comparaison du plan produit sous Windows et sous Linux reste à faire. **M1 est fait** : `model`, `diag`, `layout` (échelle, feuille, plan, états, corrections, coupures, aimantation, coupures automatiques) et `project` (format `.svgpl`) sont écrits et testés sans cgo, avec des tests de propriétés ; les panneaux de la fenêtre ne les utilisent pas encore. **M2 est fait** : `svgdoc` (analyse, constats, polices, blocs, fond, copie de travail), l'interface `Document` du moteur (chargement par chemin pour les images liées, polices par fontconfig), `app` (session, historique, plan), `viewport` et la fenêtre (panneaux Fichier source, Papier, Marges, Échelle, Positionnement ; liste des pages avec les corrections proposées ; aperçu avec zoom, déplacement, limites et numéros de page, annuler et rétablir) existent et sont essayés sous Xvfb. Les panneaux Fond et Export, les coupures à la souris, la détection du fond et l'export restent à faire (M3 et M4). Le `CHANGELOG.md` tient le détail.

Après la v1, le §14 du cahier reste la liste des suites possibles. L'aperçu du PDF réel relu par un moteur PDF indépendant (le cahier demande « un aperçu du PDF final ») est le premier candidat : la v1 s'en tient à l'aperçu produit par la même fonction de dessin, ce qui est fidèle au PDF par construction.

## 15. Risques et questions ouvertes

Les cinq premiers risques se tranchent au jalon M0. M0a en a tranché un et avancé deux (voir sous le tableau) ; la fenêtre de M0b ne va pas plus loin que ce qu'elle montre, et le moteur D reste le repli tant que le verdict n'est pas rendu.

| Risque | Effet | Parade | Tranché en |
| --- | --- | --- | --- |
| Packaging Windows de GTK4, librsvg et leurs dépendances | Installateur lourd, ou build impossible | Build sous MSYS2 UCRT64, essai sur une machine Windows propre, moteur D en repli | M0 |
| Fidélité de librsvg sur le Mini-CAD réel (texte, polices) | Rendu différent de l'original | Essai du vrai fichier, comparé à un rendu d'Inkscape | M0 |
| Taille du PDF : le dessin répété à chaque page | Fichier N fois plus gros | Rendu direct sur chaque page, sous clip : cairo n'écrit que ce qui touche la fenêtre (la surface d'enregistrement est écartée, §6) | M0a, tranché : ×1,08 |
| Résolution des éléments rastérisés (filtres) dans le PDF | Flou ou pixels visibles à l'impression | Mesure, avertissement dès l'analyse, étude du réglage de la résolution de repli | M0 |
| Déterminisme dépendant des versions de cairo et de librsvg | Octets différents d'une machine à l'autre | Versions figées dans les paquets Windows ; comparaison par rasters entre plateformes | M0 puis M5 |
| Maturité de gotk4 | Fuites mémoire, plantages | Interface mince, moteur indépendant de gotk4, scénarios sous Xvfb, repli Fyne | M2 |
| Documents très lourds | Chargement et rendu lents, mémoire | Test à 100 000 éléments, rendu de la seule fenêtre visible, chargement hors du fil de l'interface | M2 |
| Police absente sur la machine d'export | Texte substitué | Détection et avertissement avant l'export | M2 |

Après M0a (`docs/M0.md`) :

- **Taille du PDF** : tranché, ×1,08 sur le Mini-CAD.
- **Fidélité de librsvg** : avancé. Le rendu concorde avec celui de Chromium (écart moyen d'un niveau de gris) ; la comparaison avec Inkscape reste à faire.
- **Déterminisme** : avancé. Établi sur une machine, avec dates fixées ; il reste à constater entre versions et entre machines.
- **Éléments rastérisés** : non avancé, le Mini-CAD n'a aucun filtre.
- **Packaging Windows** : avancé. Le paquet (DLL de GTK 4, de cairo et de librsvg, fontconfig, chargeurs d'images, schémas, icônes et traductions de GTK) se construit sous MSYS2 UCRT64, passe `-selftest` avec le seul PATH de Windows et ouvre sa fenêtre en CI ; la comparaison du plan produit sous Windows et sous Linux reste à faire.
- **Police absente** : le risque est plus grand qu'on ne le pensait, car la substitution est silencieuse et le PDF incorpore la police de remplacement. La détection devient obligatoire, pas seulement souhaitable.
- **Temps d'export** proportionnel au nombre de pages (environ 0,25 s par page sur le Mini-CAD), et rendu d'une fenêtre d'aperçu de 0,24 à 0,41 s : traité en M2 par le rendu de la seule fenêtre visible (une marge d'un quart, plafonnée à 12 millions de pixels) et par le maintien de l'image précédente, mise à l'échelle, pendant le calcul de la suivante.

### Questions ouvertes

1. **Fichier réel.** Le Mini-CAD a été fourni et sert à M0a (non versionné). Un second SVG représentatif, avec filtres, images ou polices incorporées, serait utile pour les éléments rastérisés.
2. **Coupures verticales.** Sont-elles bien hors v1, comme le suggèrent le §14 du cahier et le test 12 ?
3. **Mode B.** La définition du §1 convient-elle : échelle qui fait tenir la portion choisie dans la hauteur utile ?
4. **Aperçu du PDF final (§9.2).** La v1 le fournit avec la même fonction de dessin que l'export. Faut-il en plus relire le fichier PDF avec un moteur PDF indépendant dès la v1 ?
5. **Texte dans le PDF.** Le moteur le donne sans effort : texte sélectionnable, polices incorporées en sous-ensembles. Reste à décider si c'est une garantie du produit, ou seulement l'apparence.
6. **Langue de l'interface.** *Tranché : français, anglais, espagnol et allemand dès la première version de l'interface (§4, « Langues »).*
7. **Versions de Linux à prendre en charge.** *Tranché pour la v1 : les systèmes qui ont GTK 4.14, GLib 2.80 et librsvg 2.52 au moins (§4, « Plateformes prises en charge »), soit Debian 13, Ubuntu 24.04, la famille RHEL 10 et Fedora, avec Windows 10 et 11.* Reste à décider si Debian 12, Ubuntu 22.04 et RHEL 9, qui sont encore très répandus, justifient plus tard un paquet qui embarque GTK (Flatpak ou AppImage), ou une version plus ancienne de gotk4 et une interface réduite. La version de librsvg varie d'un système à l'autre (2.57 à 2.63), ce qui touche la fidélité et le déterminisme : les PDF ne sont identiques qu'à bibliothèques égales (§11).

## Sources

Pages consultées le 19 octobre 2026.

- [librsvg : présentation et rendu vers cairo](https://gnome.pages.gitlab.gnome.org/librsvg/Rsvg-2.0/overview.html)
- [librsvg : mesure de la géométrie d'un élément](https://valadoc.org/librsvg-2.0/Rsvg.Handle.get_geometry_for_element.html)
- [GTK : installation et distribution sous Windows](https://gtk.org/docs/installations/windows/)
- [gotk4 : liaisons GTK4 pour Go](https://github.com/diamondburned/gotk4-libgirepository)
- [Fyne : prérequis et licence](https://github.com/fyne-io/fyne)
- [tdewolff/canvas : moteur de rasterisation](https://pkg.go.dev/github.com/tdewolff/canvas/renderers/rasterizer)
- [svg2pdf-py : liaison de svg2pdf, PDF vectoriel](https://pypi.org/project/svg2pdf-py/)

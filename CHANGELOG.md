<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Changelog

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/),
et ce projet adhère au [Semantic Versioning](https://semver.org/lang/fr/).

## [Non publié]

Aucune version n'est encore publiée : l'application n'est pas utilisable. Le dépôt contient
le squelette, les documents de référence, la bibliothèque d'unités et le prototype du moteur de
rendu (M0a) ; le prototype de l'interface GTK4 et le build sous Windows (M0b) sont la prochaine
étape.

### Ajouté

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

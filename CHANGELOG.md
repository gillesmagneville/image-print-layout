<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Changelog

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/),
et ce projet adhère au [Semantic Versioning](https://semver.org/lang/fr/).

## [Non publié]

Aucune version n'est encore publiée : l'application n'est pas utilisable. Le dépôt contient
le squelette, les documents de référence et la bibliothèque d'unités ; le prototype M0 est la
prochaine étape.

### Ajouté

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

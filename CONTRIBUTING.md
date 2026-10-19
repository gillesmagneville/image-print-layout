<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Contribuer à SVG Print Layout

Merci de l'intérêt porté à ce projet ! Ce document explique comment signaler un problème,
proposer une amélioration, ou contribuer du code.

## Signaler un bug

Ouvrez une issue en précisant :

- le système (Ubuntu ou Windows, avec la version) ;
- la version de l'application (`svgprintlayout -version`) ;
- ce que vous avez fait, le comportement observé et le comportement attendu ;
- si possible, le SVG en cause (ou un SVG minimal qui reproduit le problème) et le fichier
  projet `.svgpl`.

## Proposer une amélioration

Ouvrez une issue décrivant le besoin avant d'écrire du code, en particulier pour une règle
d'analyse du SVG ou un changement du modèle ou du format de projet : c'est plus facile d'en
discuter avant qu'après. Les fonctions listées comme hors périmètre dans
`PROJECT_CONTEXT.md` sont des évolutions possibles, pas des oublis.

## Contribuer du code

1. Forkez le dépôt et créez une branche depuis `main`.
2. Vérifiez que tout passe avant de proposer une pull request :

   ```bash
   gofmt -l .          # ne doit rien afficher
   go vet ./...
   go test ./...
   ```

   Ces commandes sont exactement celles de l'intégration continue
   (`.github/workflows/ci.yml`), qui les exécute sous Ubuntu et sous Windows avec
   `CGO_ENABLED=0`. N'utilisez pas de fonctionnalité plus récente que la version de Go
   indiquée dans `go.mod`.

3. Décrivez dans la pull request ce qui change et pourquoi, et dites ce que vous avez
   vérifié et ce que vous n'avez pas pu vérifier.

Une pull request porte **un seul changement cohérent**. Le titre du commit a la forme
`zone: résumé` (par exemple `units: …`, `layout: …`, `docs: …`), et la pull request est
fusionnée en un seul commit.

## Règles d'architecture

Le contrat complet est dans `ARCHITECTURE.md`. Les règles à ne pas enfreindre :

- **Go pur hors `ui` et `engine`.** Seuls ces deux paquets peuvent utiliser cgo. Les autres,
  `layout`, `units`, `model` et `project` en particulier, se testent partout, sans
  bibliothèque graphique. Un test échoue si `import "C"` apparaît ailleurs.
- **Une seule échelle.** Aucune fonction n'accepte deux facteurs séparés pour la largeur et
  la hauteur. L'échelle (en mm par unité du document) est stockée à un seul endroit, et les
  pages la lisent.
- **Coupures en unités du document**, pas en millimètres.
- **Toute conversion d'unité passe par `units`** (1 mm = 72 / 25,4 points ; 1 unité = 1 pixel
  CSS à 96 dpi). Aucune constante de conversion recopiée ailleurs.
- **Le SVG source n'est jamais ouvert en écriture.** Toute transformation s'applique à la
  copie de travail.
- **Rien d'aléatoire ni de daté dans ce que l'application écrit** (PDF, projet) : pas
  d'horodatage, pas d'identifiant aléatoire, et jamais de parcours d'une `map` sans trier
  ses clés, car l'ordre de parcours de Go change d'une exécution à l'autre.
- **Aucune réduction silencieuse.** Une page qui dépasse est signalée avec des corrections
  proposées, jamais corrigée d'office.
- **Pas d'accès au réseau.**
- **Messages d'erreur explicites** : ce qui a échoué, pourquoi, et ce que l'on peut faire,
  sans code technique brut.
- **Format de projet.** Toute évolution incrémente `version` et ajoute une fonction de
  migration `vN → vN+1`, testée sur des fichiers d'exemple.

## Style de code

- `gofmt` avant de proposer une pull request.
- Identifiants et commentaires de code en anglais ; textes destinés à l'utilisateur en
  français (la langue de l'interface reste à confirmer : voir « Questions ouvertes » dans
  `ARCHITECTURE.md`).
- Bibliothèque standard d'abord. Toute nouvelle dépendance doit se justifier : ce qu'elle
  apporte que la bibliothèque standard n'offre pas, sa licence, sa maintenance.
- Chaque fichier source, document ou fichier de CI commence par
  `SPDX-License-Identifier: GPL-3.0-only` (en commentaire, avec une ligne vide ensuite dans
  un fichier Go, pour qu'il ne se mêle pas au commentaire de paquet). Un test échoue sinon.

## Tests

- La logique de calcul se teste en Go pur, avec `go test` partout.
- Pour `layout`, préférez des tests de propriétés (papiers, marges, échelles et coupures
  tirés au hasard avec une graine fixe) aux seuls exemples : couverture exacte des bandes,
  largeur constante, échelle inchangée par les commandes.
- Les SVG de référence vont dans `testdata/`. Le Mini-CAD réel n'y est pas encore ; un
  générateur déterministe le remplacera en attendant (voir `ARCHITECTURE.md`).

## Ajouter un paquet dans `internal/`

Décrivez-le dans la section « Disposition du dépôt » de `ARCHITECTURE.md` : un test échoue
tant qu'un paquet de `internal/` n'y figure pas.

## Documentation

Si votre changement modifie le comportement observable (nouvelle fonction, nouveau réglage,
nouveau format de fichier), mettez à jour `CHANGELOG.md` dans la même pull request, sous
`[Non publié]`. Si le contrat technique change (interface, formule, format, valeur par
défaut), mettez aussi `ARCHITECTURE.md` à jour : c'est lui qui fait foi.

## Versions

Le projet suit le [Semantic Versioning](https://semver.org/lang/fr/). La version courante
est dans le fichier `VERSION`, que l'on injecte à la compilation :

```bash
go build -ldflags "-X main.version=$(cat VERSION)" ./cmd/svgprintlayout
```

Une release est une pull request à part, « Release X.Y.Z », qui met à jour `VERSION` et
déplace les notes de `[Non publié]` sous le nouveau numéro dans `CHANGELOG.md` ; le tag
`vX.Y.Z` est posé ensuite. Tant que `VERSION` porte un suffixe (`0.1.0-dev`), aucune
version n'est publiée.

## Questions

Pour toute question qui ne rentre pas dans les cas ci-dessus, ouvrez simplement une issue.

## Licence

En contribuant, vous acceptez que vos contributions soient publiées sous la même licence que
le projet (GPL-3.0-only).

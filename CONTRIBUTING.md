<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Contribuer à SVG Print Layout

Merci de l'intérêt porté à ce projet ! Ce document explique comment signaler un problème,
proposer une amélioration, ou contribuer du code.

SVG Print Layout est développé par Gilles MAGNEVILLE, et conçu avec l'aide de claude.ai.

## Signaler un bug

Ouvrez une issue en précisant :

- le système (la distribution et sa version, ou Windows 10 ou 11) ;
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
   (`.github/workflows/ci.yml`), qui les exécute sous Linux et sous Windows avec
   `CGO_ENABLED=0`. N'utilisez pas de fonctionnalité plus récente que la version de Go
   indiquée dans `go.mod`.

   Si vous touchez au moteur de rendu (`internal/engine`), lancez aussi ses tests avec cgo
   (`sudo apt-get install librsvg2-dev libfontconfig-dev poppler-utils fonts-ibm-plex`, puis
   `CGO_ENABLED=1 go test ./internal/engine ./internal/testsvg`) : c'est ce que fait le job
   `engine` de l'intégration continue.

   Si vous touchez à la fenêtre (`internal/ui`), ajoutez GTK 4 et un écran virtuel
   (`sudo apt-get install librsvg2-dev libfontconfig-dev libgtk-4-dev libgirepository1.0-dev xvfb xauth
   imagemagick fonts-ibm-plex`) et lancez `xvfb-run -a go test ./internal/ui
   ./cmd/svgprintlayout` : c'est le job `ui`, qui dépose aussi des captures de la fenêtre
   dans les quatre langues (artefact `screenshots`). Un texte de l'interface s'ajoute aux
   **quatre** catalogues de `internal/i18n` (le test dit lesquels l'oublient).

   Si vous touchez aux scripts de construction des paquets ou à leurs dépendances, essayez-les
   sur la distribution visée : le job `distributions` de la CI le fait dans un conteneur par
   système pris en charge (`.github/scripts/test-distribution.sh debian|redhat`, lancé depuis
   la racine du dépôt dans le conteneur ; voir `ARCHITECTURE.md`, §4 « Plateformes prises en
   charge »). Une bibliothèque plus récente que celles de ce tableau ne doit pas être exigée
   sans qu'on retire le système qui ne l'a pas.

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
- Identifiants et commentaires de code en anglais. Aucun texte destiné à l'utilisateur dans
  le code : il porte une clé (`tr.T("panel.paper.title")`) dont le texte est dans les
  quatre catalogues de `internal/i18n` (français, anglais, espagnol, allemand). Le français
  est la référence ; un test échoue si une langue oublie une clé, un texte ou un paramètre
  `{nom}`, ou si le code demande une clé qui n'existe pas. Voir « Langues » dans
  `ARCHITECTURE.md`.
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
- Les SVG de référence vont dans `testdata/`. Le Mini-CAD réel n'y est pas : il décrit une
  infrastructure réelle. Le générateur déterministe `internal/testsvg` en fournit un
  équivalent, et `SVGPL_MINICAD=/chemin/du/fichier.svg` fait passer les tests du moteur sur
  le vrai fichier (voir `docs/M0.md`).
- Un test de rendu ne compare jamais deux rasters pixel par pixel : un rastériseur ne
  redessine pas deux fois la même image à l'identique. Comparez la géométrie écrite dans le
  PDF, ou des moyennes par carrés (voir « Test de couture » dans `ARCHITECTURE.md`).

- Les scripts de construction des paquets (`build-deb.sh`, `build-rpm.sh`,
  `windows/build-windows.sh`) passent sous `shellcheck` sans avertissement. Un changement qui
  les touche se vérifie en construisant le paquet, en l'installant et en lançant
  `svgprintlayout -selftest` ; la CI le fait pour les trois formats. Deux constructions du
  même code doivent donner des fichiers identiques : ne mettez dans un paquet ni date, ni
  chemin de la machine, ni ordre de fichiers laissé au hasard.
- Les trois scripts (`build-deb.sh`, `build-rpm.sh`, `windows/build-windows.ps1`) ont les
  mêmes options et la même gestion de `VERSION` que ceux de Planche-Contact, et ne partagent
  aucun code : si vous changez l'un de ces comportements, changez-le dans les trois. Le test
  de `repocheck` et la CI (`.github/scripts/`) signalent les écarts.
- Les fichiers `.ps1` et `installer.nsi` sont en UTF-8 **avec** marque d'ordre des octets
  (BOM), nécessaire à Windows PowerShell 5.1 et à NSIS pour lire les accents ; les scripts
  lus par bash gardent des fins de ligne Unix (`.gitattributes`).

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

Les scripts de construction des paquets gèrent `VERSION` : avec `--major`, `--minor` ou
`--patch` (ou le menu qu'ils affichent sans option), ils l'incrémentent une fois le paquet
construit ; avec `--no-version-change`, ils la laissent telle quelle. **Pour un essai de
construction, utilisez `--no-version-change`**, sans quoi vous modifiez `VERSION` sans le
vouloir. Une version qui porte un suffixe (`0.1.0-dev`) est d'abord publiée : `--minor` et
`--patch` donnent `0.1.0`, `--major` donne `1.0.0`.

Une release est une pull request à part, « Release X.Y.Z » :

1. construisez le premier format avec l'incrément voulu (`./build-deb.sh --minor`) : le
   script met `VERSION` à jour ;
2. construisez les autres avec `--no-version-change` (`./build-rpm.sh --no-version-change`,
   `.\build-windows.ps1 -NoVersionChange`) ;
3. mettez à jour **tous les fichiers qui nomment la version**, dans la même pull request :
   - `CHANGELOG.md` : déplacez les notes de `[Non publié]` sous
     `## [X.Y.Z] - AAAA-MM-JJ` (le plus récent en premier) ;
   - `metainfo/io.github.gillesmagneville.svgprintlayout.metainfo.xml` : ajoutez
     `<release version="X.Y.Z" date="AAAA-MM-JJ">` en tête de `<releases>`, avec sa
     description ;
   - `docs/svgprintlayout.1`, `docs/fr/svgprintlayout.1`, `docs/es/svgprintlayout.1` et
     `docs/de/svgprintlayout.1` : la date et `svg-print-layout X.Y.Z` de la ligne `.TH` ;
   - `README.md`, `ARCHITECTURE.md` (§14, « État ») et les rapports de `docs/` quand ce
     qu'ils annoncent a changé avec la version ;

   `go test ./internal/repocheck` échoue tant que `VERSION`, le CHANGELOG, le fichier
   AppStream et les quatre pages de manuel ne disent pas la même version et la même date ;
   la CI le lance. Un texte de la fiche AppStream (description de la release) s'écrit en
   anglais et dans les trois autres langues : `repocheck` le vérifie aussi ;
4. fusionnez ; le tag `vX.Y.Z` est posé ensuite.

Tant que `VERSION` porte un suffixe (`0.1.0-dev`), aucune version n'est publiée.

## Questions

Pour toute question qui ne rentre pas dans les cas ci-dessus, ouvrez simplement une issue.

## Licence

En contribuant, vous acceptez que vos contributions soient publiées sous la même licence que
le projet (GPL-3.0-only).

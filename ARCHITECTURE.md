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
- **Cadrage.** `rsvg_handle_render_document` met le document à l'échelle proportionnellement dans un viewport et le centre. On lui donne un viewport de même proportion que le dessin, pour ne dépendre d'aucun centrage implicite.
- **Boîtes des éléments.** [librsvg mesure un élément par son identifiant](https://valadoc.org/librsvg-2.0/Rsvg.Handle.get_geometry_for_element.html), sans tenir compte des clips. L'analyse ajoute des identifiants à la copie de travail pour mesurer chaque bloc (§7).

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

Le reste s'appuie sur la bibliothèque standard (XML, JSON, SHA-256). La relecture du PDF produit, pour les vérifications du §11, utilisera une bibliothèque Go de lecture PDF à choisir en M0.

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

Les flèches vont de l'appelant vers l'appelé. Seuls `ui` et `engine` dépendent de cgo : `svgdoc` et `export` n'appellent `engine` que par l'interface `Engine`, qu'un faux moteur remplace dans les tests. Le point d'entrée `cmd/svgprintlayout` se contente de câbler ces paquets.

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
├── cmd/svgprintlayout/        # point d'entrée : câble les paquets
├── internal/
│   ├── model/                 # types de données partagés, sans logique
│   ├── units/                 # conversions SVG / px / pt / mm, longueurs SVG,
│   │                          #   formats papier, marges, zone utile, échelle
│   ├── diag/                  # diagnostics destinés à l'utilisateur
│   ├── layout/                # plan de pages, états, coupures automatiques
│   ├── project/               # fichier projet .svgpl
│   ├── svgdoc/                # copie de travail, analyse du SVG, détection du fond
│   ├── engine/                # moteur de rendu derrière l'interface Engine (cgo)
│   ├── export/                # export PDF et vérifications
│   ├── app/                   # état de l'application et commandes
│   ├── ui/                    # interface GTK4 (cgo)
│   └── repocheck/             # tests sur le dépôt lui-même (en-têtes, versions, docs)
└── testdata/                  # SVG de référence
```

Chaque paquet de `internal/` est décrit dans ce document ; un test échoue s'il en manque un.
Les paquets hors `ui` et `engine` n'utilisent pas cgo, et un test le vérifie.

## 5. Modèle géométrique

Tout repose sur trois repères et une échelle unique : les unités du document, les millimètres sur la feuille et les points du PDF.

| Repère | Unité | Sert à |
| --- | --- | --- |
| Document | Unité utilisateur SVG, origine en haut à gauche | Stocker les coupures et mesurer les éléments |
| Feuille | Millimètre, origine au coin haut gauche de la feuille | Marges, positions, affichage des dimensions |
| PDF et cairo | Point (1/72 pouce) | Dessiner ; 1 mm = 72 / 25,4 = 2,834646 points, valeur fixe |

### Dimensions du document

Les unités utilisateur sont celles de la `viewBox`. Pour la taille native, utilisée par l'échelle à 100 % :

- `width` et `height` en unité absolue (mm, cm, pouce, pt, pc, px) : conversion standard.
- Sans unité, en pourcentage ou absents : une unité vaut un pixel CSS à 96 dpi, soit 25,4 / 96 = 0,26458 mm.
- Ni `viewBox` ni dimensions : étendue du contenu mesurée par librsvg ; si elle est vide, erreur explicite.

Si le Mini-CAD est en pixels, ses 1 280 × 5 479 unités font 338,7 × 1 449,7 mm à 100 %.

### Échelle

L'échelle `s` est un nombre unique, en mm par unité, stocké à un seul endroit. Aucune page n'en garde une copie : elles la lisent.

```latex
s = \frac{L}{W_d} \qquad h_i = (y_i - y_{i-1})\, s \qquad w = W_d\, s = L
```

`W_d` est la largeur du document en unités, `L` la largeur choisie en mm, `y_i` la position de la coupure i en unités, `h_i` la hauteur de la page i en mm.

| Mode | Calcul de `s` |
| --- | --- |
| A. Largeur | `L` = largeur utile du papier (largeur du papier moins marges gauche et droite) |
| B. Hauteur | `s` = hauteur utile divisée par la hauteur en unités de la portion choisie, par défaut la première page. La largeur obtenue est ensuite comparée à la largeur utile |
| C. Manuel | `L` saisie en mm, ou facteur `f` en % : `s` = `f` × échelle native |

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

Le document est rendu une seule fois dans une surface d'enregistrement cairo, puis peint sur chaque page. Le PDF ne devrait donc pas contenir le dessin autant de fois qu'il a de pages. La mesure en M0 le confirme ; sinon, le repli est une surface de rendu par page avec clip, au prix d'un fichier plus gros.

### Aperçu

En vue Document, le dessin est peint en tuiles et les limites de page sont des surimpressions. En vue Feuilles, chaque page est peinte avec sa propre matrice sur une feuille blanche. Coupures, poignées et numéros appartiennent à l'interface : ils ne sont jamais dans le PDF.

### Export

Pour chaque page : taille de page fixée, fond de page (§8), sauvegarde du contexte, clip, matrice, peinture du dessin, restauration, page suivante. L'exporteur lit `s` dans le plan et ne le modifie pas.

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
4. **Changer de format** : premier format dont la zone utile contient la bande la plus haute à l'échelle actuelle.

Pour un dépassement en largeur, seules les propositions 3 et 4 existent. Aucune n'est appliquée sans un clic, ce qui garantit l'absence de réduction silencieuse.

### Coupures

Une coupure porte un identifiant, un axe (réservé, `y` seul en v1) et une position en unités. Invariants : `0 < y < H_d`, ordre strict, écart minimal de 0,5 mm physique avec les bords et les autres coupures. Ajouter, déplacer et supprimer sont des commandes pures qui renvoient un nouvel état ; une propriété testée établit que `s` est identique avant et après chacune d'elles.

Pendant le glisser, la coupure s'aimante aux vides détectés dans un rayon de 8 pixels d'écran ; maintenir Alt désactive l'aimantation.

### Blocs et détection de traversée

Les **blocs** sont les groupes de plus haut niveau qui portent du contenu visible, en descendant dans les calques Inkscape. Le rectangle de fond détecté (§8) est exclu, sinon toute position le traverserait. Le moteur mesure chaque bloc ; `layout` ne manipule que ces boîtes.

Une coupure traverse un bloc si sa position est strictement à l'intérieur de la boîte du bloc (tolérance 0,05 unité). Elle reçoit alors une icône d'avertissement, et le panneau de droite nomme les blocs traversés.

### Coupures automatiques

1. **Nombre de pages minimal** : `n_min` = plafond de (hauteur du document en mm / hauteur utile).
2. **Vides** : l'union des intervalles verticaux des blocs, complémentée, donne les zones où rien n'est traversé. Le coût d'une position est le nombre de blocs qu'elle traverse, nul dans un vide.
3. **Recherche** pour `n` de `n_min` à `n_min + 2` : programmation dynamique sur les positions candidates (centre de chaque vide et positions idéales `j × H_d / n`), avec comme contraintes que chaque bande tienne dans la hauteur utile et ne soit pas inférieure à 40 % de la bande idéale. Critères, dans l'ordre : moins de blocs traversés, puis bandes les plus équilibrées.
4. **Proximité** : le centre d'un vide maximise la distance aux blocs voisins ; un vide de moins de 2 mm physiques compte comme « à proximité » et produit un avertissement.
5. **Résultat** : une proposition, appliquée par une seule commande annulable. Si aucune solution n'existe jusqu'à `n_min + 2`, la meilleure est renvoyée avec ses avertissements. L'utilisateur déplace ensuite chaque coupure.

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

Une `GtkDrawingArea` peint, dans l'ordre : zone de travail, feuilles, dessin par tuiles, limites et numéros de page, lignes de coupure, poignées.

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

Pendant un glisser, seul le calque des coupures est redessiné : le dessin vient du cache de tuiles et le plan est recalculé à chaque mouvement, ce qui est peu coûteux puisque `layout` est pur.

**Tuiles.** 512 × 512 pixels, en cache LRU par niveau de zoom. Elles sont produites hors du fil de l'interface, avec un compteur de génération qui abandonne les tuiles devenues obsolètes. En attente, on affiche la tuile du niveau précédent, floue.

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
  "scale": { "mode": "width", "widthMm": 200 },
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

Le contrôle 9 compare des points : un A4 vaut 595,28 × 841,89 pt. La correspondance des coupures avec l'aperçu vient de la construction (même plan, §6) ; l'exporteur la prouve en écrivant dans un manifeste interne les rectangles de clip réellement utilisés, comparés au plan. La vérification visuelle équivalente est faite par les tests (§13).

### Déterminisme

- **Pas d'horodatage ni d'identifiant aléatoire** dans le PDF : métadonnées fixes, aucune date de création.
- **Entrées stables** : les instructions de fond sont rejouées dans un ordre fixe, et `layout` est séquentiel, sans parallélisme qui changerait l'ordre des opérations flottantes.
- **Limite assumée** : l'égalité d'octets est garantie à versions de cairo et de librsvg, et à polices, identiques. Entre deux machines différentes, les tests comparent des rasters avec une tolérance plutôt que des octets.
- **À prouver en M0** : deux exports successifs du même projet, sur la même machine, donnent des fichiers de même SHA-256.

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
| Famille de police absente du système | Avertissement | Police « X » absente, remplacée par « Y » |
| Image liée par chemin, fichier introuvable | Avertissement | Image liée introuvable, nom et chemin attendus |
| Image liée par URL distante | Avertissement | Non téléchargée : l'application n'accède jamais au réseau |
| Image matricielle incorporée | Information | Conservée en bitmap, à sa résolution d'origine |
| Ni dimension ni `viewBox` exploitable | Erreur | Taille du document indéterminée |

À confirmer en M0 sur des fichiers réels, avant de figer les règles : le comportement face au texte encadré d'Inkscape (`flowRoot`), aux polices web (`@font-face`) et à `foreignObject`.

Le rapport s'affiche dans la section « Fichier source » avec un compteur par niveau. Cliquer une entrée sélectionne et centre l'élément concerné dans l'aperçu, grâce à sa boîte mesurée. Le rapport peut être copié en texte brut.

Le texte est dessiné avec les polices du système. Son intégration dans le PDF et sa sélectionnabilité sont à vérifier en M0 ; une police absente à l'export est substituée et signalée comme ci-dessus.

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
| 13 | Coupures conformes à l'aperçu | Test de couture (ci-dessous) et rectangles de clip égaux au plan | Auto |
| 14 | Réouverture du projet | Enregistrer, recharger, égalité profonde des réglages ; JSON de référence | Auto |

### Test de couture

On rastérise le PDF produit, on découpe chaque page à sa fenêtre de contenu, on recolle les fenêtres dans l'ordre, puis on compare le résultat au rendu du document entier à la même résolution. L'écart toléré aux coutures est de un pixel. Ce test établit à la fois l'absence de perte ou de doublon aux coupures et l'égalité de largeur : c'est lui qui répond au critère final du cahier, un PDF fidèle à l'aperçu sans redimensionnement silencieux.

### Organisation

- **Trois niveaux.** Go pur (modèle, unités, mise en page, analyse, projet) avec `go test` partout sans cgo ; intégration du moteur (chargement, tuiles, export) avec librsvg installé ; interface sous Xvfb sur Linux, plus une liste de contrôle manuelle sur Windows.
- **Tests de propriétés** sur `layout` : papiers, marges, échelles et coupures tirés au hasard, avec pour invariants la couverture exacte, `w` constant, `s` inchangé par les commandes et des états cohérents.
- **Jeu d'essai.** Le Mini-CAD réel n'est pas fourni. Un générateur déterministe produit un SVG de 1 280 × 5 479 unités (fond gris, blocs avec texte, bandeau pleine largeur) ; il sert aussi de test de charge avec 100 000 éléments. Le vrai fichier le remplace dès qu'il est disponible.
- **Intégration continue** sur Ubuntu avec les paquets système, et sur Windows sous MSYS2 UCRT64 avec le paquet `mingw-w64-ucrt-x86_64-gtk4` et ses dépendances.
- **Références.** Les rasters de référence sont stockés par plateforme, avec tolérance, et jamais comparés d'une plateforme à l'autre.

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

**État.** Le squelette du dépôt et le paquet `units` existent ; M0 n'est pas commencé. Le `CHANGELOG.md` tient le détail.

Après la v1, le §14 du cahier reste la liste des suites possibles. L'aperçu du PDF réel relu par un moteur PDF indépendant (le cahier demande « un aperçu du PDF final ») est le premier candidat : la v1 s'en tient à l'aperçu produit par la même fonction de dessin, ce qui est fidèle au PDF par construction.

## 15. Risques et questions ouvertes

Les cinq premiers risques se tranchent au jalon M0 ; aucun développement d'interface n'est engagé avant son verdict.

| Risque | Effet | Parade | Tranché en |
| --- | --- | --- | --- |
| Packaging Windows de GTK4, librsvg et leurs dépendances | Installateur lourd, ou build impossible | Build sous MSYS2 UCRT64, essai sur une machine Windows propre, moteur D en repli | M0 |
| Fidélité de librsvg sur le Mini-CAD réel (texte, polices) | Rendu différent de l'original | Essai du vrai fichier, comparé à un rendu d'Inkscape | M0 |
| Taille du PDF : le dessin répété à chaque page | Fichier N fois plus gros | Dessin enregistré une fois puis peint sur chaque page ; sinon une surface par page avec clip | M0 |
| Résolution des éléments rastérisés (filtres) dans le PDF | Flou ou pixels visibles à l'impression | Mesure, avertissement dès l'analyse, étude du réglage de la résolution de repli | M0 |
| Déterminisme dépendant des versions de cairo et de librsvg | Octets différents d'une machine à l'autre | Versions figées dans les paquets Windows ; comparaison par rasters entre plateformes | M0 puis M5 |
| Maturité de gotk4 | Fuites mémoire, plantages | Interface mince, moteur indépendant de gotk4, scénarios sous Xvfb, repli Fyne | M2 |
| Documents très lourds | Chargement et tuiles lents, mémoire | Test à 100 000 éléments, cache LRU, chargement hors du fil de l'interface | M2 |
| Police absente sur la machine d'export | Texte substitué | Détection et avertissement avant l'export | M2 |

### Questions ouvertes

1. **Fichier réel.** Pouvez-vous fournir le Mini-CAD, et un second SVG représentatif, pour valider M0 ?
2. **Coupures verticales.** Sont-elles bien hors v1, comme le suggèrent le §14 du cahier et le test 12 ?
3. **Mode B.** La définition du §1 convient-elle : échelle qui fait tenir la portion choisie dans la hauteur utile ?
4. **Aperçu du PDF final (§9.2).** La v1 le fournit avec la même fonction de dessin que l'export. Faut-il en plus relire le fichier PDF avec un moteur PDF indépendant dès la v1 ?
5. **Texte dans le PDF.** Faut-il garantir un texte sélectionnable avec polices incorporées, ou l'apparence suffit-elle ?
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

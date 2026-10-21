<!-- SPDX-License-Identifier: GPL-3.0-only -->

# PROJECT_CONTEXT.md

# Image Print Layout

**Image Print Layout** est une application de bureau, écrite en **Go**, qui prépare un
document de dimensions quelconques, dessin vectoriel SVG ou image (JPEG, PNG, TIFF, WebP,
BMP, GIF), pour l'impression sur plusieurs feuilles de papier standard : on choisit le
papier et la largeur du document, on place les coupures entre les pages sur un aperçu,
puis on exporte **un seul PDF multipage** (vectoriel pour un SVG).

Elle s'adresse à toute personne qui doit imprimer un grand dessin (un schéma, par exemple)
sans connaître les coordonnées SVG, la programmation ou la géométrie.

> **Auteur :** Image Print Layout est développé par Gilles MAGNEVILLE, et conçu avec l'aide de claude.ai.

> **Note sur ce document :** PROJECT_CONTEXT.md dit *pourquoi* le projet existe et *ce qu'il
> valorise*. Il ne répète ni les formules, ni les interfaces, ni les formats de fichier :
> ceux-ci vivent une seule fois, de façon canonique, dans `ARCHITECTURE.md`, pour que les
> deux documents ne puissent pas diverger en silence. Les exigences détaillées sont celles du
> cahier des charges (version 1.0).

---

# Vision

Imprimer un grand dessin sur plusieurs feuilles pose toujours la même question :

> **« Où couper, à quelle largeur, et le résultat sera-t-il exactement ce que je vois ? »**

Image Print Layout répond à cette question avec un aperçu interactif : le dessin entier, les
limites de chaque page, des lignes de coupure que l'on déplace à la souris, et pour chaque
page ses dimensions, son échelle et son état. Le PDF exporté est celui que l'aperçu montrait.

Le nombre de pages découle des dimensions du dessin, du format du papier, de la largeur
choisie et des positions des coupures. Il n'est jamais imposé.

---

# Principes du projet

Image Print Layout suit cinq principes.

## 1. Jamais de déformation silencieuse

Le dessin a **une seule échelle**, la même pour toutes les pages. Les proportions sont
conservées, sauf demande explicite de l'utilisateur. Ajouter ou déplacer une coupure, ou
changer un alignement, ne modifie jamais la largeur du dessin.

Quand une portion est trop haute pour une feuille, le logiciel ne réduit **pas** le dessin
pour la faire tenir : il le dit, et propose.

Mauvais :

```
La page 3 a été réduite à 95 % pour tenir sur la feuille.
```

Bon :

```
La page 3 dépasse de 14 mm la hauteur utile (277 mm). Rien n'a été modifié.

Vous pouvez : ajouter une coupure, déplacer la coupure voisine,
réduire l'échelle à 95 % (largeur 190 mm), ou choisir un format plus grand.
```

Aucune de ces propositions n'est appliquée sans un clic.

---

## 2. Le fichier source n'est jamais modifié

Le fichier d'origine (SVG ou image) n'est jamais ouvert en écriture. Toute transformation (remplacer ou
supprimer le fond, par exemple) s'applique à une copie de travail, et le projet n'en garde
que l'instruction, rejouée à chaque chargement.

---

## 3. L'aperçu est le PDF

L'aperçu est la fonction centrale de l'application. Aperçu et export dessinent à partir du
même plan de pages : ce que l'utilisateur voit, ce sont les coupures, l'échelle et les
positions que le PDF reproduira.

Ce qui ne peut pas être reproduit fidèlement (effet rastérisé, police absente, fonction SVG
non prise en charge) est **signalé avant l'export**, jamais reconstitué en silence.

---

## 4. L'utilisateur garde le contrôle

Les coupures automatiques, les corrections et les réglages détectés (comme le rectangle de
fond) sont des **propositions**. L'utilisateur peut toujours les déplacer, les refuser ou les
annuler.

Les opérations courantes se font dans la fenêtre principale, sans enchaîner les boîtes de
dialogue. Chaque message d'erreur dit ce qui a échoué, pourquoi, et ce que l'on peut faire.

---

## 5. Déterministe et hors ligne

À paramètres identiques, le résultat est identique (à versions de bibliothèques et à polices
égales : les limites exactes sont dans `ARCHITECTURE.md`). L'application ne recourt à aucun
service en ligne et n'envoie aucune donnée.

---

# Périmètre

Image Print Layout couvre, pour la version 1.0 :

* l'import de SVG 1.1 et de SVG compatibles Inkscape ;
* depuis la version 1.2, l'import d'images matricielles (JPEG, PNG, TIFF, WebP, BMP, GIF), qui se
  découpent et s'impriment comme un SVG ;
* les formats A0 à A5, Letter, Legal, Tabloid et un format personnalisé, en portrait ou en
  paysage, avec quatre marges indépendantes ;
* trois modes d'échelle : ajustement à la largeur, à la hauteur, ou échelle manuelle ;
* l'aperçu interactif, les coupures manuelles et la proposition de coupures automatiques ;
* le calcul des pages et de leur état (conforme ou dépassement) ;
* la gestion du fond : détection, remplacement par du blanc, suppression, conservation ;
* le positionnement du dessin sur chaque feuille ;
* l'export en un seul PDF multipage, avec vérifications avant et après ;
* l'enregistrement et la réouverture d'un projet de mise en page.

Image Print Layout **n'est pas**, dans la version 1.0 :

* un éditeur de dessin : la seule modification du contenu est le réglage du fond ;
* un outil d'impression en mosaïque avec chevauchement, ni de marques de raccord ;
* un outil de numérotation ou de pied de page ;
* un convertisseur : l'entrée est un SVG ou une image, jamais un PDF, et le programme ne
  retouche pas les pixels (pas de recadrage, de rééchantillonnage ni de correction de couleur) ;
* un outil qui tient compte des marges non imprimables propres à une imprimante.

Ces fonctions sont listées au §14 du cahier des charges comme possibles ultérieurement.

---

# Plateformes visées

* Linux, notamment Ubuntu ;
* Windows 10 et Windows 11.

Langage : Go. Format de sortie principal : PDF multipage vectoriel.

Les versions d'Ubuntu à prendre en charge ne sont pas encore fixées : elles dépendent de la
version de librsvg fournie (voir « Questions ouvertes » dans `ARCHITECTURE.md`).

---

# Critère de réussite

Le logiciel est considéré comme fonctionnel lorsque les 14 tests d'acceptation du cahier des
charges réussissent : du schéma Mini-CAD (1 280 × 5 479 unités SVG) importé sur A4 portrait à
200 mm de largeur jusqu'à la réouverture du projet.

> Le test est réussi si le PDF final est fidèle à l'aperçu et si aucune page ne subit de
> redimensionnement silencieux.

La correspondance entre ces 14 tests et leur vérification automatique est dans
`ARCHITECTURE.md` (« Stratégie de tests »).

---

# Principes de code

* Go ; la bibliothèque standard d'abord, une dépendance seulement quand elle se justifie.
* La logique de calcul (modèle, unités, mise en page, projet) ne dépend ni de l'interface ni
  de cgo : elle se teste partout, sans bibliothèque graphique.
* Une seule échelle dans le modèle : aucune fonction n'accepte deux facteurs séparés pour la
  largeur et la hauteur.
* Aucun horodatage, aucun identifiant aléatoire, aucun ordre dépendant d'une table de hachage
  dans ce que l'application écrit (PDF, projet).

---

# Évolutions possibles

Après la version 1.0 (cahier des charges, §14) :

* impression en mosaïque avec chevauchement pour assemblage ;
* traits de repérage et marques de raccord ;
* numérotation et pied de page ;
* rotation automatique des pages ;
* import de PDF (les images matricielles sont lues depuis la version 1.2) ;
* détection avancée des groupes logiques du SVG ;
* prévisualisation des marges non imprimables propres à l'imprimante.

Aucune n'est nécessaire pour que la version 1.0 soit utile.

---

# Valeurs

Image Print Layout valorise :

* la fidélité à l'aperçu
* la clarté
* le contrôle de l'utilisateur
* la reproductibilité
* la transparence sur ce qui n'est pas pris en charge
* la portabilité

Avant tout :

**Aucune page ne subit de redimensionnement silencieux.**

<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Politique de sécurité

🇬🇧 [English version](SECURITY.md)

Image Print Layout est maintenu par une seule personne, à titre bénévole. Cette page explique comment signaler un problème de sécurité et ce que vous pouvez en attendre.

## Versions prises en charge

Seule la **dernière version publiée** reçoit les correctifs de sécurité. Si vous utilisez une version plus ancienne, mettez-la d'abord à jour : les paquets sont sur la page des [Releases](https://github.com/gillesmagneville/image-print-layout/releases) et les changements sont listés dans le [CHANGELOG](CHANGELOG.fr.md).

## Signaler une vulnérabilité

**Merci de ne pas signaler une vulnérabilité dans une issue, une pull request ou une discussion publique** : n'importe qui pourrait la lire avant qu'un correctif existe.

Signalez-la en privé, de l'une de ces deux façons :

1. **Sur GitHub** : ouvrez l'[onglet Security](https://github.com/gillesmagneville/image-print-layout/security) du dépôt et cliquez sur **Report a vulnerability**. Seuls vous et le mainteneur voyez le signalement.
2. **Par courriel** : écrivez à [gilles@magneville.fr](mailto:gilles@magneville.fr) avec l'objet « Image Print Layout sécurité ».

Indiquez, autant que possible :

- la version (`imageprintlayout -version`), la façon dont vous l'avez installé (`.deb`, `.rpm`, installeur ou version portable Windows) et votre système d'exploitation ;
- la description du problème et les étapes pour le reproduire ;
- l'impact que vous lui voyez ;
- si le problème vient d'un fichier particulier, ce fichier — seulement si vous avez le droit de le partager.

## Ce que vous pouvez attendre

Ce sont des objectifs, pas des garanties, puisqu'il n'y a qu'un seul mainteneur :

- un accusé de réception de votre signalement sous **7 jours** ;
- une réponse indiquant s'il est considéré comme une vulnérabilité, puis des nouvelles au moins toutes les quelques semaines jusqu'à ce que le sujet soit réglé ;
- un correctif publié dans une **nouvelle version**, mentionné dans le CHANGELOG ; pour un problème grave, un avis de sécurité GitHub également ;
- vous êtes cité dans les notes de version, sauf si vous préférez rester anonyme.

Merci de garder les détails confidentiels jusqu'à la publication d'une version corrigée, pour que les utilisateurs aient le temps de mettre à jour.

## Ce qui est concerné

Image Print Layout travaille sur votre ordinateur, sur vos propres documents, et n'utilise pas le réseau. Il **ouvre en revanche des fichiers qui peuvent venir de n'importe où** : documents SVG, images et fichiers de projet. Un signalement est pertinent, par exemple, si :

- un SVG, une image ou un fichier de projet fait planter le programme d'une façon exploitable, lui fait exécuter du code, ou lui fait lire ou écrire des fichiers qu'il ne devrait pas — par exemple un SVG dont les liens mettent dans le PDF ou la page HTML exportés des fichiers venant d'ailleurs sur l'ordinateur ;
- une page HTML exportée exécute du code provenant du document dont elle est issue ;
- un paquet `.deb`, `.rpm`, un installeur ou une version portable Windows, son script d'installation, les scripts de construction ou le workflow de publication peuvent être détournés ;
- un fichier d'une version publiée ne correspond pas à son empreinte `.sha256` publiée.

## Ce qui n'est pas concerné

- Une vulnérabilité dans une **bibliothèque développée par quelqu'un d'autre** (GTK, librsvg, gdk-pixbuf, cairo, fontconfig…) : merci de la signaler à ce projet. Sous Linux, ces bibliothèques viennent de votre distribution et les mettre à jour suffit. Sous **Windows, elles sont livrées dans le paquet** : prévenez-moi aussi, car le correctif demande une nouvelle version.
- Une vulnérabilité dans le **langage Go ou dans un module Go** intégré au programme : merci de la signaler à ce projet, et prévenez-moi aussi, puisque le correctif demande une nouvelle version.
- Les problèmes qui supposent un ordinateur déjà compromis, ou un accès physique à celui-ci.
- Les bugs ordinaires sans conséquence pour la sécurité : ouvrez plutôt une [issue](https://github.com/gillesmagneville/image-print-layout/issues).

## Vérifier ce que vous téléchargez

Téléchargez les paquets uniquement depuis la page des [Releases](https://github.com/gillesmagneville/image-print-layout/releases) de ce dépôt. Chaque paquet est publié avec son propre fichier `.sha256`. Pour vérifier un téléchargement, placez le paquet et son fichier `.sha256` dans le même dossier :

```sh
sha256sum -c <paquet>.sha256
```

Sous Windows, comparez le résultat de `Get-FileHash <paquet>` (PowerShell) avec le contenu du fichier `.sha256`. L'installeur Windows n'est pas signé numériquement (voir [`windows/README.fr.md`](windows/README.fr.md)) : cette vérification n'en est que plus utile.

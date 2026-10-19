<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Paquets de SVG Print Layout

Ce dossier contient ce qu'il faut pour construire les trois paquets du projet. Les
scripts se lancent depuis la racine du dépôt :

| Paquet | Commande | Résultat dans `dist/` |
| --- | --- | --- |
| Debian, Ubuntu | `./build-deb.sh` | `svgprintlayout_<version>_amd64.deb` |
| Fedora, RHEL et dérivés | `./build-rpm.sh` | `svgprintlayout-<version>-<release>.x86_64.rpm` |
| Windows 10 et 11 | `.\build-windows.ps1` | `svgprintlayout_<version>_windows-portable.zip` et `..._windows-setup.exe` |

Chaque script accepte `--help` (`-Help` pour PowerShell) et `--clean` (`-Clean`), qui
supprime le dossier de travail et les paquets déjà construits. Le dossier `dist/` n'est
pas versionné.

**Ces scripts ne modifient jamais le fichier `VERSION`.** Le numéro vient de lui, et
une version se change par une pull request « Release » (voir `CONTRIBUTING.md`) : une
construction ne peut donc pas laisser le dépôt dans un état incohérent.

## Numéros de version

`VERSION` suit SemVer. Une version d'avant-publication (`0.1.0-dev`) doit se classer
avant la version finale (`0.1.0`), ce que chaque format exprime à sa façon :

| `VERSION` | `.deb` | `.rpm` (Version, Release) | Windows |
| --- | --- | --- | --- |
| `0.1.0-dev` | `0.1.0~dev-1` | `0.1.0`, `0.1.dev` | `0.1.0-dev` |
| `0.1.0` | `0.1.0-1` | `0.1.0`, `1` | `0.1.0` |

## Ce que contiennent les paquets Linux

| Fichier installé | Rôle |
| --- | --- |
| `/usr/bin/svgprintlayout` | le programme |
| `/usr/share/applications/io.github.gillesmagneville.svgprintlayout.desktop` | l'entrée de menu |
| `/usr/share/metainfo/io.github.gillesmagneville.svgprintlayout.metainfo.xml` | les métadonnées AppStream, que lisent les logiciels de gestion de paquets |
| `/usr/share/icons/hicolor/scalable/apps/svgprintlayout.svg` et `256x256/apps/svgprintlayout.png` | l'icône |
| `/usr/share/man/man1/svgprintlayout.1.gz` | la page de manuel |
| `/usr/share/doc/svgprintlayout/` | README, journal des modifications, copyright (`.deb`) |
| `/usr/share/licenses/svgprintlayout/LICENSE` | la licence (`.rpm`) |

L'identifiant AppStream est `io.github.gillesmagneville.svgprintlayout` (nom de domaine
inversé, comme l'exigent les spécifications d'AppStream et de freedesktop.org) ; le nom
du fichier `.desktop` et la balise `<launchable>` du fichier AppStream doivent s'y
accorder, et `appstreamcli validate` et `desktop-file-validate` le vérifient.

Les dépendances sont déduites du binaire : `dpkg-shlibdeps` pour le `.deb`, `rpmbuild`
pour le `.rpm` (qui en tire des `Requires` sur les bibliothèques partagées). Les
dépendances du `.deb` portent donc les noms des paquets de la machine de construction,
avec le suffixe `t64` d'Ubuntu 24.04 et de Debian 13 ; un `.deb` construit sur Ubuntu
24.04 s'installe sur Ubuntu 24.04 et suivantes, pas sur 22.04. De même, un `.rpm` est lié
aux versions de glibc et de librsvg de sa machine de construction : pour viser une
distribution, construisez dessus, ou dans un conteneur de cette distribution.

## Pourquoi `dpkg-deb` et `rpmbuild`, et pas fpm ou nfpm

Les outils natifs sont déjà là sur toute machine Debian ou Fedora, ne demandent ni Ruby
ni accès à un dépôt de modules, et sont ceux que la CI et les mainteneurs de
distributions connaissent. Les scripts préparent l'arborescence eux-mêmes : `dpkg-deb`
et `rpmbuild` ne compilent rien, ils empaquettent.

## Construction reproductible

À code, version, compilateur et bibliothèques égaux, deux constructions donnent des
fichiers identiques à l'octet (la CI le vérifie pour le `.deb` et le `.rpm`) :

- `-trimpath`, un identifiant de construction vide et `-buildvcs=false` rendent le
  binaire indépendant du dossier et de l'heure de la construction ;
- `SOURCE_DATE_EPOCH` vaut la date du dernier commit ; toutes les dates des fichiers du
  paquet en découlent ;
- l'ordre des fichiers, les propriétaires (root) et les permissions sont fixés.

L'égalité n'est promise qu'à versions de Go, de librsvg et de cairo égales, comme pour
le PDF (`ARCHITECTURE.md`, « Déterminisme »).

## Le test de fumée : `-version` et `-selftest`

Le programme sait vérifier son moteur de rendu sur la machine où il tourne :

```sh
svgprintlayout -version    # version, plate-forme, versions de librsvg et de cairo
svgprintlayout -selftest   # charge un SVG intégré, le mesure, le dessine, l'exporte en PDF
```

`-selftest` contrôle ce dont dépend un paquet installé : la bibliothèque SVG, les
polices (le texte doit rester du texte dans le PDF), le décodage d'une image PNG
incorporée, l'export PDF et son déterminisme. Il rend le code 0 si tout fonctionne, 1
sinon, avec un message qui dit ce qui manque. La CI l'exécute sur chaque paquet
installé, et c'est aussi la première commande à demander à quelqu'un dont l'export
échoue.

## Windows

Voir [`windows/README.md`](windows/README.md) : prérequis (MSYS2, NSIS), contenu du
paquet, avertissement SmartScreen, dépannage.

## Ce qui n'est pas fait

- **Le programme n'a pas d'interface graphique** : les paquets installent un programme
  qui n'offre pour l'instant que `-version` et `-selftest`. L'interface GTK4 vient avec
  M0b et M2 (`ARCHITECTURE.md`, §14) ; ce changement ajoutera ses dépendances aux
  paquets.
- **Pas de signature** : ni `.deb`, ni `.rpm`, ni installeur Windows ne sont signés.
- **arm64** : les scripts Linux reconnaissent `aarch64`, mais rien n'a été construit ni
  essayé sur cette architecture.
- **Pas de dépôt de paquets** : les paquets se téléchargent et s'installent à la main.

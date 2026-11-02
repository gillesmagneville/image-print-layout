<!-- SPDX-License-Identifier: GPL-3.0-only -->

# Security Policy

🇫🇷 [Version française](SECURITY.fr.md)

Image Print Layout is maintained by one person, on a voluntary basis. This page explains how to report a security problem and what you can expect in return.

## Supported versions

Only the **latest release** receives security fixes. If you use an older version, please update first: the packages are on the [Releases page](https://github.com/gillesmagneville/image-print-layout/releases) and the changes are listed in the [CHANGELOG](CHANGELOG.md).

## Reporting a vulnerability

**Please do not report a vulnerability in a public issue, pull request or discussion**: anyone could read it before a fix exists.

Report it privately, in one of these two ways:

1. **On GitHub**: open the [Security tab](https://github.com/gillesmagneville/image-print-layout/security) of the repository and click **Report a vulnerability**. Only you and the maintainer can see the report.
2. **By email**: write to [gilles@magneville.fr](mailto:gilles@magneville.fr) with the subject "Image Print Layout security".

Please include, as far as you can:

- the version (`imageprintlayout -version`), how you installed it (`.deb`, `.rpm`, Windows installer or portable version) and your operating system;
- a description of the problem and the steps to reproduce it;
- what you think the impact is;
- if the problem comes from a particular file, the file itself — only if you may share it.

## What to expect

These are goals, not guarantees, since there is only one maintainer:

- an acknowledgement of your report within **7 days**;
- an answer on whether it is considered a vulnerability, then news at least every few weeks until it is settled;
- a fix published in a **new release**, mentioned in the CHANGELOG; for a serious problem, a GitHub security advisory too;
- you are credited in the release notes, unless you prefer to stay anonymous.

Please keep the details private until a fixed version is available, so that users have the time to update.

## What is covered

Image Print Layout works on your computer, on your own documents, and does not use the network. It does, however, **open files that may come from anywhere**: SVG documents, images and project files. A report is relevant, for example, if:

- an SVG, an image or a project file makes the program crash in an exploitable way, run code, or read or write files it should not — for example an SVG whose links put files from elsewhere on the computer into the exported PDF or HTML page;
- an exported HTML page runs code coming from the document it was made from;
- a `.deb`, `.rpm`, Windows installer or portable package, its installation script, the build scripts or the release workflow can be abused;
- a file of a release does not match its published `.sha256` fingerprint.

## What is not covered

- A vulnerability in a **library developed by someone else** (GTK, librsvg, gdk-pixbuf, cairo, fontconfig…): please report it to that project. On Linux these libraries come from your distribution, and updating them is enough. On **Windows they are shipped inside the package**, so do tell me as well: the fix needs a new release.
- A vulnerability in the **Go language or in a Go module** built into the program: please report it to that project, and tell me as well, since the fix needs a new release.
- Problems that need a computer which is already compromised, or physical access to it.
- Ordinary bugs without security consequences: please open an [issue](https://github.com/gillesmagneville/image-print-layout/issues) instead.

## Checking what you download

Download the packages only from the [Releases page](https://github.com/gillesmagneville/image-print-layout/releases) of this repository. Each package is published with its own `.sha256` file. To check a download, put the package and its `.sha256` file in the same folder:

```sh
sha256sum -c <package>.sha256
```

On Windows, compare the result of `Get-FileHash <package>` (PowerShell) with the content of the `.sha256` file. The Windows installer is not code-signed (see [`windows/README.md`](windows/README.md)), so this check is all the more useful.

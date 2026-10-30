<!-- SPDX-License-Identifier: GPL-3.0-only -->

# PROJECT_CONTEXT.md

🇫🇷 [Version française](PROJECT_CONTEXT.fr.md)

# Image Print Layout

**Image Print Layout** is a desktop application, written in **Go**, that prepares a
document of any dimensions, SVG vector drawing or image (JPEG, PNG, TIFF, WebP,
BMP, GIF), for printing on several sheets of standard paper: you choose the
paper and the width of the document, place the breaks between pages on a preview,
then export **a single multi-page PDF** (vector for an SVG).

It is aimed at anyone who has to print a large drawing (a diagram, for example)
without knowing SVG coordinates, programming or geometry.

> **Author:** Image Print Layout is developed by Gilles MAGNEVILLE, and designed with the help of claude.ai.

> **Note on this document:** PROJECT_CONTEXT.md says *why* the project exists and *what it
> values*. It repeats neither the formulas, nor the interfaces, nor the file formats:
> these live once, canonically, in `ARCHITECTURE.md`, so that the
> two documents cannot silently diverge. The detailed requirements are those of the
> specification (version 1.0).

---

# Vision

Printing a large drawing on several sheets always raises the same question:

> **"Where to cut, at what width, and will the result be exactly what I see?"**

Image Print Layout answers this question with an interactive preview: the whole drawing, the
limits of each page, cut lines that you move with the mouse, and for each
page its dimensions, its scale and its status. The exported PDF is the one the preview showed.

The number of pages follows from the dimensions of the drawing, the paper size, the chosen
width and the positions of the cuts. It is never imposed.

---

# Project principles

Image Print Layout follows five principles.

## 1. Never a silent distortion

The drawing has **a single scale**, the same for all pages. Proportions are
kept, unless the user explicitly asks otherwise. Adding or moving a cut, or
changing an alignment, never changes the width of the drawing.

When a portion is too tall for a sheet, the software does **not** shrink the drawing
to make it fit: it says so, and offers options.

Bad:

```
Page 3 was reduced to 95% to fit on the sheet.
```

Good:

```
Page 3 exceeds the usable height (277 mm) by 14 mm. Nothing was changed.

You can: add a cut, move the neighboring cut,
reduce the scale to 95% (width 190 mm), or choose a larger format.
```

None of these proposals is applied without a click.

---

## 2. The source file is never modified

The original file (SVG or image) is never opened for writing. Any transformation (replacing or
removing the background, for example) is applied to a working copy, and the project keeps
only the instruction, replayed on each load.

---

## 3. The preview is the PDF

The preview is the central function of the application. Preview and export draw from the
same page plan: what the user sees are the cuts, the scale and the
positions that the PDF will reproduce.

What cannot be faithfully reproduced (rasterized effect, missing font, unsupported
SVG feature) is **reported before the export**, never silently reconstructed.

---

## 4. The user stays in control

Automatic cuts, corrections and detected settings (such as the background
rectangle) are **proposals**. The user can always move them, refuse them or
undo them.

Everyday operations are done in the main window, without chaining dialog
boxes. Each error message says what failed, why, and what can be done.

---

## 5. Deterministic and offline

With identical parameters, the result is identical (given equal library versions and
fonts: the exact limits are in `ARCHITECTURE.md`). The application uses no online
service and sends no data.

---

# Scope

For version 1.0, Image Print Layout covers:

* import of SVG 1.1 and of Inkscape-compatible SVG;
* since version 1.2, import of raster images (JPEG, PNG, TIFF, WebP, BMP, GIF), which are
  cut and printed like an SVG;
* the formats A0 to A5, Letter, Legal, Tabloid and a custom format, in portrait or
  landscape, with four independent margins;
* three scale modes: fit to width, to height, or manual scale;
* the interactive preview, manual cuts and the proposal of automatic cuts;
* the computation of the pages and of their status (compliant or overflow);
* background handling: detection, replacement by white, removal, keeping;
* the positioning of the drawing on each sheet;
* export to a single multi-page PDF, with checks before and after;
* saving and reopening a layout project.

In version 1.0, Image Print Layout **is not**:

* a drawing editor: the only change to the content is the background setting;
* a tiled printing tool with overlap, nor does it provide alignment marks;
* a numbering or footer tool;
* a converter: the input is an SVG or an image, never a PDF, and the program does not
  touch up the pixels (no cropping, resampling or color correction);
* a tool that takes into account the non-printable margins specific to a printer.

These functions are listed in §14 of the specification as possible later additions.

---

# Target platforms

* Linux, notably Ubuntu;
* Windows 10 and Windows 11.

Language: Go. Main output format: vector multi-page PDF.

The Ubuntu versions to support are not yet fixed: they depend on the
version of librsvg provided (see "Open questions" in `ARCHITECTURE.md`).

---

# Success criterion

The software is considered functional when the 14 acceptance tests of the
specification pass: from the Mini-CAD diagram (1,280 × 5,479 SVG units) imported on A4 portrait at
200 mm width to the reopening of the project.

> The test passes if the final PDF is faithful to the preview and if no page undergoes
> silent resizing.

The correspondence between these 14 tests and their automatic verification is in
`ARCHITECTURE.md` ("Test strategy").

---

# Code principles

* Go; the standard library first, a dependency only when it is justified.
* The computation logic (model, units, layout, project) depends neither on the interface nor
  on cgo: it is tested everywhere, without a graphics library.
* A single scale in the model: no function accepts two separate factors for the
  width and the height.
* No timestamp, no random identifier, no order depending on a hash table
  in what the application writes (PDF, project).

---

# Possible future developments

After version 1.0 (specification, §14):

* tiled printing with overlap for assembly;
* registration lines and alignment marks;
* numbering and footer;
* automatic page rotation;
* PDF import (raster images have been read since version 1.2);
* advanced detection of the logical groups of the SVG;
* preview of the non-printable margins specific to the printer.

None is needed for version 1.0 to be useful.

---

# Values

Image Print Layout values:

* fidelity to the preview
* clarity
* user control
* reproducibility
* transparency about what is not supported
* portability

Above all:

**No page undergoes silent resizing.**

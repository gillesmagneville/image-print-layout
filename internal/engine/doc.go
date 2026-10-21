// SPDX-License-Identifier: GPL-3.0-only

// Package engine isolates the rendering back end behind a small interface
// (Document): load an SVG or a picture (JPEG, PNG, TIFF, WebP, BMP, GIF),
// measure block boxes, render a preview window, export the page plan to PDF.
//
// The implementation targets librsvg and gdk-pixbuf + cairo through cgo. Keeping it
// behind an interface lets the back end be swapped (for instance for a
// sidecar process) if the M0 prototype shows that it cannot meet the
// fidelity or determinism requirements.
package engine

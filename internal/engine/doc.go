// SPDX-License-Identifier: GPL-3.0-only

// Package engine isolates the rendering back end behind two small
// interfaces (Engine and Doc): load an SVG, measure block boxes, render a
// preview tile, export the page plan to PDF.
//
// The first implementation targets librsvg + cairo through cgo. Keeping it
// behind an interface lets the back end be swapped (for instance for a
// sidecar process) if the M0 prototype shows that it cannot meet the
// fidelity or determinism requirements.
package engine

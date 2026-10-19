// SPDX-License-Identifier: GPL-3.0-only

// Package layout is the geometric core: given the document size, the paper
// settings, the global scale and the cut positions (stored in document
// units), it computes the page plan — bands, physical sizes, positions on the
// sheet, conformity states — and proposes automatic cuts.
//
// The drawing is never cut geometrically: each page is the same drawing
// translated and clipped to its band. A single plan feeds both the on-screen
// preview and the PDF export so that the two cannot diverge.
//
// Pure Go, no cgo, fully unit-tested.
package layout

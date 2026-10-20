// SPDX-License-Identifier: GPL-3.0-only

// Package acceptance holds the acceptance tests of the specification
// (ARCHITECTURE §13), run on the whole chain: the session, the rendering
// engine, the export and the files it writes. It has no code of its own. The
// tests that need the rendering engine are built with cgo; the tests that read
// a raster need poppler (pdftoppm, pdfinfo).
package acceptance

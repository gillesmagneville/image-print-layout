// SPDX-License-Identifier: GPL-3.0-only

package engine

import (
	"errors"
	"image"
	"image/color"
	"io"
)

// Dpi is the resolution librsvg uses to turn physical SVG lengths (mm, in,
// pt...) into user units. SVG and CSS say 96; librsvg's own default is not
// guaranteed to be the same, so it is always set explicitly.
const Dpi = 96.0

// Box is a rectangle in document units (SVG user units, origin top left).
type Box struct {
	X, Y, W, H float64
}

// PageSpec places one band of the document on one sheet. Sizes are in PDF
// points; the band is in document units.
type PageSpec struct {
	WidthPt, HeightPt    float64 // the sheet
	BandTop, BandBottom  float64 // the part of the document drawn on it
	OriginXPt, OriginYPt float64 // where the band's top-left corner lands
	PtPerUnit            float64 // the single global scale
}

// PDFOptions are the export settings.
type PDFOptions struct {
	// WhitePage paints an opaque white rectangle the size of each sheet
	// before the drawing.
	WhitePage bool
	// FixedDate is written as the creation and modification date, so that two
	// exports of the same input are identical. ISO 8601.
	FixedDate string
}

// DefaultFixedDate is the date written in the PDF metadata.
const DefaultFixedDate = "2000-01-01T00:00:00Z"

// Document is a loaded SVG document, as the rest of the program sees it. A
// document is not safe for use by several goroutines at once, and it must be
// closed.
type Document interface {
	// Size is the size of the document in user units, as the engine resolves
	// it at Dpi.
	Size() (w, h float64)
	// Measure is the ink box of the element with the given id, in document
	// units, as it sits in the drawing.
	Measure(id string) (Box, error)
	// MeasureBlocks measures many elements. An element that cannot be
	// measured (nothing to draw, unusable identifier) is missing from the
	// result.
	MeasureBlocks(ids []string) map[string]Box
	// Render draws the window win, in document units, at pxPerUnit pixels per
	// unit. A nil background leaves the pixels transparent.
	Render(win Box, pxPerUnit float64, background color.Color) (*image.RGBA, error)
	// ExportPDF writes one PDF page per PageSpec.
	ExportPDF(out io.Writer, pages []PageSpec, opt PDFOptions) error
	// Close releases the document; it is safe to call more than once.
	Close()
}

// Loader reads a document. path is where the file sits (it may be empty): the
// images the document links to are looked for next to it.
type Loader func(svg []byte, path string) (Document, error)

// FontMatcher tells which installed family the system gives for a requested
// one: the same name when it is installed, another when it is substituted.
type FontMatcher func(family string) string

// ErrNoEngine is the error of a build without the rendering engine.
var ErrNoEngine = errors.New("engine: this build of the program has no rendering engine")

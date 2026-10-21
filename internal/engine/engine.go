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
	// Progress, when set, is called after each page is written, with the
	// number of pages done and the total. Returning false stops the export:
	// ExportPDF then returns ErrCancelled and writes no complete file.
	Progress func(done, total int) bool
}

// ErrCancelled is the error of an export stopped by its Progress function.
var ErrCancelled = errors.New("engine: export cancelled")

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
	// RenderPage draws one sheet as an image, pxPerPt pixels to the point,
	// with the function that writes the PDF: what is previewed is what is
	// exported. A transparent page (whitePage false) is left transparent.
	RenderPage(page PageSpec, pxPerPt float64, whitePage bool) (*image.RGBA, error)
	// ExportPDF writes one PDF page per PageSpec.
	ExportPDF(out io.Writer, pages []PageSpec, opt PDFOptions) error
	// Close releases the document; it is safe to call more than once.
	Close()
}

// Loader reads a document: an SVG or a picture, told apart by the first bytes
// of data, never by the name of the file. path is where the file sits (it may
// be empty): the images an SVG links to are looked for next to it.
type Loader func(data []byte, path string) (Document, error)

// FontMatcher tells which installed family the system gives for a requested
// one: the same name when it is installed, another when it is substituted.
type FontMatcher func(family string) string

// ErrNoEngine is the error of a build without the rendering engine.
var ErrNoEngine = errors.New("engine: this build of the program has no rendering engine")

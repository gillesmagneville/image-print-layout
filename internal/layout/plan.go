// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"math"

	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/model"
	"github.com/gillesmagneville/image-print-layout/internal/units"
)

// State says whether the content of a page fits its sheet.
type State int

const (
	// Conforming: the content is inside the usable area. Export allowed.
	Conforming State = iota
	// MarginOverflow: inside the sheet but over a margin. A warning, and a
	// confirmation at export.
	MarginOverflow
	// SheetOverflow: outside the sheet. Export is blocked.
	SheetOverflow
)

// String names the state for logs, tests and catalog keys ("state.<name>").
func (s State) String() string {
	switch s {
	case Conforming:
		return "conforming"
	case MarginOverflow:
		return "margin_overflow"
	case SheetOverflow:
		return "sheet_overflow"
	}
	return "unknown"
}

// Rect is a rectangle on a sheet, in millimetres from its top left corner.
type Rect struct {
	X, Y, W, H float64
}

// Page is one sheet of the plan.
type Page struct {
	Number int // from 1

	// Top and Bottom are the band of the document drawn on the page, in
	// document units.
	Top, Bottom float64
	// CutAbove and CutBelow are the ids of the cuts that bound the band, 0
	// at an edge of the document.
	CutAbove, CutBelow int

	// ContentW and ContentH are the physical size of the content, in
	// millimetres.
	ContentW, ContentH float64
	// X and Y are the position of the top left corner of the content on the
	// sheet, in millimetres.
	X, Y float64
	// Horizontal and Vertical are the alignments that gave X and Y, after
	// the override of the page.
	Horizontal model.HAlign
	Vertical   model.VAlign

	State State
	// Empty is set when the blocks were measured and none touches the band.
	Empty bool
	Items []diag.Item
}

// Rect is the rectangle the content occupies on the sheet. It is also the
// clip of the page: what the cut materialises.
func (p Page) Rect() Rect { return Rect{p.X, p.Y, p.ContentW, p.ContentH} }

// Units is the height of the band in document units.
func (p Page) Units() float64 { return p.Bottom - p.Top }

// Plan is what the preview shows and what the export draws: the one place
// where the positions of the pages are decided. It is computed again from the
// project each time and never edited.
type Plan struct {
	Doc   Doc
	Sheet Sheet
	// Scale is the single scale, in millimetres per document unit. No page
	// keeps a copy of it.
	Scale   float64
	Percent float64 // of the native size
	// ContentWidthMM is the width of the drawing on every page.
	ContentWidthMM float64
	// TotalHeightMM is the height of the whole drawing, all pages together.
	TotalHeightMM float64
	Pages         []Page
	// Corrections are the proposals when a page overflows. None is applied
	// by itself.
	Corrections []Correction
}

// Exportable is false when a page is outside its sheet.
func (pl *Plan) Exportable() bool {
	for _, p := range pl.Pages {
		if p.State == SheetOverflow {
			return false
		}
	}
	return true
}

// AllConforming is true when every page is inside the usable area.
func (pl *Plan) AllConforming() bool {
	for _, p := range pl.Pages {
		if p.State != Conforming {
			return false
		}
	}
	return true
}

// Compute is the page plan of a project for a document. It is a pure function:
// the same arguments give the same plan, nothing is read or written, and the
// scale is never modified to make a page fit. blocks may be nil when the
// document was not measured; empty pages are then not detected.
func Compute(p model.Project, doc Doc, blocks []Block) (*Plan, error) {
	pl, err := build(p, doc, blocks)
	if err != nil {
		return nil, err
	}
	pl.Corrections = corrections(p, pl, blocks)
	return pl, nil
}

// build computes the plan without the corrections.
func build(p model.Project, doc Doc, blocks []Block) (*Plan, error) {
	sheet, s, err := Resolve(p, doc)
	if err != nil {
		return nil, err
	}
	if err := checkStructure(p.Cuts, doc); err != nil {
		return nil, err
	}

	pl := &Plan{
		Doc:            doc,
		Sheet:          sheet,
		Scale:          s,
		Percent:        s / NativeMMPerUnit * 100,
		ContentWidthMM: doc.W * s,
		TotalHeightMM:  doc.H * s,
	}

	tops := make([]float64, 0, len(p.Cuts)+1)
	ids := make([]int, 0, len(p.Cuts)+1)
	tops = append(tops, 0)
	ids = append(ids, 0)
	for _, c := range p.Cuts {
		tops = append(tops, c.Position)
		ids = append(ids, c.ID)
	}
	for i, top := range tops {
		bottom, below := doc.H, 0
		if i+1 < len(tops) {
			bottom, below = tops[i+1], ids[i+1]
		}
		page := Page{
			Number: i + 1,
			Top:    top, Bottom: bottom,
			CutAbove: ids[i], CutBelow: below,
			ContentW: doc.W * s,
			ContentH: (bottom - top) * s,
		}
		place(&page, p, sheet)
		judge(&page, sheet)
		if len(blocks) > 0 && !touches(blocks, top, bottom) {
			page.Empty = true
			page.Items = append(page.Items, diag.New("page.empty", page.Number))
		}
		pl.Pages = append(pl.Pages, page)
	}
	return pl, nil
}

// effective is the placement of a page: the default one, changed by the
// override of the page.
func effective(p model.Project, number int) model.Placement {
	pl := p.Placement
	if pl.Horizontal == "" {
		pl.Horizontal = model.CenterH
	}
	if pl.Vertical == "" {
		pl.Vertical = model.Top
	}
	if o, ok := p.Override(number); ok {
		if o.Horizontal != model.InheritH {
			pl.Horizontal = o.Horizontal
		}
		if o.Vertical != model.InheritV {
			pl.Vertical = o.Vertical
		}
		if o.OffsetXMM != nil {
			pl.OffsetXMM = *o.OffsetXMM
		}
		if o.OffsetYMM != nil {
			pl.OffsetYMM = *o.OffsetYMM
		}
	}
	return pl
}

// place decides where the content sits on the sheet (ARCHITECTURE §7). The
// formulas read the size of the content, never the scale: changing an
// alignment cannot change the width of the drawing.
func place(page *Page, p model.Project, sheet Sheet) {
	pl := effective(p, page.Number)
	page.Horizontal, page.Vertical = pl.Horizontal, pl.Vertical
	switch pl.Horizontal {
	case model.Left:
		page.X = sheet.UsableLeft
	case model.Right:
		page.X = sheet.WidthMM - sheet.Margins.Right - page.ContentW
	case model.CustomH:
		page.X = sheet.UsableLeft + pl.OffsetXMM
	default: // centred
		page.X = sheet.UsableLeft + (sheet.UsableWidth-page.ContentW)/2
	}
	switch pl.Vertical {
	case model.CenterV:
		page.Y = sheet.UsableTop + (sheet.UsableHeight-page.ContentH)/2
	default: // at the top, below the margin
		page.Y = sheet.UsableTop + pl.OffsetYMM
	}
}

// judge sets the state of a page and its messages.
func judge(page *Page, sheet Sheet) {
	const tol = units.ToleranceMM
	r := page.Rect()
	insideUsable := r.X >= sheet.UsableLeft-tol && r.Y >= sheet.UsableTop-tol &&
		r.X+r.W <= sheet.UsableRight()+tol && r.Y+r.H <= sheet.UsableBottom()+tol
	insideSheet := r.X >= -tol && r.Y >= -tol &&
		r.X+r.W <= sheet.WidthMM+tol && r.Y+r.H <= sheet.HeightMM+tol

	switch {
	case insideUsable:
		page.State = Conforming
	case insideSheet:
		page.State = MarginOverflow
	default:
		page.State = SheetOverflow
	}
	if page.State == Conforming {
		return
	}
	tall := r.H > sheet.UsableHeight+tol
	wide := r.W > sheet.UsableWidth+tol
	if tall {
		page.Items = append(page.Items, diag.New("page.too_tall", page.Number, "h", r.H, "usable", sheet.UsableHeight))
	}
	if wide {
		page.Items = append(page.Items, diag.New("page.too_wide", page.Number, "w", r.W, "usable", sheet.UsableWidth))
	}
	if !tall && !wide {
		page.Items = append(page.Items, diag.New("page.shifted", page.Number))
	}
	if page.State == SheetOverflow {
		page.Items = append(page.Items, diag.New("page.off_sheet", page.Number))
	}
}

// overflowsSize reports whether the content is larger than the usable area,
// whatever its position.
func (p Page) overflowsSize(sheet Sheet) (tall, wide bool) {
	const tol = units.ToleranceMM
	return p.ContentH > sheet.UsableHeight+tol, p.ContentW > sheet.UsableWidth+tol
}

// touches reports whether a block overlaps the band [top, bottom].
func touches(blocks []Block, top, bottom float64) bool {
	for _, b := range blocks {
		if b.Bottom > top+CrossTolerance && b.Top < bottom-CrossTolerance {
			return true
		}
	}
	return false
}

// MM converts a length in document units to millimetres on the sheet.
func (pl *Plan) MM(unitsLength float64) float64 { return unitsLength * pl.Scale }

// ceilPages is the smallest number of pages that can hold the document at
// the scale s, whatever the cuts: the height of the drawing over the usable
// height, rounded up.
func ceilPages(totalMM, usable float64) int {
	n := int(math.Ceil(totalMM/usable - 1e-9))
	return max(n, 1)
}

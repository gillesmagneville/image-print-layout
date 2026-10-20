// SPDX-License-Identifier: GPL-3.0-only

package model

// Orientation is the way a sheet is held.
type Orientation string

const (
	Portrait  Orientation = "portrait"
	Landscape Orientation = "landscape"
)

// Valid reports whether o is one of the two orientations.
func (o Orientation) Valid() bool { return o == Portrait || o == Landscape }

// CustomFormat is the name of the format whose dimensions the user types.
const CustomFormat = "custom"

// Paper is the sheet: a format of the catalog (A4, Letter...) or the custom
// format with its dimensions, in millimetres, in portrait.
type Paper struct {
	Format      string // a name of the catalog, or CustomFormat
	Orientation Orientation
	// CustomWidthMM and CustomHeightMM are read when Format is CustomFormat.
	CustomWidthMM, CustomHeightMM float64
}

// Margins are the four independent margins of a sheet, in millimetres.
type Margins struct {
	Top, Bottom, Left, Right float64
}

// ScaleMode is the way the scale is chosen.
type ScaleMode string

const (
	// FitWidth (mode A) gives the drawing the usable width of the paper.
	FitWidth ScaleMode = "fit-width"
	// FitHeight (mode B) fits a portion of the drawing to the usable height
	// of a sheet.
	FitHeight ScaleMode = "fit-height"
	// Width (mode C) gives the drawing a width in millimetres.
	Width ScaleMode = "width"
	// Factor (mode C) gives the drawing a percentage of its native size.
	Factor ScaleMode = "factor"
)

// Valid reports whether m is one of the four modes.
func (m ScaleMode) Valid() bool {
	switch m {
	case FitWidth, FitHeight, Width, Factor:
		return true
	}
	return false
}

// Scale is the choice of the scale. Only the fields of the mode are read.
//
// The scale itself, in millimetres per document unit, is computed from this
// choice, the paper, the margins and the size of the document, and never from
// the cuts: moving a cut cannot change it.
type Scale struct {
	Mode ScaleMode
	// WidthMM is the width of the drawing, for Width.
	WidthMM float64
	// Percent is the share of the native size, for Factor (100 is the size
	// the document declares).
	Percent float64
	// PortionUnits is the height, in document units, of the portion that
	// FitHeight fits to the usable height of a sheet. Zero is the whole
	// document.
	PortionUnits float64
}

// HAlign is the horizontal position of the drawing on a sheet.
type HAlign string

const (
	Left    HAlign = "left"
	CenterH HAlign = "center"
	Right   HAlign = "right"
	// CustomH is the left margin plus an offset.
	CustomH HAlign = "custom"
	// InheritH, in a page override, keeps the default position.
	InheritH HAlign = ""
)

// Valid reports whether a is one of the four positions.
func (a HAlign) Valid() bool {
	switch a {
	case Left, CenterH, Right, CustomH:
		return true
	}
	return false
}

// VAlign is the vertical position of the drawing on a sheet.
type VAlign string

const (
	// Top is the top margin plus an offset.
	Top     VAlign = "top"
	CenterV VAlign = "center"
	// InheritV, in a page override, keeps the default position.
	InheritV VAlign = ""
)

// Valid reports whether a is one of the two positions.
func (a VAlign) Valid() bool { return a == Top || a == CenterV }

// Placement is the position of the content of a page on its sheet. It never
// reads the scale: changing it cannot change the width of the drawing.
type Placement struct {
	Horizontal HAlign
	Vertical   VAlign
	// OffsetXMM is added to the left margin when Horizontal is CustomH.
	OffsetXMM float64
	// OffsetYMM is added to the top margin when Vertical is Top.
	OffsetYMM float64
}

// PageOverride changes the position of one page. An empty alignment and a
// nil offset keep the default placement.
type PageOverride struct {
	Page       int // from 1
	Horizontal HAlign
	Vertical   VAlign
	OffsetXMM  *float64
	OffsetYMM  *float64
}

// DocBackground is what is done to the background rectangle of the document.
type DocBackground string

const (
	// KeepBackground leaves the document as it is.
	KeepBackground DocBackground = "keep"
	// WhitenBackground fills the background rectangle with opaque white.
	WhitenBackground DocBackground = "whiten"
	// RemoveBackground takes the background rectangle out of the drawing.
	RemoveBackground DocBackground = "remove"
)

// Valid reports whether b is one of the three choices.
func (b DocBackground) Valid() bool {
	return b == KeepBackground || b == WhitenBackground || b == RemoveBackground
}

// PageBackground is what the sheet is painted with before the drawing.
type PageBackground string

const (
	WhitePage       PageBackground = "white"
	TransparentPage PageBackground = "transparent"
)

// Valid reports whether b is one of the two choices.
func (b PageBackground) Valid() bool { return b == WhitePage || b == TransparentPage }

// Background groups the three things that have nothing to do with each other
// and that the program never mixes up: the background of the document (an
// element of the SVG), the background of the page (the sheet), and the colour
// of the work area of the window, which is not a setting.
type Background struct {
	Document DocBackground
	// Element identifies the element Document applies to: its id, or its path
	// of indices (/svg/g[1]/rect[1]) when it has none. Empty when the
	// analysis found no background.
	Element string
	Page    PageBackground
}

// Axis is the direction of a cut. Only the horizontal cuts (positions on the
// vertical axis, "y") exist in this version; the field keeps the format open.
type Axis string

const AxisY Axis = "y"

// Cut is a line along which the drawing is cut into two pages. Its position
// is in document units from the top of the document, unrounded.
type Cut struct {
	// ID identifies the cut while the project is open, whatever its position
	// becomes. It is not saved: the cuts are numbered again at each opening.
	ID       int
	Axis     Axis
	Position float64
}

// Source is the SVG document the project is about. The document is never
// stored in the project, only its place and a fingerprint.
type Source struct {
	// Path is the path as the project file writes it, relative to the project
	// file and with "/" as separator when that was possible.
	Path string
	// AbsolutePath is where the document was when the project was saved.
	AbsolutePath string
	// SHA256 is the fingerprint of the file, in hexadecimal.
	SHA256 string
	// Width and Height are the size of the document in units.
	Width, Height float64
}

// Export is what the export remembers.
type Export struct {
	OutputPath string
}

// Project is everything the user chose. It holds settings and no result:
// the number of pages, their sizes and their states are computed again from
// it each time.
//
// A Project is a value: a command copies what it changes (Clone) and returns
// a new one, which is what makes undo possible.
type Project struct {
	Source        Source
	Paper         Paper
	Margins       Margins
	Scale         Scale
	Placement     Placement
	Background    Background
	Cuts          []Cut // in increasing position
	PageOverrides []PageOverride
	Export        Export
	// NextCutID is the identifier the next cut will get.
	NextCutID int
}

// Default is the project of a document just opened: A4 portrait, margins of
// 10 mm above and below and 5 mm at the sides, the drawing at the usable
// width of the paper, centred horizontally and at the top of the sheet, the
// document background kept and the sheet opaque white.
func Default() Project {
	return Project{
		Paper:      Paper{Format: "A4", Orientation: Portrait},
		Margins:    Margins{Top: 10, Bottom: 10, Left: 5, Right: 5},
		Scale:      Scale{Mode: FitWidth},
		Placement:  Placement{Horizontal: CenterH, Vertical: Top},
		Background: Background{Document: KeepBackground, Page: WhitePage},
		NextCutID:  1,
	}
}

// Clone returns a copy that shares no slice with p.
func (p Project) Clone() Project {
	q := p
	q.Cuts = append([]Cut(nil), p.Cuts...)
	q.PageOverrides = make([]PageOverride, len(p.PageOverrides))
	for i, o := range p.PageOverrides {
		o.OffsetXMM = cloneFloat(o.OffsetXMM)
		o.OffsetYMM = cloneFloat(o.OffsetYMM)
		q.PageOverrides[i] = o
	}
	if len(p.PageOverrides) == 0 {
		q.PageOverrides = nil
	}
	if len(p.Cuts) == 0 {
		q.Cuts = nil
	}
	return q
}

func cloneFloat(f *float64) *float64 {
	if f == nil {
		return nil
	}
	v := *f
	return &v
}

// Override returns the override of a page, if the project has one.
func (p Project) Override(page int) (PageOverride, bool) {
	for _, o := range p.PageOverrides {
		if o.Page == page {
			return o, true
		}
	}
	return PageOverride{}, false
}

// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"math"

	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/model"
	"github.com/gillesmagneville/image-print-layout/internal/units"
)

// Doc is the size of the document in document units: the pixels of its
// declared size at 96 dpi, which is also what the rendering engine draws in
// (ARCHITECTURE §5).
type Doc struct {
	W, H float64
}

func (d Doc) check() error {
	if !(d.W > 0) || !(d.H > 0) || math.IsInf(d.W, 0) || math.IsInf(d.H, 0) {
		return diag.Fail("doc.invalid_size", "w", d.W, "h", d.H)
	}
	return nil
}

// NativeMMPerUnit is the size of a document unit when the document is
// printed at 100 %.
const NativeMMPerUnit = units.MMPerInch / units.CSSPixelsPerInch

// Sheet is a sheet of paper held in an orientation, with its margins and the
// usable area they leave. Every length is in millimetres, from the top left
// corner of the sheet.
type Sheet struct {
	Format      string
	Orientation model.Orientation
	WidthMM     float64
	HeightMM    float64
	Margins     model.Margins

	UsableLeft, UsableTop     float64
	UsableWidth, UsableHeight float64
}

// UsableRight is the position of the right edge of the usable area.
func (s Sheet) UsableRight() float64 { return s.UsableLeft + s.UsableWidth }

// UsableBottom is the position of the bottom edge of the usable area.
func (s Sheet) UsableBottom() float64 { return s.UsableTop + s.UsableHeight }

// NewSheet resolves the format and the orientation of the paper and removes
// the margins from it.
func NewSheet(paper model.Paper, m model.Margins) (Sheet, error) {
	var p units.Paper
	if paper.Format == model.CustomFormat {
		c, err := units.Custom(paper.CustomWidthMM, paper.CustomHeightMM)
		if err != nil {
			return Sheet{}, diag.Fail("paper.custom_invalid", "w", paper.CustomWidthMM, "h", paper.CustomHeightMM)
		}
		p = c
	} else {
		found, ok := units.Lookup(paper.Format)
		if !ok {
			return Sheet{}, diag.Fail("paper.unknown", "format", paper.Format)
		}
		p = found
	}
	orientation := paper.Orientation
	if orientation == "" {
		orientation = model.Portrait
	}
	if !orientation.Valid() {
		return Sheet{}, diag.Fail("paper.orientation", "orientation", string(orientation))
	}
	w, h := p.Oriented(orientation == model.Landscape)

	for _, v := range []float64{m.Top, m.Bottom, m.Left, m.Right} {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return Sheet{}, diag.Fail("margins.invalid")
		}
	}
	uw, uh := w-m.Left-m.Right, h-m.Top-m.Bottom
	if uw <= 0 || uh <= 0 {
		return Sheet{}, diag.Fail("margins.no_usable", "w", uw, "h", uh)
	}
	return Sheet{
		Format: paper.Format, Orientation: orientation,
		WidthMM: w, HeightMM: h, Margins: m,
		UsableLeft: m.Left, UsableTop: m.Top, UsableWidth: uw, UsableHeight: uh,
	}, nil
}

// ScaleOf computes the scale, in millimetres per document unit, from the
// choice of the user, the sheet and the size of the document. It is the only
// place where a scale is computed, and it reads no cut: adding, moving or
// deleting a cut cannot change it. The same scale is applied to both axes, so
// the proportions of the drawing cannot change.
func ScaleOf(sc model.Scale, sheet Sheet, doc Doc) (float64, error) {
	if err := doc.check(); err != nil {
		return 0, err
	}
	var s float64
	switch sc.Mode {
	case model.FitWidth:
		s = sheet.UsableWidth / doc.W
	case model.FitHeight:
		portion := sc.PortionUnits
		if portion <= 0 {
			portion = doc.H
		}
		s = sheet.UsableHeight / portion
	case model.Width:
		if !(sc.WidthMM > 0) || math.IsInf(sc.WidthMM, 0) {
			return 0, diag.Fail("scale.invalid_width")
		}
		s = sc.WidthMM / doc.W
	case model.Factor:
		if !(sc.Percent > 0) || math.IsInf(sc.Percent, 0) {
			return 0, diag.Fail("scale.invalid_percent")
		}
		s = sc.Percent / 100 * NativeMMPerUnit
	default:
		return 0, diag.Fail("scale.invalid")
	}
	if !(s > 0) || math.IsInf(s, 0) {
		return 0, diag.Fail("scale.invalid")
	}
	return s, nil
}

// Resolve gives the sheet and the scale of a project for a document.
func Resolve(p model.Project, doc Doc) (Sheet, float64, error) {
	sheet, err := NewSheet(p.Paper, p.Margins)
	if err != nil {
		return Sheet{}, 0, err
	}
	s, err := ScaleOf(p.Scale, sheet, doc)
	if err != nil {
		return Sheet{}, 0, err
	}
	return sheet, s, nil
}

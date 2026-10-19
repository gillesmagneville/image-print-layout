// SPDX-License-Identifier: GPL-3.0-only

package units

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Paper is a sheet format. Dimensions are in millimetres.
type Paper struct {
	Name     string
	WidthMM  float64
	HeightMM float64
}

// catalog lists the formats required by the specification (section 3), in
// portrait orientation.
var catalog = []Paper{
	{"A0", 841, 1189},
	{"A1", 594, 841},
	{"A2", 420, 594},
	{"A3", 297, 420},
	{"A4", 210, 297},
	{"A5", 148, 210},
	{"Letter", 215.9, 279.4},
	{"Legal", 215.9, 355.6},
	{"Tabloid", 279.4, 431.8},
}

// Catalog returns the standard formats, in display order. The returned slice
// is a copy.
func Catalog() []Paper {
	out := make([]Paper, len(catalog))
	copy(out, catalog)
	return out
}

// Lookup returns the standard format with the given name (case-insensitive).
func Lookup(name string) (Paper, bool) {
	for _, p := range catalog {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Paper{}, false
}

// Custom builds a user-defined format from dimensions in millimetres.
func Custom(widthMM, heightMM float64) (Paper, error) {
	for _, v := range []float64{widthMM, heightMM} {
		if !(v > 0) || math.IsInf(v, 0) {
			return Paper{}, fmt.Errorf("units: custom paper dimensions must be positive, got %v x %v mm", widthMM, heightMM)
		}
	}
	return Paper{Name: "Custom", WidthMM: widthMM, HeightMM: heightMM}, nil
}

// Oriented returns the width and height of the sheet in the requested
// orientation: portrait always yields the short side as width, landscape the
// long side, whatever order the dimensions were declared in.
func (p Paper) Oriented(landscape bool) (widthMM, heightMM float64) {
	short, long := math.Min(p.WidthMM, p.HeightMM), math.Max(p.WidthMM, p.HeightMM)
	if landscape {
		return long, short
	}
	return short, long
}

// Margins are the four independent page margins, in millimetres.
type Margins struct {
	Top, Bottom, Left, Right float64
}

// ErrNoUsableArea is returned when the margins leave no printable area.
var ErrNoUsableArea = errors.New("units: margins leave no usable area on the sheet")

// UsableArea returns the width and height of the printable area of a sheet
// of the given size once the margins are removed.
func UsableArea(sheetWidthMM, sheetHeightMM float64, m Margins) (widthMM, heightMM float64, err error) {
	for _, v := range []float64{m.Top, m.Bottom, m.Left, m.Right} {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, 0, fmt.Errorf("units: margins must be zero or positive, got %+v", m)
		}
	}
	widthMM = sheetWidthMM - m.Left - m.Right
	heightMM = sheetHeightMM - m.Top - m.Bottom
	if widthMM <= 0 || heightMM <= 0 {
		return 0, 0, ErrNoUsableArea
	}
	return widthMM, heightMM, nil
}

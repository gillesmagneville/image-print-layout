// SPDX-License-Identifier: GPL-3.0-only

// Package units converts between SVG lengths, CSS pixels, points and
// millimetres, and holds the catalogue of standard paper sizes.
//
// It is pure Go (no cgo, no I/O) so it can be tested on every platform.
//
// Conventions used throughout the project:
//
//   - 1 SVG user unit = 1 CSS pixel = 1/96 inch (SVG 1.1 / CSS 3).
//   - 1 inch = 25.4 mm = 72 PDF points.
//   - The layout scale is expressed in millimetres per document unit.
package units

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	MMPerInch        = 25.4
	PointsPerInch    = 72.0
	CSSPixelsPerInch = 96.0

	// ToleranceMM is the tolerance (in millimetres) under which two
	// physical positions are considered identical by the verification
	// checks.
	ToleranceMM = 0.005
)

// MMToPt converts millimetres to PDF points.
func MMToPt(mm float64) float64 { return mm * PointsPerInch / MMPerInch }

// PtToMM converts PDF points to millimetres.
func PtToMM(pt float64) float64 { return pt * MMPerInch / PointsPerInch }

// MMToPx converts millimetres to CSS pixels (SVG user units at 96 dpi).
func MMToPx(mm float64) float64 { return mm * CSSPixelsPerInch / MMPerInch }

// PxToMM converts CSS pixels (SVG user units at 96 dpi) to millimetres.
func PxToMM(px float64) float64 { return px * MMPerInch / CSSPixelsPerInch }

// ApproxEqualMM reports whether a and b (in millimetres) differ by no more
// than ToleranceMM.
func ApproxEqualMM(a, b float64) bool { return math.Abs(a-b) <= ToleranceMM }

// MMPerUnit returns the scale s, in millimetres per document unit, that makes
// a document docWidthUnits wide measure exactly targetMM on paper. The same s
// is applied to both axes: proportions are never altered.
func MMPerUnit(targetMM, docWidthUnits float64) (float64, error) {
	if !(targetMM > 0) || math.IsInf(targetMM, 0) {
		return 0, fmt.Errorf("units: target width must be a positive number of millimetres, got %v", targetMM)
	}
	if !(docWidthUnits > 0) || math.IsInf(docWidthUnits, 0) {
		return 0, fmt.Errorf("units: document width must be positive, got %v", docWidthUnits)
	}
	return targetMM / docWidthUnits, nil
}

// Unit is the unit suffix of an SVG length.
type Unit int

const (
	UnitNone    Unit = iota // no suffix: user units
	UnitPx                  // px
	UnitPt                  // pt
	UnitPc                  // pc
	UnitMm                  // mm
	UnitCm                  // cm
	UnitIn                  // in
	UnitPercent             // %
	UnitEm                  // em
	UnitEx                  // ex
)

var unitSuffix = map[string]Unit{
	"":   UnitNone,
	"px": UnitPx,
	"pt": UnitPt,
	"pc": UnitPc,
	"mm": UnitMm,
	"cm": UnitCm,
	"in": UnitIn,
	"%":  UnitPercent,
	"em": UnitEm,
	"ex": UnitEx,
}

// ErrRelativeUnit is returned when a length is relative (%, em, ex) and
// therefore cannot be converted without more context (viewport, font size).
// Callers should report the feature to the user rather than guess.
var ErrRelativeUnit = errors.New("units: relative length (%, em, ex) needs a context to be resolved")

// Length is a parsed SVG length such as "210mm" or "1280".
type Length struct {
	Value float64
	Unit  Unit
}

// ParseLength parses an SVG length attribute value. Surrounding whitespace is
// ignored; whitespace between the number and the unit is not allowed, as in
// SVG.
func ParseLength(s string) (Length, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Length{}, errors.New("units: empty length")
	}
	end := scanNumber(s)
	if end == 0 {
		return Length{}, fmt.Errorf("units: invalid length %q", s)
	}
	v, err := strconv.ParseFloat(s[:end], 64)
	if err != nil {
		return Length{}, fmt.Errorf("units: invalid number in length %q: %w", s, err)
	}
	u, ok := unitSuffix[s[end:]]
	if !ok {
		return Length{}, fmt.Errorf("units: unknown unit %q in length %q", s[end:], s)
	}
	return Length{Value: v, Unit: u}, nil
}

// scanNumber returns the length of the leading SVG number in s, or 0 if there
// is none. The exponent is only consumed when followed by digits, so "1em" is
// the number 1 followed by the unit "em".
func scanNumber(s string) int {
	i, digits := 0, 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	for i < len(s) && isDigit(s[i]) {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isDigit(s[i]) {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		k := j
		for k < len(s) && isDigit(s[k]) {
			k++
		}
		if k > j {
			i = k
		}
	}
	return i
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// ToPx converts the length to CSS pixels (= SVG user units at the default
// scale). It returns ErrRelativeUnit for %, em and ex.
func (l Length) ToPx() (float64, error) {
	switch l.Unit {
	case UnitNone, UnitPx:
		return l.Value, nil
	case UnitPt:
		return l.Value * CSSPixelsPerInch / PointsPerInch, nil
	case UnitPc:
		return l.Value * 16, nil // 1 pc = 12 pt = 16 px
	case UnitMm:
		return MMToPx(l.Value), nil
	case UnitCm:
		return MMToPx(l.Value * 10), nil
	case UnitIn:
		return l.Value * CSSPixelsPerInch, nil
	default:
		return 0, ErrRelativeUnit
	}
}

// ToMM converts the length to millimetres. It returns ErrRelativeUnit for %,
// em and ex.
func (l Length) ToMM() (float64, error) {
	px, err := l.ToPx()
	if err != nil {
		return 0, err
	}
	return PxToMM(px), nil
}

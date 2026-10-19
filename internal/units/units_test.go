// SPDX-License-Identifier: GPL-3.0-only

package units

import (
	"errors"
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestMMConversions(t *testing.T) {
	if got := MMToPt(25.4); !near(got, 72) {
		t.Errorf("MMToPt(25.4) = %v, want 72", got)
	}
	if got := PtToMM(72); !near(got, 25.4) {
		t.Errorf("PtToMM(72) = %v, want 25.4", got)
	}
	if got := MMToPx(25.4); !near(got, 96) {
		t.Errorf("MMToPx(25.4) = %v, want 96", got)
	}
	if got := PxToMM(96); !near(got, 25.4) {
		t.Errorf("PxToMM(96) = %v, want 25.4", got)
	}
	// A4 width in PDF points: 595.2756 (the usual rounded value is 595).
	if got := MMToPt(210); math.Abs(got-595.2756) > 1e-3 {
		t.Errorf("MMToPt(210) = %v, want ~595.2756", got)
	}
	// Round trip.
	for _, mm := range []float64{0, 0.005, 10, 200, 1189} {
		if got := PtToMM(MMToPt(mm)); !near(got, mm) {
			t.Errorf("PtToMM(MMToPt(%v)) = %v", mm, got)
		}
		if got := PxToMM(MMToPx(mm)); !near(got, mm) {
			t.Errorf("PxToMM(MMToPx(%v)) = %v", mm, got)
		}
	}
}

func TestApproxEqualMM(t *testing.T) {
	// Values on the exact boundary are avoided on purpose: 10.005-10 is not
	// exactly 0.005 in binary floating point.
	if !ApproxEqualMM(10, 10.004) {
		t.Error("10 and 10.004 should be equal within tolerance")
	}
	if !ApproxEqualMM(10.004, 10) {
		t.Error("ApproxEqualMM should be symmetric")
	}
	if ApproxEqualMM(10, 10.006) {
		t.Error("10 and 10.006 should differ")
	}
}

func TestParseLength(t *testing.T) {
	tests := []struct {
		in     string
		want   Length
		wantMM float64
	}{
		{"100", Length{100, UnitNone}, PxToMM(100)},
		{"100px", Length{100, UnitPx}, PxToMM(100)},
		{"  1280 ", Length{1280, UnitNone}, PxToMM(1280)},
		{"10mm", Length{10, UnitMm}, 10},
		{"2.54cm", Length{2.54, UnitCm}, 25.4},
		{"1in", Length{1, UnitIn}, 25.4},
		{"72pt", Length{72, UnitPt}, 25.4},
		{"6pc", Length{6, UnitPc}, 25.4},
		{".5mm", Length{0.5, UnitMm}, 0.5},
		{"-3", Length{-3, UnitNone}, PxToMM(-3)},
		{"+3.5e1mm", Length{35, UnitMm}, 35},
		{"1E2", Length{100, UnitNone}, PxToMM(100)},
	}
	for _, tc := range tests {
		got, err := ParseLength(tc.in)
		if err != nil {
			t.Errorf("ParseLength(%q) error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseLength(%q) = %+v, want %+v", tc.in, got, tc.want)
			continue
		}
		mm, err := got.ToMM()
		if err != nil || !near(mm, tc.wantMM) {
			t.Errorf("ParseLength(%q).ToMM() = %v, %v; want %v", tc.in, mm, err, tc.wantMM)
		}
	}
}

func TestParseLengthRelativeUnits(t *testing.T) {
	for _, in := range []string{"50%", "1em", "2.5ex", "1e2em"} {
		l, err := ParseLength(in)
		if err != nil {
			t.Errorf("ParseLength(%q) error: %v", in, err)
			continue
		}
		if _, err := l.ToPx(); !errors.Is(err, ErrRelativeUnit) {
			t.Errorf("%q.ToPx() error = %v, want ErrRelativeUnit", in, err)
		}
		if _, err := l.ToMM(); !errors.Is(err, ErrRelativeUnit) {
			t.Errorf("%q.ToMM() error = %v, want ErrRelativeUnit", in, err)
		}
	}
}

func TestParseLengthErrors(t *testing.T) {
	for _, in := range []string{"", "   ", "abc", "mm", "10 mm", "10furlong", "1e", "--3", "."} {
		if l, err := ParseLength(in); err == nil {
			t.Errorf("ParseLength(%q) = %+v, want an error", in, l)
		}
	}
}

func TestMMPerUnit(t *testing.T) {
	s, err := MMPerUnit(200, 1280)
	if err != nil || !near(s, 0.15625) {
		t.Fatalf("MMPerUnit(200, 1280) = %v, %v; want 0.15625", s, err)
	}
	for _, c := range [][2]float64{{0, 1280}, {-1, 1280}, {200, 0}, {200, -5}, {math.NaN(), 1}, {1, math.Inf(1)}} {
		if _, err := MMPerUnit(c[0], c[1]); err == nil {
			t.Errorf("MMPerUnit(%v, %v) should fail", c[0], c[1])
		}
	}
}

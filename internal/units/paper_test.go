package units

import (
	"math"
	"testing"
)

func TestCatalogContainsRequiredFormats(t *testing.T) {
	for _, name := range []string{"A0", "A1", "A2", "A3", "A4", "A5", "Letter", "Legal", "Tabloid"} {
		if _, ok := Lookup(name); !ok {
			t.Errorf("format %s missing from the catalogue", name)
		}
	}
	if len(Catalog()) != 9 {
		t.Errorf("catalogue has %d formats, want 9", len(Catalog()))
	}
}

func TestCatalogIsACopy(t *testing.T) {
	c := Catalog()
	c[0].WidthMM = 1
	if p, _ := Lookup("A0"); p.WidthMM != 841 {
		t.Error("mutating the returned catalogue changed the package state")
	}
}

func TestLookupIsCaseInsensitive(t *testing.T) {
	p, ok := Lookup("a4")
	if !ok || p.Name != "A4" {
		t.Errorf("Lookup(\"a4\") = %+v, %v", p, ok)
	}
	if _, ok := Lookup("B5"); ok {
		t.Error("Lookup(\"B5\") should not be found")
	}
}

func TestISOSeriesHalving(t *testing.T) {
	// Each A size is the previous one folded in half: long side becomes the
	// short side of the next format.
	names := []string{"A0", "A1", "A2", "A3", "A4", "A5"}
	for i := 1; i < len(names); i++ {
		prev, _ := Lookup(names[i-1])
		cur, _ := Lookup(names[i])
		pw, ph := prev.Oriented(false)
		cw, ch := cur.Oriented(false)
		if math.Abs(ch-pw) > 1 { // nominal sizes are rounded to the millimetre
			t.Errorf("%s height %v does not match %s width %v", names[i], ch, names[i-1], pw)
		}
		if cw >= pw || ch >= ph {
			t.Errorf("%s must be smaller than %s", names[i], names[i-1])
		}
	}
}

func TestOriented(t *testing.T) {
	a4, _ := Lookup("A4")
	if w, h := a4.Oriented(false); w != 210 || h != 297 {
		t.Errorf("A4 portrait = %v x %v", w, h)
	}
	if w, h := a4.Oriented(true); w != 297 || h != 210 {
		t.Errorf("A4 landscape = %v x %v", w, h)
	}
	// A custom paper declared wide still gives the short side as portrait width.
	c, err := Custom(500, 300)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := c.Oriented(false); w != 300 || h != 500 {
		t.Errorf("custom portrait = %v x %v", w, h)
	}
	if w, h := c.Oriented(true); w != 500 || h != 300 {
		t.Errorf("custom landscape = %v x %v", w, h)
	}
}

func TestCustomValidation(t *testing.T) {
	for _, c := range [][2]float64{{0, 100}, {100, 0}, {-1, 100}, {100, math.NaN()}, {math.Inf(1), 100}} {
		if _, err := Custom(c[0], c[1]); err == nil {
			t.Errorf("Custom(%v, %v) should fail", c[0], c[1])
		}
	}
}

func TestUsableArea(t *testing.T) {
	a4, _ := Lookup("A4")
	w, h := a4.Oriented(false)
	uw, uh, err := UsableArea(w, h, Margins{Top: 10, Bottom: 10, Left: 5, Right: 5})
	if err != nil || uw != 200 || uh != 277 {
		t.Errorf("UsableArea = %v x %v, %v; want 200 x 277", uw, uh, err)
	}
	if _, _, err := UsableArea(w, h, Margins{Left: 105, Right: 105}); err != ErrNoUsableArea {
		t.Errorf("zero-width area: err = %v, want ErrNoUsableArea", err)
	}
	if _, _, err := UsableArea(w, h, Margins{Top: -1}); err == nil {
		t.Error("negative margin should be rejected")
	}
}

// TestMiniCADAcceptanceNumbers pins the figures of acceptance tests 1-4 of the
// specification: a 1280 x 5479 unit document on A4 portrait at 200 mm width.
func TestMiniCADAcceptanceNumbers(t *testing.T) {
	const docW, docH = 1280.0, 5479.0

	a4, _ := Lookup("A4")
	sw, sh := a4.Oriented(false)
	uw, uh, err := UsableArea(sw, sh, Margins{Top: 10, Bottom: 10, Left: 5, Right: 5})
	if err != nil {
		t.Fatal(err)
	}

	s, err := MMPerUnit(200, docW)
	if err != nil {
		t.Fatal(err)
	}
	if 200 > uw {
		t.Fatalf("200 mm does not fit the usable width %v mm", uw)
	}

	totalHeight := docH * s // proportions preserved: same s on both axes
	if !near(totalHeight, 856.09375) {
		t.Errorf("scaled height = %v mm, want 856.09375", totalHeight)
	}
	if pages := int(math.Ceil(totalHeight / uh)); pages != 4 {
		t.Errorf("minimum page count = %d, want 4 (usable height %v mm)", pages, uh)
	}
}

// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"math"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/model"
)

// On the Mini-CAD equivalent, the proposal makes the fewest pages the usable
// height allows and crosses no block, since voids exist (criterion of M3).
func TestAutoCutsOnTheMiniCAD(t *testing.T) {
	p := acceptance()
	blocks := miniCADBlocks()
	prop, err := AutoCuts(p, miniCAD, blocks)
	if err != nil {
		t.Fatal(err)
	}
	if prop.Pages != 4 || len(prop.Positions) != 3 {
		t.Fatalf("%d pages, cuts %v; want 4 pages", prop.Pages, prop.Positions)
	}
	if prop.Crossings != 0 {
		t.Errorf("%d blocks crossed", prop.Crossings)
	}
	for _, y := range prop.Positions {
		if got := Crossed(y, blocks); len(got) != 0 {
			t.Errorf("the cut at %v crosses %v", y, got)
		}
	}

	// Applied, the cuts make a plan whose pages all fit.
	q, err := SetCuts(p, miniCAD, prop.Positions)
	if err != nil {
		t.Fatal(err)
	}
	pl := mustPlan(t, q, miniCAD, blocks)
	if !pl.AllConforming() || len(pl.Pages) != 4 {
		for _, pg := range pl.Pages {
			t.Logf("page %d: %v %v h=%v", pg.Number, pg.State, pg.Items, pg.ContentH)
		}
		t.Fatal("the proposal does not give four conforming pages")
	}
	// The pages are balanced: none is far from the ideal height.
	ideal := miniCAD.H / 4
	for _, pg := range pl.Pages {
		if math.Abs(pg.Units()-ideal) > 0.35*ideal {
			t.Errorf("page %d is %v units tall; the ideal is %v", pg.Number, pg.Units(), ideal)
		}
	}
	// The user keeps control: each cut can then be moved.
	if _, err := MoveCut(q, miniCAD, q.Cuts[0].ID, q.Cuts[0].Position+10); err != nil {
		t.Errorf("moving a proposed cut: %v", err)
	}
}

func TestAutoCutsAreDeterministic(t *testing.T) {
	p := acceptance()
	a, err := AutoCuts(p, miniCAD, miniCADBlocks())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := AutoCuts(p, miniCAD, miniCADBlocks())
	if !equalFloats(a.Positions, b.Positions) {
		t.Errorf("two runs differ: %v and %v", a.Positions, b.Positions)
	}
}

func TestAutoCutsWithoutBlocksAreEven(t *testing.T) {
	prop, err := AutoCuts(acceptance(), miniCAD, nil)
	if err != nil {
		t.Fatal(err)
	}
	if prop.Pages != 4 {
		t.Fatalf("%d pages, want 4", prop.Pages)
	}
	for i, y := range prop.Positions {
		if want := float64(i+1) * miniCAD.H / 4; !near(y, want, 1e-6) {
			t.Errorf("cut %d at %v, want %v", i+1, y, want)
		}
	}
}

func TestAutoCutsNeedNoCutWhenItFits(t *testing.T) {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 100}
	prop, err := AutoCuts(p, Doc{W: 1000, H: 2000}, nil) // 200 mm tall: one page
	if err != nil {
		t.Fatal(err)
	}
	if len(prop.Positions) != 0 || prop.Pages != 1 || !hasCode(prop.Items, "auto.no_cut_needed") {
		t.Errorf("proposal = %+v", prop)
	}
}

// The number of pages follows the drawing, the paper and the width: it is
// never imposed.
func TestAutoCutsPageCountFollowsTheDrawing(t *testing.T) {
	for _, tt := range []struct {
		height float64 // units, at 200 mm wide for 1000 units: s = 0.2
		pages  int
	}{
		{1000, 1}, {1385, 1}, {1386, 2}, {2770, 2}, {2771, 3}, {6925, 5}, {6926, 6}, {13000, 10},
	} {
		p := model.Default()
		p.Scale = model.Scale{Mode: model.Width, WidthMM: 200}
		prop, err := AutoCuts(p, Doc{W: 1000, H: tt.height}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if prop.Pages != tt.pages {
			t.Errorf("height %v: %d pages, want %d", tt.height, prop.Pages, tt.pages)
		}
	}
}

// With a block that is taller than a page, a cut must cross it: the proposal
// is made anyway, with a warning, and it still fits.
func TestAutoCutsWarnWhenABlockMustBeCrossed(t *testing.T) {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 200} // s = 0.2; a page holds 1385 units
	doc := Doc{W: 1000, H: 3000}
	blocks := []Block{{ID: "tout", Top: 100, Bottom: 2900}}
	prop, err := AutoCuts(p, doc, blocks)
	if err != nil {
		t.Fatal(err)
	}
	if prop.Crossings == 0 || !hasCode(prop.Items, "auto.cut_crosses") {
		t.Errorf("no warning for a block that cannot be avoided: %+v", prop)
	}
	q, err := SetCuts(p, doc, prop.Positions)
	if err != nil {
		t.Fatal(err)
	}
	if pl := mustPlan(t, q, doc, blocks); !pl.AllConforming() {
		t.Error("the pages of the proposal do not fit")
	}
}

// A cut closer than 2 mm to a block is flagged.
func TestAutoCutsFlagCutsNearABlock(t *testing.T) {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 200} // s = 0.2, 2 mm = 10 units
	doc := Doc{W: 1000, H: 2000}                           // 400 mm: two pages
	// The only void is 6 units (1.2 mm) wide, around the ideal position.
	blocks := []Block{{ID: "a", Top: 0, Bottom: 997}, {ID: "b", Top: 1003, Bottom: 2000}}
	prop, err := AutoCuts(p, doc, blocks)
	if err != nil {
		t.Fatal(err)
	}
	if prop.Crossings != 0 || !near(prop.Positions[0], 1000, 1) {
		t.Fatalf("proposal = %+v", prop)
	}
	if !hasCode(prop.Items, "auto.cut_near") {
		t.Errorf("no warning for a cut 1.2 mm from the blocks: %+v", prop.Items)
	}
}

// Up to two more pages than the minimum are tried when that avoids crossing.
func TestAutoCutsMayUseMorePagesToAvoidCrossing(t *testing.T) {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 200} // a page holds 1385 units
	doc := Doc{W: 1000, H: 2700}                           // minimum: 2 pages
	// A block from 600 to 1900 hides every position of a 2-page split (the
	// cut must lie between 1315 and 1385); a 3-page split can use the voids
	// above and below it.
	blocks := []Block{{ID: "big", Top: 600, Bottom: 1900}}
	prop, err := AutoCuts(p, doc, blocks)
	if err != nil {
		t.Fatal(err)
	}
	if prop.Pages != 3 || prop.Crossings != 0 || !hasCode(prop.Items, "auto.more_pages") {
		t.Errorf("proposal = %+v", prop)
	}
}

func TestAutoCutsErrors(t *testing.T) {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width}
	if _, err := AutoCuts(p, miniCAD, nil); errCode(err) != "scale.invalid_width" {
		t.Errorf("error = %v", err)
	}
}

// A large document with many blocks is solved quickly.
func TestAutoCutsOnManyBlocks(t *testing.T) {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 200}
	doc := Doc{W: 1000, H: 200000} // 40 000 mm: about 145 pages
	var blocks []Block
	for y := 10.0; y+30 < doc.H; y += 45 {
		blocks = append(blocks, Block{ID: "b", Top: y, Bottom: y + 30})
	}
	prop, err := AutoCuts(p, doc, blocks)
	if err != nil {
		t.Fatal(err)
	}
	if prop.Crossings != 0 || prop.Pages < 145 {
		t.Errorf("%d pages, %d crossings", prop.Pages, prop.Crossings)
	}
}

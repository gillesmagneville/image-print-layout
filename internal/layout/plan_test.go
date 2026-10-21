// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/model"
	"github.com/gillesmagneville/image-print-layout/internal/units"
)

// Acceptance tests 2, 3 and 4, at the level of the logic.
func TestAcceptanceA4PortraitAt200mm(t *testing.T) {
	p := acceptance()
	pl := mustPlan(t, p, miniCAD, nil)

	// 2. A4 portrait: 200 x 277 mm usable with the default margins.
	if pl.Sheet.WidthMM != 210 || pl.Sheet.HeightMM != 297 {
		t.Errorf("sheet = %v x %v mm, want 210 x 297", pl.Sheet.WidthMM, pl.Sheet.HeightMM)
	}
	if pl.Sheet.UsableWidth != 200 || pl.Sheet.UsableHeight != 277 {
		t.Errorf("usable area = %v x %v mm, want 200 x 277", pl.Sheet.UsableWidth, pl.Sheet.UsableHeight)
	}
	// 3. A width of 200 mm: s = 200 / 1280 = 0.15625 mm per unit.
	if !near(pl.Scale, 0.15625, 1e-12) || !near(pl.ContentWidthMM, 200, units.ToleranceMM) {
		t.Errorf("scale = %v, width = %v mm, want 0.15625 and 200", pl.Scale, pl.ContentWidthMM)
	}
	if !near(pl.TotalHeightMM, 856.09375, 1e-6) {
		t.Errorf("total height = %v mm, want 856.09375", pl.TotalHeightMM)
	}
	// 4. Proportions kept: one scale for both axes.
	if !near(pl.ContentWidthMM/pl.TotalHeightMM, miniCAD.W/miniCAD.H, 1e-12) {
		t.Error("the drawing does not keep the proportions of the document")
	}
	// No cut yet: a single page that is too tall, which is flagged, not shrunk.
	if len(pl.Pages) != 1 || pl.Pages[0].State != MarginOverflow && pl.Pages[0].State != SheetOverflow {
		t.Fatalf("pages = %+v", pl.Pages)
	}
	if !hasCode(pl.Pages[0].Items, "page.too_tall") {
		t.Errorf("no page.too_tall in %v", pl.Pages[0].Items)
	}
	if !near(pl.Scale, 0.15625, 1e-12) {
		t.Error("the scale changed to make the page fit")
	}
}

func TestScaleModes(t *testing.T) {
	sheet, err := NewSheet(model.Paper{Format: "A4", Orientation: model.Portrait}, model.Margins{Top: 10, Bottom: 10, Left: 5, Right: 5})
	if err != nil {
		t.Fatal(err)
	}
	doc := Doc{W: 1000, H: 2000}
	tests := []struct {
		name string
		sc   model.Scale
		want float64
	}{
		{"A: fit width", model.Scale{Mode: model.FitWidth}, 200.0 / 1000},
		{"B: fit height, whole document", model.Scale{Mode: model.FitHeight}, 277.0 / 2000},
		{"B: fit height, a portion", model.Scale{Mode: model.FitHeight, PortionUnits: 500}, 277.0 / 500},
		{"C: width 190 mm", model.Scale{Mode: model.Width, WidthMM: 190}, 0.19},
		{"C: 100 %", model.Scale{Mode: model.Factor, Percent: 100}, 25.4 / 96},
		{"C: 125 %", model.Scale{Mode: model.Factor, Percent: 125}, 1.25 * 25.4 / 96},
	}
	for _, tt := range tests {
		got, err := ScaleOf(tt.sc, sheet, doc)
		if err != nil || !near(got, tt.want, 1e-12) {
			t.Errorf("%s: s = %v, %v; want %v", tt.name, got, err, tt.want)
		}
	}

	for name, sc := range map[string]model.Scale{
		"width zero":     {Mode: model.Width},
		"width negative": {Mode: model.Width, WidthMM: -3},
		"percent zero":   {Mode: model.Factor},
		"unknown mode":   {Mode: "stretch"},
		"empty mode":     {},
	} {
		if _, err := ScaleOf(sc, sheet, doc); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := ScaleOf(model.Scale{Mode: model.FitWidth}, sheet, Doc{W: 0, H: 10}); errCode(err) != "doc.invalid_size" {
		t.Errorf("empty document: %v", err)
	}
}

// Mode B depends on the sheet, not on the cuts: a cut cannot change the scale.
func TestFitHeightDoesNotReadTheCuts(t *testing.T) {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.FitHeight}
	doc := Doc{W: 1000, H: 2000}
	before := mustPlan(t, p, doc, nil).Scale
	q, _ := mustAdd(t, p, doc, 700)
	if after := mustPlan(t, q, doc, nil).Scale; after != before {
		t.Errorf("s went from %v to %v when a cut was added", before, after)
	}
}

func TestPaperCatalogAndOrientation(t *testing.T) {
	m := model.Margins{Top: 10, Bottom: 10, Left: 5, Right: 5}
	for _, tt := range []struct {
		format string
		o      model.Orientation
		w, h   float64
	}{
		{"A0", model.Portrait, 841, 1189}, {"A1", model.Portrait, 594, 841},
		{"A2", model.Portrait, 420, 594}, {"A3", model.Portrait, 297, 420},
		{"A4", model.Portrait, 210, 297}, {"A5", model.Portrait, 148, 210},
		{"Letter", model.Portrait, 215.9, 279.4}, {"Legal", model.Portrait, 215.9, 355.6},
		{"Tabloid", model.Portrait, 279.4, 431.8},
		{"A4", model.Landscape, 297, 210}, {"Letter", model.Landscape, 279.4, 215.9},
		{"A4", "", 210, 297},
	} {
		s, err := NewSheet(model.Paper{Format: tt.format, Orientation: tt.o}, m)
		if err != nil || s.WidthMM != tt.w || s.HeightMM != tt.h {
			t.Errorf("%s %s: %v x %v mm, %v", tt.format, tt.o, s.WidthMM, s.HeightMM, err)
		}
		if !near(s.UsableWidth, tt.w-10, 1e-9) || !near(s.UsableHeight, tt.h-20, 1e-9) {
			t.Errorf("%s %s: usable %v x %v", tt.format, tt.o, s.UsableWidth, s.UsableHeight)
		}
	}
	s, err := NewSheet(model.Paper{Format: model.CustomFormat, Orientation: model.Landscape, CustomWidthMM: 100, CustomHeightMM: 150}, m)
	if err != nil || s.WidthMM != 150 || s.HeightMM != 100 {
		t.Errorf("custom landscape: %+v, %v", s, err)
	}
}

func TestPaperAndMarginErrors(t *testing.T) {
	ok := model.Margins{Top: 10, Bottom: 10, Left: 5, Right: 5}
	for name, tt := range map[string]struct {
		paper model.Paper
		m     model.Margins
		code  string
	}{
		"unknown format":      {model.Paper{Format: "B7"}, ok, "paper.unknown"},
		"bad orientation":     {model.Paper{Format: "A4", Orientation: "diagonal"}, ok, "paper.orientation"},
		"custom without size": {model.Paper{Format: model.CustomFormat}, ok, "paper.custom_invalid"},
		"negative margin":     {model.Paper{Format: "A4"}, model.Margins{Top: -1}, "margins.invalid"},
		"margins too large":   {model.Paper{Format: "A5"}, model.Margins{Left: 80, Right: 80}, "margins.no_usable"},
	} {
		if _, err := NewSheet(tt.paper, tt.m); errCode(err) != tt.code {
			t.Errorf("%s: error %v, want code %s", name, err, tt.code)
		}
	}
}

// The four margins are independent: each moves the usable area on its own.
func TestMarginsAreIndependent(t *testing.T) {
	s, err := NewSheet(model.Paper{Format: "A4"}, model.Margins{Top: 12, Bottom: 8, Left: 20, Right: 3})
	if err != nil {
		t.Fatal(err)
	}
	if s.UsableLeft != 20 || s.UsableTop != 12 || s.UsableWidth != 187 || s.UsableHeight != 277 {
		t.Errorf("usable = %+v", s)
	}
	if s.UsableRight() != 207 || s.UsableBottom() != 289 {
		t.Errorf("right/bottom = %v/%v", s.UsableRight(), s.UsableBottom())
	}
}

// The bands cover the document with no gap and no overlap, and every page has
// the same width.
func TestBandsCoverTheDocument(t *testing.T) {
	p := acceptance()
	var ids []int
	for _, y := range []float64{1380.5, 2760, 4120.25} {
		var id int
		p, id = mustAdd(t, p, miniCAD, y)
		ids = append(ids, id)
	}
	pl := mustPlan(t, p, miniCAD, nil)
	if len(pl.Pages) != 4 {
		t.Fatalf("%d pages, want 4", len(pl.Pages))
	}
	prev := 0.0
	for i, pg := range pl.Pages {
		if pg.Number != i+1 || pg.Top != prev {
			t.Errorf("page %d: number %d, top %v, want top %v", i+1, pg.Number, pg.Top, prev)
		}
		prev = pg.Bottom
		if !near(pg.ContentW, 200, units.ToleranceMM) {
			t.Errorf("page %d: width %v mm", pg.Number, pg.ContentW)
		}
		if !near(pg.ContentH, pg.Units()*pl.Scale, 1e-9) {
			t.Errorf("page %d: height %v mm for %v units", pg.Number, pg.ContentH, pg.Units())
		}
	}
	if prev != miniCAD.H {
		t.Errorf("the last band ends at %v, want %v", prev, miniCAD.H)
	}
	if pl.Pages[0].CutAbove != 0 || pl.Pages[0].CutBelow != ids[0] || pl.Pages[3].CutBelow != 0 || pl.Pages[3].CutAbove != ids[2] {
		t.Errorf("cut ids on the pages: %+v", pl.Pages)
	}
}

// Acceptance test 8: every page 10 mm from the top of the sheet.
func TestAllPagesAtTenMillimetresFromTheTop(t *testing.T) {
	p := acceptance()
	for _, y := range []float64{1380.5, 2760, 4120.25} {
		p, _ = mustAdd(t, p, miniCAD, y)
	}
	pl := mustPlan(t, p, miniCAD, nil)
	for _, pg := range pl.Pages {
		if pg.Y != 10 {
			t.Errorf("page %d at y = %v mm, want 10", pg.Number, pg.Y)
		}
		if pg.State != Conforming {
			t.Errorf("page %d: %v %v", pg.Number, pg.State, pg.Items)
		}
	}
	if !pl.AllConforming() || !pl.Exportable() || len(pl.Corrections) != 0 {
		t.Errorf("plan: conforming %v, exportable %v, corrections %v", pl.AllConforming(), pl.Exportable(), pl.Corrections)
	}
}

func TestHorizontalPlacement(t *testing.T) {
	doc := Doc{W: 1000, H: 1000}
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 150}
	p.Margins = model.Margins{Top: 10, Bottom: 10, Left: 20, Right: 5}
	for _, tt := range []struct {
		align model.HAlign
		off   float64
		x     float64
	}{
		{model.Left, 0, 20},
		{model.CenterH, 0, 20 + (185-150)/2.0},
		{model.Right, 0, 210 - 5 - 150},
		{model.CustomH, 7.5, 27.5},
	} {
		p.Placement = model.Placement{Horizontal: tt.align, Vertical: model.Top, OffsetXMM: tt.off}
		pl := mustPlan(t, p, doc, nil)
		if got := pl.Pages[0].X; !near(got, tt.x, 1e-9) {
			t.Errorf("%s: x = %v, want %v", tt.align, got, tt.x)
		}
	}
}

func TestVerticalPlacement(t *testing.T) {
	doc := Doc{W: 1000, H: 1000}
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 100} // 100 mm tall
	p.Placement = model.Placement{Horizontal: model.CenterH, Vertical: model.Top, OffsetYMM: 5}
	if y := mustPlan(t, p, doc, nil).Pages[0].Y; y != 15 {
		t.Errorf("top with an offset of 5 mm: y = %v, want 15", y)
	}
	p.Placement.Vertical = model.CenterV
	if y := mustPlan(t, p, doc, nil).Pages[0].Y; !near(y, 10+(277-100)/2.0, 1e-9) {
		t.Errorf("centred: y = %v, want %v", y, 10+(277-100)/2.0)
	}
}

// The placement reads the size of the content and never the scale: changing
// an alignment cannot change the width of the drawing (§8 of the
// specification).
func TestPlacementNeverChangesTheWidth(t *testing.T) {
	p := acceptance()
	p, _ = mustAdd(t, p, miniCAD, 2000)
	want := mustPlan(t, p, miniCAD, nil)
	for _, h := range []model.HAlign{model.Left, model.CenterH, model.Right, model.CustomH} {
		for _, v := range []model.VAlign{model.Top, model.CenterV} {
			q := p.Clone()
			q.Placement = model.Placement{Horizontal: h, Vertical: v, OffsetXMM: 3, OffsetYMM: 4}
			got := mustPlan(t, q, miniCAD, nil)
			if got.Scale != want.Scale || got.ContentWidthMM != want.ContentWidthMM {
				t.Errorf("%s/%s: scale %v width %v, want %v %v", h, v, got.Scale, got.ContentWidthMM, want.Scale, want.ContentWidthMM)
			}
			for i := range got.Pages {
				if got.Pages[i].ContentW != want.Pages[i].ContentW || got.Pages[i].ContentH != want.Pages[i].ContentH {
					t.Errorf("%s/%s: page %d changed size", h, v, i+1)
				}
			}
		}
	}
}

func TestPageOverrides(t *testing.T) {
	p := acceptance()
	p, _ = mustAdd(t, p, miniCAD, 1500)
	p, _ = mustAdd(t, p, miniCAD, 3000)
	offset := 4.0
	p.PageOverrides = []model.PageOverride{
		{Page: 2, Vertical: model.CenterV},
		{Page: 3, Horizontal: model.Left, OffsetYMM: &offset},
	}
	pl := mustPlan(t, p, miniCAD, nil)
	if pl.Pages[0].Vertical != model.Top || pl.Pages[0].Y != 10 {
		t.Errorf("page 1 should keep the default: %+v", pl.Pages[0])
	}
	if pl.Pages[1].Vertical != model.CenterV || !near(pl.Pages[1].Y, 10+(277-pl.Pages[1].ContentH)/2, 1e-9) {
		t.Errorf("page 2 should be centred: %+v", pl.Pages[1])
	}
	if pl.Pages[2].Horizontal != model.Left || pl.Pages[2].X != 5 || pl.Pages[2].Y != 14 {
		t.Errorf("page 3 should be at the left, 4 mm lower: %+v", pl.Pages[2])
	}
	for _, pg := range pl.Pages {
		if pg.ContentW != pl.ContentWidthMM {
			t.Errorf("page %d: width %v differs from %v", pg.Number, pg.ContentW, pl.ContentWidthMM)
		}
	}
}

func TestStates(t *testing.T) {
	doc := Doc{W: 1000, H: 1000}
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 100}
	p.Placement = model.Placement{Horizontal: model.CenterH, Vertical: model.Top}
	check := func(name string, q model.Project, want State, code string) {
		t.Helper()
		pg := mustPlan(t, q, doc, nil).Pages[0]
		if pg.State != want {
			t.Errorf("%s: state %v, want %v (%v)", name, pg.State, want, pg.Items)
		}
		if code != "" && !hasCode(pg.Items, code) {
			t.Errorf("%s: no %s in %v", name, code, pg.Items)
		}
		if code == "" && len(pg.Items) != 0 {
			t.Errorf("%s: unexpected items %v", name, pg.Items)
		}
	}
	check("fits", p, Conforming, "")

	// Exactly the usable width: never refused for a rounding error.
	exact := p.Clone()
	exact.Scale = model.Scale{Mode: model.Width, WidthMM: 200}
	exact.Placement.OffsetYMM = 0
	exact.Scale.WidthMM = 200.004
	check("within the tolerance", exact, Conforming, "")

	// Over the bottom margin but on the sheet.
	low := p.Clone()
	low.Placement.OffsetYMM = 195 // 10 + 195 + 100 = 305 > 297: off the sheet
	check("off the sheet", low, SheetOverflow, "page.shifted")
	low.Placement.OffsetYMM = 185 // 10 + 185 + 100 = 295: on the sheet, over the margin
	check("over the margin", low, MarginOverflow, "page.shifted")

	wide := p.Clone()
	wide.Scale = model.Scale{Mode: model.Width, WidthMM: 205}
	check("wider than the usable width", wide, MarginOverflow, "page.too_wide")
	wide.Scale = model.Scale{Mode: model.Width, WidthMM: 230}
	check("wider than the sheet", wide, SheetOverflow, "page.too_wide")

	// 280 mm tall: over the bottom margin, on the sheet.
	tallDoc := Doc{W: 1000, H: 2800}
	tallPlan := mustPlan(t, p, tallDoc, nil)
	if pg := tallPlan.Pages[0]; pg.State != MarginOverflow || !hasCode(pg.Items, "page.too_tall") || hasCode(pg.Items, "page.too_wide") {
		t.Errorf("taller than the usable height: %v %v", pg.State, pg.Items)
	}
	tallDoc.H = 3200 // 320 mm tall: off the sheet
	if pg := mustPlan(t, p, tallDoc, nil).Pages[0]; pg.State != SheetOverflow || !hasCode(pg.Items, "page.off_sheet") {
		t.Errorf("taller than the sheet: %v %v", pg.State, pg.Items)
	}
}

func TestEmptyPage(t *testing.T) {
	doc := Doc{W: 1000, H: 1000}
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 100}
	p, _ = mustAdd(t, p, doc, 500)
	blocks := []Block{{ID: "a", Top: 10, Bottom: 200}}
	pl := mustPlan(t, p, doc, blocks)
	if pl.Pages[0].Empty || !pl.Pages[1].Empty || !hasCode(pl.Pages[1].Items, "page.empty") {
		t.Errorf("pages: %+v", pl.Pages)
	}
	// Without measured blocks, nothing is said.
	for _, pg := range mustPlan(t, p, doc, nil).Pages {
		if pg.Empty {
			t.Errorf("page %d is flagged empty without any measure", pg.Number)
		}
	}
}

func TestComputeRefusesBrokenCuts(t *testing.T) {
	doc := Doc{W: 1000, H: 1000}
	for name, cuts := range map[string][]model.Cut{
		"at the top":    {{ID: 1, Axis: model.AxisY, Position: 0}},
		"at the bottom": {{ID: 1, Axis: model.AxisY, Position: 1000}},
		"outside":       {{ID: 1, Axis: model.AxisY, Position: 1200}},
		"unordered":     {{ID: 1, Axis: model.AxisY, Position: 600}, {ID: 2, Axis: model.AxisY, Position: 300}},
		"twice":         {{ID: 1, Axis: model.AxisY, Position: 300}, {ID: 2, Axis: model.AxisY, Position: 300}},
		"unknown axis":  {{ID: 1, Axis: "z", Position: 300}},
	} {
		p := model.Default()
		p.Cuts = cuts
		if _, err := Compute(p, doc, nil); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// A cut closer than the minimal gap, left by a change of scale, does not make
// the project unopenable.
func TestComputeToleratesCloseCuts(t *testing.T) {
	doc := Doc{W: 1000, H: 1000}
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 100}
	p.Cuts = []model.Cut{{ID: 1, Axis: model.AxisY, Position: 500}, {ID: 2, Axis: model.AxisY, Position: 500.01}}
	if _, err := Compute(p, doc, nil); err != nil {
		t.Errorf("Compute: %v", err)
	}
}

// Compute is a pure function: the same arguments give the same plan, and the
// project is not modified.
func TestComputeIsPure(t *testing.T) {
	p := acceptance()
	p, _ = mustAdd(t, p, miniCAD, 1500)
	snapshot := p.Clone()
	a := mustPlan(t, p, miniCAD, miniCADBlocks())
	b := mustPlan(t, p, miniCAD, miniCADBlocks())
	if a.Scale != b.Scale || len(a.Pages) != len(b.Pages) {
		t.Fatal("two plans differ")
	}
	for i := range a.Pages {
		if a.Pages[i].X != b.Pages[i].X || a.Pages[i].Y != b.Pages[i].Y || a.Pages[i].State != b.Pages[i].State {
			t.Errorf("page %d differs", i+1)
		}
	}
	if len(p.Cuts) != len(snapshot.Cuts) || p.Cuts[0] != snapshot.Cuts[0] {
		t.Error("Compute modified the project")
	}
}

// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"math"
	"math/rand"
	"testing"

	"github.com/gillesmagneville/svg-print-layout/internal/model"
	"github.com/gillesmagneville/svg-print-layout/internal/units"
)

// The property tests draw papers, margins, scales, documents and cuts at
// random, with a fixed seed so that a failure can be replayed. Their
// invariants are those of the specification: the bands cover the document
// exactly, the width is the same on every page, and no command changes the
// scale (acceptance tests 6, 7 and 12 at the level of the logic).

const iterations = 400

func randomProject(r *rand.Rand) (model.Project, Doc) {
	p := model.Default()
	catalog := units.Catalog()
	p.Paper = model.Paper{Format: catalog[r.Intn(len(catalog))].Name, Orientation: model.Portrait}
	if r.Intn(2) == 0 {
		p.Paper.Orientation = model.Landscape
	}
	if r.Intn(8) == 0 {
		p.Paper = model.Paper{Format: model.CustomFormat, Orientation: p.Paper.Orientation,
			CustomWidthMM: 80 + r.Float64()*400, CustomHeightMM: 100 + r.Float64()*600}
	}
	p.Margins = model.Margins{Top: r.Float64() * 30, Bottom: r.Float64() * 30, Left: r.Float64() * 30, Right: r.Float64() * 30}
	doc := Doc{W: 50 + r.Float64()*4000, H: 50 + r.Float64()*12000}
	switch r.Intn(4) {
	case 0:
		p.Scale = model.Scale{Mode: model.FitWidth}
	case 1:
		p.Scale = model.Scale{Mode: model.FitHeight, PortionUnits: r.Float64() * doc.H}
	case 2:
		p.Scale = model.Scale{Mode: model.Width, WidthMM: 20 + r.Float64()*400}
	default:
		p.Scale = model.Scale{Mode: model.Factor, Percent: 10 + r.Float64()*400}
	}
	hs := []model.HAlign{model.Left, model.CenterH, model.Right, model.CustomH}
	vs := []model.VAlign{model.Top, model.CenterV}
	p.Placement = model.Placement{Horizontal: hs[r.Intn(4)], Vertical: vs[r.Intn(2)],
		OffsetXMM: r.Float64()*20 - 10, OffsetYMM: r.Float64()*20 - 10}
	return p, doc
}

// withRandomCuts adds up to n cuts at random places, ignoring the refusals.
func withRandomCuts(r *rand.Rand, p model.Project, doc Doc, n int) model.Project {
	for i := 0; i < n; i++ {
		if q, _, err := AddCut(p, doc, r.Float64()*doc.H); err == nil {
			p = q
		}
	}
	return p
}

func checkPlan(t *testing.T, label string, p model.Project, doc Doc, pl *Plan) {
	t.Helper()
	// The bands cover the document exactly, with no gap and no overlap.
	prev := 0.0
	sumH := 0.0
	for i, pg := range pl.Pages {
		if pg.Number != i+1 {
			t.Fatalf("%s: page %d is numbered %d", label, i+1, pg.Number)
		}
		if pg.Top != prev || pg.Bottom <= pg.Top {
			t.Fatalf("%s: page %d band [%v, %v] after %v", label, pg.Number, pg.Top, pg.Bottom, prev)
		}
		prev = pg.Bottom
		sumH += pg.ContentH
		// Every page has the width of the drawing: one scale for all.
		if pg.ContentW != pl.ContentWidthMM || pg.ContentW != doc.W*pl.Scale {
			t.Fatalf("%s: page %d is %v mm wide, the drawing %v", label, pg.Number, pg.ContentW, pl.ContentWidthMM)
		}
		if pg.ContentH != (pg.Bottom-pg.Top)*pl.Scale {
			t.Fatalf("%s: page %d height %v does not follow the scale", label, pg.Number, pg.ContentH)
		}
	}
	if prev != doc.H {
		t.Fatalf("%s: the bands end at %v, the document at %v", label, prev, doc.H)
	}
	if !near(sumH, pl.TotalHeightMM, 1e-6*math.Max(1, pl.TotalHeightMM)) {
		t.Fatalf("%s: the pages add up to %v mm, the drawing is %v mm", label, sumH, pl.TotalHeightMM)
	}
	if len(pl.Pages) != len(p.Cuts)+1 {
		t.Fatalf("%s: %d cuts give %d pages", label, len(p.Cuts), len(pl.Pages))
	}
	// A state is the position of the rectangle against the sheet.
	for _, pg := range pl.Pages {
		r := pg.Rect()
		const tol = units.ToleranceMM
		inUsable := r.X >= pl.Sheet.UsableLeft-tol && r.Y >= pl.Sheet.UsableTop-tol &&
			r.X+r.W <= pl.Sheet.UsableRight()+tol && r.Y+r.H <= pl.Sheet.UsableBottom()+tol
		inSheet := r.X >= -tol && r.Y >= -tol && r.X+r.W <= pl.Sheet.WidthMM+tol && r.Y+r.H <= pl.Sheet.HeightMM+tol
		want := SheetOverflow
		if inUsable {
			want = Conforming
		} else if inSheet {
			want = MarginOverflow
		}
		if pg.State != want {
			t.Fatalf("%s: page %d is %v, its rectangle %+v says %v", label, pg.Number, pg.State, r, want)
		}
	}
}

func TestPropertyPlansAreConsistent(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < iterations; i++ {
		p, doc := randomProject(r)
		p = withRandomCuts(r, p, doc, r.Intn(12))
		pl, err := Compute(p, doc, nil)
		if err != nil {
			// Margins may leave no usable area; nothing else is allowed to fail.
			if code := errCode(err); code != "margins.no_usable" {
				t.Fatalf("iteration %d: Compute: %v", i, err)
			}
			continue
		}
		checkPlan(t, "plan", p, doc, pl)
	}
}

// Acceptance tests 6 and 7 for every project: adding, moving or deleting a cut
// leaves the scale, and so the width of the drawing, exactly as it was.
func TestPropertyCutCommandsNeverChangeTheScale(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	done := 0
	for i := 0; i < iterations; i++ {
		p, doc := randomProject(r)
		sheet, s0, err := Resolve(p, doc)
		if err != nil {
			continue
		}
		_ = sheet
		scaleOf := func(q model.Project) float64 {
			_, s, err := Resolve(q, doc)
			if err != nil {
				t.Fatalf("Resolve after a command: %v", err)
			}
			return s
		}
		for step := 0; step < 15; step++ {
			switch r.Intn(3) {
			case 0:
				if q, _, err := AddCut(p, doc, r.Float64()*doc.H); err == nil {
					p = q
					done++
				}
			case 1:
				if len(p.Cuts) > 0 {
					c := p.Cuts[r.Intn(len(p.Cuts))]
					lo, hi, err := CutBounds(p, doc, c.ID)
					if err != nil {
						t.Fatalf("CutBounds: %v", err)
					}
					if hi < lo {
						continue
					}
					y := lo + r.Float64()*(hi-lo)
					q, err := MoveCut(p, doc, c.ID, y)
					if err != nil {
						t.Fatalf("MoveCut inside the bounds [%v, %v] at %v: %v", lo, hi, y, err)
					}
					p = q
					done++
				}
			default:
				if len(p.Cuts) > 0 {
					q, err := DeleteCut(p, p.Cuts[r.Intn(len(p.Cuts))].ID)
					if err != nil {
						t.Fatal(err)
					}
					p = q
					done++
				}
			}
			if got := scaleOf(p); got != s0 {
				t.Fatalf("iteration %d step %d: the scale went from %v to %v", i, step, s0, got)
			}
			pl, err := Compute(p, doc, nil)
			if err != nil {
				t.Fatalf("a command produced a project that has no plan: %v", err)
			}
			checkPlan(t, "after a command", p, doc, pl)
			if pl.Scale != s0 || pl.ContentWidthMM != doc.W*s0 {
				t.Fatalf("the plan has another scale (%v) than before the command (%v)", pl.Scale, s0)
			}
		}
	}
	if done < iterations {
		t.Errorf("only %d commands ran: the test is not exercising anything", done)
	}
}

// Changing the placement changes neither the scale nor the size of a page.
func TestPropertyPlacementNeverChangesTheDrawing(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for i := 0; i < iterations; i++ {
		p, doc := randomProject(r)
		p = withRandomCuts(r, p, doc, r.Intn(6))
		a, err := Compute(p, doc, nil)
		if err != nil {
			continue
		}
		q := p.Clone()
		hs := []model.HAlign{model.Left, model.CenterH, model.Right, model.CustomH}
		vs := []model.VAlign{model.Top, model.CenterV}
		q.Placement = model.Placement{Horizontal: hs[r.Intn(4)], Vertical: vs[r.Intn(2)], OffsetXMM: r.Float64() * 50, OffsetYMM: r.Float64() * 50}
		b, err := Compute(q, doc, nil)
		if err != nil {
			t.Fatal(err)
		}
		if a.Scale != b.Scale || a.ContentWidthMM != b.ContentWidthMM {
			t.Fatalf("the placement changed the scale: %v -> %v", a.Scale, b.Scale)
		}
		for k := range a.Pages {
			if a.Pages[k].ContentW != b.Pages[k].ContentW || a.Pages[k].ContentH != b.Pages[k].ContentH {
				t.Fatalf("the placement changed the size of page %d", k+1)
			}
		}
	}
}

// The scale does not depend on the cuts at all: the same project without its
// cuts has the same scale.
func TestPropertyScaleIgnoresCuts(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	for i := 0; i < iterations; i++ {
		p, doc := randomProject(r)
		_, s0, err := Resolve(p, doc)
		if err != nil {
			continue
		}
		q := withRandomCuts(r, p, doc, r.Intn(20))
		if _, s1, err := Resolve(q, doc); err != nil || s1 != s0 {
			t.Fatalf("scale %v with %d cuts, %v without (%v)", s1, len(q.Cuts), s0, err)
		}
	}
}

// Every proposal of automatic cuts gives pages that fit the sheet, in order,
// with the minimal gaps, and never fewer pages than the drawing needs.
func TestPropertyAutoCutsFit(t *testing.T) {
	r := rand.New(rand.NewSource(5))
	for i := 0; i < 150; i++ {
		p, doc := randomProject(r)
		p.Scale = model.Scale{Mode: model.Width, WidthMM: 40 + r.Float64()*150}
		p.Paper = model.Paper{Format: "A4", Orientation: model.Portrait}
		p.Margins = model.Margins{Top: 10, Bottom: 10, Left: 5, Right: 5}
		p.Placement = model.Placement{Horizontal: model.CenterH, Vertical: model.Top}
		sheet, s, err := Resolve(p, doc)
		if err != nil || sheet.UsableWidth < doc.W*s-units.ToleranceMM {
			continue
		}
		var blocks []Block
		if r.Intn(4) != 0 {
			for y := r.Float64() * 50; y < doc.H; {
				h := 5 + r.Float64()*300
				blocks = append(blocks, Block{ID: "b", Top: y, Bottom: math.Min(y+h, doc.H)})
				y += h + r.Float64()*120
			}
		}
		prop, err := AutoCuts(p, doc, blocks)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		nMin := ceilPages(doc.H*s, sheet.UsableHeight)
		if prop.Pages < nMin || prop.Pages > nMin+2 || len(prop.Positions) != prop.Pages-1 {
			t.Fatalf("iteration %d: %d pages for a minimum of %d, cuts %v", i, prop.Pages, nMin, prop.Positions)
		}
		q, err := SetCuts(p, doc, prop.Positions)
		if err != nil {
			t.Fatalf("iteration %d: the proposal cannot be applied: %v", i, err)
		}
		pl := mustPlan(t, q, doc, nil)
		if !pl.AllConforming() {
			t.Fatalf("iteration %d: the pages of the proposal do not fit: %+v", i, pl.Pages)
		}
		checkPlan(t, "auto", q, doc, pl)
	}
}

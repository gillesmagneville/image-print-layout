// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"testing"

	"github.com/gillesmagneville/svg-print-layout/internal/model"
)

func find(cs []Correction, kind CorrectionKind) (Correction, bool) {
	for _, c := range cs {
		if c.Kind == kind {
			return c, true
		}
	}
	return Correction{}, false
}

// A page that does not fit is flagged and four proposals are made; the scale
// is not touched, and nothing is applied.
func TestCorrectionsForATallPage(t *testing.T) {
	p := acceptance()
	blocks := miniCADBlocks()
	pl := mustPlan(t, p, miniCAD, blocks)
	if pl.AllConforming() {
		t.Fatal("856 mm of drawing cannot fit one page")
	}
	if pl.Scale != 0.15625 {
		t.Errorf("the scale was changed to %v", pl.Scale)
	}
	for _, kind := range []CorrectionKind{CorrectionAddCut, CorrectionMoveCut, CorrectionReduceScale, CorrectionChangePaper} {
		_, ok := find(pl.Corrections, kind)
		if kind == CorrectionMoveCut {
			if ok {
				t.Errorf("a single page has no cut to move")
			}
			continue
		}
		if !ok {
			t.Errorf("no %s proposal in %+v", kind, pl.Corrections)
		}
	}

	add, _ := find(pl.Corrections, CorrectionAddCut)
	if add.Page != 1 || len(Crossed(add.Position, blocks)) != 0 {
		t.Errorf("add-cut proposal = %+v; it should fall in a void", add)
	}
	if add.Resolves {
		t.Error("one more cut cannot fix 856 mm of drawing on pages of 277 mm")
	}

	reduce, _ := find(pl.Corrections, CorrectionReduceScale)
	// The whole drawing on one page: 277 mm of height, i.e. 277/856.09 of the scale.
	want := 277.0 / 856.09375 * 200
	if reduce.WidthMM > want || reduce.WidthMM < want-0.1 {
		t.Errorf("reduce-scale width = %v mm, want %v rounded down to a tenth", reduce.WidthMM, want)
	}
	if !reduce.Resolves {
		t.Error("the reduction must fix the overflow")
	}
	if !near(reduce.Percent, reduce.Scale/NativeMMPerUnit*100, 1e-9) {
		t.Errorf("percent %v does not match scale %v", reduce.Percent, reduce.Scale)
	}

	paper, _ := find(pl.Corrections, CorrectionChangePaper)
	if paper.Paper == "A4" && paper.Orientation == model.Portrait {
		t.Errorf("paper proposal = the current paper")
	}
}

func TestApplyAddCutAndTheChainOfProposals(t *testing.T) {
	p := acceptance()
	blocks := miniCADBlocks()
	// Follow the "add a cut" proposal until everything fits.
	for i := 0; i < 10; i++ {
		pl := mustPlan(t, p, miniCAD, blocks)
		if pl.AllConforming() {
			if len(pl.Pages) != 4 {
				t.Errorf("%d pages, want 4", len(pl.Pages))
			}
			return
		}
		add, ok := find(pl.Corrections, CorrectionAddCut)
		if !ok {
			t.Fatalf("no add-cut proposal at step %d: %+v", i, pl.Corrections)
		}
		var err error
		if p, err = Apply(p, miniCAD, add); err != nil {
			t.Fatal(err)
		}
		if pl.Scale != mustPlan(t, p, miniCAD, blocks).Scale {
			t.Fatal("adding a cut changed the scale")
		}
	}
	t.Fatal("the proposals never converged")
}

func TestApplyReduceScale(t *testing.T) {
	p := acceptance()
	pl := mustPlan(t, p, miniCAD, nil)
	reduce, _ := find(pl.Corrections, CorrectionReduceScale)
	q, err := Apply(p, miniCAD, reduce)
	if err != nil {
		t.Fatal(err)
	}
	if q.Scale.Mode != model.Width || q.Scale.WidthMM != reduce.WidthMM {
		t.Errorf("scale = %+v", q.Scale)
	}
	if after := mustPlan(t, q, miniCAD, nil); !after.AllConforming() {
		t.Errorf("not conforming after the reduction: %+v", after.Pages[0].Items)
	}
	if p.Scale.WidthMM != 200 {
		t.Error("Apply modified its argument")
	}
}

// Moving the neighbouring cut: the cut goes up to the greatest height the
// page accepts, in a void when there is one.
func TestMoveCutProposal(t *testing.T) {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 200} // s = 0.2; a page holds 1385 units
	doc := Doc{W: 1000, H: 3000}
	p, id := mustAdd(t, p, doc, 1600) // page 1 is 1600 units: too tall
	blocks := []Block{{"a", 0, 1300}, {"b", 1320, 1590}, {"c", 1700, 3000}}
	pl := mustPlan(t, p, doc, blocks)
	move, ok := find(pl.Corrections, CorrectionMoveCut)
	if !ok || move.CutID != id || move.Page != 1 {
		t.Fatalf("proposals = %+v", pl.Corrections)
	}
	if move.Position > 1385 || move.Position < 1300 || len(Crossed(move.Position, blocks)) != 0 {
		t.Errorf("proposed position %v: it should be at most 1385 and in the void 1300-1320", move.Position)
	}
	q, err := Apply(p, doc, move)
	if err != nil {
		t.Fatal(err)
	}
	if q.Cuts[0].Position != move.Position || q.Cuts[0].ID != id {
		t.Errorf("cut after Apply: %+v", q.Cuts[0])
	}
	// Page 2 becomes 1700 units: now too tall; so the proposal does not resolve everything.
	if move.Resolves {
		t.Error("page 2 would then be too tall")
	}
}

func TestMoveCutProposalOnTheLastPage(t *testing.T) {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 200}
	doc := Doc{W: 1000, H: 2700}
	p, id := mustAdd(t, p, doc, 1200) // page 2 is 1500 units: too tall
	pl := mustPlan(t, p, doc, nil)
	move, ok := find(pl.Corrections, CorrectionMoveCut)
	if !ok || move.CutID != id || move.Page != 2 || !near(move.Position, 2700-1385, 1e-6) || !move.Resolves {
		t.Errorf("proposals = %+v", pl.Corrections)
	}
}

// A page that is too wide has only the proposals that change the scale or
// the paper.
func TestCorrectionsForAWidePage(t *testing.T) {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 230}
	pl := mustPlan(t, p, Doc{W: 1000, H: 1000}, nil)
	if _, ok := find(pl.Corrections, CorrectionAddCut); ok {
		t.Error("a cut cannot fix a width")
	}
	reduce, ok := find(pl.Corrections, CorrectionReduceScale)
	if !ok || reduce.WidthMM != 200 || !reduce.Resolves {
		t.Errorf("reduce = %+v", reduce)
	}
	paper, ok := find(pl.Corrections, CorrectionChangePaper)
	if !ok {
		t.Fatalf("no paper proposal in %+v", pl.Corrections)
	}
	q, err := Apply(p, Doc{W: 1000, H: 1000}, paper)
	if err != nil || !mustPlan(t, q, Doc{W: 1000, H: 1000}, nil).AllConforming() {
		t.Errorf("paper %s %s does not fix the overflow: %v", paper.Paper, paper.Orientation, err)
	}
}

func TestNoCorrectionWhenEverythingFits(t *testing.T) {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 100}
	if pl := mustPlan(t, p, Doc{W: 1000, H: 1000}, nil); len(pl.Corrections) != 0 {
		t.Errorf("corrections = %+v", pl.Corrections)
	}
	if _, err := Apply(p, Doc{W: 1000, H: 1000}, Correction{}); errCode(err) != "correction.none" {
		t.Errorf("an empty correction: %v", err)
	}
}

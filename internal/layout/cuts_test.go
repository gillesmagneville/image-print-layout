// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"testing"

	"github.com/gillesmagneville/pageslice/internal/model"
)

func positions(p model.Project) []float64 {
	var out []float64
	for _, c := range p.Cuts {
		out = append(out, c.Position)
	}
	return out
}

func equalFloats(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAddCutKeepsTheOrderAndGivesIDs(t *testing.T) {
	p := acceptance()
	p, a := mustAdd(t, p, miniCAD, 3000)
	p, b := mustAdd(t, p, miniCAD, 1000)
	p, c := mustAdd(t, p, miniCAD, 2000)
	if !equalFloats(positions(p), []float64{1000, 2000, 3000}) {
		t.Errorf("positions = %v", positions(p))
	}
	if a == b || b == c || a == c || a == 0 {
		t.Errorf("ids %d %d %d are not distinct and positive", a, b, c)
	}
	if p.Cuts[0].ID != b || p.Cuts[1].ID != c || p.Cuts[2].ID != a {
		t.Errorf("ids follow the cuts: %+v", p.Cuts)
	}
}

func TestCommandsDoNotModifyTheirArgument(t *testing.T) {
	p := acceptance()
	p, id := mustAdd(t, p, miniCAD, 1500)
	before := p.Clone()
	if _, _, err := AddCut(p, miniCAD, 3000); err != nil {
		t.Fatal(err)
	}
	if _, err := MoveCut(p, miniCAD, id, 1600); err != nil {
		t.Fatal(err)
	}
	if _, err := DeleteCut(p, id); err != nil {
		t.Fatal(err)
	}
	if !equalFloats(positions(p), positions(before)) || p.NextCutID != before.NextCutID {
		t.Errorf("the project was modified: %+v", p)
	}
}

func TestAddCutRefusals(t *testing.T) {
	p := acceptance() // s = 0.15625, so the gap is 0.5 / 0.15625 = 3.2 units
	p, _ = mustAdd(t, p, miniCAD, 1000)
	for name, tt := range map[string]struct {
		y    float64
		code string
	}{
		"at the top":           {0, "cut.out_of_range"},
		"above the top":        {-5, "cut.out_of_range"},
		"at the bottom":        {5479, "cut.out_of_range"},
		"below the bottom":     {6000, "cut.out_of_range"},
		"too near the top":     {3, "cut.too_close"},
		"too near the bottom":  {5476, "cut.too_close"},
		"too near another cut": {1003, "cut.too_close"},
		"on another cut":       {1000, "cut.too_close"},
	} {
		if _, _, err := AddCut(p, miniCAD, tt.y); errCode(err) != tt.code {
			t.Errorf("%s: error %v, want %s", name, err, tt.code)
		}
	}
	// Just far enough is accepted.
	if _, _, err := AddCut(p, miniCAD, 1003.3); err != nil {
		t.Errorf("a cut 3.3 units (0.52 mm) away: %v", err)
	}
	if _, _, err := AddCut(p, miniCAD, 3.3); err != nil {
		t.Errorf("a cut 3.3 units from the top: %v", err)
	}
}

func TestMoveCut(t *testing.T) {
	p := acceptance()
	p, a := mustAdd(t, p, miniCAD, 1000)
	p, b := mustAdd(t, p, miniCAD, 2000)

	q, err := MoveCut(p, miniCAD, a, 1500.123456789)
	if err != nil {
		t.Fatal(err)
	}
	if q.Cuts[0].Position != 1500.123456789 {
		t.Errorf("the position is stored as given, not rounded: %v", q.Cuts[0].Position)
	}
	if q.Cuts[0].ID != a {
		t.Error("a cut keeps its id when it moves")
	}
	if _, err := MoveCut(p, miniCAD, a, 2100); errCode(err) != "cut.too_close" {
		t.Errorf("jumping over another cut: %v", err)
	}
	if _, err := MoveCut(p, miniCAD, a, 1997); errCode(err) != "cut.too_close" {
		t.Errorf("moving onto another cut: %v", err)
	}
	if _, err := MoveCut(p, miniCAD, b, 6000); errCode(err) != "cut.out_of_range" {
		t.Errorf("moving out of the document: %v", err)
	}
	if _, err := MoveCut(p, miniCAD, 999, 1500); errCode(err) != "cut.unknown" {
		t.Errorf("unknown id: %v", err)
	}
	// A cut can stay where it is, and move a hair.
	if _, err := MoveCut(p, miniCAD, a, 1000); err != nil {
		t.Errorf("moving a cut to its own position: %v", err)
	}
}

func TestCutBoundsClampADrag(t *testing.T) {
	p := acceptance()
	p, a := mustAdd(t, p, miniCAD, 1000)
	p, b := mustAdd(t, p, miniCAD, 2000)
	g := 0.5 / 0.15625
	lo, hi, err := CutBounds(p, miniCAD, a)
	if err != nil || !near(lo, g, 1e-9) || !near(hi, 2000-g, 1e-9) {
		t.Errorf("first cut: [%v, %v], %v", lo, hi, err)
	}
	lo, hi, err = CutBounds(p, miniCAD, b)
	if err != nil || !near(lo, 1000+g, 1e-9) || !near(hi, 5479-g, 1e-9) {
		t.Errorf("second cut: [%v, %v], %v", lo, hi, err)
	}
	// Both ends of the stretch are accepted by MoveCut.
	if _, err := MoveCut(p, miniCAD, b, hi); err != nil {
		t.Errorf("moving to the upper bound: %v", err)
	}
	if _, err := MoveCut(p, miniCAD, b, lo); err != nil {
		t.Errorf("moving to the lower bound: %v", err)
	}
	if _, _, err := CutBounds(p, miniCAD, 99); errCode(err) != "cut.unknown" {
		t.Errorf("unknown id: %v", err)
	}
}

func TestDeleteCut(t *testing.T) {
	p := acceptance()
	p, a := mustAdd(t, p, miniCAD, 1000)
	p, b := mustAdd(t, p, miniCAD, 2000)
	q, err := DeleteCut(p, a)
	if err != nil || len(q.Cuts) != 1 || q.Cuts[0].ID != b {
		t.Fatalf("after deleting the first: %+v, %v", q.Cuts, err)
	}
	q, err = DeleteCut(q, b)
	if err != nil || len(q.Cuts) != 0 {
		t.Fatalf("after deleting the last: %+v, %v", q.Cuts, err)
	}
	if _, err := DeleteCut(q, b); errCode(err) != "cut.unknown" {
		t.Errorf("deleting twice: %v", err)
	}
	// An id is never reused, even after a delete.
	_, c := mustAdd(t, q, miniCAD, 3000)
	if c == a || c == b {
		t.Errorf("id %d was reused", c)
	}
}

func TestSetCuts(t *testing.T) {
	p := acceptance()
	p, _ = mustAdd(t, p, miniCAD, 500)
	p.PageOverrides = []model.PageOverride{{Page: 2, Vertical: model.CenterV}}
	q, err := SetCuts(p, miniCAD, []float64{3000, 1400, 4200})
	if err != nil {
		t.Fatal(err)
	}
	if !equalFloats(positions(q), []float64{1400, 3000, 4200}) {
		t.Errorf("positions = %v", positions(q))
	}
	if len(q.PageOverrides) != 0 {
		t.Errorf("the overrides of pages that no longer exist were kept: %+v", q.PageOverrides)
	}
	if _, err := SetCuts(p, miniCAD, []float64{1000, 1001}); errCode(err) != "cut.too_close" {
		t.Errorf("two cuts too close: %v", err)
	}
	if q, err := SetCuts(p, miniCAD, nil); err != nil || len(q.Cuts) != 0 {
		t.Errorf("no cut: %+v, %v", q.Cuts, err)
	}
}

func TestOverridesFollowTheCuts(t *testing.T) {
	p := acceptance()
	p, a := mustAdd(t, p, miniCAD, 2000) // pages 1, 2
	x := 3.0
	p.PageOverrides = []model.PageOverride{{Page: 1, Vertical: model.CenterV}, {Page: 2, Horizontal: model.Left, OffsetXMM: &x}}

	// Cutting page 1 in two: both halves keep its override, page 2 becomes 3.
	p, b := mustAdd(t, p, miniCAD, 800)
	pages := func() map[int]model.PageOverride {
		m := map[int]model.PageOverride{}
		for _, o := range p.PageOverrides {
			m[o.Page] = o
		}
		return m
	}
	m := pages()
	if len(m) != 3 || m[1].Vertical != model.CenterV || m[2].Vertical != model.CenterV || m[3].Horizontal != model.Left {
		t.Fatalf("after cutting page 1: %+v", p.PageOverrides)
	}
	if m[3].OffsetXMM == nil || *m[3].OffsetXMM != 3 {
		t.Errorf("the offset of page 3 was lost: %+v", m[3])
	}
	// The copies do not share their offsets with the original.
	p2, _ := mustAdd(t, p, miniCAD, 1500)
	*p2.PageOverrides[len(p2.PageOverrides)-1].OffsetXMM = 99
	if *m[3].OffsetXMM != 3 {
		t.Error("an override was shared between two projects")
	}

	// Deleting the cut between pages 1 and 2 keeps the override of page 1 and
	// drops that of page 2; page 3 becomes 2.
	var err error
	p, err = DeleteCut(p, b)
	if err != nil {
		t.Fatal(err)
	}
	m = pages()
	if len(m) != 2 || m[1].Vertical != model.CenterV || m[2].Horizontal != model.Left {
		t.Fatalf("after deleting a cut: %+v", p.PageOverrides)
	}
	_ = a
}

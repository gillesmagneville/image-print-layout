// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"math"
	"testing"
	"time"

	"github.com/gillesmagneville/pageslice/internal/layout"
	"github.com/gillesmagneville/pageslice/internal/model"
	"github.com/gillesmagneville/pageslice/internal/testsvg"
)

// blockSession is a session on a document with blocks: with the fake engine,
// block i covers [120+100i, 180+100i], so the voids are the 40 units between.
func blockSession(t *testing.T) *Session {
	t.Helper()
	svc, _ := fakeServices()
	o, err := Prepare(svc, testsvg.Generate(testsvg.Options{}), "/tmp/mini-cad.svg", model.Background{})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if err := s.Open(o.Document); err != nil {
		t.Fatal(err)
	}
	return s
}

func scaleOf(t *testing.T, s *Session) float64 {
	t.Helper()
	return s.Plan().Scale
}

// Acceptance tests 6 and 7: adding a cut leaves the width of the drawing, and
// moving one leaves the scale.
func TestAddingAndMovingCutsKeepTheScale(t *testing.T) {
	s := blockSession(t)
	s0, w0 := scaleOf(t, s), s.Plan().ContentWidthMM
	if math.Abs(w0-200) > 0.005 {
		t.Fatalf("width = %v mm, want 200", w0)
	}

	var ids []int
	for _, y := range []float64{1000, 2000, 3000, 4000} {
		id, err := s.AddCut(y, 0)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		if scaleOf(t, s) != s0 || s.Plan().ContentWidthMM != w0 {
			t.Fatalf("adding a cut at %v changed the scale or the width", y)
		}
		for _, p := range s.Plan().Pages {
			if p.ContentW != w0 {
				t.Fatalf("page %d is %v mm wide, want %v", p.Number, p.ContentW, w0)
			}
		}
	}
	if len(s.Plan().Pages) != 5 {
		t.Fatalf("%d pages, want 5 (four cuts)", len(s.Plan().Pages))
	}
	for _, y := range []float64{1100, 1300, 1500} {
		if _, err := s.DragCut(ids[0], y, 0); err != nil {
			t.Fatal(err)
		}
		if scaleOf(t, s) != s0 {
			t.Fatalf("dragging a cut to %v changed the scale", y)
		}
	}
	s.EndDrag()
	if err := s.MoveCut(ids[1], 2222); err != nil {
		t.Fatal(err)
	}
	if scaleOf(t, s) != s0 || s.Plan().ContentWidthMM != w0 {
		t.Error("moving a cut to a typed position changed the scale")
	}
}

func TestADragOfACutIsOneEntryOfTheHistory(t *testing.T) {
	s := blockSession(t)
	id, err := s.AddCut(1800, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, y := range []float64{1850, 1900, 1950, 2000} {
		if _, err := s.DragCut(id, y, 0); err != nil {
			t.Fatal(err)
		}
	}
	s.EndDrag()
	if got := s.Project().Cuts[0].Position; got != 2000 {
		t.Fatalf("cut at %v", got)
	}
	if !s.Undo() || s.Project().Cuts[0].Position != 1800 {
		t.Errorf("undo of the drag gives %v, want 1800", s.Project().Cuts[0].Position)
	}
	if !s.Undo() || len(s.Project().Cuts) != 0 {
		t.Error("the second undo does not take the added cut back")
	}
}

func TestDragSnapsToAVoidUnlessSnappingIsOff(t *testing.T) {
	s := blockSession(t)
	id, err := s.AddCut(1000, 0)
	if err != nil {
		t.Fatal(err)
	}
	// 1350 is inside block 12 ([1320, 1380]): the voids around are 20 units off.
	got, err := s.DragCut(id, 1350, 40)
	if err != nil {
		t.Fatal(err)
	}
	if got == 1350 || len(layout.Crossed(got, s.Blocks())) != 0 {
		t.Errorf("the cut stopped at %v, which is not in a void", got)
	}
	if math.Abs(got-1350) > 40 {
		t.Errorf("the cut jumped %v units, more than the radius", math.Abs(got-1350))
	}
	got, err = s.DragCut(id, 1350, 0)
	if err != nil || got != 1350 {
		t.Errorf("without snapping the cut is at %v (%v), want 1350", got, err)
	}
	if v := s.Cuts(); len(v) != 1 || v[0].Crossed != 1 {
		t.Errorf("the cut view says %+v, want one block crossed", v)
	}
	s.EndDrag()
}

func TestADragStopsAgainstTheNextCut(t *testing.T) {
	s := blockSession(t)
	a, _ := s.AddCut(1000, 0)
	b, _ := s.AddCut(2000, 0)
	got, err := s.DragCut(a, 2500, 0)
	if err != nil {
		t.Fatalf("a drag past the next cut is refused: %v", err)
	}
	if want := 2000 - layout.MinCutGapMM/scaleOf(t, s); math.Abs(got-want) > 1e-6 {
		t.Errorf("the cut stopped at %v, want %v (the minimal gap before the next cut)", got, want)
	}
	got, err = s.DragCut(b, -50, 0)
	if err != nil || got <= s.Project().Cuts[0].Position {
		t.Errorf("the second cut went to %v (%v), above the first", got, err)
	}
	s.EndDrag()
	if got := s.Project().Cuts; got[0].Position >= got[1].Position {
		t.Errorf("the cuts are out of order: %+v", got)
	}
}

func TestNudgesOfTheSameCutAreOneEntry(t *testing.T) {
	s := blockSession(t)
	now := time.Unix(1000, 0)
	s.Now = func() time.Time { return now }
	id, _ := s.AddCut(1000, 0)
	for i := 0; i < 5; i++ {
		now = now.Add(200 * time.Millisecond)
		if err := s.NudgeCut(id, 10); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.Project().Cuts[0].Position; got != 1050 {
		t.Fatalf("cut at %v, want 1050", got)
	}
	s.Undo()
	if got := s.Project().Cuts[0].Position; got != 1000 {
		t.Errorf("one undo gives %v, want 1000", got)
	}
	// Pushed against the edge, a nudge stops there.
	for i := 0; i < 3; i++ {
		_ = s.NudgeCut(id, -5000)
	}
	if got := s.Project().Cuts[0].Position; got <= 0 {
		t.Errorf("cut at %v, outside the document", got)
	}
}

func TestMoveCutRefusesAnImpossiblePosition(t *testing.T) {
	s := blockSession(t)
	id, _ := s.AddCut(1000, 0)
	if err := s.MoveCut(id, 99999); err == nil {
		t.Error("a cut moved outside the document")
	}
	if got := s.Project().Cuts[0].Position; got != 1000 {
		t.Errorf("the refused move left the cut at %v", got)
	}
}

func TestDeleteCut(t *testing.T) {
	s := blockSession(t)
	id, _ := s.AddCut(1000, 0)
	if err := s.DeleteCut(id); err != nil || len(s.Project().Cuts) != 0 || len(s.Plan().Pages) != 1 {
		t.Fatalf("delete: %v, %d cuts", err, len(s.Project().Cuts))
	}
	if err := s.DeleteCut(id); err == nil {
		t.Error("a cut that is gone was deleted again")
	}
	s.Undo()
	if len(s.Project().Cuts) != 1 {
		t.Error("undo did not bring the cut back")
	}
}

func TestAddCutSnapsToAVoid(t *testing.T) {
	s := blockSession(t)
	id, err := s.AddCut(1350, 40)
	if err != nil {
		t.Fatal(err)
	}
	y := s.Project().Cuts[0].Position
	if id == 0 || len(layout.Crossed(y, s.Blocks())) != 0 {
		t.Errorf("the cut at %v crosses a block", y)
	}
}

func TestCutsViewNamesTheBlocksCrossed(t *testing.T) {
	s := blockSession(t)
	if _, err := s.AddCut(150, 0); err != nil { // inside the first block
		t.Fatal(err)
	}
	v := s.Cuts()
	if len(v) != 1 || v[0].Number != 1 || v[0].Position != 150 || v[0].Crossed != 1 || len(v[0].Names) != 1 || v[0].Names[0] == "" {
		t.Errorf("view = %+v", v)
	}
}

func TestAutoCutsReplaceTheCutsAndCanBeUndone(t *testing.T) {
	s := blockSession(t)
	if _, err := s.AddCut(500, 0); err != nil {
		t.Fatal(err)
	}
	before := s.Project().Cuts
	prop, err := s.AutoCuts()
	if err != nil {
		t.Fatal(err)
	}
	if prop.Pages != len(s.Plan().Pages) || len(s.Project().Cuts) != len(prop.Positions) {
		t.Errorf("proposal %d pages / %d cuts, session %d pages / %d cuts", prop.Pages, len(prop.Positions), len(s.Plan().Pages), len(s.Project().Cuts))
	}
	if !s.Plan().AllConforming() {
		t.Error("the automatic cuts leave a page that does not fit")
	}
	if prop.Crossings != 0 {
		t.Errorf("%d cuts cross a block although there are voids", prop.Crossings)
	}
	if !s.Undo() || len(s.Project().Cuts) != len(before) || s.Project().Cuts[0].Position != 500 {
		t.Errorf("undo does not bring back the cut of the user: %+v", s.Project().Cuts)
	}
}

func TestCutCommandsNeedADocument(t *testing.T) {
	s := NewSession()
	if _, err := s.AddCut(100, 0); err == nil {
		t.Error("AddCut without a document")
	}
	if _, err := s.DragCut(1, 100, 0); err == nil {
		t.Error("DragCut without a document")
	}
	if _, err := s.AutoCuts(); err == nil {
		t.Error("AutoCuts without a document")
	}
	if len(s.Cuts()) != 0 || len(s.Voids()) != 0 {
		t.Error("a session without a document has cuts or voids")
	}
}

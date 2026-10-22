// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package acceptance

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/app"
	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/export"
	"github.com/gillesmagneville/image-print-layout/internal/layout"
	"github.com/gillesmagneville/image-print-layout/internal/model"
	"github.com/gillesmagneville/image-print-layout/internal/project"
	"github.com/gillesmagneville/image-print-layout/internal/units"
)

// The acceptance tests 1 to 7 and 14 of the specification, on the whole chain
// (ARCHITECTURE §13): the document is read by the engine, the commands are the
// ones the window calls, and the project goes through a file. Tests 8 to 13
// are in scenario_cgo_test.go.

// widths lists the content width of every page of a plan, and the plan's
// scale: what the user must never see change by adding or moving a cut.
func widths(pl *layout.Plan) (scale float64, content float64, perPage []float64) {
	for _, p := range pl.Pages {
		perPage = append(perPage, p.ContentW)
	}
	return pl.Scale, pl.ContentWidthMM, perPage
}

func sameWidths(t *testing.T, what string, before, after *layout.Plan) {
	t.Helper()
	s0, c0, p0 := widths(before)
	s1, c1, p1 := widths(after)
	// Exactly equal: nothing recomputes the scale, so nothing may differ in a bit.
	if s0 != s1 || c0 != c1 {
		t.Errorf("%s changed the scale: %v → %v mm per unit, %v → %v mm wide", what, s0, s1, c0, c1)
	}
	for i, w := range p1 {
		if w != c1 {
			t.Errorf("%s: page %d is %v mm wide, the drawing %v mm", what, i+1, w, c1)
		}
	}
	for i, w := range p0 {
		if w != c0 {
			t.Errorf("%s: before, page %d was %v mm wide, the drawing %v mm", what, i+1, w, c0)
		}
	}
}

func TestAcceptance1To7(t *testing.T) {
	for _, fx := range fixtures(t) {
		t.Run(fx.name, func(t *testing.T) {
			// 1. Import the Mini-CAD: 1 280 × 5 479 units, nothing that stops the work.
			sc := newScenario(t, fx.svg)
			doc := sc.sess.Document()
			if doc.Size.W != 1280 || doc.Size.H != 5479 {
				t.Fatalf("test 1: the document is %v × %v units, want 1280 × 5479", doc.Size.W, doc.Size.H)
			}
			for _, f := range doc.Findings {
				if f.Level == diag.Failure {
					t.Errorf("test 1: the analysis reports a failure: %v", f)
				}
			}
			if len(doc.Blocks) == 0 {
				t.Error("test 1: no block was measured")
			}

			// 2. A4 portrait: the usable area of the default margins.
			sc.must(sc.sess.Apply("undo.paper", "", app.Edit(func(p *model.Project) {
				p.Paper = model.Paper{Format: "A4", Orientation: model.Portrait}
				p.Margins = model.Margins{Top: 10, Bottom: 10, Left: 5, Right: 5}
			})))
			sheet := sc.sess.Plan().Sheet
			if sheet.WidthMM != 210 || sheet.HeightMM != 297 {
				t.Errorf("test 2: the sheet is %v × %v mm", sheet.WidthMM, sheet.HeightMM)
			}
			if !near(sheet.UsableWidth, 200, 1e-9) || !near(sheet.UsableHeight, 277, 1e-9) {
				t.Errorf("test 2: the usable area is %v × %v mm, want 200 × 277", sheet.UsableWidth, sheet.UsableHeight)
			}

			// 3. A content width of 200 mm.
			sc.must(sc.sess.Apply("undo.scale", "", app.Edit(func(p *model.Project) {
				p.Scale = model.Scale{Mode: model.Width, WidthMM: 200}
			})))
			plan := sc.sess.Plan()
			if !near(plan.Scale, 200.0/1280, 1e-12) || !near(plan.ContentWidthMM, 200, units.ToleranceMM) {
				t.Errorf("test 3: scale %v mm per unit, width %v mm", plan.Scale, plan.ContentWidthMM)
			}

			// 4. The proportions are kept: the height follows the one scale.
			if want := 5479 * plan.Scale; !near(plan.TotalHeightMM, want, 1e-9) {
				t.Errorf("test 4: the drawing is %v mm tall, want %v (the scale of the width)", plan.TotalHeightMM, want)
			}
			if !near(plan.TotalHeightMM/plan.ContentWidthMM, 5479.0/1280, 1e-9) {
				t.Errorf("test 4: the proportions are %v, the document's are %v", plan.TotalHeightMM/plan.ContentWidthMM, 5479.0/1280)
			}

			// 5. Cuts placed by hand, the way the mouse does: a double click adds
			// one, a drag moves it, and a drag is one entry of the history.
			ids := map[float64]int{}
			for _, y := range []float64{1400, 2700, 4100} {
				id, err := sc.sess.AddCut(y, 0)
				if err != nil {
					t.Fatalf("test 5: cut at %v: %v", y, err)
				}
				ids[y] = id
			}
			undoable := 0
			for sc.sess.CanUndo() {
				sc.sess.Undo()
				undoable++
			}
			for sc.sess.CanRedo() {
				sc.sess.Redo()
			}
			if len(sc.sess.Cuts()) != 3 || undoable < 3 {
				t.Errorf("test 5: %d cuts, %d entries of history", len(sc.sess.Cuts()), undoable)
			}
			entries := 0
			for sc.sess.CanUndo() {
				entries++
				sc.sess.Undo()
			}
			for sc.sess.CanRedo() {
				sc.sess.Redo()
			}
			// A drag of many movements is one entry.
			before := entries
			for _, y := range []float64{2705, 2720, 2760, 2790, 2800} {
				if _, err := sc.sess.DragCut(ids[2700], y, 0); err != nil {
					t.Fatalf("test 5: drag to %v: %v", y, err)
				}
			}
			sc.sess.EndDrag()
			entries = 0
			for sc.sess.CanUndo() {
				entries++
				sc.sess.Undo()
			}
			for sc.sess.CanRedo() {
				sc.sess.Redo()
			}
			if entries != before+1 {
				t.Errorf("test 5: a drag made %d entries of history, want 1", entries-before)
			}
			if got := sc.sess.Cuts(); len(got) != 3 || got[1].Position != 2800 {
				t.Errorf("test 5: cuts = %+v", got)
			}
			if n := len(sc.sess.Plan().Pages); n != 4 {
				t.Errorf("test 5: %d pages for 3 cuts, want 4", n)
			}

			// 6. Adding a cut does not change the width of the drawing.
			before6 := sc.sess.Plan()
			if _, err := sc.sess.AddCut(3500, 0); err != nil {
				t.Fatalf("test 6: %v", err)
			}
			if n := len(sc.sess.Plan().Pages); n != 5 {
				t.Errorf("test 6: %d pages after the new cut, want 5", n)
			}
			sameWidths(t, "test 6: adding a cut", before6, sc.sess.Plan())

			// 7. Moving a cut does not change the scale: by a drag, by a typed
			// position, by the keyboard.
			for _, move := range []struct {
				name string
				do   func() error
			}{
				{"a drag", func() error { _, err := sc.sess.DragCut(ids[4100], 4300, 0); sc.sess.EndDrag(); return err }},
				{"a typed position", func() error { return sc.sess.MoveCut(ids[1400], 1500.5) }},
				{"the keyboard", func() error { return sc.sess.NudgeCut(ids[2700], 10) }},
			} {
				b := sc.sess.Plan()
				if err := move.do(); err != nil {
					t.Fatalf("test 7: %s: %v", move.name, err)
				}
				sameWidths(t, "test 7: "+move.name, b, sc.sess.Plan())
			}
			// Whatever the cuts, every page gets the same placement: at the top
			// margin of 10 mm.
			sc.must(sc.sess.Apply("undo.placement", "", app.Edit(func(p *model.Project) {
				p.Placement = model.Placement{Horizontal: model.CenterH, Vertical: model.Top}
			})))
			for _, pg := range sc.sess.Plan().Pages {
				if pg.Y != 10 {
					t.Errorf("page %d is %v mm from the top", pg.Number, pg.Y)
				}
			}
		})
	}
}

func TestAcceptance14ReopenTheProject(t *testing.T) {
	for _, fx := range fixtures(t) {
		t.Run(fx.name, func(t *testing.T) {
			sc := newScenario(t, fx.svg)
			sourceBefore := fileHash(t, sc.path)
			sc.layOut(scenarioCuts)
			sc.must(sc.sess.SetDocBackground(model.WhitenBackground))
			sc.must(sc.sess.Apply("undo.placement", "", app.Edit(func(p *model.Project) {
				y := 25.0
				p.Placement = model.Placement{Horizontal: model.CenterH, Vertical: model.Top, OffsetYMM: 10}
				p.PageOverrides = []model.PageOverride{{Page: 2, OffsetYMM: &y}}
			})))

			// The PDF of the work in progress.
			first, _ := sc.exportWith(model.WhitenBackground, "first.pdf")

			// Save, in a folder of its own, next to a copy of nothing: the project
			// finds the document by its relative path.
			projectPath := filepath.Join(sc.dir, "mini-cad.imgpl")
			sc.must(sc.sess.SaveProject(projectPath))
			if sc.sess.Dirty() {
				t.Error("a saved project has unsaved changes")
			}

			// Open it in a new session, from the file only.
			pf, err := app.ReadProject(projectPath)
			if err != nil {
				t.Fatalf("test 14: %v", err)
			}
			if pf.SourcePath != sc.path {
				t.Fatalf("test 14: the document is found at %q, want %q", pf.SourcePath, sc.path)
			}
			o, err := app.Open(services(), pf.SourcePath, pf.Project.Background)
			if err != nil {
				t.Fatalf("test 14: %v", err)
			}
			defer o.Engine.Close()
			reopened := app.NewSession()
			items, err := reopened.OpenProject(o.Document, pf)
			if err != nil {
				t.Fatalf("test 14: %v", err)
			}
			if len(items) != 0 {
				t.Errorf("test 14: a project that matches its document reports %v", items)
			}

			// Every setting is back.
			a, b := sc.sess.Project(), reopened.Project()
			a.Source, b.Source = model.Source{}, model.Source{}
			ea, _ := project.Encode(a)
			eb, _ := project.Encode(b)
			if !bytes.Equal(ea, eb) {
				t.Errorf("test 14: the settings differ:\n%s\n%s", ea, eb)
			}
			p := reopened.Project()
			for name, ok := range map[string]bool{
				"paper":        p.Paper.Format == "A4" && p.Paper.Orientation == model.Portrait,
				"margins":      p.Margins == model.Margins{Top: 10, Bottom: 10, Left: 5, Right: 5},
				"scale":        p.Scale.Mode == model.Width && p.Scale.WidthMM == 200,
				"background":   p.Background.Document == model.WhitenBackground && p.Background.Page == model.WhitePage,
				"cuts":         fmt.Sprint(positions(reopened)) == fmt.Sprint(scenarioCuts),
				"placement":    p.Placement.Vertical == model.Top && p.Placement.OffsetYMM == 10,
				"page 2 shift": len(p.PageOverrides) == 1 && p.PageOverrides[0].Page == 2 && *p.PageOverrides[0].OffsetYMM == 25,
			} {
				if !ok {
					t.Errorf("test 14: the %s was not restored: %+v", name, p)
				}
			}
			if reopened.Dirty() || reopened.ProjectPath() != projectPath {
				t.Errorf("test 14: dirty=%v path=%q", reopened.Dirty(), reopened.ProjectPath())
			}

			// The plan is the same, so the PDF is the same: the project is the whole
			// of what the layout needs.
			second := filepath.Join(sc.dir, "second.pdf")
			if _, err := export.Run(export.Request{Doc: o.Engine, Plan: reopened.Plan(), Project: reopened.Project(), Path: second, Progress: traceThreads(second)}); err != nil {
				t.Fatalf("test 14: export after reopening: %v", err)
			}
			oneThread(t, second)
			if fileHash(t, first) != fileHash(t, second) {
				t.Errorf("test 14: the PDF exported after reopening the project is not the one exported before: %s", describePDFDifference(t, first, second))
			}

			// The SVG is intact, and so is the project when saved again.
			if fileHash(t, sc.path) != sourceBefore {
				t.Error("test 14: the SVG was modified")
			}
			saved, _ := os.ReadFile(projectPath)
			sc.must(reopened.SaveProject(projectPath))
			again, _ := os.ReadFile(projectPath)
			if !bytes.Equal(saved, again) {
				t.Error("test 14: saving the reopened project changed the file")
			}
		})
	}
}

func positions(s *app.Session) []float64 {
	var out []float64
	for _, c := range s.Cuts() {
		out = append(out, c.Position)
	}
	return out
}

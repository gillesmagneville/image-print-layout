// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"testing"
	"time"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/engine"
	"github.com/gillesmagneville/pageslice/internal/layout"
	"github.com/gillesmagneville/pageslice/internal/model"
	"github.com/gillesmagneville/pageslice/internal/testsvg"
)

// fakeDoc is an engine document that draws nothing: the use cases are tested
// without the rendering engine.
type fakeDoc struct {
	w, h   float64
	closed bool
	loaded []byte
}

func (d *fakeDoc) Size() (float64, float64)                                        { return d.w, d.h }
func (d *fakeDoc) Measure(string) (engine.Box, error)                              { return engine.Box{}, fmt.Errorf("fake") }
func (d *fakeDoc) Close()                                                          { d.closed = true }
func (d *fakeDoc) RenderPage(engine.PageSpec, float64, bool) (*image.RGBA, error)  { return nil, nil }
func (d *fakeDoc) ExportPDF(io.Writer, []engine.PageSpec, engine.PDFOptions) error { return nil }
func (d *fakeDoc) Render(engine.Box, float64, color.Color) (*image.RGBA, error) {
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
}

// MeasureBlocks puts the blocks one under the other, 100 units apart, 60 high.
func (d *fakeDoc) MeasureBlocks(ids []string) map[string]engine.Box {
	out := map[string]engine.Box{}
	for i, id := range ids {
		out[id] = engine.Box{X: 0, Y: 120 + float64(i)*100, W: 1200, H: 60}
	}
	return out
}

func fakeServices() (Services, *[]*fakeDoc) {
	var docs []*fakeDoc
	return Services{
		Load: func(svg []byte, path string) (engine.Document, error) {
			d := &fakeDoc{w: 1280, h: 5479, loaded: svg}
			docs = append(docs, d)
			return d, nil
		},
		Match: func(f string) string {
			if f == "Arial" || f == "sans-serif" {
				return f
			}
			return "DejaVu Sans"
		},
	}, &docs
}

var miniCAD = layout.Doc{W: 1280, H: 5479}

func session(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if err := s.Open(&Document{Path: "/tmp/mini-cad.svg", Size: miniCAD}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestOpenAnalysesMeasuresAndLoadsTheWorkingCopy(t *testing.T) {
	svc, docs := fakeServices()
	src := testsvg.Generate(testsvg.Options{})
	o, err := Prepare(svc, src, "/tmp/doc.svg", model.Background{Document: model.WhitenBackground, Element: testsvg.BackgroundID})
	if err != nil {
		t.Fatal(err)
	}
	if len(*docs) != 1 {
		t.Fatalf("%d documents loaded", len(*docs))
	}
	if string((*docs)[0].loaded) == string(src) {
		t.Error("the engine received the source, not the working copy")
	}
	d := o.Document
	if d.Size != miniCAD || d.Path != "/tmp/doc.svg" {
		t.Errorf("document = %+v", d)
	}
	if len(d.Blocks) == 0 || len(d.Blocks) != len(d.Analysis.Blocks) {
		t.Errorf("%d blocks measured for %d found", len(d.Blocks), len(d.Analysis.Blocks))
	}
	if d.Blocks[0].Top != 120 || d.Blocks[0].Bottom != 180 {
		t.Errorf("first block = %+v", d.Blocks[0])
	}
	// The font of the test document is not installed on this fake system.
	var missing bool
	for _, it := range d.Findings {
		if it.Code == "svg.font_missing" {
			missing = true
		}
	}
	if !missing {
		t.Errorf("no finding about the fonts: %v", d.Findings)
	}

	// Another instruction for the background: the same analysis, another copy.
	again, err := Rebuild(svc, d, model.Background{Document: model.RemoveBackground, Element: testsvg.BackgroundID})
	if err != nil {
		t.Fatal(err)
	}
	if len(*docs) != 2 || string((*docs)[1].loaded) == string((*docs)[0].loaded) {
		t.Error("the background instruction did not change the working copy")
	}
	_ = again
}

func TestOpenFailsWithTheErrorOfTheEngine(t *testing.T) {
	svc := Services{Load: func([]byte, string) (engine.Document, error) { return nil, fmt.Errorf("engine: nope") }}
	if _, err := Prepare(svc, []byte("<svg/>"), "/tmp/x.svg", model.Background{}); err == nil || err.Error() != "engine: nope" {
		t.Errorf("err = %v", err)
	}
	if _, err := Open(svc, "/nonexistent/file.svg", model.Background{}); err == nil {
		t.Error("a missing file should fail")
	}
}

func TestANewSessionHasNoPlan(t *testing.T) {
	s := NewSession()
	if s.Plan() != nil || s.Document() != nil || s.Dirty() || s.CanUndo() {
		t.Error("a new session is not empty")
	}
	// Settings can be changed before a document is open, if the sheet exists.
	if err := s.Apply("undo.margins", "margins", Edit(func(p *model.Project) { p.Margins.Top = 20 })); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply("undo.margins", "", Edit(func(p *model.Project) { p.Margins.Top, p.Margins.Bottom = 200, 200 })); err == nil {
		t.Error("margins that leave no room were accepted")
	}
}

func TestOpenKeepsTheWaysOfPrintingAndDropsTheCuts(t *testing.T) {
	s := session(t)
	must(t, s.Apply("undo.paper", "paper", Edit(func(p *model.Project) {
		p.Paper = model.Paper{Format: "A3", Orientation: model.Landscape}
		p.Scale = model.Scale{Mode: model.Width, WidthMM: 250}
	})))
	must(t, s.Apply("undo.cut", "", func(p model.Project, d layout.Doc) (model.Project, error) {
		q, _, err := layout.AddCut(p, d, 2000)
		return q, err
	}))
	if len(s.Project().Cuts) != 1 {
		t.Fatal("no cut")
	}

	must(t, s.Open(&Document{Path: "/tmp/other.svg", Size: layout.Doc{W: 800, H: 600}}))
	p := s.Project()
	if p.Paper.Format != "A3" || p.Paper.Orientation != model.Landscape || p.Scale.WidthMM != 250 {
		t.Errorf("the settings were lost: %+v", p)
	}
	if len(p.Cuts) != 0 || p.NextCutID != 1 || len(p.PageOverrides) != 0 {
		t.Errorf("the cuts of the previous drawing were kept: %+v", p)
	}
	if s.CanUndo() || s.Dirty() {
		t.Error("a document that was just opened has history or unsaved changes")
	}
	if s.Plan() == nil || s.Plan().ContentWidthMM != 250 {
		t.Errorf("plan = %+v", s.Plan())
	}
}

func TestApplyRefusesWhatCannotBePlanned(t *testing.T) {
	s := session(t)
	gen := s.Generation()
	err := s.Apply("undo.margins", "margins", Edit(func(p *model.Project) { p.Margins.Left, p.Margins.Right = 120, 120 }))
	if diag.CodeOf(err) != "margins.no_usable" {
		t.Fatalf("err = %v", err)
	}
	if s.Project().Margins.Left != 5 || s.CanUndo() || s.Generation() != gen {
		t.Error("a refused change left a trace")
	}

	err = s.Apply("undo.scale", "scale", Edit(func(p *model.Project) { p.Scale = model.Scale{Mode: model.Width, WidthMM: 0} }))
	if diag.CodeOf(err) != "scale.invalid_width" {
		t.Errorf("err = %v", err)
	}
}

func TestACommandThatChangesNothingLeavesNoEntry(t *testing.T) {
	s := session(t)
	must(t, s.Apply("undo.margins", "margins", Edit(func(p *model.Project) { p.Margins.Top = 10 })))
	if s.CanUndo() || s.Dirty() {
		t.Error("a change to the same value is an entry")
	}
}

func TestUndoAndRedo(t *testing.T) {
	s := session(t)
	must(t, s.Apply("undo.paper", "paper", Edit(func(p *model.Project) { p.Paper.Format = "A3" })))
	must(t, s.Apply("undo.orientation", "orientation", Edit(func(p *model.Project) { p.Paper.Orientation = model.Landscape })))
	if s.UndoLabel() != "undo.orientation" || s.CanRedo() {
		t.Errorf("label = %q", s.UndoLabel())
	}
	wide := s.Plan().Sheet.WidthMM

	if !s.Undo() || s.Project().Paper.Orientation != model.Portrait || s.RedoLabel() != "undo.orientation" {
		t.Fatalf("undo: %+v", s.Project().Paper)
	}
	if s.Plan().Sheet.WidthMM == wide {
		t.Error("the plan did not follow the undo")
	}
	if !s.Undo() || s.Project().Paper.Format != "A4" {
		t.Fatalf("second undo: %+v", s.Project().Paper)
	}
	if s.Undo() {
		t.Error("undo with nothing to undo")
	}
	if !s.Redo() || !s.Redo() || s.Project().Paper.Format != "A3" || s.Project().Paper.Orientation != model.Landscape {
		t.Fatalf("redo: %+v", s.Project().Paper)
	}
	if s.Redo() {
		t.Error("redo with nothing to redo")
	}

	// A new change forgets what could have been redone.
	s.Undo()
	must(t, s.Apply("undo.margins", "margins", Edit(func(p *model.Project) { p.Margins.Top = 15 })))
	if s.CanRedo() {
		t.Error("redo survived a new change")
	}
}

func TestChangesOfTheSameSettingInQuickSuccessionAreOneEntry(t *testing.T) {
	s := session(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	for _, top := range []float64{1, 12, 125 / 10.0} {
		top := top
		must(t, s.Apply("undo.margins", "margins", Edit(func(p *model.Project) { p.Margins.Top = top })))
		now = now.Add(300 * time.Millisecond)
	}
	if len(s.undo) != 1 {
		t.Fatalf("%d entries, want 1", len(s.undo))
	}
	s.Undo()
	if s.Project().Margins.Top != 10 {
		t.Errorf("undo went back to %v, want the value before the first change (10)", s.Project().Margins.Top)
	}

	// Another setting, or a pause, starts a new entry.
	must(t, s.Apply("undo.margins", "margins", Edit(func(p *model.Project) { p.Margins.Top = 20 })))
	must(t, s.Apply("undo.margins", "margins", Edit(func(p *model.Project) { p.Margins.Top = 21 })))
	now = now.Add(5 * time.Second)
	must(t, s.Apply("undo.margins", "margins", Edit(func(p *model.Project) { p.Margins.Top = 22 })))
	must(t, s.Apply("undo.margins", "other", Edit(func(p *model.Project) { p.Margins.Top = 23 })))
	if len(s.undo) != 3 {
		t.Errorf("%d entries, want 3", len(s.undo))
	}
}

func TestADragIsOneEntry(t *testing.T) {
	s := session(t)
	must(t, s.Apply("undo.cut", "", func(p model.Project, d layout.Doc) (model.Project, error) {
		q, _, err := layout.AddCut(p, d, 1800)
		return q, err
	}))
	id := s.Project().Cuts[0].ID
	gen := s.Generation()
	for _, y := range []float64{1850, 1900, 1950, 2000} {
		y := y
		must(t, s.Drag("undo.move_cut", func(p model.Project, d layout.Doc) (model.Project, error) {
			return layout.MoveCut(p, d, id, y)
		}))
	}
	if s.Project().Cuts[0].Position != 2000 || s.Generation() == gen {
		t.Fatal("the drag did not move the cut")
	}
	if len(s.undo) != 1 {
		t.Errorf("history has %d entries during the drag, want the 1 of the added cut", len(s.undo))
	}
	s.EndDrag()
	if len(s.undo) != 2 || s.UndoLabel() != "undo.move_cut" {
		t.Fatalf("history has %d entries after the drag", len(s.undo))
	}
	s.Undo()
	if s.Project().Cuts[0].Position != 1800 {
		t.Errorf("undo of the drag gives %v, want the start of the gesture (1800)", s.Project().Cuts[0].Position)
	}

	// A gesture that comes back to where it started leaves nothing.
	must(t, s.Drag("undo.move_cut", func(p model.Project, d layout.Doc) (model.Project, error) { return layout.MoveCut(p, d, id, 2100) }))
	must(t, s.Drag("undo.move_cut", func(p model.Project, d layout.Doc) (model.Project, error) { return layout.MoveCut(p, d, id, 1800) }))
	n := len(s.undo)
	s.EndDrag()
	if len(s.undo) != n {
		t.Error("a drag that changed nothing made an entry")
	}

	// A refused step of the drag does not break it.
	if err := s.Drag("undo.move_cut", func(p model.Project, d layout.Doc) (model.Project, error) { return layout.MoveCut(p, d, id, 99999) }); err == nil {
		t.Error("a cut moved outside the document")
	}
	if s.Project().Cuts[0].Position != 1800 {
		t.Error("the refused step moved the cut")
	}
	s.EndDrag()
}

func TestApplyEndsTheDragInProgress(t *testing.T) {
	s := session(t)
	must(t, s.Apply("undo.cut", "", func(p model.Project, d layout.Doc) (model.Project, error) {
		q, _, err := layout.AddCut(p, d, 1800)
		return q, err
	}))
	id := s.Project().Cuts[0].ID
	must(t, s.Drag("undo.move_cut", func(p model.Project, d layout.Doc) (model.Project, error) { return layout.MoveCut(p, d, id, 2000) }))
	must(t, s.Apply("undo.margins", "margins", Edit(func(p *model.Project) { p.Margins.Top = 12 })))
	if len(s.undo) != 3 {
		t.Errorf("%d entries, want the cut, the drag and the margin", len(s.undo))
	}
}

func TestDirtyFollowsTheSavedProject(t *testing.T) {
	s := session(t)
	if s.Dirty() {
		t.Fatal("dirty on opening")
	}
	must(t, s.Apply("undo.margins", "margins", Edit(func(p *model.Project) { p.Margins.Top = 12 })))
	if !s.Dirty() {
		t.Error("not dirty after a change")
	}
	s.Undo()
	if s.Dirty() {
		t.Error("dirty after going back to the saved project")
	}
	s.Redo()
	s.SetProjectPath("/tmp/work.pageslice")
	if s.Dirty() || s.ProjectPath() != "/tmp/work.pageslice" {
		t.Error("dirty after saving")
	}
	s.Undo()
	if !s.Dirty() {
		t.Error("not dirty after undoing past the save")
	}
}

func TestApplyCorrection(t *testing.T) {
	s := session(t)
	// At 200 mm the 5479 units make 1 023 mm: one A4 page overflows.
	must(t, s.Apply("undo.scale", "scale", Edit(func(p *model.Project) { p.Scale = model.Scale{Mode: model.Width, WidthMM: 200} })))
	if s.Plan().AllConforming() || len(s.Plan().Corrections) == 0 {
		t.Fatal("no overflow to correct")
	}
	for _, c := range s.Plan().Corrections {
		if c.Kind == layout.CorrectionAddCut {
			must(t, s.ApplyCorrection(c))
			break
		}
	}
	if len(s.Project().Cuts) != 1 || s.UndoLabel() != "undo.correction" {
		t.Errorf("cuts = %v, label %q", s.Project().Cuts, s.UndoLabel())
	}
	// The width of the drawing is what the user set, whatever the correction.
	if s.Plan().ContentWidthMM != 200 {
		t.Errorf("width = %v", s.Plan().ContentWidthMM)
	}
	if err := s.ApplyCorrection(layout.Correction{Kind: "nope"}); diag.CodeOf(err) != "correction.none" {
		t.Errorf("err = %v", err)
	}
}

func TestReplaceTakesAProjectAndStartsTheHistoryAgain(t *testing.T) {
	s := session(t)
	must(t, s.Apply("undo.margins", "margins", Edit(func(p *model.Project) { p.Margins.Top = 12 })))
	q := model.Default()
	q.Paper.Format = "A5"
	must(t, s.Replace(q, "/tmp/p.pageslice"))
	if s.Project().Paper.Format != "A5" || s.CanUndo() || s.Dirty() || s.ProjectPath() != "/tmp/p.pageslice" {
		t.Error("Replace")
	}
	q.Margins = model.Margins{Top: 200, Bottom: 200}
	if err := s.Replace(q, ""); err == nil {
		t.Error("a project that cannot be planned was taken")
	}
	if s.Project().Paper.Format != "A5" {
		t.Error("a refused project replaced the current one")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gillesmagneville/svg-print-layout/internal/diag"
	"github.com/gillesmagneville/svg-print-layout/internal/engine"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
	"github.com/gillesmagneville/svg-print-layout/internal/project"
	"github.com/gillesmagneville/svg-print-layout/internal/testsvg"
)

// savedWork opens the synthetic Mini-CAD from a folder, lays it out the way a
// user would, and saves the project next to it.
func savedWork(t *testing.T) (dir, svgPath, projectPath string, s *Session) {
	t.Helper()
	dir = t.TempDir()
	svgPath = filepath.Join(dir, "mini-cad.svg")
	if err := os.WriteFile(svgPath, testsvg.Generate(testsvg.Options{}), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, _ := fakeServices()
	o, err := Open(svc, svgPath, model.Background{Document: model.KeepBackground, Page: model.WhitePage})
	if err != nil {
		t.Fatal(err)
	}
	s = NewSession()
	if err := s.Open(o.Document); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply("undo.paper", "", Edit(func(p *model.Project) {
		p.Paper = model.Paper{Format: "A3", Orientation: model.Landscape}
		p.Margins = model.Margins{Top: 12, Bottom: 8, Left: 6, Right: 9}
		p.Scale = model.Scale{Mode: model.Width, WidthMM: 250}
		p.Placement = model.Placement{Horizontal: model.CenterH, Vertical: model.Top, OffsetYMM: 10}
	})); err != nil {
		t.Fatal(err)
	}
	for _, y := range []float64{1380.5, 2760, 4120.25} {
		if _, err := s.AddCut(y, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetDocBackground(model.WhitenBackground); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPageBackground(model.TransparentPage); err != nil {
		t.Fatal(err)
	}
	projectPath = filepath.Join(dir, "mini-cad.svgpl")
	return dir, svgPath, projectPath, s
}

// reopen reads a project file and opens it in a new session the way the window
// does.
func reopen(t *testing.T, svc Services, path string) (*Session, *ProjectFile, []diag.Item, error) {
	t.Helper()
	pf, err := ReadProject(path)
	if err != nil {
		return nil, nil, nil, err
	}
	if pf.SourcePath == "" {
		return nil, pf, nil, nil
	}
	o, err := Open(svc, pf.SourcePath, pf.Project.Background)
	if err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	items, err := s.OpenProject(o.Document, pf)
	return s, pf, items, err
}

func encoded(t *testing.T, p model.Project) []byte {
	t.Helper()
	p.Source = model.Source{}
	b, err := project.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAProjectIsSavedAndOpenedAgainWithEverySetting(t *testing.T) {
	_, svgPath, projectPath, s := savedWork(t)
	before, err := os.ReadFile(svgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Dirty() {
		t.Fatal("changes are not unsaved changes")
	}
	if err := s.SaveProject(projectPath); err != nil {
		t.Fatal(err)
	}
	if s.Dirty() || s.ProjectPath() != projectPath {
		t.Errorf("after a save: dirty=%v path=%q", s.Dirty(), s.ProjectPath())
	}
	src := s.Project().Source
	if src.Path != "mini-cad.svg" || src.AbsolutePath != svgPath || len(src.SHA256) != 64 || src.Width != 1280 || src.Height != 5479 {
		t.Errorf("source = %+v", src)
	}

	svc, _ := fakeServices()
	r, pf, items, err := reopen(t, svc, projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("a project that matches its document reports %v", items)
	}
	if pf.SourcePath != svgPath {
		t.Errorf("document found at %q", pf.SourcePath)
	}
	if !bytes.Equal(encoded(t, r.Project()), encoded(t, s.Project())) {
		t.Errorf("the settings are not the same:\n%s\n%s", encoded(t, r.Project()), encoded(t, s.Project()))
	}
	if r.Dirty() || r.ProjectPath() != projectPath || r.CanUndo() {
		t.Errorf("a project just opened: dirty=%v path=%q undo=%v", r.Dirty(), r.ProjectPath(), r.CanUndo())
	}
	if got, want := r.Plan(), s.Plan(); len(got.Pages) != len(want.Pages) || got.Scale != want.Scale || got.ContentWidthMM != want.ContentWidthMM {
		t.Errorf("plan differs: %d pages scale %v, want %d pages scale %v", len(got.Pages), got.Scale, len(want.Pages), want.Scale)
	}
	if got := len(r.Cuts()); got != 3 {
		t.Errorf("%d cuts", got)
	}
	after, _ := os.ReadFile(svgPath)
	if !bytes.Equal(before, after) {
		t.Error("the SVG was modified")
	}
	// Saving again changes nothing in the file.
	first, _ := os.ReadFile(projectPath)
	if err := r.SaveProject(projectPath); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(projectPath)
	if !bytes.Equal(first, second) {
		t.Error("saving an unchanged project changed the file")
	}
}

func TestChangesAfterTheOpeningAreUnsavedChanges(t *testing.T) {
	_, _, projectPath, s := savedWork(t)
	if err := s.SaveProject(projectPath); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddCut(500, 0); err != nil {
		t.Fatal(err)
	}
	if !s.Dirty() {
		t.Error("a new cut is not an unsaved change")
	}
	s.Undo()
	if s.Dirty() {
		t.Error("undoing back to the saved state leaves a star")
	}
}

func TestSavingNeedsADocument(t *testing.T) {
	err := NewSession().SaveProject(filepath.Join(t.TempDir(), "x.svgpl"))
	if e, ok := diag.As(err); !ok || e.Code != "app.no_document" {
		t.Errorf("err = %v", err)
	}
}

func TestAProjectFindsItsDocumentWhereverBothWent(t *testing.T) {
	dir, svgPath, projectPath, s := savedWork(t)
	if err := s.SaveProject(projectPath); err != nil {
		t.Fatal(err)
	}
	// Both moved together: the relative path finds the document.
	moved := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-moved")
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(moved)
	pf, err := ReadProject(filepath.Join(moved, "mini-cad.svgpl"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(moved, "mini-cad.svg"); pf.SourcePath != want {
		t.Errorf("found %q, want %q", pf.SourcePath, want)
	}
	// The document gone: the settings are kept and the document is to be located.
	if err := os.Remove(filepath.Join(moved, "mini-cad.svg")); err != nil {
		t.Fatal(err)
	}
	pf, err = ReadProject(filepath.Join(moved, "mini-cad.svgpl"))
	if err != nil {
		t.Fatal(err)
	}
	if pf.SourcePath != "" || pf.Project.Paper.Format != "A3" || len(pf.Project.Cuts) != 3 {
		t.Errorf("project = %+v, source %q", pf.Project, pf.SourcePath)
	}
	_ = svgPath
}

func TestAChangedDocumentIsReportedAndNothingIsBlocked(t *testing.T) {
	_, svgPath, projectPath, s := savedWork(t)
	if err := s.SaveProject(projectPath); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(svgPath)
	if err := os.WriteFile(svgPath, append(data, []byte("\n<!-- edited -->\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, _ := fakeServices()
	r, _, items, err := reopen(t, svc, projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Code != "project.source_changed" || items[0].Level != diag.Warning {
		t.Errorf("items = %v", items)
	}
	if len(r.Cuts()) != 3 {
		t.Errorf("%d cuts kept", len(r.Cuts()))
	}
}

func TestCutsOutsideAShorterDocumentAreRemovedAndSayWhy(t *testing.T) {
	_, _, projectPath, s := savedWork(t)
	if err := s.SaveProject(projectPath); err != nil {
		t.Fatal(err)
	}
	short := Services{Load: func(svg []byte, path string) (engine.Document, error) {
		return &fakeDoc{w: 1280, h: 3000, loaded: svg}, nil
	}}
	pf, err := ReadProject(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	o, err := Open(short, pf.SourcePath, pf.Project.Background)
	if err != nil {
		t.Fatal(err)
	}
	if got := OutsideCuts(pf.Project, o.Document); len(got) != 1 {
		t.Fatalf("outside cuts = %v, want the one at 4120.25", got)
	}
	r := NewSession()
	items, err := r.OpenProject(o.Document, pf)
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, it := range items {
		codes[it.Code] = true
	}
	if !codes["project.cuts_outside"] || !codes["project.size_changed"] {
		t.Errorf("items = %v", items)
	}
	if cuts := r.Cuts(); len(cuts) != 2 || cuts[0].Position != 1380.5 || cuts[1].Position != 2760 {
		t.Errorf("cuts = %+v", r.Cuts())
	}
	if !r.Dirty() {
		t.Error("removing cuts is a change to save")
	}
}

func TestABackgroundInstructionThatFindsNothingIsDropped(t *testing.T) {
	_, _, projectPath, s := savedWork(t)
	if err := s.SaveProject(projectPath); err != nil {
		t.Fatal(err)
	}
	pf, err := ReadProject(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	pf.Project.Background.Element = "no-such-element"
	svc, _ := fakeServices()
	o, err := Open(svc, pf.SourcePath, pf.Project.Background)
	if err != nil {
		t.Fatal(err)
	}
	r := NewSession()
	items, err := r.OpenProject(o.Document, pf)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Code != "project.background_lost" {
		t.Errorf("items = %v", items)
	}
	if bg := r.Project().Background; bg.Document != model.KeepBackground || bg.Element != "" {
		t.Errorf("background = %+v", bg)
	}
}

func TestAProjectThatCannotBeLaidOutChangesNothing(t *testing.T) {
	_, _, projectPath, s := savedWork(t)
	if err := s.SaveProject(projectPath); err != nil {
		t.Fatal(err)
	}
	pf, err := ReadProject(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	pf.Project.Paper.Format = "Z9"
	svc, _ := fakeServices()
	o, err := Open(svc, pf.SourcePath, pf.Project.Background)
	if err != nil {
		t.Fatal(err)
	}
	other := session(t)
	before := other.Document()
	if _, err := other.OpenProject(o.Document, pf); err == nil {
		t.Fatal("a paper that does not exist was accepted")
	}
	if other.Document() != before || other.ProjectPath() != "" {
		t.Error("a refused project changed the session")
	}
}

// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"testing"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/model"
	"github.com/gillesmagneville/pageslice/internal/testsvg"
)

func openedWithBackground(t *testing.T) *Session {
	t.Helper()
	svc, _ := fakeServices()
	o, err := Prepare(svc, testsvg.Generate(testsvg.Options{}), "/tmp/doc.svg", model.Background{})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if err := s.Open(o.Document); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTheBackgroundOfTheDocumentIsDescribed(t *testing.T) {
	s := openedWithBackground(t)
	info, ok := s.DocumentBackground()
	if !ok || info.Fill == "" || info.Ref == "" {
		t.Fatalf("background = %+v, %v", info, ok)
	}
	if _, ok := session(t).DocumentBackground(); ok {
		t.Error("a document without analysis has a background")
	}
}

func TestChoosingTheBackgroundIsOneEntryOfTheHistoryAndKeepsTheScale(t *testing.T) {
	s := openedWithBackground(t)
	before := s.Plan().Scale
	for _, mode := range []model.DocBackground{model.WhitenBackground, model.RemoveBackground, model.KeepBackground} {
		if err := s.SetDocBackground(mode); err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if got := s.Project().Background.Document; got != mode {
			t.Errorf("background = %s, want %s", got, mode)
		}
		if s.Plan().Scale != before {
			t.Errorf("%s changed the scale: %v -> %v", mode, before, s.Plan().Scale)
		}
	}
	if err := s.SetDocBackground(model.WhitenBackground); err != nil {
		t.Fatal(err)
	}
	if s.Project().Background.Element == "" {
		t.Error("the instruction does not name the element")
	}
	if !s.Undo() || s.Project().Background.Document != model.KeepBackground {
		t.Errorf("undo left %s", s.Project().Background.Document)
	}
	if s.UndoLabel() != "undo.background_document" {
		t.Errorf("undo label = %q", s.UndoLabel())
	}
}

func TestADocumentWithoutBackgroundRefusesToReplaceIt(t *testing.T) {
	s := session(t) // no analysis, so no background
	err := s.SetDocBackground(model.WhitenBackground)
	if diag.CodeOf(err) != "background.none" {
		t.Errorf("error = %v, want background.none", err)
	}
	if s.Project().Background.Document != model.KeepBackground || s.CanUndo() {
		t.Error("a refused choice left a trace")
	}
	if err := s.SetDocBackground(model.KeepBackground); err != nil {
		t.Errorf("keeping the background is always possible: %v", err)
	}
	if err := s.SetDocBackground("sepia"); err == nil {
		t.Error("an unknown choice was accepted")
	}
}

func TestThePageBackgroundIsNotTheDocumentBackground(t *testing.T) {
	s := openedWithBackground(t)
	if err := s.SetPageBackground(model.TransparentPage); err != nil {
		t.Fatal(err)
	}
	p := s.Project().Background
	if p.Page != model.TransparentPage || p.Document != model.KeepBackground {
		t.Errorf("background = %+v", p)
	}
	if err := s.SetPageBackground("pink"); err == nil {
		t.Error("an unknown page background was accepted")
	}
}

func TestCheckExportSaysWhatIsWrongBeforeTheFileIsWritten(t *testing.T) {
	s := openedWithBackground(t)
	codes := func() map[string]bool {
		m := map[string]bool{}
		for _, it := range s.CheckExport() {
			m[it.Code] = true
		}
		return m
	}
	// One page of 200 mm by 855 mm does not fit an A4 sheet.
	if !codes()["export.outside_sheet"] {
		t.Errorf("a page taller than the sheet was not stopped: %v", s.CheckExport())
	}
	if _, err := s.AutoCuts(); err != nil {
		t.Fatal(err)
	}
	for _, it := range s.CheckExport() {
		if it.Level == diag.Failure {
			t.Errorf("with the automatic cuts the export is still stopped: %v", it)
		}
	}
	if NewSession().CheckExport()[0].Code != "app.no_document" {
		t.Error("no document, no export")
	}
}

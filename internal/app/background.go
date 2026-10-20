// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"github.com/gillesmagneville/svg-print-layout/internal/diag"
	"github.com/gillesmagneville/svg-print-layout/internal/export"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
)

// BackgroundInfo describes the rectangle the analysis took for the background
// of the document (ARCHITECTURE §8).
type BackgroundInfo struct {
	// Element is the kind of element: "rect" or "path".
	Element string
	// Fill is its fill as the document writes it.
	Fill string
	// Ref identifies it: its id, or its path of indices.
	Ref string
}

// DocumentBackground is the background rectangle of the open document, when it
// has one.
func (s *Session) DocumentBackground() (BackgroundInfo, bool) {
	if s.doc == nil || s.doc.Analysis == nil || s.doc.Analysis.Background == nil {
		return BackgroundInfo{}, false
	}
	b := s.doc.Analysis.Background
	return BackgroundInfo{Element: b.Element, Fill: b.Fill, Ref: b.Ref}, true
}

// SetDocBackground chooses what is done to the background of the document:
// keep it, fill it with white, or remove it. A document with no background
// rectangle has nothing to replace, and the choice is refused rather than
// recorded as one with no effect. The window loads the working copy again when
// the choice changes.
func (s *Session) SetDocBackground(mode model.DocBackground) error {
	if s.doc == nil {
		return errNoDocument
	}
	if !mode.Valid() {
		return diag.Fail("project.invalid", "field", "background.document")
	}
	info, found := s.DocumentBackground()
	if mode != model.KeepBackground && !found {
		return diag.Fail("background.none")
	}
	return s.Apply("undo.background_document", "", Edit(func(p *model.Project) {
		p.Background.Document = mode
		p.Background.Element = ""
		if found {
			p.Background.Element = info.Ref
		}
	}))
}

// SetPageBackground chooses what the sheet is painted with before the
// drawing: opaque white, or nothing. It is not the background of the document.
func (s *Session) SetPageBackground(mode model.PageBackground) error {
	if !mode.Valid() {
		return diag.Fail("project.invalid", "field", "background.page")
	}
	return s.Apply("undo.background_page", "", Edit(func(p *model.Project) { p.Background.Page = mode }))
}

// CheckExport runs the checks that come before an export (ARCHITECTURE §11).
func (s *Session) CheckExport() []diag.Item {
	if s.doc == nil || s.plan == nil {
		return []diag.Item{diag.New("app.no_document", 0)}
	}
	_, found := s.DocumentBackground()
	return export.Check(export.Input{
		Project:       s.project,
		Plan:          s.plan,
		HasBackground: found,
		Findings:      s.doc.Findings,
	})
}

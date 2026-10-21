// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/image-print-layout/internal/export"
	"github.com/gillesmagneville/image-print-layout/internal/i18n"
	"github.com/gillesmagneville/image-print-layout/internal/model"
)

// ---- background ---------------------------------------------------------------------

// buildBackground is the section of the three backgrounds that must not be
// mixed up (ARCHITECTURE §8): the one of the document, the one of the page,
// and the work area of the window, which is no setting.
func (s *settingsPanel) buildBackground() gtk.Widgetter {
	box := s.column()
	w := s.w

	s.bgNone = newLabel()
	s.bgNone.AddCSSClass("dim-label")
	s.bgFound = newLabel()
	s.bgFound.SetSelectable(true)

	s.bgKeep, s.bgWhiten, s.bgRemove = gtk.NewCheckButton(), gtk.NewCheckButton(), gtk.NewCheckButton()
	s.bgWhiten.SetGroup(s.bgKeep)
	s.bgRemove.SetGroup(s.bgKeep)
	for _, c := range []struct {
		btn  *gtk.CheckButton
		mode model.DocBackground
		key  string
	}{{s.bgKeep, model.KeepBackground, "bg.doc.keep"}, {s.bgWhiten, model.WhitenBackground, "bg.doc.whiten"}, {s.bgRemove, model.RemoveBackground, "bg.doc.remove"}} {
		c := c
		w.texts.bind(func(tr *i18n.Translator) { c.btn.SetLabel(tr.T(c.key)) })
		c.btn.ConnectToggled(func() {
			if s.updating || !c.btn.Active() {
				return
			}
			w.setDocBackground(c.mode)
		})
	}

	s.pageWhite, s.pageTransparent = gtk.NewCheckButton(), gtk.NewCheckButton()
	s.pageTransparent.SetGroup(s.pageWhite)
	for _, c := range []struct {
		btn  *gtk.CheckButton
		mode model.PageBackground
		key  string
	}{{s.pageWhite, model.WhitePage, "bg.page.white"}, {s.pageTransparent, model.TransparentPage, "bg.page.transparent"}} {
		c := c
		w.texts.bind(func(tr *i18n.Translator) { c.btn.SetLabel(tr.T(c.key)) })
		c.btn.ConnectToggled(func() {
			if s.updating || !c.btn.Active() {
				return
			}
			w.setPageBackground(c.mode)
		})
	}

	title := func(key string) *gtk.Label {
		l := newLabel()
		l.AddCSSClass("heading")
		w.texts.bind(func(tr *i18n.Translator) { l.SetLabel(tr.T(key)) })
		return l
	}
	box.Append(title("bg.doc.title"))
	box.Append(s.bgNone)
	box.Append(s.bgFound)
	box.Append(s.bgKeep)
	box.Append(s.bgWhiten)
	box.Append(s.bgRemove)
	box.Append(s.note("bg.doc.hint"))
	box.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
	box.Append(title("bg.page.title"))
	box.Append(s.pageWhite)
	box.Append(s.pageTransparent)
	box.Append(s.note("bg.page.hint"))
	box.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
	box.Append(s.note("bg.workspace.hint"))
	return box
}

// setDocBackground carries out the choice of the background of the document.
func (w *window) setDocBackground(mode model.DocBackground) {
	if err := w.session.SetDocBackground(mode); err != nil {
		w.refuse(err)
	} else {
		w.clearRefusal()
	}
	w.refreshAll()
}

// setPageBackground carries out the choice of the background of the page.
func (w *window) setPageBackground(mode model.PageBackground) {
	if err := w.session.SetPageBackground(mode); err != nil {
		w.refuse(err)
	} else {
		w.clearRefusal()
	}
	w.refreshAll()
}

// syncBackground shows the project and the analysis in the section.
func (s *settingsPanel) syncBackground() {
	tr := s.tr()
	project := s.w.session.Project().Background
	info, found := s.w.session.DocumentBackground()
	s.updating = true
	defer func() { s.updating = false }()

	s.bgNone.SetLabel(tr.T("bg.doc.none"))
	s.bgNone.SetVisible(s.w.session.Document() != nil && !found)
	s.bgFound.SetVisible(found)
	if found {
		element := tr.T("bg.element.rect")
		if info.Element == "path" {
			element = tr.T("bg.element.path")
		}
		s.bgFound.SetLabel(tr.T("bg.doc.found", "element", element, "fill", info.Fill))
	}
	// Without a background rectangle there is nothing to replace or remove.
	s.bgWhiten.SetSensitive(found)
	s.bgRemove.SetSensitive(found)
	switch project.Document {
	case model.WhitenBackground:
		s.bgWhiten.SetActive(true)
	case model.RemoveBackground:
		s.bgRemove.SetActive(true)
	default:
		s.bgKeep.SetActive(true)
	}
	if project.Page == model.TransparentPage {
		s.pageTransparent.SetActive(true)
	} else {
		s.pageWhite.SetActive(true)
	}
}

// ---- export ----------------------------------------------------------------------------

func (s *settingsPanel) buildExport() gtk.Widgetter {
	box := s.column()
	w := s.w

	s.exportSummary = newLabel()
	s.exportSummary.AddCSSClass("heading")
	s.exportState = newLabel()

	s.exportButton = gtk.NewButton()
	s.exportButton.SetActionName("win.export")
	s.exportButton.AddCSSClass("suggested-action")
	preview := gtk.NewButton()
	preview.ConnectClicked(func() { w.showSheets(0) })
	w.texts.bind(func(tr *i18n.Translator) {
		s.exportButton.SetLabel(tr.T("action.export"))
		preview.SetLabel(tr.T("export.preview"))
	})

	s.exportProgress = gtk.NewProgressBar()
	s.exportLabel = newLabel()
	s.exportLabel.AddCSSClass("caption")
	cancel := gtk.NewButton()
	w.texts.bind(func(tr *i18n.Translator) { cancel.SetLabel(tr.T("export.cancel")) })
	cancel.ConnectClicked(w.cancelExportRun)
	s.exportBar = gtk.NewBox(gtk.OrientationVertical, 4)
	s.exportBar.Append(s.exportProgress)
	s.exportBar.Append(s.exportLabel)
	s.exportBar.Append(cancel)
	s.exportBar.SetVisible(false)

	box.Append(s.exportSummary)
	box.Append(s.exportState)
	box.Append(s.exportButton)
	box.Append(preview)
	box.Append(s.exportBar)
	box.Append(s.note("export.notes"))
	return box
}

// syncExport shows what the export would write, and what stands in its way.
func (s *settingsPanel) syncExport() {
	tr := s.tr()
	plan := s.w.session.Plan()
	has := plan != nil && s.w.session.Document() != nil
	s.exportSummary.SetVisible(has)
	s.exportState.SetVisible(has)
	s.exportButton.SetSensitive(has && !s.w.exporting)
	if !has {
		return
	}
	s.exportSummary.SetLabel(tr.N("export.summary", len(plan.Pages),
		"w", shown(tr, plan.Sheet.WidthMM), "h", shown(tr, plan.Sheet.HeightMM)))
	items := s.w.session.CheckExport()
	key := "export.state.ready"
	css := "success"
	switch {
	case export.Blocked(items):
		key, css = "export.state.blocked", "error"
	case export.NeedsConfirmation(items):
		key, css = "export.state.warnings", "warning"
	}
	for _, c := range []string{"success", "warning", "error"} {
		s.exportState.RemoveCSSClass(c)
	}
	s.exportState.AddCSSClass(css)
	s.exportState.SetLabel(tr.T(key))
}

// showExportProgress shows the progress of the export.
func (s *settingsPanel) showExportProgress(done, total int) {
	tr := s.tr()
	s.exportBar.SetVisible(true)
	if total > 0 {
		s.exportProgress.SetFraction(float64(done) / float64(total))
	}
	s.exportLabel.SetLabel(tr.T("export.progress", "done", done, "total", total))
}

// hideExportProgress takes the progress away.
func (s *settingsPanel) hideExportProgress() { s.exportBar.SetVisible(false) }

// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/svg-print-layout/internal/diag"
	"github.com/gillesmagneville/svg-print-layout/internal/export"
	"github.com/gillesmagneville/svg-print-layout/internal/i18n"
)

// The export (ARCHITECTURE §6 and §11): the checks on the plan, the choice of
// the file, the writing off the thread of the window with a progress that can
// be stopped, and the checks on the file that was written.

// exportPDF starts the export: it checks the plan first, then asks where to
// write.
func (w *window) exportPDF() {
	if w.session.Document() == nil || w.exporting {
		return
	}
	if w.rebuilding || w.loadedBG != w.session.Project().Background.Document {
		// The working copy is being prepared with the background just chosen:
		// the export waits for it, so that the file shows what the preview shows.
		w.afterRebuild = append(w.afterRebuild, w.exportPDF)
		return
	}
	items := w.session.CheckExport()
	switch {
	case export.Blocked(items):
		w.showReport("dialog.export_blocked.title", "dialog.export_blocked.detail", items, nil)
	case export.NeedsConfirmation(items):
		w.showReport("dialog.export_confirm.title", "dialog.export_confirm.detail", items, []dialogButton{
			{label: "dialog.cancel"},
			{label: "dialog.export_confirm.go", primary: true, run: w.chooseExportFile},
		})
	default:
		w.chooseExportFile()
	}
}

// chooseExportFile asks where to write the PDF, then writes it.
func (w *window) chooseExportFile() {
	tr := w.texts.tr
	doc := w.session.Document()
	if doc == nil {
		return
	}
	filter := gtk.NewFileFilter()
	filter.SetName(tr.T("dialog.filter.pdf"))
	filter.AddMIMEType("application/pdf")
	filter.AddPattern("*.pdf")
	filter.AddPattern("*.PDF")
	filters := gio.NewListStore(gtk.GTypeFileFilter)
	filters.Append(filter.Object)

	dialog := gtk.NewFileDialog()
	dialog.SetTitle(tr.T("dialog.export.title"))
	dialog.SetFilters(filters)
	dialog.SetDefaultFilter(filter)
	base := baseName(doc.Path)
	dialog.SetInitialName(strings.TrimSuffix(base, filepath.Ext(base)) + ".pdf")
	folder := w.lastExportDir
	if folder == "" {
		folder = filepath.Dir(doc.Path)
	}
	dialog.SetInitialFolder(gio.NewFileForPath(folder))
	dialog.Save(context.Background(), &w.win.Window, func(res gio.AsyncResulter) {
		file, err := dialog.SaveFinish(res)
		if err != nil || file == nil {
			return // cancelled
		}
		path := file.Path()
		if !strings.EqualFold(filepath.Ext(path), ".pdf") {
			path += ".pdf"
		}
		w.runExport(path)
	})
}

// runExport writes the PDF of the plan the window shows, to path.
func (w *window) runExport(path string) {
	doc := w.engineDoc
	plan := w.session.Plan()
	if doc == nil || plan == nil || w.exporting {
		return
	}
	project := w.session.Project()
	name := baseName(path)
	w.exporting = true
	w.cancelExport.Store(false)
	w.refreshExportState()
	w.setStatus("status.exporting", "name", name)
	w.panel.showExportProgress(0, len(plan.Pages))
	w.busy++
	w.engine.do(func() {
		res, err := export.Run(export.Request{
			Doc: doc, Plan: plan, Project: project, Path: path,
			Progress: func(done, total int) bool {
				glib.IdleAdd(func() { w.panel.showExportProgress(done, total) })
				return !w.cancelExport.Load()
			},
		})
		glib.IdleAdd(func() {
			w.busy--
			w.exporting = false
			w.panel.hideExportProgress()
			w.exportDone(path, res, err)
			w.refreshExportState()
		})
	})
}

// exportDone says how the export ended.
func (w *window) exportDone(path string, res *export.Result, err error) {
	tr := w.texts.tr
	var verify *export.VerifyError
	switch {
	case err == nil:
		w.lastExportDir = filepath.Dir(path)
		kb := int(math.Max(1, math.Round(float64(res.Bytes)/1024)))
		w.setStatus("status.exported", plural{res.Pages}, "name", baseName(path), "kb", kb)
	case export.IsCancelled(err):
		w.setStatus("status.export_cancelled")
	case errors.As(err, &verify):
		w.setStatus("status.ready")
		w.showReport("dialog.export_verify.title", "dialog.export_verify.detail", verify.Items, nil)
	default:
		w.setStatus("status.ready")
		w.showMessage(tr.T("dialog.export_failed.title"), tr.T("dialog.export_failed.detail", "reason", errorText(tr, err)))
	}
}

// cancelExportRun stops the export in progress, at the end of the page being
// drawn.
func (w *window) cancelExportRun() { w.cancelExport.Store(true) }

// ---- the reports of the checks ---------------------------------------------------------------

// dialogButton is a button of a report: the key of its text, whether it is the
// answer the dialog suggests, and what it does once the dialog is closed.
type dialogButton struct {
	label   string
	primary bool
	run     func()
}

// itemText writes a diagnostic for a list: the page it is about, then what it
// says.
func itemText(tr *i18n.Translator, it diag.Item) string {
	text := diagText(tr, it.Code, it.Args)
	if it.Page > 0 {
		return tr.T("pages.row.title", "n", it.Page) + " : " + text
	}
	return text
}

// showReport opens a modal window with a heading, an explanation, the list of
// diagnostics and buttons. Without buttons it has one to close it.
func (w *window) showReport(headingKey, detailKey string, items []diag.Item, buttons []dialogButton) *gtk.Window {
	tr := w.texts.tr
	return w.showDialog(tr.T(headingKey), tr.T(detailKey), items, buttons)
}

// showDialog is showReport for texts that are already written.
func (w *window) showDialog(heading, detailText string, items []diag.Item, buttons []dialogButton) *gtk.Window {
	tr := w.texts.tr
	dialog := gtk.NewWindow()
	dialog.SetApplication(w.app)
	dialog.SetTransientFor(&w.win.Window)
	dialog.SetModal(true)
	dialog.SetTitle("SVG Print Layout")
	dialog.SetDefaultSize(520, -1)

	column := gtk.NewBox(gtk.OrientationVertical, 12)
	column.SetMarginTop(18)
	column.SetMarginBottom(18)
	column.SetMarginStart(18)
	column.SetMarginEnd(18)

	title := gtk.NewLabel(heading)
	title.AddCSSClass("title-3")
	title.SetXAlign(0)
	title.SetWrap(true)
	detail := gtk.NewLabel(detailText)
	detail.SetXAlign(0)
	detail.SetWrap(true)
	detail.SetMaxWidthChars(60)

	list := gtk.NewBox(gtk.OrientationVertical, 8)
	for _, it := range items {
		l := gtk.NewLabel(itemText(tr, it))
		l.SetXAlign(0)
		l.SetWrap(true)
		l.SetSelectable(true)
		l.SetMaxWidthChars(64)
		switch it.Level {
		case diag.Failure:
			l.AddCSSClass("error")
		case diag.Warning:
			l.AddCSSClass("warning")
		}
		list.Append(l)
	}
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetMinContentHeight(60)
	scroll.SetMaxContentHeight(280)
	scroll.SetPropagateNaturalHeight(true)
	scroll.SetChild(list)

	if len(buttons) == 0 {
		buttons = []dialogButton{{label: "dialog.close", primary: true}}
	}
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	row.SetHAlign(gtk.AlignEnd)
	var primary *gtk.Button
	for _, b := range buttons {
		b := b
		button := gtk.NewButtonWithLabel(tr.T(b.label))
		button.ConnectClicked(func() {
			dialog.Close()
			if b.run != nil {
				b.run()
			}
		})
		if b.primary {
			button.AddCSSClass("suggested-action")
			primary = button
		}
		row.Append(button)
	}

	column.Append(title)
	column.Append(detail)
	if len(items) > 0 {
		column.Append(scroll)
	}
	column.Append(row)
	dialog.SetChild(column)
	if primary != nil {
		dialog.SetDefaultWidget(primary)
	}
	dialog.Present()
	w.lastReport = dialog
	return dialog
}

// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/pageslice/internal/app"
	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/i18n"
)

// The project file in the window (ARCHITECTURE §9 and §10): open and save, the
// document that has to be located when it has moved, what a project no longer
// matches, and the star of unsaved changes with the question asked before they
// would be lost.

// projectExt is the extension of a project file.
const projectExt = app.ProjectExtension

// buildProjectMenu is the menu of the Project button.
func (w *window) buildProjectMenu(tr *i18n.Translator) *gio.Menu {
	menu := gio.NewMenu()
	menu.Append(tr.T("menu.project.open"), "win.open-project")
	menu.Append(tr.T("menu.project.save"), "win.save-project")
	menu.Append(tr.T("menu.project.save_as"), "win.save-project-as")
	return menu
}

// refreshProjectState enables saving when there is a document to save.
func (w *window) refreshProjectState() {
	if w.saveAct == nil {
		return
	}
	has := w.session.Document() != nil
	w.saveAct.SetEnabled(has)
	w.saveAsAct.SetEnabled(has)
}

func projectFilter(tr *i18n.Translator) *gio.ListStore {
	filter := gtk.NewFileFilter()
	filter.SetName(tr.T("dialog.filter.project"))
	// The files of the program's first name (.svgpl) open too; they are always
	// written back under the current extension.
	for _, ext := range []string{app.ProjectExtension, app.LegacyProjectExtension} {
		filter.AddPattern("*" + ext)
		filter.AddPattern("*" + strings.ToUpper(ext))
	}
	filters := gio.NewListStore(gtk.GTypeFileFilter)
	filters.Append(filter.Object)
	return filters
}

// ---- opening -------------------------------------------------------------------------------

// openProjectDialog asks for a project file.
func (w *window) openProjectDialog() {
	tr := w.texts.tr
	dialog := gtk.NewFileDialog()
	dialog.SetTitle(tr.T("dialog.open_project.title"))
	dialog.SetFilters(projectFilter(tr))
	if doc := w.session.Document(); doc != nil {
		dialog.SetInitialFolder(gio.NewFileForPath(filepath.Dir(doc.Path)))
	}
	dialog.Open(context.Background(), &w.win.Window, func(res gio.AsyncResulter) {
		file, err := dialog.OpenFinish(res)
		if err != nil || file == nil {
			return // cancelled
		}
		w.openProject(file.Path())
	})
}

// openProject reads a project file and opens its document. A document that is
// not where the project says is asked for, and the settings wait for it.
func (w *window) openProject(path string) {
	tr := w.texts.tr
	pf, err := app.ReadProject(path)
	if err != nil {
		w.showMessage(tr.T("dialog.error.open_project"),
			tr.T("dialog.error.open_project.detail", "path", path, "reason", errorText(tr, err)))
		return
	}
	if pf.SourcePath == "" {
		w.locateDocument(pf)
		return
	}
	w.loadProject(pf)
}

// locateDocument says that the document of a project is not to be found and
// offers to look for it.
func (w *window) locateDocument(pf *app.ProjectFile) {
	tr := w.texts.tr
	svg := baseName(pf.Project.Source.AbsolutePath)
	if pf.Project.Source.AbsolutePath == "" {
		svg = baseName(pf.Project.Source.Path)
	}
	w.showDialog(tr.T("dialog.locate.title"),
		tr.T("dialog.locate.detail", "project", baseName(pf.Path), "svg", svg), nil,
		[]dialogButton{
			{label: "dialog.cancel"},
			{label: "dialog.locate.go", primary: true, run: func() { w.pickDocumentFor(pf) }},
		})
}

// pickDocumentFor asks for the SVG a project is about, then opens the project.
func (w *window) pickDocumentFor(pf *app.ProjectFile) {
	tr := w.texts.tr
	filter := gtk.NewFileFilter()
	filter.SetName(tr.T("dialog.filter.svg"))
	filter.AddMIMEType("image/svg+xml")
	filter.AddPattern("*.svg")
	filter.AddPattern("*.SVG")
	filters := gio.NewListStore(gtk.GTypeFileFilter)
	filters.Append(filter.Object)
	dialog := gtk.NewFileDialog()
	dialog.SetTitle(tr.T("dialog.locate.pick"))
	dialog.SetFilters(filters)
	dialog.SetDefaultFilter(filter)
	dialog.SetInitialFolder(gio.NewFileForPath(filepath.Dir(pf.Path)))
	dialog.Open(context.Background(), &w.win.Window, func(res gio.AsyncResulter) {
		file, err := dialog.OpenFinish(res)
		if err != nil || file == nil {
			return // cancelled
		}
		pf.SourcePath = file.Path()
		w.loadProject(pf)
	})
}

// loadProject opens the document of a project off the thread of the window,
// with the background the project chose.
func (w *window) loadProject(pf *app.ProjectFile) {
	w.setStatus("status.loading", "name", baseName(pf.SourcePath))
	w.busy++
	w.engine.do(func() {
		opened, err := app.Open(w.services, pf.SourcePath, pf.Project.Background)
		glib.IdleAdd(func() {
			w.busy--
			if err != nil {
				w.setStatus("status.ready")
				w.showOpenError(pf.SourcePath, err)
				return
			}
			w.adoptProject(opened, pf)
		})
	})
}

// adoptProject asks before removing cuts that are outside the document, then
// puts the project and its document in place.
func (w *window) adoptProject(o *app.Opened, pf *app.ProjectFile) {
	outside := app.OutsideCuts(pf.Project, o.Document)
	if len(outside) == 0 {
		w.finishProject(o, pf)
		return
	}
	w.setStatus("status.ready")
	w.showReport("dialog.cuts_outside.title", "dialog.cuts_outside.detail",
		[]diag.Item{diag.New("project.cuts_outside", 0, "count", len(outside))},
		[]dialogButton{
			{label: "dialog.cancel", run: func() { w.engine.do(o.Engine.Close) }},
			{label: "dialog.cuts_outside.go", primary: true, run: func() { w.finishProject(o, pf) }},
		})
}

// finishProject makes the project and its document the current ones, and says
// what the project no longer matches.
func (w *window) finishProject(o *app.Opened, pf *app.ProjectFile) {
	tr := w.texts.tr
	items, err := w.session.OpenProject(o.Document, pf)
	if err != nil {
		w.engine.do(o.Engine.Close)
		w.setStatus("status.ready")
		w.showMessage(tr.T("dialog.error.open_project"),
			tr.T("dialog.error.open_project.detail", "path", pf.Path, "reason", errorText(tr, err)))
		return
	}
	w.install(o)
	w.setStatus("status.project_loaded", "name", baseName(pf.Path))
	if len(items) > 0 {
		w.showReport("dialog.project_warnings.title", "dialog.project_warnings.detail", items, nil)
	}
}

// ---- saving ----------------------------------------------------------------------------------

// saveProject saves to the file of the project, or asks for one the first time.
func (w *window) saveProject() {
	w.saveThen(nil)
}

// saveThen saves, then runs next once the project is written. Nothing runs if
// the save is cancelled or fails.
func (w *window) saveThen(next func()) {
	if w.session.Document() == nil {
		return
	}
	if path := w.session.ProjectPath(); path != "" {
		if w.writeProject(path) && next != nil {
			next()
		}
		return
	}
	w.saveProjectAs(next)
}

// saveProjectAs asks where to write the project, then writes it and runs next.
func (w *window) saveProjectAs(next func()) {
	tr := w.texts.tr
	doc := w.session.Document()
	if doc == nil {
		return
	}
	dialog := gtk.NewFileDialog()
	dialog.SetTitle(tr.T("dialog.save_project.title"))
	dialog.SetFilters(projectFilter(tr))
	name := baseName(doc.Path)
	folder := filepath.Dir(doc.Path)
	if path := w.session.ProjectPath(); path != "" {
		name, folder = baseName(path), filepath.Dir(path)
	}
	dialog.SetInitialName(strings.TrimSuffix(name, filepath.Ext(name)) + projectExt)
	dialog.SetInitialFolder(gio.NewFileForPath(folder))
	dialog.Save(context.Background(), &w.win.Window, func(res gio.AsyncResulter) {
		file, err := dialog.SaveFinish(res)
		if err != nil || file == nil {
			return // cancelled
		}
		path := file.Path()
		if !strings.EqualFold(filepath.Ext(path), projectExt) {
			path += projectExt
		}
		if w.writeProject(path) && next != nil {
			next()
		}
	})
}

// writeProject writes the project, and says so or says why it could not.
func (w *window) writeProject(path string) bool {
	tr := w.texts.tr
	if err := w.session.SaveProject(path); err != nil {
		w.showMessage(tr.T("dialog.error.save_project"),
			tr.T("dialog.error.save_project.detail", "path", path, "reason", errorText(tr, err)))
		return false
	}
	w.updateTitle()
	w.setStatus("status.project_saved", "name", baseName(path))
	return true
}

// ---- unsaved changes ---------------------------------------------------------------------

// guardUnsaved runs next at once when nothing would be lost, and otherwise asks
// whether to save first, to go on without saving or to stop.
func (w *window) guardUnsaved(next func()) {
	if w.session.Document() == nil || !w.session.Dirty() {
		next()
		return
	}
	w.showReport("dialog.unsaved.title", "dialog.unsaved.detail", nil, []dialogButton{
		{label: "dialog.cancel"},
		{label: "dialog.unsaved.discard", run: next},
		{label: "dialog.unsaved.save", primary: true, run: func() { w.saveThen(next) }},
	})
}

// closeRequested is called when the window is asked to close. It stops the
// closing (true) to ask about unsaved changes, and lets it go on (false)
// otherwise.
func (w *window) closeRequested() bool {
	if w.forceClose || w.session.Document() == nil || !w.session.Dirty() {
		return false
	}
	w.guardUnsaved(func() {
		w.forceClose = true
		w.win.Close()
	})
	return true
}

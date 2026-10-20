// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"

	"github.com/gillesmagneville/svg-print-layout/internal/app"
	"github.com/gillesmagneville/svg-print-layout/internal/engine"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
)

// The background of the document is an instruction on the working copy
// (ARCHITECTURE §8): when the project says another one than the engine
// document was loaded with, the working copy is loaded again, off the thread of
// the window, and the drawing is made again. The preview shows the old drawing
// until the new one is there. The project is the source of truth, so undo and
// redo need no code of their own.

// syncBackground makes the engine document follow the background the project
// asks for.
func (w *window) syncBackground() {
	doc := w.session.Document()
	if doc == nil || w.engineDoc == nil || w.rebuilding {
		return
	}
	want := w.session.Project().Background
	if want.Document == w.loadedBG {
		return
	}
	w.rebuilding = true
	w.busy++
	svc := w.services
	w.engine.do(func() {
		fresh, err := app.Rebuild(svc, doc, want)
		glib.IdleAdd(func() {
			w.busy--
			w.rebuilding = false
			w.backgroundReady(doc, want, fresh, err)
		})
	})
}

// backgroundReady takes the working copy loaded with another background in
// place of the one the preview and the export use.
func (w *window) backgroundReady(doc *app.Document, want model.Background, fresh engine.Document, err error) {
	if w.session.Document() != doc {
		// Another document was opened while this one was being prepared.
		if err == nil {
			w.engine.do(fresh.Close)
		}
		return
	}
	if err != nil {
		// Go back to the background the engine document has: the project must
		// say what the preview shows.
		back := w.loadedBG
		_ = w.session.Apply("undo.background_document", "", app.Edit(func(p *model.Project) { p.Background.Document = back }))
		w.setStatus("status.background_failed", "reason", err.Error())
		w.refreshAll()
		w.runAfterRebuild()
		return
	}
	previous := w.engineDoc
	w.engineDoc = fresh
	w.loadedBG = want.Document
	w.docRev++
	if previous != nil {
		w.engine.do(previous.Close)
	}
	w.renderView(true)
	w.sheets.update()
	// The project may have changed again while the copy was being loaded.
	w.syncBackground()
	w.runAfterRebuild()
}

// runAfterRebuild runs what waited for the working copy to be ready, when it is.
func (w *window) runAfterRebuild() {
	if w.rebuilding || w.engineDoc == nil || w.loadedBG != w.session.Project().Background.Document {
		return
	}
	jobs := w.afterRebuild
	w.afterRebuild = nil
	for _, run := range jobs {
		run()
	}
}

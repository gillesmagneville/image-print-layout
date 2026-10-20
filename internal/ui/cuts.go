// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

// wireCuts gives the preview the commands of the window for the gestures on the
// cuts. The session does the work; the window shows the result.
func (w *window) wireCuts() {
	w.pv.hooks = cutHooks{
		add:        w.addCutAt,
		drag:       w.dragCut,
		dragEnd:    w.endCutDrag,
		remove:     w.deleteCut,
		nudge:      w.nudgeCut,
		moveTo:     w.moveCutTo,
		selectCut:  w.selectCut,
		selectPage: w.selectPage,
	}
}

// snapRadius is the radius of the snapping in document units, 0 when it is off.
func (w *window) snapRadius(snap bool) float64 {
	if !snap {
		return 0
	}
	return w.pv.snapRadius()
}

// done shows the result of a command on the cuts: the refusal, or the new state.
func (w *window) done(err error) {
	if err != nil {
		w.refuse(err)
	} else {
		w.clearRefusal()
	}
	w.refreshAll()
}

func (w *window) addCutAt(y float64, snap bool) {
	id, err := w.session.AddCut(y, w.snapRadius(snap))
	if err == nil {
		w.selCut, w.selPage = id, 0
	}
	w.done(err)
}

// addCutInView adds a cut in the middle of what the preview shows, in a void
// when one is near.
func (w *window) addCutInView() {
	if w.session.Document() == nil {
		return
	}
	vis := w.pv.view.Visible()
	w.addCutAt(vis.Y+vis.H/2, true)
}

// dragCut is a step of a drag of a cut; it gives the position the cut took.
func (w *window) dragCut(id int, y float64, snap bool) float64 {
	taken, err := w.session.DragCut(id, y, w.snapRadius(snap))
	if err != nil {
		for _, c := range w.session.Cuts() {
			if c.ID == id {
				taken = c.Position
			}
		}
	}
	w.done(err)
	return taken
}

func (w *window) endCutDrag() {
	w.session.EndDrag()
	w.updateHistory()
}

func (w *window) deleteCut(id int) {
	err := w.session.DeleteCut(id)
	if err == nil && w.selCut == id {
		w.selCut = 0
	}
	w.done(err)
}

func (w *window) nudgeCut(id int, delta float64) {
	w.done(w.session.NudgeCut(id, delta))
}

// moveCutTo puts a cut at a distance from the top of the drawing, in
// millimetres, as typed in the menu of the cut.
func (w *window) moveCutTo(id int, mm float64) {
	plan := w.session.Plan()
	if plan == nil || plan.Scale <= 0 {
		return
	}
	w.done(w.session.MoveCut(id, mm/plan.Scale))
}

// autoCuts replaces the cuts by the proposal of the layout.
func (w *window) autoCuts() {
	if w.session.Document() == nil {
		return
	}
	prop, err := w.session.AutoCuts()
	w.done(err)
	if err != nil {
		return
	}
	if len(prop.Positions) == 0 {
		w.setStatus("status.auto_cuts.none")
		return
	}
	w.setStatus("status.auto_cuts", plural{len(prop.Positions)}, "pages", prop.Pages)
}

// ---- selection ------------------------------------------------------------------

// selectCut selects a cut of the preview (0 for none): the line, its row in the
// list.
func (w *window) selectCut(id int) {
	w.selCut = id
	if id != 0 {
		w.selPage = 0
	}
	w.showSelection()
}

// selectPage selects a page (0 for none).
func (w *window) selectPage(n int) {
	w.selPage = n
	if n != 0 {
		w.selCut = 0
	}
	w.showSelection()
	if n != 0 {
		w.pages.scrollToSelection()
	}
}

// selectPageFromList is the selection of a row of the list of pages: the
// preview moves to show the page.
func (w *window) selectPageFromList(n int) {
	w.selPage = n
	if n != 0 {
		w.selCut = 0
	}
	w.showSelection()
	if plan := w.session.Plan(); plan != nil && n > 0 && n <= len(plan.Pages) {
		page := plan.Pages[n-1]
		w.pv.reveal(page.Top, page.Bottom)
	}
}

// selectCutFromList is the selection of a row of the list of cuts.
func (w *window) selectCutFromList(id int) {
	w.selCut = id
	if id != 0 {
		w.selPage = 0
	}
	w.showSelection()
	for _, c := range w.session.Cuts() {
		if c.ID == id {
			w.pv.reveal(c.Position-20, c.Position+20)
		}
	}
}

// showSelection shows the selection in the preview and the lists.
func (w *window) showSelection() {
	w.pv.setSelection(w.selCut, w.selPage)
	w.pages.selectInLists()
}

// pruneSelection forgets a selection that no longer exists.
func (w *window) pruneSelection() {
	if plan := w.session.Plan(); plan == nil || w.selPage > len(plan.Pages) {
		w.selPage = 0
	}
	found := false
	for _, c := range w.cuts {
		if c.ID == w.selCut {
			found = true
		}
	}
	if !found {
		w.selCut = 0
	}
}

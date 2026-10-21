// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/image-print-layout/internal/app"
	"github.com/gillesmagneville/image-print-layout/internal/i18n"
)

// The cuts on the preview (ARCHITECTURE §9): the lines are drawn by the
// preview and moved by the mouse and the keyboard; what a gesture means for the
// project is the window's business, which the preview reaches through hooks.

// cutHooks are what the preview asks of the window when the user acts on the
// cuts or the pages. Positions are in document units.
type cutHooks struct {
	// add asks for a cut at y; snap says whether it may move to a void.
	add func(y float64, snap bool)
	// dragBegin, drag and dragEnd are the three moments of a gesture on a cut.
	// drag gives the position the cut took, which is not always where the
	// pointer is: a void it stuck to, or the neighbour it stopped against.
	dragBegin func(id int)
	drag      func(id int, y float64, snap bool) float64
	dragEnd   func()
	remove    func(id int)
	// nudge moves a cut by a number of units (a key).
	nudge func(id int, delta float64)
	// moveTo puts a cut at a distance from the top of the drawing, in
	// millimetres (the field of the menu).
	moveTo     func(id int, mm float64)
	selectCut  func(id int)
	selectPage func(n int)
}

const (
	// cutHit is how close, in pixels, the pointer must be to a line to hold it.
	cutHit = 6.0
	// cutOverhang is how far a line goes beyond the sheet on each side.
	cutOverhang = 14.0
	// snapPixels is the radius of the snapping to the voids, in pixels (Alt
	// turns the snapping off).
	snapPixels = 8.0
	// clickSlop is how far, in pixels, a press may move and still be a click.
	clickSlop = 3.0
)

// dragMode says what the gesture in progress does.
type dragMode int

const (
	dragNone dragMode = iota
	dragPan
	dragCut
)

// cutState is what the preview remembers of the cuts: the cuts, which one is
// selected or under the pointer, and the gesture in progress.
type cutState struct {
	hooks cutHooks
	cuts  []app.CutView

	selCut, hoverCut int
	selPage          int

	mode           dragMode
	dragID         int
	dragStart      float64 // the position of the cut when the gesture began
	dragTaken      float64
	pressX, pressY float64
	pressButton    uint
	menu           *gtk.Popover
	menuField      *numField
}

// setCuts gives the cuts to draw. A selected cut that is gone is forgotten.
func (p *preview) setCuts(cuts []app.CutView) {
	p.cuts = cuts
	found := false
	for _, c := range cuts {
		if c.ID == p.selCut {
			found = true
		}
	}
	if !found {
		p.selCut = 0
	}
	p.marks.QueueDraw()
}

// setSelection marks the selected cut and page (0 for none).
func (p *preview) setSelection(cut, page int) {
	p.selCut, p.selPage = cut, page
	p.marks.QueueDraw()
}

// sheetX is the horizontal extent of the document on the screen.
func (p *preview) sheetX() (x0, x1 float64) {
	x0, _ = p.view.ToScreen(0, 0)
	x1, _ = p.view.ToScreen(p.view.DocW, 0)
	return math.Round(x0), math.Round(x1)
}

// cutAt is the cut whose line passes within cutHit pixels of (x, y); the nearest
// when two do.
func (p *preview) cutAt(x, y float64) (app.CutView, bool) {
	if !p.hasDoc || p.view.Zoom <= 0 {
		return app.CutView{}, false
	}
	x0, x1 := p.sheetX()
	if x < x0-cutOverhang-4 || x > x1+cutOverhang+4 {
		return app.CutView{}, false
	}
	var best app.CutView
	bestDist := cutHit + 1
	for _, c := range p.cuts {
		_, sy := p.view.ToScreen(0, c.Position)
		if d := math.Abs(sy - y); d <= cutHit && d < bestDist {
			best, bestDist = c, d
		}
	}
	return best, bestDist <= cutHit
}

// pageAt is the number of the page drawn at the screen height y, or 0.
func (p *preview) pageAt(y float64) int {
	if p.plan == nil {
		return 0
	}
	_, doc := p.view.ToDoc(0, y)
	for _, page := range p.plan.Pages {
		if doc >= page.Top && doc < page.Bottom {
			return page.Number
		}
	}
	return 0
}

// snapRadius is the radius of the snapping, in document units.
func (p *preview) snapRadius() float64 {
	if p.view.Zoom <= 0 {
		return 0
	}
	return snapPixels / p.view.Zoom
}

// ---- gestures -----------------------------------------------------------------

// beginDrag starts a gesture with a button pressed at (x, y): on a line of a
// cut it moves the cut, anywhere else it moves the view.
func (p *preview) beginDrag(x, y float64, button uint) {
	p.startOX, p.startOY = p.view.OX, p.view.OY
	p.pressX, p.pressY, p.pressButton = x, y, button
	p.mode = dragPan
	if button == 1 {
		if c, ok := p.cutAt(x, y); ok {
			p.mode, p.dragID, p.dragStart, p.dragTaken = dragCut, c.ID, c.Position, c.Position
			if p.hooks.dragBegin != nil {
				p.hooks.dragBegin(c.ID)
			}
			p.selectCut(c.ID)
			p.marks.SetCursorFromName("ns-resize")
			return
		}
	}
	p.marks.SetCursorFromName("grabbing")
}

// updateDrag follows the pointer, offset (dx, dy) pixels from where the
// gesture began. alt turns the snapping off.
func (p *preview) updateDrag(dx, dy float64, alt bool) {
	switch p.mode {
	case dragCut:
		if p.hooks.drag != nil {
			p.dragTaken = p.hooks.drag(p.dragID, p.dragStart+dy/p.view.Zoom, !alt)
		}
		p.marks.QueueDraw()
	case dragPan:
		p.fit = fitFree
		p.view.OX, p.view.OY = p.startOX, p.startOY
		p.view.PanPx(dx, dy)
		p.changed()
	}
}

// endDrag ends the gesture. A press that hardly moved is a click: on the
// drawing it selects the page under the pointer.
func (p *preview) endDrag(dx, dy float64) {
	mode := p.mode
	p.mode = dragNone
	switch mode {
	case dragCut:
		if p.hooks.dragEnd != nil {
			p.hooks.dragEnd()
		}
	case dragPan:
		if p.pressButton == 1 && math.Abs(dx) <= clickSlop && math.Abs(dy) <= clickSlop {
			p.clickAt(p.pressX, p.pressY)
		}
	}
	p.marks.SetCursorFromName("default")
	p.hover(p.pointerX, p.pointerY)
}

// clickAt selects what is under (x, y): a cut, or else the page.
func (p *preview) clickAt(x, y float64) {
	if c, ok := p.cutAt(x, y); ok {
		p.selectCut(c.ID)
		return
	}
	p.selectCut(0)
	if n := p.pageAt(y); n != 0 && p.hooks.selectPage != nil {
		p.hooks.selectPage(n)
	}
}

func (p *preview) selectCut(id int) {
	if p.hooks.selectCut != nil {
		p.hooks.selectCut(id)
	} else {
		p.setSelection(id, 0)
	}
}

// doubleClickAt adds a cut where the drawing was double-clicked, unless the
// click is on a line.
func (p *preview) doubleClickAt(x, y float64, alt bool) {
	if _, ok := p.cutAt(x, y); ok || !p.hasDoc {
		return
	}
	_, doc := p.view.ToDoc(0, y)
	if doc <= 0 || doc >= p.view.DocH {
		return
	}
	if p.hooks.add != nil {
		p.hooks.add(doc, !alt)
	}
}

// hover shows the line under the pointer.
func (p *preview) hover(x, y float64) {
	p.pointerX, p.pointerY = x, y
	if p.mode != dragNone {
		return
	}
	id := 0
	if c, ok := p.cutAt(x, y); ok {
		id = c.ID
	}
	if id == p.hoverCut {
		return
	}
	p.hoverCut = id
	if id != 0 {
		p.marks.SetCursorFromName("ns-resize")
	} else {
		p.marks.SetCursorFromName("default")
	}
	p.marks.QueueDraw()
}

// keyPressed handles the keys that act on the selected cut: Delete removes it,
// the arrows move it by a unit (ten with Shift, a tenth of a millimetre with
// Ctrl), Escape lets it go.
func (p *preview) keyPressed(keyval uint, state gdk.ModifierType) bool {
	switch keyval {
	case gdk.KEY_Escape:
		if p.selCut == 0 && p.selPage == 0 {
			return false
		}
		p.selectCut(0)
		if p.hooks.selectPage != nil {
			p.hooks.selectPage(0)
		}
		return true
	case gdk.KEY_Delete, gdk.KEY_KP_Delete, gdk.KEY_BackSpace:
		if p.selCut == 0 || p.hooks.remove == nil {
			return false
		}
		p.hooks.remove(p.selCut)
		return true
	case gdk.KEY_Up, gdk.KEY_Down, gdk.KEY_KP_Up, gdk.KEY_KP_Down:
		if p.selCut == 0 || p.hooks.nudge == nil {
			return false
		}
		step := 1.0
		switch {
		case state&gdk.ControlMask != 0:
			if p.plan == nil || p.plan.Scale <= 0 {
				return false
			}
			step = 0.1 / p.plan.Scale
		case state&gdk.ShiftMask != 0:
			step = 10
		}
		if keyval == gdk.KEY_Up || keyval == gdk.KEY_KP_Up {
			step = -step
		}
		p.hooks.nudge(p.selCut, step)
		return true
	}
	return false
}

// reveal scrolls the view so that the stretch [top, bottom] of the document is
// in sight; a stretch taller than the view is shown from its top.
func (p *preview) reveal(top, bottom float64) {
	if !p.hasDoc || p.view.Zoom <= 0 || p.view.Height <= 0 {
		return
	}
	vis := p.view.Visible()
	if top >= vis.Y && bottom <= vis.Bottom() {
		return
	}
	p.fit = fitFree
	height := p.view.Height / p.view.Zoom
	if bottom-top >= height-2*p.view.Margin/p.view.Zoom {
		p.view.OY = top - p.view.Margin/p.view.Zoom
	} else {
		p.view.OY = (top+bottom)/2 - height/2
	}
	p.view.Clamp()
	p.changed()
}

// connectCutGestures adds the gestures of the cuts to the preview: a drag moves
// a cut or the view, a double click adds a cut, a right click opens the menu,
// the keys move or remove the selected cut.
func (p *preview) connectCutGestures() {
	motion := gtk.NewEventControllerMotion()
	motion.ConnectMotion(func(x, y float64) { p.hover(x, y) })
	motion.ConnectLeave(func() {
		p.pointerX, p.pointerY = -1, -1
		if p.hoverCut != 0 && p.mode == dragNone {
			p.hoverCut = 0
			p.marks.QueueDraw()
		}
	})
	p.marks.AddController(motion)

	drag := gtk.NewGestureDrag()
	drag.SetButton(0)
	drag.ConnectDragBegin(func(x, y float64) {
		button := drag.CurrentButton()
		if button != 1 && button != 2 {
			return
		}
		p.marks.GrabFocus()
		p.beginDrag(x, y, button)
	})
	drag.ConnectDragUpdate(func(dx, dy float64) {
		p.updateDrag(dx, dy, drag.CurrentEventState()&gdk.AltMask != 0)
	})
	drag.ConnectDragEnd(func(dx, dy float64) {
		if p.mode != dragNone {
			p.endDrag(dx, dy)
		}
	})
	p.marks.AddController(drag)

	click := gtk.NewGestureClick()
	click.SetButton(0)
	click.ConnectPressed(func(n int, x, y float64) {
		switch click.CurrentButton() {
		case 1:
			if n == 2 {
				p.doubleClickAt(x, y, click.CurrentEventState()&gdk.AltMask != 0)
			}
		case 3:
			p.marks.GrabFocus()
			p.openMenu(x, y)
		}
	})
	p.marks.AddController(click)

	keys := gtk.NewEventControllerKey()
	keys.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		return p.keyPressed(keyval, state)
	})
	p.marks.AddController(keys)
}

// ---- menu ---------------------------------------------------------------------

// openMenu opens the context menu at (x, y): for a cut, its position and a
// button to delete it; elsewhere, a button to add a cut there.
func (p *preview) openMenu(x, y float64) {
	if !p.hasDoc || p.texts == nil {
		return
	}
	tr := p.texts.tr
	pop := gtk.NewPopover()
	pop.SetParent(p.marks)
	area := gdk.NewRectangle(int(x), int(y), 1, 1)
	pop.SetPointingTo(&area)

	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.SetMarginTop(10)
	box.SetMarginBottom(10)
	box.SetMarginStart(10)
	box.SetMarginEnd(10)

	if c, ok := p.cutAt(x, y); ok {
		p.selectCut(c.ID)
		title := gtk.NewLabel(tr.T("cut.menu.title", "n", c.Number))
		title.AddCSSClass("heading")
		title.SetXAlign(0)
		caption := gtk.NewLabel(tr.T("cut.menu.position"))
		caption.AddCSSClass("caption")
		caption.AddCSSClass("dim-label")
		caption.SetXAlign(0)

		scale := 1.0
		total := p.view.DocH
		if p.plan != nil && p.plan.Scale > 0 {
			scale = p.plan.Scale
			total = p.view.DocH * scale
		}
		field := newNumField(func() *i18n.Translator { return p.texts.tr }, 0, total, 0.1, 1, "mm")
		field.SetValue(round(c.Position*scale, 1))
		id := c.ID
		field.onChange = func(mm float64) {
			if p.hooks.moveTo != nil {
				p.hooks.moveTo(id, mm)
			}
		}
		p.menuField = field

		del := gtk.NewButtonWithLabel(tr.T("cut.menu.delete"))
		del.AddCSSClass("destructive-action")
		del.ConnectClicked(func() {
			pop.Popdown()
			if p.hooks.remove != nil {
				p.hooks.remove(id)
			}
		})
		box.Append(title)
		box.Append(caption)
		box.Append(field.box)
		box.Append(del)
	} else {
		_, doc := p.view.ToDoc(0, y)
		add := gtk.NewButtonWithLabel(tr.T("cut.menu.add_here"))
		add.AddCSSClass("flat")
		add.ConnectClicked(func() {
			pop.Popdown()
			if p.hooks.add != nil {
				p.hooks.add(doc, true)
			}
		})
		box.Append(add)
	}
	pop.SetChild(box)
	pop.ConnectClosed(func() {
		// The popover is taken off its parent once its own closing is over.
		glib.IdleAdd(func() { pop.Unparent() })
		p.menuField = nil
		p.marks.GrabFocus()
	})
	p.menu = pop
	pop.Popup()
}

// ---- painting ------------------------------------------------------------------

// drawCuts paints the lines of the cuts, their handles and warnings, the
// position of the cut being moved, and the outline of the selected page.
func (p *preview) drawCuts(cr *cairo.Context, x0, x1, height float64) {
	if p.plan != nil && p.selPage > 0 && p.selPage <= len(p.plan.Pages) {
		page := p.plan.Pages[p.selPage-1]
		_, yt := p.view.ToScreen(0, page.Top)
		_, yb := p.view.ToScreen(0, page.Bottom)
		cr.SetSourceRGBA(0.16, 0.36, 0.80, 0.95)
		cr.SetLineWidth(2.5)
		cr.Rectangle(x0+1, math.Round(yt)+1, x1-x0-2, math.Round(yb)-math.Round(yt)-2)
		cr.Stroke()
	}
	for _, c := range p.cuts {
		_, y := p.view.ToScreen(0, c.Position)
		if y < -20 || y > height+20 {
			continue
		}
		y = math.Round(y) + 0.5
		active := c.ID == p.selCut || c.ID == p.hoverCut || (p.mode == dragCut && c.ID == p.dragID)
		cr.SetSourceRGB(cutColour[0], cutColour[1], cutColour[2])
		cr.SetLineWidth(1.5)
		if active {
			cr.SetLineWidth(2.5)
		}
		cr.SetDash([]float64{8, 5}, 0)
		cr.MoveTo(x0-cutOverhang, y)
		cr.LineTo(x1+cutOverhang, y)
		cr.Stroke()
		cr.SetDash(nil, 0)

		// The handle, on the left of the line.
		hw, hh := 8.0, 20.0
		if active {
			hw, hh = 10, 24
		}
		cr.SetSourceRGB(cutColour[0], cutColour[1], cutColour[2])
		roundedRect(cr, x0-cutOverhang-hw/2, y-hh/2, hw, hh, 3)
		cr.Fill()
		if c.ID == p.selCut {
			cr.SetSourceRGBA(1, 1, 1, 0.9)
			cr.SetLineWidth(1)
			cr.MoveTo(x0-cutOverhang-1.5, y-5)
			cr.LineTo(x0-cutOverhang-1.5, y+5)
			cr.MoveTo(x0-cutOverhang+1.5, y-5)
			cr.LineTo(x0-cutOverhang+1.5, y+5)
			cr.Stroke()
		}

		if c.Crossed > 0 {
			p.drawWarning(cr, x0+14, y)
		}
	}
	p.drawCutTip(cr, x0, x1, height)
}

// drawWarning paints the sign of a cut that crosses a block.
func (p *preview) drawWarning(cr *cairo.Context, cx, y float64) {
	cr.SetSourceRGB(0.95, 0.60, 0.05)
	cr.MoveTo(cx, y-9)
	cr.LineTo(cx+9, y+7)
	cr.LineTo(cx-9, y+7)
	cr.ClosePath()
	cr.Fill()
	cr.SetSourceRGB(0.15, 0.10, 0)
	cr.SelectFontFace("sans-serif", cairo.FontSlantNormal, cairo.FontWeightBold)
	cr.SetFontSize(11)
	ext := cr.TextExtents("!")
	cr.MoveTo(cx-ext.Width/2-ext.XBearing, y+5.5)
	cr.ShowText("!")
}

// drawCutTip says where the cut being moved (or the selected cut under the
// pointer) is: millimetres from the top of the drawing, and units.
func (p *preview) drawCutTip(cr *cairo.Context, x0, x1, height float64) {
	id := 0
	switch {
	case p.mode == dragCut:
		id = p.dragID
	case p.hoverCut != 0:
		id = p.hoverCut
	}
	if id == 0 || p.plan == nil || p.texts == nil {
		return
	}
	var pos float64
	found := false
	for _, c := range p.cuts {
		if c.ID == id {
			pos, found = c.Position, true
		}
	}
	if !found {
		return
	}
	tr := p.texts.tr
	text := tr.T("cut.tip", "mm", tr.Number(round(pos*p.plan.Scale, 1), -1), "units", tr.Number(round(pos, 1), -1))
	cr.SelectFontFace("sans-serif", cairo.FontSlantNormal, cairo.FontWeightNormal)
	cr.SetFontSize(12)
	ext := cr.TextExtents(text)
	w, h := ext.Width+16, 22.0
	_, y := p.view.ToScreen(0, pos)
	x := math.Max(x1-w-10, x0+10)
	top := y - h - 8
	if top < 4 {
		top = y + 8
	}
	cr.SetSourceRGBA(0.12, 0.12, 0.12, 0.88)
	roundedRect(cr, x, top, w, h, 4)
	cr.Fill()
	cr.SetSourceRGB(1, 1, 1)
	cr.MoveTo(x+8-ext.XBearing, top+h/2-ext.Height/2-ext.YBearing)
	cr.ShowText(text)
}

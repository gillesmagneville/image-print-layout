// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"image"
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/svg-print-layout/internal/i18n"
	"github.com/gillesmagneville/svg-print-layout/internal/layout"
	"github.com/gillesmagneville/svg-print-layout/internal/viewport"
)

// workspace is the colour of the area around the sheet: a neutral grey, the
// same whatever the theme and whatever the colour of the document, so that a
// grey background in an SVG is never taken for the window (ARCHITECTURE §8).
var workspace = [3]float64{0.43, 0.43, 0.43}

// renderDelay is how long the preview waits after the last movement of the
// view before it asks the engine for a sharper drawing.
const renderDelay = 70 // ms

// fitMode says what the view does when the widget changes size.
type fitMode int

const (
	fitFree  fitMode = iota // the user zoomed or moved: keep the view
	fitAll                  // the whole document is shown
	fitWidth                // the document is as wide as the widget
)

// preview is the central area (ARCHITECTURE §9): the work surface, the sheet
// of the document, the drawing the engine made of the part in view, and above
// them the limits and numbers of the pages. Three widgets are stacked:
//
//	ground   the work surface and the sheet (drawn with cairo)
//	picture  the drawing, a texture that the toolkit scales and places, so
//	         that zooming and moving do not wait for the engine
//	marks    the limits, the numbers, and every gesture
type preview struct {
	root    *gtk.Overlay
	ground  *gtk.DrawingArea
	fixed   *gtk.Fixed
	picture *gtk.Picture
	marks   *gtk.DrawingArea
	tools   *gtk.Box
	zoomLbl *gtk.Label

	view   *viewport.View
	fit    fitMode
	hasDoc bool

	// The drawing that is shown: what it is a drawing of, and at what scale.
	imgWin   viewport.Rect
	imgScale float64 // pixels per document unit
	hasImage bool

	plan      *layout.Plan
	pageLabel func(n int) string

	// Hooks of the window.
	onNeedRender func()
	onView       func()

	texts *translations

	pointerX, pointerY float64
	startOX, startOY   float64
	render             debouncer

	// The cuts and the gestures that act on them (cutedit.go).
	cutState
}

func newPreview(texts *translations) *preview {
	p := &preview{view: &viewport.View{Margin: 24}, fit: fitAll}

	p.ground = gtk.NewDrawingArea()
	p.ground.SetContentWidth(320)
	p.ground.SetContentHeight(240)
	p.ground.SetDrawFunc(p.drawGround)

	p.picture = gtk.NewPicture()
	p.picture.SetCanShrink(true)
	p.picture.SetContentFit(gtk.ContentFitFill)
	p.picture.SetVisible(false)
	p.fixed = gtk.NewFixed()
	p.fixed.SetCanTarget(false)
	p.fixed.Put(p.picture, 0, 0)

	p.marks = gtk.NewDrawingArea()
	p.marks.SetDrawFunc(p.drawMarks)
	p.marks.SetFocusable(true)

	p.tools = p.buildTools(texts)

	p.root = gtk.NewOverlay()
	p.root.SetHExpand(true)
	p.root.SetVExpand(true)
	p.root.SetChild(p.ground)
	p.root.AddOverlay(p.fixed)
	p.root.SetClipOverlay(p.fixed, true)
	p.root.AddOverlay(p.marks)
	p.root.AddOverlay(p.tools)

	p.marks.ConnectResize(func(w, h int) { p.resized(float64(w), float64(h)) })
	p.connectGestures()
	p.connectCutGestures()
	return p
}

// widget is what the window puts in its layout.
func (p *preview) widget() gtk.Widgetter { return p.root }

// buildTools makes the zoom buttons in the corner of the preview.
func (p *preview) buildTools(texts *translations) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.SetHAlign(gtk.AlignEnd)
	box.SetVAlign(gtk.AlignEnd)
	box.SetMarginEnd(12)
	box.SetMarginBottom(12)
	box.AddCSSClass("osd")

	add := func(button *gtk.Button, run func(), tooltip string) {
		button.AddCSSClass("flat")
		button.ConnectClicked(run)
		texts.bind(func(tr *i18n.Translator) { button.SetTooltipText(tr.T(tooltip)) })
		box.Append(button)
	}
	add(gtk.NewButtonFromIconName("zoom-out-symbolic"), p.zoomOut, "preview.zoom_out")
	p.zoomLbl = gtk.NewLabel("")
	p.zoomLbl.SetWidthChars(6)
	p.zoomLbl.SetMarginStart(6)
	p.zoomLbl.SetMarginEnd(6)
	box.Append(p.zoomLbl)
	add(gtk.NewButtonFromIconName("zoom-in-symbolic"), p.zoomIn, "preview.zoom_in")
	add(gtk.NewButtonFromIconName("zoom-fit-best-symbolic"), p.showAll, "preview.fit_all")
	add(gtk.NewButtonFromIconName("view-fullscreen-symbolic"), p.showWidth, "preview.fit_width")
	texts.bind(func(tr *i18n.Translator) { p.updateZoomLabel(tr) })
	p.texts = texts
	return box
}

// ---- state ------------------------------------------------------------

// setDocument shows a document of the given size, whole.
func (p *preview) setDocument(w, h float64) {
	p.hasDoc = true
	p.view.DocW, p.view.DocH = w, h
	p.hasImage = false
	p.picture.SetVisible(false)
	p.fit = fitAll
	p.view.FitAll()
	p.changed()
}

// clear removes the document.
func (p *preview) clear() {
	p.hasDoc = false
	p.hasImage = false
	p.plan = nil
	p.picture.SetVisible(false)
	p.changed()
}

// setPlan gives the pages to mark.
func (p *preview) setPlan(plan *layout.Plan) {
	p.plan = plan
	p.marks.QueueDraw()
}

// invalidate drops the drawing: the document has changed, and what is shown
// is not it any more. The next render replaces it.
func (p *preview) invalidate() {
	p.hasImage = false
	p.picture.SetVisible(false)
	p.requestRender(0)
}

// resized follows the widget.
func (p *preview) resized(w, h float64) {
	if w <= 0 || h <= 0 {
		return
	}
	if p.view.Width == 0 || p.view.Height == 0 {
		p.view.Width, p.view.Height = w, h
		p.refit()
	} else {
		switch p.fit {
		case fitAll, fitWidth:
			p.view.Width, p.view.Height = w, h
			p.refit()
		default:
			p.view.Resize(w, h)
		}
	}
	p.changed()
}

func (p *preview) refit() {
	switch p.fit {
	case fitWidth:
		p.view.FitWidth()
	default:
		p.view.FitAll()
	}
}

// changed redraws what depends on the view, and asks for a sharper drawing.
func (p *preview) changed() {
	p.place()
	p.ground.QueueDraw()
	p.marks.QueueDraw()
	if p.texts != nil {
		p.updateZoomLabel(p.texts.tr)
	}
	if p.onView != nil {
		p.onView()
	}
	p.requestRender(renderDelay)
}

// ---- drawing of the engine ---------------------------------------------

// scaleFactor is the number of device pixels per logical pixel.
func (p *preview) scaleFactor() float64 {
	if sf := p.marks.ScaleFactor(); sf > 0 {
		return float64(sf)
	}
	return 1
}

// requestRender asks, after a delay, for a drawing when the one shown no
// longer serves.
func (p *preview) requestRender(ms uint) {
	if !p.hasDoc || p.view.Width == 0 {
		return
	}
	f := func() {
		if p.onNeedRender != nil {
			p.onNeedRender()
		}
	}
	if ms == 0 {
		p.render.cancel()
		f()
		return
	}
	p.render.triggerAfter(ms, f)
}

// wantedWindow is the part of the document and the scale (pixels per unit) to
// ask the engine for, or ok = false when the drawing on show still serves.
func (p *preview) wantedWindow(force bool) (win viewport.Rect, scale float64, ok bool) {
	if !p.hasDoc || p.view.Width == 0 || p.view.Zoom <= 0 {
		return viewport.Rect{}, 0, false
	}
	scale = p.view.Zoom * p.scaleFactor()
	if !force && p.hasImage && !p.view.Needs(p.imgWin, p.imgScale, scale) {
		return viewport.Rect{}, 0, false
	}
	win = p.view.Window(scale)
	return win, scale, win.W > 0 && win.H > 0
}

// setImage shows the drawing the engine made of win at scale.
func (p *preview) setImage(img *image.RGBA, win viewport.Rect, scale float64) {
	b := img.Bounds()
	bytes := glib.NewBytesWithGo(img.Pix)
	tex := gdk.NewMemoryTexture(b.Dx(), b.Dy(), gdk.MemoryR8G8B8A8Premultiplied, bytes, uint(img.Stride))
	p.picture.SetPaintable(tex)
	p.imgWin, p.imgScale, p.hasImage = win, scale, true
	p.picture.SetVisible(true)
	p.place()
}

// place puts the drawing where the view says it belongs, scaled to the zoom.
func (p *preview) place() {
	if !p.hasImage || !p.hasDoc || p.view.Zoom <= 0 {
		return
	}
	x, y := p.view.ToScreen(p.imgWin.X, p.imgWin.Y)
	w := math.Max(1, math.Round(p.imgWin.W*p.view.Zoom))
	h := math.Max(1, math.Round(p.imgWin.H*p.view.Zoom))
	p.picture.SetSizeRequest(int(w), int(h))
	p.fixed.Move(p.picture, math.Round(x), math.Round(y))
}

// ---- gestures ---------------------------------------------------------------

func (p *preview) connectGestures() {
	scroll := gtk.NewEventControllerScroll(gtk.EventControllerScrollBothAxes)
	scroll.ConnectScroll(func(dx, dy float64) bool {
		state := scroll.CurrentEventState()
		switch {
		case state&gdk.ControlMask != 0:
			p.fit = fitFree
			p.view.ZoomAt(math.Pow(1.15, -dy), p.pointerX, p.pointerY)
		case state&gdk.ShiftMask != 0:
			p.fit = fitFree
			p.view.PanPx(-(dy+dx)*scrollStep, 0)
		default:
			p.fit = fitFree
			p.view.PanPx(-dx*scrollStep, -dy*scrollStep)
		}
		p.changed()
		return true
	})
	p.marks.AddController(scroll)

	zoom := gtk.NewGestureZoom()
	var startZoom float64
	zoom.ConnectBegin(func(*gdk.EventSequence) { startZoom = p.view.Zoom })
	zoom.ConnectScaleChanged(func(scale float64) {
		p.fit = fitFree
		x, y, _ := zoom.BoundingBoxCenter()
		p.view.SetZoomAt(startZoom*scale, x, y)
		p.changed()
	})
	p.marks.AddController(zoom)
}

// scrollStep is how many pixels one notch of the wheel moves the view.
const scrollStep = 60

// ---- zoom commands ----------------------------------------------------------

func (p *preview) zoomIn()  { p.zoomBy(1.25) }
func (p *preview) zoomOut() { p.zoomBy(0.8) }

func (p *preview) zoomBy(f float64) {
	p.fit = fitFree
	p.view.ZoomAt(f, p.view.Width/2, p.view.Height/2)
	p.changed()
}

func (p *preview) showAll() {
	p.fit = fitAll
	p.view.FitAll()
	p.changed()
}

func (p *preview) showWidth() {
	p.fit = fitWidth
	p.view.FitWidth()
	p.changed()
}

// zoomPercent is the zoom of the view in percent of the document's own size
// (one unit is one pixel at 100 %).
func (p *preview) zoomPercent() float64 { return p.view.Zoom * 100 }

// updateZoomLabel writes the zoom in the button that shows it.
func (p *preview) updateZoomLabel(tr *i18n.Translator) {
	if p.zoomLbl != nil {
		p.zoomLbl.SetLabel(tr.Number(math.Round(p.zoomPercent()), 0) + " %")
	}
}

// ---- painting -----------------------------------------------------------------

func (p *preview) drawGround(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
	cr.SetSourceRGB(workspace[0], workspace[1], workspace[2])
	cr.Paint()
	if !p.hasDoc || p.view.Zoom <= 0 {
		return
	}
	x0, y0 := p.view.ToScreen(0, 0)
	x1, y1 := p.view.ToScreen(p.view.DocW, p.view.DocH)
	// The sheet: the page colour under the drawing (white; the background of
	// the document, if it has one, is part of the drawing).
	cr.SetSourceRGB(1, 1, 1)
	cr.Rectangle(math.Round(x0), math.Round(y0), math.Round(x1)-math.Round(x0), math.Round(y1)-math.Round(y0))
	cr.Fill()
}

// Colours of the marks of the pages.
var (
	cutColour    = [3]float64{0.80, 0.16, 0.16}
	badgeColours = map[layout.State][3]float64{
		layout.Conforming:     {0.16, 0.27, 0.55},
		layout.MarginOverflow: {0.80, 0.45, 0.05},
		layout.SheetOverflow:  {0.75, 0.12, 0.12},
	}
)

func (p *preview) drawMarks(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
	if !p.hasDoc || p.view.Zoom <= 0 {
		return
	}
	x0, _ := p.view.ToScreen(0, 0)
	x1, _ := p.view.ToScreen(p.view.DocW, 0)
	x0, x1 = math.Round(x0), math.Round(x1)

	// Outline of the sheet.
	_, top := p.view.ToScreen(0, 0)
	_, bottom := p.view.ToScreen(0, p.view.DocH)
	cr.SetSourceRGBA(0, 0, 0, 0.35)
	cr.SetLineWidth(1)
	cr.Rectangle(x0-0.5, math.Round(top)-0.5, x1-x0+1, math.Round(bottom)-math.Round(top)+1)
	cr.Stroke()

	if p.plan == nil || len(p.plan.Pages) == 0 {
		return
	}
	pages := p.plan.Pages
	cr.SelectFontFace("sans-serif", cairo.FontSlantNormal, cairo.FontWeightBold)
	cr.SetFontSize(13)
	for i, page := range pages {
		_, yt := p.view.ToScreen(0, page.Top)
		_, yb := p.view.ToScreen(0, page.Bottom)
		if yb < 0 || yt > float64(height) {
			continue
		}
		yt, yb = math.Round(yt), math.Round(yb)
		if i%2 == 1 {
			cr.SetSourceRGBA(0.16, 0.36, 0.80, 0.06)
			cr.Rectangle(x0, yt, x1-x0, yb-yt)
			cr.Fill()
		}
		p.drawBadge(cr, page, x0, yt, yb, float64(height))
	}
	p.drawCuts(cr, x0, x1, float64(height))
}

// drawBadge writes the number of a page at the top of its band; the badge
// stays in view while the band is scrolled.
func (p *preview) drawBadge(cr *cairo.Context, page layout.Page, x0, yt, yb, height float64) {
	label := page.Number
	text := ""
	if p.pageLabel != nil {
		text = p.pageLabel(label)
	}
	if text == "" {
		return
	}
	ext := cr.TextExtents(text)
	w, h := ext.Width+18, 24.0
	x := math.Max(x0+8, 8)
	y := math.Max(yt+8, 8)
	if y+h > yb-4 {
		y = yb - 4 - h
	}
	if y < yt+2 || y+h > height {
		if y+h > height {
			return
		}
	}
	c := badgeColours[page.State]
	cr.SetSourceRGBA(c[0], c[1], c[2], 0.92)
	roundedRect(cr, x, y, w, h, 5)
	cr.Fill()
	cr.SetSourceRGB(1, 1, 1)
	cr.MoveTo(x+9-ext.XBearing, y+h/2-ext.Height/2-ext.YBearing)
	cr.ShowText(text)
}

func roundedRect(cr *cairo.Context, x, y, w, h, r float64) {
	cr.NewSubPath()
	cr.Arc(x+w-r, y+r, r, -math.Pi/2, 0)
	cr.Arc(x+w-r, y+h-r, r, 0, math.Pi/2)
	cr.Arc(x+r, y+h-r, r, math.Pi/2, math.Pi)
	cr.Arc(x+r, y+r, r, math.Pi, 3*math.Pi/2)
	cr.ClosePath()
}

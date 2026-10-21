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

	"github.com/gillesmagneville/pageslice/internal/engine"
	"github.com/gillesmagneville/pageslice/internal/export"
	"github.com/gillesmagneville/pageslice/internal/i18n"
	"github.com/gillesmagneville/pageslice/internal/layout"
)

// The sheets view (ARCHITECTURE §9): the pages side by side, each drawn by the
// function that writes the PDF, with its own position on the sheet. It is the
// preview of the file before it is written.

// The sheets are shown as large as fits the height of the view, within these
// limits, in pixels of the window to the point of the PDF (an A4 sheet at 0.8 is
// 476 by 673 pixels).
const (
	sheetScaleMin     = 0.25
	sheetScaleMax     = 1.0
	sheetScaleDefault = 0.8
	// sheetChrome is the height around a sheet: the margins of the row, the
	// title under the sheet, the scroll bar.
	sheetChrome = 24 + 12 + 8 + 24 + 16
)

// sheetsView is the scrolled row of sheets.
type sheetsView struct {
	w      *window
	root   *gtk.Box
	scroll *gtk.ScrolledWindow
	row    *gtk.Box
	legend *gtk.Label
	cards  []*sheetCard

	// shown is set while the view is on screen: nothing is drawn for a view
	// that is not.
	shown bool
	// scale is the size the sheets are shown at.
	scale float64
}

// sheetCard is one sheet.
type sheetCard struct {
	box     *gtk.Box
	ground  *gtk.DrawingArea
	picture *gtk.Picture
	guide   *gtk.DrawingArea
	title   *gtk.Label

	// want is what the plan asks the sheet to be; drawn is what the picture
	// shows. They differ until the drawing of the new state arrives.
	want struct {
		spec  engine.PageSpec
		white bool
	}
	drawn struct {
		spec  engine.PageSpec
		white bool
		rev   int
		scale float64
		ok    bool
	}
	scale float64 // the size of the sheet in the view
	asked bool    // a drawing is on its way

	sheet    layout.Sheet
	selected bool
	index    int
}

func newSheetsView(w *window) *sheetsView {
	v := &sheetsView{w: w, scale: sheetScaleDefault}
	v.row = gtk.NewBox(gtk.OrientationHorizontal, 24)
	v.row.SetMarginTop(24)
	v.row.SetMarginBottom(12)
	v.row.SetMarginStart(24)
	v.row.SetMarginEnd(24)
	v.row.SetVAlign(gtk.AlignStart)

	v.scroll = gtk.NewScrolledWindow()
	v.scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	v.scroll.SetHExpand(true)
	v.scroll.SetVExpand(true)
	v.scroll.SetChild(v.row)
	// The scale follows the height of the view: the adjustment changes when the
	// view is resized.
	v.scroll.VAdjustment().ConnectChanged(func() { v.update() })

	v.legend = newLabel()
	v.legend.AddCSSClass("caption")
	v.legend.AddCSSClass("dim-label")
	v.legend.SetMarginStart(12)
	v.legend.SetMarginEnd(12)
	v.legend.SetMarginTop(4)
	v.legend.SetMarginBottom(6)

	v.root = gtk.NewBox(gtk.OrientationVertical, 0)
	v.root.Append(v.scroll)
	v.root.Append(v.legend)
	w.texts.bind(func(tr *i18n.Translator) { v.legend.SetLabel(tr.T("sheets.legend")) })
	return v
}

func (v *sheetsView) widget() gtk.Widgetter { return v.root }

// setShown tells the view whether it is on screen.
func (v *sheetsView) setShown(on bool) {
	v.shown = on
	if on {
		v.update()
	}
}

// newCard makes the widgets of a sheet.
func (v *sheetsView) newCard(index int) *sheetCard {
	c := &sheetCard{index: index}
	c.ground = gtk.NewDrawingArea()
	c.ground.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		cr.SetSourceRGB(1, 1, 1)
		cr.Paint()
		if !c.want.white {
			drawChequerboard(cr, 0, 0, float64(width), float64(height))
		}
	})
	c.picture = gtk.NewPicture()
	c.picture.SetCanShrink(true)
	c.picture.SetContentFit(gtk.ContentFitFill)
	c.picture.SetVisible(false)
	c.guide = gtk.NewDrawingArea()
	c.guide.SetDrawFunc(c.drawGuide)

	overlay := gtk.NewOverlay()
	overlay.SetChild(c.ground)
	overlay.AddOverlay(c.picture)
	overlay.AddOverlay(c.guide)

	click := gtk.NewGestureClick()
	click.ConnectPressed(func(_ int, _, _ float64) { v.w.selectPage(c.index + 1) })
	overlay.AddController(click)

	c.title = gtk.NewLabel("")
	c.title.AddCSSClass("heading")
	c.box = gtk.NewBox(gtk.OrientationVertical, 8)
	c.box.SetVAlign(gtk.AlignStart)
	c.box.Append(overlay)
	c.box.Append(c.title)
	return c
}

// drawGuide draws the edge of the sheet, and the dotted limit of its usable
// area: the margins are what lies between them.
func (c *sheetCard) drawGuide(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
	w, h := float64(width), float64(height)
	cr.SetLineWidth(1)
	cr.SetSourceRGBA(0, 0, 0, 0.45)
	cr.Rectangle(0.5, 0.5, w-1, h-1)
	cr.Stroke()

	mm := c.scale * 72 / 25.4 // window pixels to the millimetre
	s := c.sheet
	if s.UsableWidth > 0 && s.UsableHeight > 0 {
		cr.SetSourceRGBA(0.16, 0.27, 0.55, 0.55)
		cr.SetDash([]float64{3, 3}, 0)
		cr.Rectangle(math.Round(s.UsableLeft*mm)+0.5, math.Round(s.UsableTop*mm)+0.5, math.Round(s.UsableWidth*mm), math.Round(s.UsableHeight*mm))
		cr.Stroke()
		cr.SetDash(nil, 0)
	}
	if c.selected {
		cr.SetSourceRGBA(0.16, 0.45, 0.9, 1)
		cr.SetLineWidth(3)
		cr.Rectangle(1.5, 1.5, w-3, h-3)
		cr.Stroke()
	}
}

// fit sets the scale from the height of the view and of the sheet, and tells
// whether it changed enough to draw the sheets again.
func (v *sheetsView) fit(sheetHeightPt float64) bool {
	avail := v.scroll.VAdjustment().PageSize() - sheetChrome
	if avail <= 0 || sheetHeightPt <= 0 {
		return false
	}
	scale := math.Min(sheetScaleMax, math.Max(sheetScaleMin, avail/sheetHeightPt))
	// Round to a hundredth, and ignore a change of less than 3 %: the drawings
	// are not made again for the width of a scroll bar.
	scale = math.Floor(scale*100) / 100
	if math.Abs(scale-v.scale) < 0.03*v.scale {
		return false
	}
	v.scale = scale
	return true
}

// update brings the cards to the state of the plan and asks for the drawing of
// the ones that are out of date.
func (v *sheetsView) update() {
	if !v.shown {
		return
	}
	w := v.w
	plan := w.session.Plan()
	if plan == nil || w.engineDoc == nil {
		return
	}
	specs := export.Specs(plan)
	white := export.WhitePage(w.session.Project())
	v.fit(specs[0].HeightPt)
	for len(v.cards) < len(specs) {
		c := v.newCard(len(v.cards))
		v.cards = append(v.cards, c)
		v.row.Append(c.box)
	}
	for len(v.cards) > len(specs) {
		last := v.cards[len(v.cards)-1]
		v.row.Remove(last.box)
		v.cards = v.cards[:len(v.cards)-1]
	}
	tr := w.texts.tr
	for i, c := range v.cards {
		c.sheet = plan.Sheet
		c.selected = w.selPage == i+1
		c.title.SetLabel(tr.T("pages.row.title", "n", i+1))
		c.scale = v.scale
		c.ground.SetContentWidth(int(math.Ceil(specs[i].WidthPt * v.scale)))
		c.ground.SetContentHeight(int(math.Ceil(specs[i].HeightPt * v.scale)))
		c.guide.QueueDraw()
		if c.want.white != white {
			c.ground.QueueDraw()
		}
		c.want.spec, c.want.white = specs[i], white
		stale := !c.drawn.ok || c.drawn.spec != specs[i] || c.drawn.white != white || c.drawn.rev != w.docRev || c.drawn.scale != v.scale
		if stale && !c.asked {
			v.draw(c, i)
		}
	}
}

// draw asks the worker for the drawing of one sheet. The answer is dropped when
// the sheet is no longer the one asked for; a newer request is made then.
func (v *sheetsView) draw(c *sheetCard, i int) {
	w := v.w
	doc, spec, white, rev, scale := w.engineDoc, c.want.spec, c.want.white, w.docRev, v.scale
	sf := float64(c.picture.ScaleFactor())
	if sf < 1 {
		sf = 1
	}
	c.asked = true
	w.busy++
	w.engine.do(func() {
		img, err := doc.RenderPage(spec, scale*sf, white)
		glib.IdleAdd(func() {
			w.busy--
			c.asked = false
			if doc != w.engineDoc || i >= len(v.cards) || v.cards[i] != c {
				// The sheet, or the document, is not the one this drawing was
				// made for. update() passed over the sheet while the drawing was
				// on its way: without this call, nothing would ask for the
				// drawing of the new document, and the sheet would keep the
				// old one, with the old background.
				v.update()
				return
			}
			if err != nil {
				w.setStatus("status.rendering_failed", "reason", err.Error())
				return
			}
			c.set(img, spec, white, rev, scale)
			// The state may have changed while the sheet was drawn.
			v.update()
		})
	})
}

// set shows a drawing in the card.
func (c *sheetCard) set(img *image.RGBA, spec engine.PageSpec, white bool, rev int, scale float64) {
	b := img.Bounds()
	tex := gdk.NewMemoryTexture(b.Dx(), b.Dy(), gdk.MemoryR8G8B8A8Premultiplied, glib.NewBytesWithGo(img.Pix), uint(img.Stride))
	c.picture.SetPaintable(tex)
	c.picture.SetVisible(true)
	// What was drawn, which may not be what the plan asks for any more.
	c.drawn.spec, c.drawn.white, c.drawn.rev, c.drawn.scale, c.drawn.ok = spec, white, rev, scale, true
}

// scrollTo brings the sheet of page n into view.
func (v *sheetsView) scrollTo(n int) {
	if n < 1 || n > len(v.cards) {
		return
	}
	c := v.cards[n-1]
	glib.IdleAdd(func() {
		adj := v.scroll.HAdjustment()
		alloc := c.box.Allocation()
		x := float64(alloc.X())
		if x < 0 {
			return
		}
		view := adj.PageSize()
		left := adj.Value()
		if x-24 < left || x+float64(alloc.Width())+24 > left+view {
			adj.SetValue(x - 24)
		}
	})
}

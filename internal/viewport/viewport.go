// SPDX-License-Identifier: GPL-3.0-only

// Package viewport is the arithmetic of the central preview: which part of the
// document is on screen at a zoom and a position, how a gesture moves it, and
// which window of the document to ask the engine to draw. It knows nothing of
// the toolkit, so it is tested without a display.
//
// Lengths on the document are in document units; on the screen, in logical
// pixels of the widget.
package viewport

import "math"

// Zoom limits, in logical pixels per document unit.
const (
	MinZoom = 0.005
	MaxZoom = 32.0
)

// Rect is a rectangle on the document, in document units.
type Rect struct {
	X, Y, W, H float64
}

// Right and Bottom are the far edges.
func (r Rect) Right() float64  { return r.X + r.W }
func (r Rect) Bottom() float64 { return r.Y + r.H }

// View is the document seen through the widget.
type View struct {
	// DocW and DocH are the size of the document, in document units.
	DocW, DocH float64
	// Width and Height are the size of the widget, in logical pixels.
	Width, Height float64
	// Margin is the room, in pixels, that a fit leaves around the document.
	Margin float64
	// Zoom is the number of logical pixels per document unit.
	Zoom float64
	// OX and OY are the position on the document of the top left corner of
	// the widget.
	OX, OY float64
}

// New is a view of a document in a widget of the given size, showing it whole.
func New(docW, docH, width, height float64) *View {
	v := &View{DocW: docW, DocH: docH, Width: width, Height: height, Margin: 24}
	v.FitAll()
	return v
}

// ready tells whether the view has sizes to work with.
func (v *View) ready() bool {
	return v.DocW > 0 && v.DocH > 0 && v.Width > 0 && v.Height > 0
}

// FitAll shows the whole document, centred.
func (v *View) FitAll() {
	if !v.ready() {
		return
	}
	z := math.Min((v.Width-2*v.Margin)/v.DocW, (v.Height-2*v.Margin)/v.DocH)
	v.Zoom = clampZoom(z)
	v.OX, v.OY = 0, 0
	v.Clamp()
}

// FitWidth shows the document as wide as the widget allows, from its top.
func (v *View) FitWidth() {
	if !v.ready() {
		return
	}
	v.Zoom = clampZoom((v.Width - 2*v.Margin) / v.DocW)
	v.OX, v.OY = 0, -v.Margin/v.Zoom
	v.Clamp()
}

func clampZoom(z float64) float64 {
	if !(z > MinZoom) {
		return MinZoom
	}
	return math.Min(z, MaxZoom)
}

// ZoomAt multiplies the zoom by factor, keeping the point of the document
// under the pixel (px, py) where it is.
func (v *View) ZoomAt(factor, px, py float64) {
	v.SetZoomAt(v.Zoom*factor, px, py)
}

// SetZoomAt sets the zoom, keeping the point of the document under the pixel
// (px, py) where it is.
func (v *View) SetZoomAt(zoom, px, py float64) {
	if !v.ready() {
		return
	}
	zoom = clampZoom(zoom)
	x, y := v.ToDoc(px, py)
	v.Zoom = zoom
	v.OX, v.OY = x-px/zoom, y-py/zoom
	v.Clamp()
}

// PanPx moves the document by (dx, dy) pixels on the screen: the content
// follows the pointer.
func (v *View) PanPx(dx, dy float64) {
	if !v.ready() {
		return
	}
	v.OX -= dx / v.Zoom
	v.OY -= dy / v.Zoom
	v.Clamp()
}

// Resize gives the widget a new size. The point of the document at the centre
// of the widget stays at its centre.
func (v *View) Resize(width, height float64) {
	if v.Zoom > 0 && v.ready() {
		cx, cy := v.ToDoc(v.Width/2, v.Height/2)
		v.Width, v.Height = width, height
		v.OX, v.OY = cx-width/2/v.Zoom, cy-height/2/v.Zoom
	} else {
		v.Width, v.Height = width, height
	}
	v.Clamp()
}

// Clamp keeps the document in view. A document smaller than the widget along
// an axis is centred on it; a larger one cannot leave the widget by more than
// the margin.
func (v *View) Clamp() {
	if !v.ready() || v.Zoom <= 0 {
		return
	}
	v.OX = clampAxis(v.OX, v.DocW, v.Width, v.Zoom, v.Margin)
	v.OY = clampAxis(v.OY, v.DocH, v.Height, v.Zoom, v.Margin)
}

func clampAxis(o, doc, view, zoom, margin float64) float64 {
	if doc*zoom <= view-2*margin+1e-9 {
		return -(view - doc*zoom) / 2 / zoom
	}
	lo := -margin / zoom
	hi := doc - (view-margin)/zoom
	return math.Min(math.Max(o, lo), hi)
}

// ToScreen converts a point of the document to a pixel of the widget.
func (v *View) ToScreen(x, y float64) (sx, sy float64) {
	return (x - v.OX) * v.Zoom, (y - v.OY) * v.Zoom
}

// ToDoc converts a pixel of the widget to a point of the document.
func (v *View) ToDoc(sx, sy float64) (x, y float64) {
	return sx/v.Zoom + v.OX, sy/v.Zoom + v.OY
}

// Visible is the part of the document that the widget shows.
func (v *View) Visible() Rect {
	if !v.ready() || !(v.Zoom > 0) {
		return Rect{}
	}
	r := Rect{v.OX, v.OY, v.Width / v.Zoom, v.Height / v.Zoom}
	return intersect(r, Rect{0, 0, v.DocW, v.DocH})
}

func intersect(a, b Rect) Rect {
	x0, y0 := math.Max(a.X, b.X), math.Max(a.Y, b.Y)
	x1, y1 := math.Min(a.Right(), b.Right()), math.Min(a.Bottom(), b.Bottom())
	if x1 < x0 || y1 < y0 {
		return Rect{}
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// Contains tells whether a covers b.
func (r Rect) Contains(b Rect) bool {
	const eps = 1e-6
	return b.X >= r.X-eps && b.Y >= r.Y-eps && b.Right() <= r.Right()+eps && b.Bottom() <= r.Bottom()+eps
}

// MaxPixels is the most pixels a drawing asked of the engine may have: a 4K
// screen with some room around is about 12 million.
const MaxPixels = 12e6

// Window is the part of the document to draw for the current view: what is
// visible plus a share of it on each side (so that a pan shows drawing and not
// blank), cut to the document. scale is the number of pixels the engine draws
// per document unit (the zoom times the scale factor of the screen). The window
// starts on a whole pixel of the drawing, so that the image is not blurred when
// it is placed.
func (v *View) Window(scale float64) Rect {
	vis := v.Visible()
	if vis.W <= 0 || vis.H <= 0 || scale <= 0 {
		return Rect{}
	}
	for _, share := range []float64{0.25, 0.1, 0.0} {
		r := Rect{vis.X - vis.W*share, vis.Y - vis.H*share, vis.W * (1 + 2*share), vis.H * (1 + 2*share)}
		r = intersect(r, Rect{0, 0, v.DocW, v.DocH})
		r = snap(r, scale, v.DocW, v.DocH)
		if r.W*scale*r.H*scale <= MaxPixels || share == 0 {
			return r
		}
	}
	return vis
}

// snap enlarges a rectangle to whole pixels of the drawing, within the document.
func snap(r Rect, scale, docW, docH float64) Rect {
	x0 := math.Floor(r.X*scale) / scale
	y0 := math.Floor(r.Y*scale) / scale
	x1 := math.Min(math.Ceil(r.Right()*scale)/scale, docW)
	y1 := math.Min(math.Ceil(r.Bottom()*scale)/scale, docH)
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// Needs tells whether an image drawn for the window win at the scale imgScale
// (pixels per unit) must be drawn again for the current view: when it no
// longer covers what is visible, or when the zoom has changed so much that it
// would look soft. The tolerance keeps a small zoom step from drawing twice.
func (v *View) Needs(win Rect, imgScale, scale float64) bool {
	if win.W <= 0 || imgScale <= 0 {
		return true
	}
	if !win.Contains(v.Visible()) {
		return true
	}
	ratio := scale / imgScale
	return ratio > 1.02 || ratio < 0.98
}

// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"image"
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// workspace is the colour of the area around the sheet: a neutral grey, the
// same whatever the theme and whatever the colour of the document, so that a
// grey background in an SVG is never taken for the window (ARCHITECTURE §8).
var workspace = [3]float64{0.43, 0.43, 0.43}

// preview is the central area: the work surface, the sheet and the drawing.
type preview struct {
	area    *gtk.DrawingArea
	surface *cairo.Surface // the drawing as the engine rendered it
	size    image.Point    // its size in pixels
}

func newPreview() *preview {
	p := &preview{area: gtk.NewDrawingArea()}
	p.area.SetHExpand(true)
	p.area.SetVExpand(true)
	p.area.SetContentWidth(320)
	p.area.SetContentHeight(240)
	p.area.SetDrawFunc(p.draw)
	return p
}

// setImage replaces the drawing and asks for a redraw.
func (p *preview) setImage(img *image.RGBA) {
	p.surface = cairo.CreateSurfaceFromImage(img)
	p.size = img.Rect.Size()
	p.area.QueueDraw()
}

// clear removes the drawing.
func (p *preview) clear() {
	p.surface = nil
	p.size = image.Point{}
	p.area.QueueDraw()
}

func (p *preview) draw(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
	cr.SetSourceRGB(workspace[0], workspace[1], workspace[2])
	cr.Paint()
	if p.surface == nil {
		return
	}
	// Whole pixels, so that the image is not blurred by a half-pixel offset.
	x := math.Round(float64(width-p.size.X) / 2)
	y := math.Round(float64(height-p.size.Y) / 2)
	// The sheet: the page colour under the drawing (white; the background of
	// the document, if it has one, is part of the drawing).
	cr.SetSourceRGB(1, 1, 1)
	cr.Rectangle(x, y, float64(p.size.X), float64(p.size.Y))
	cr.Fill()
	cr.SetSourceSurface(p.surface, x, y)
	cr.Paint()
}

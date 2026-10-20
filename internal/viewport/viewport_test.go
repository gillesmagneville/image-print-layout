// SPDX-License-Identifier: GPL-3.0-only

package viewport

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// The Mini-CAD (1280 x 5479) in a 900 x 700 widget.
func miniCAD() *View { return New(1280, 5479, 900, 700) }

func TestFitAllShowsTheWholeDocumentCentred(t *testing.T) {
	v := miniCAD()
	want := (700.0 - 48) / 5479
	if !near(v.Zoom, want) {
		t.Errorf("zoom = %v, want %v", v.Zoom, want)
	}
	vis := v.Visible()
	if vis.X != 0 || vis.Y != 0 || !near(vis.W, 1280) || !near(vis.H, 5479) {
		t.Errorf("visible = %+v, want the whole document", vis)
	}
	// Centred on the widget.
	x0, _ := v.ToScreen(0, 0)
	x1, _ := v.ToScreen(1280, 0)
	if !near(x0, 900-x1) {
		t.Errorf("not centred: %v and %v from the edges", x0, 900-x1)
	}
}

func TestFitWidthStartsAtTheTop(t *testing.T) {
	v := miniCAD()
	v.FitWidth()
	if !near(v.Zoom, (900.0-48)/1280) {
		t.Errorf("zoom = %v", v.Zoom)
	}
	x0, y0 := v.ToScreen(0, 0)
	if !near(x0, 24) || !near(y0, 24) {
		t.Errorf("top left corner at %v, %v, want 24, 24", x0, y0)
	}
}

func TestZoomKeepsThePointUnderThePointer(t *testing.T) {
	v := miniCAD()
	v.FitWidth()
	px, py := 300.0, 200.0
	x, y := v.ToDoc(px, py)
	for _, f := range []float64{1.5, 2, 0.8, 3} {
		v.ZoomAt(f, px, py)
		gx, gy := v.ToDoc(px, py)
		if !near(gx, x) || !near(gy, y) {
			t.Fatalf("after x%v the point under the pointer is %v, %v, was %v, %v", f, gx, gy, x, y)
		}
	}
}

func TestZoomIsBounded(t *testing.T) {
	v := miniCAD()
	for i := 0; i < 100; i++ {
		v.ZoomAt(2, 10, 10)
	}
	if v.Zoom != MaxZoom {
		t.Errorf("zoom = %v", v.Zoom)
	}
	for i := 0; i < 100; i++ {
		v.ZoomAt(0.5, 10, 10)
	}
	if v.Zoom != MinZoom {
		t.Errorf("zoom = %v", v.Zoom)
	}
}

func TestPanFollowsThePointerAndStaysOnTheDocument(t *testing.T) {
	v := miniCAD()
	v.FitWidth()
	_, y := v.ToDoc(0, 0)
	v.PanPx(0, -100) // drag the content up by 100 pixels
	_, y2 := v.ToDoc(0, 0)
	if !near(y2-y, 100/v.Zoom) {
		t.Errorf("moved by %v units, want %v", y2-y, 100/v.Zoom)
	}
	v.PanPx(0, -1e9)
	if vis := v.Visible(); !near(vis.Bottom(), 5479) {
		t.Errorf("the bottom of the document is out of reach: visible %+v", vis)
	}
	v.PanPx(0, 1e9)
	if _, sy := v.ToScreen(0, 0); sy > v.Margin+1e-6 {
		t.Errorf("the document left the widget: top at %v", sy)
	}
}

func TestADocumentSmallerThanTheWidgetIsCentred(t *testing.T) {
	v := New(100, 100, 900, 700)
	v.SetZoomAt(1, 450, 350)
	x0, y0 := v.ToScreen(0, 0)
	if !near(x0, 400) || !near(y0, 300) {
		t.Errorf("top left at %v, %v, want 400, 300", x0, y0)
	}
	v.PanPx(300, 300)
	if x1, _ := v.ToScreen(0, 0); !near(x1, 400) {
		t.Errorf("a pan moved a document that fits")
	}
}

func TestResizeKeepsTheCentre(t *testing.T) {
	v := miniCAD()
	v.FitWidth()
	v.ZoomAt(2, 450, 350)
	cx, cy := v.ToDoc(450, 350)
	v.Resize(1200, 900)
	gx, gy := v.ToDoc(600, 450)
	if !near(gx, cx) || !near(gy, cy) {
		t.Errorf("centre moved from %v, %v to %v, %v", cx, cy, gx, gy)
	}
}

func TestWindowCoversTheVisiblePartWithRoomAround(t *testing.T) {
	v := miniCAD()
	v.FitWidth()
	v.ZoomAt(2, 450, 350)
	scale := v.Zoom
	w := v.Window(scale)
	vis := v.Visible()
	if !w.Contains(vis) {
		t.Errorf("window %+v does not cover %+v", w, vis)
	}
	if w.W <= vis.W && w.H <= vis.H && vis.W < 1280 && vis.H < 5479 {
		t.Errorf("no room around the visible part: %+v and %+v", w, vis)
	}
	if w.X < 0 || w.Y < 0 || w.Right() > 1280+1e-9 || w.Bottom() > 5479+1e-9 {
		t.Errorf("window %+v leaves the document", w)
	}
	// It starts on a pixel of the drawing.
	if px := w.X * scale; !near(px, math.Round(px)) {
		t.Errorf("window starts at %v pixels", px)
	}
	if px := w.Y * scale; !near(px, math.Round(px)) {
		t.Errorf("window starts at %v pixels", px)
	}
}

func TestWindowStaysUnderTheLimitOfPixels(t *testing.T) {
	v := New(20000, 20000, 1920, 1080)
	v.SetZoomAt(1, 960, 540)
	scale := v.Zoom * 2 // a screen with a scale factor of 2
	w := v.Window(scale)
	if px := w.W * scale * w.H * scale; px > MaxPixels*1.05 {
		t.Errorf("%v pixels", px)
	}
	if !w.Contains(v.Visible()) {
		t.Error("the window does not cover the visible part")
	}
}

func TestNeeds(t *testing.T) {
	v := miniCAD()
	v.FitWidth()
	scale := v.Zoom
	w := v.Window(scale)
	if v.Needs(w, scale, scale) {
		t.Error("a fresh image needs drawing again")
	}
	v.ZoomAt(1.01, 100, 100)
	if v.Needs(v.Window(v.Zoom), scale, v.Zoom) && !v.Window(v.Zoom).Contains(v.Visible()) {
		t.Error("window does not cover")
	}
	if !v.Needs(w, scale, scale*1.5) {
		t.Error("a zoom of 1.5 does not need drawing again")
	}
	v.PanPx(0, -5000)
	if !v.Needs(w, scale, v.Zoom) {
		t.Error("a pan out of the window does not need drawing again")
	}
	if !v.Needs(Rect{}, 0, 1) {
		t.Error("no image needs drawing")
	}
}

func TestAViewWithoutSizesDoesNothing(t *testing.T) {
	v := &View{}
	v.FitAll()
	v.FitWidth()
	v.ZoomAt(2, 0, 0)
	v.PanPx(10, 10)
	v.Clamp()
	if v.Zoom != 0 {
		t.Errorf("zoom = %v", v.Zoom)
	}
	if w := v.Window(1); w.W != 0 {
		t.Errorf("window = %+v", w)
	}
}

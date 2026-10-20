// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"image"
	"image/color"
	"io"
	"testing"

	"github.com/gillesmagneville/svg-print-layout/internal/engine"
)

func TestBlocksOfRows(t *testing.T) {
	rows := []bool{false, true, true, false, false, true, false, true}
	got := blocksOfRows(rows, 2) // two rows per unit
	want := [][2]float64{{0.5, 1.5}, {2.5, 3}, {3.5, 4}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, w := range want {
		if got[i].Top != w[0] || got[i].Bottom != w[1] {
			t.Errorf("block %d: got [%g, %g], want %v", i, got[i].Top, got[i].Bottom, w)
		}
	}
	if blocksOfRows(nil, 1) != nil {
		t.Error("no row, no block")
	}
	if g := blocksOfRows([]bool{true, true}, 1); len(g) != 1 || g[0].Top != 0 || g[0].Bottom != 2 {
		t.Errorf("a run to the end of the image is closed: %+v", g)
	}
}

// inkEngine is an engine document that draws bands of ink.
type inkEngine struct {
	w, h  float64
	bands [][2]int // rows with ink, at 1 pixel per unit
	asked engine.Box
	scale float64
}

func (e *inkEngine) Size() (float64, float64)                                        { return e.w, e.h }
func (e *inkEngine) Measure(string) (engine.Box, error)                              { return engine.Box{}, nil }
func (e *inkEngine) MeasureBlocks([]string) map[string]engine.Box                    { return nil }
func (e *inkEngine) RenderPage(engine.PageSpec, float64, bool) (*image.RGBA, error)  { return nil, nil }
func (e *inkEngine) ExportPDF(io.Writer, []engine.PageSpec, engine.PDFOptions) error { return nil }
func (e *inkEngine) Close()                                                          {}
func (e *inkEngine) Render(win engine.Box, scale float64, _ color.Color) (*image.RGBA, error) {
	e.asked, e.scale = win, scale
	img := image.NewRGBA(image.Rect(0, 0, int(win.W*scale), int(win.H*scale)))
	for _, b := range e.bands {
		for y := b[0]; y < b[1]; y++ {
			img.SetRGBA(img.Bounds().Dx()/2, y, color.RGBA{A: 255})
		}
	}
	return img, nil
}

func TestInkBlocks(t *testing.T) {
	e := &inkEngine{w: 100, h: 300, bands: [][2]int{{10, 50}, {50, 60}, {120, 130}}}
	blocks, err := inkBlocks(e)
	if err != nil {
		t.Fatal(err)
	}
	if e.asked.W != 100 || e.asked.H != 300 || e.scale != 1 {
		t.Errorf("the whole document is drawn at 1 pixel per unit: %+v at %g", e.asked, e.scale)
	}
	// Touching bands are one block; a void of 60 rows lies between the two.
	if len(blocks) != 2 || blocks[0].Top != 10 || blocks[0].Bottom != 60 || blocks[1].Top != 120 || blocks[1].Bottom != 130 {
		t.Errorf("got %+v", blocks)
	}
}

func TestInkBlocksLimitsThePixels(t *testing.T) {
	e := &inkEngine{w: 20000, h: 20000}
	e.bands = nil
	if _, err := inkBlocks(e); err != nil {
		t.Fatal(err)
	}
	if px := e.asked.W * e.scale * e.asked.H * e.scale; px > maxProfilePixels*1.01 {
		t.Errorf("%g pixels drawn, more than the %g allowed", px, maxProfilePixels)
	}
}

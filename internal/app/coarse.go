// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"fmt"
	"image"
	"math"

	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/layout"
)

// MaxExactBlocks is how many blocks the engine measures one by one. Measuring
// an element makes the rendering library walk the whole drawing, so the time
// of measuring every block grows with the square of their number: 4 000
// blocks take under a second, 100 000 would take minutes. Past this number,
// the extent of the ink is read from an image instead (ARCHITECTURE §7).
const MaxExactBlocks = 3000

// maxProfilePixels bounds the image drawn to find where the ink is.
const maxProfilePixels = 16e6

// inkBlocks returns the stretches of the vertical axis where the document has
// ink, as blocks. What the layout reads of the blocks is the union of their
// vertical extents (the voids are its complement), and a row of an image with
// no ink at all is a void: so the stretches of rows that have ink are the same
// information as the extent of every block, at the resolution of the image,
// for one drawing instead of one measure per block. A void thinner than a row
// is lost, which errs on the safe side: the cut is not proposed there.
//
// The document must be loaded without its background rectangle, which would
// cover every row.
func inkBlocks(eng engine.Document) ([]layout.Block, error) {
	w, h := eng.Size()
	if !(w > 0 && h > 0) {
		return nil, nil
	}
	scale := math.Min(1, math.Sqrt(maxProfilePixels/(w*h)))
	img, err := eng.Render(engine.Box{W: w, H: h}, scale, nil)
	if err != nil {
		return nil, err
	}
	return blocksOfRows(inkRows(img), scale), nil
}

// inkRows tells, for each row of the image, whether any pixel of it is drawn.
func inkRows(img *image.RGBA) []bool {
	b := img.Bounds()
	rows := make([]bool, b.Dy())
	for y := range rows {
		line := img.Pix[y*img.Stride : y*img.Stride+b.Dx()*4]
		for x := 3; x < len(line); x += 4 {
			if line[x] != 0 {
				rows[y] = true
				break
			}
		}
	}
	return rows
}

// blocksOfRows turns the runs of rows with ink into blocks, in document units.
func blocksOfRows(rows []bool, scale float64) []layout.Block {
	var out []layout.Block
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		out = append(out, layout.Block{
			ID:     fmt.Sprintf("ink-%d", len(out)+1),
			Top:    float64(start) / scale,
			Bottom: float64(end) / scale,
		})
		start = -1
	}
	for y, ink := range rows {
		if ink && start < 0 {
			start = y
		}
		if !ink {
			flush(y)
		}
	}
	flush(len(rows))
	return out
}

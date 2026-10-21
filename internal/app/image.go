// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"math"

	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/layout"
)

// How a stretch of an image is told from its background (see rasterBlocks).
const (
	// backgroundTolerance is how far, in levels of a channel out of 255, a
	// pixel may be from the background and still be the background: the noise
	// of a JPEG on a flat colour, the grain of a scan.
	backgroundTolerance = 16
	// specksShare is the share of the pixels of a row that may differ from the
	// background without the row being ink: a dust mark does not make a block.
	specksShare = 0.001
	// backgroundShare is how much of the border of the picture the background
	// colour must cover for there to be a background at all.
	backgroundShare = 0.5
)

// prepareImage opens a raster image: it loads it into the engine, and finds
// the blocks of the picture from its pixels.
func prepareImage(svc Services, src []byte, path string, format engine.Format) (*Opened, error) {
	eng, err := svc.Load(src, path)
	if err != nil {
		return nil, imageError(err)
	}
	w, h := eng.Size()
	sum := sha256.Sum256(src)
	d := &Document{
		Path: path, SHA256: hex.EncodeToString(sum[:]), Format: format,
		Size: layout.Doc{W: w, H: h}, source: src,
	}
	if id, ok := eng.(engine.ImageDocument); ok {
		info := id.ImageInfo()
		d.Image = &info
		d.Findings = imageFindings(info)
	}
	blocks, err := rasterBlocks(eng, d.Image)
	if err != nil {
		eng.Close()
		return nil, err
	}
	d.Blocks = blocks
	return &Opened{Document: d, Engine: eng}, nil
}

// imageError turns what the engine says of an image it cannot open into a
// diagnostic the window writes in its language. Other errors are kept.
func imageError(err error) error {
	var ie *engine.ImageError
	if !errors.As(err, &ie) {
		return err
	}
	switch ie.Kind {
	case engine.ImageTooLarge:
		return diag.Fail("image.too_large", "w", ie.W, "h", ie.H,
			"mp", float64(ie.W)*float64(ie.H)/1e6, "max", engine.MaxImagePixels/1_000_000)
	case engine.ImageUnsupported:
		return diag.Fail("image.unsupported", "format", ie.Format.Label())
	}
	return diag.Fail("image.unreadable", "reason", ie.Reason)
}

// imageFindings are what the user should know of a picture before printing it.
func imageFindings(info engine.ImageInfo) []diag.Item {
	var items []diag.Item
	if info.Truncated {
		items = append(items, diag.New("image.truncated", 0))
	}
	if info.MultiPage {
		items = append(items, diag.New("image.multipage", 0))
	}
	if info.Animated {
		items = append(items, diag.New("image.animated", 0))
	}
	if !info.HasResolution() {
		items = append(items, diag.New("image.no_dpi", 0))
	}
	return items
}

// rasterBlocks finds the blocks of a picture: the stretches of rows that are
// not the colour of its background. A scanned page gives its lines of text, a
// drawing on a white sheet its figures; the voids between them are where a cut
// crosses nothing, which is all the layout reads of the blocks (see
// inkBlocks). A picture that has no background (a photograph) or no void
// has no blocks: a cut crosses nothing anywhere, and the automatic cuts divide
// it evenly.
//
// The background is the colour that most of the border of the picture has,
// once it is laid on white as the sheet shows it. A pixel is ink when it is
// further than backgroundTolerance from it, and a row is ink when it has more
// than specksShare of its pixels of ink.
func rasterBlocks(eng engine.Document, info *engine.ImageInfo) ([]layout.Block, error) {
	w, h := eng.Size()
	if !(w > 0 && h > 0) {
		return nil, nil
	}
	// No finer than the file: more pixels than it has says nothing more.
	native := 1.0
	if info != nil {
		px, py := info.PixelsPerUnit()
		native = math.Max(px, py)
	}
	scale := math.Min(native, math.Sqrt(maxProfilePixels/(w*h)))
	img, err := eng.Render(engine.Box{W: w, H: h}, scale, nil)
	if err != nil {
		return nil, err
	}
	bg, ok := borderBackground(img)
	if !ok {
		return nil, nil
	}
	blocks := blocksOfRows(inkRowsAgainst(img, bg), scale)
	if len(layout.Voids(blocks, h)) == 0 {
		return nil, nil
	}
	return blocks, nil
}

// onWhite is a pixel of a picture as it looks on a white sheet. The pixels of
// an image.RGBA are premultiplied: what the pixel does not cover is white.
func onWhite(pix []uint8) (r, g, b uint8) {
	a := int(pix[3])
	return uint8(int(pix[0]) + 255 - a), uint8(int(pix[1]) + 255 - a), uint8(int(pix[2]) + 255 - a)
}

// borderBackground is the colour that most of the pixels on the four edges of
// the picture have, within backgroundTolerance. ok is false when no colour
// covers half of them: there is no background to tell the picture from.
func borderBackground(img *image.RGBA) (bg [3]uint8, ok bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return bg, false
	}
	type bin struct{ n, r, g, b int }
	bins := map[[3]uint8]*bin{}
	total := 0
	add := func(x, y int) {
		r, g, bl := onWhite(img.Pix[y*img.Stride+x*4:])
		key := [3]uint8{r >> 4, g >> 4, bl >> 4} // 16 levels a channel
		e := bins[key]
		if e == nil {
			e = &bin{}
			bins[key] = e
		}
		e.n++
		e.r += int(r)
		e.g += int(g)
		e.b += int(bl)
		total++
	}
	for x := 0; x < w; x++ {
		add(x, 0)
		if h > 1 {
			add(x, h-1)
		}
	}
	for y := 1; y < h-1; y++ {
		add(0, y)
		if w > 1 {
			add(w-1, y)
		}
	}
	var best *bin
	for _, e := range bins {
		if best == nil || e.n > best.n {
			best = e
		}
	}
	if best == nil || float64(best.n) < backgroundShare*float64(total) {
		return bg, false
	}
	return [3]uint8{uint8(best.r / best.n), uint8(best.g / best.n), uint8(best.b / best.n)}, true
}

// inkRowsAgainst tells, for each row, whether it has ink: more than
// specksShare of its pixels are further than backgroundTolerance from bg.
func inkRowsAgainst(img *image.RGBA, bg [3]uint8) []bool {
	b := img.Bounds()
	w := b.Dx()
	limit := int(float64(w) * specksShare)
	rows := make([]bool, b.Dy())
	far := func(a, b uint8) bool {
		d := int(a) - int(b)
		return d > backgroundTolerance || d < -backgroundTolerance
	}
	for y := range rows {
		line := img.Pix[y*img.Stride : y*img.Stride+w*4]
		n := 0
		for x := 0; x < len(line); x += 4 {
			r, g, bl := onWhite(line[x:])
			if far(r, bg[0]) || far(g, bg[1]) || far(bl, bg[2]) {
				if n++; n > limit {
					rows[y] = true
					break
				}
			}
		}
	}
	return rows
}

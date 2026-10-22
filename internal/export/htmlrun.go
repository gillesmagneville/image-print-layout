// SPDX-License-Identifier: GPL-3.0-only

package export

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/layout"
)

// HTMLRequest is an HTML export to run.
type HTMLRequest struct {
	Plan    *layout.Plan
	Source  HTMLSource
	Options HTMLOptions
	// Path is the file to write.
	Path string
}

// RunHTML writes the page. As for the PDF, it is written beside its
// destination under a temporary name, read back from the disk and compared with
// the plan, and only then takes the place of the destination: a failed export
// never leaves a half-written file, nor replaces a good one with a bad one.
// The result counts the pieces of the page.
func RunHTML(req HTMLRequest) (*Result, error) {
	if req.Plan == nil || len(req.Plan.Pages) == 0 {
		return nil, diag.Fail("app.no_document")
	}
	var page bytes.Buffer
	if err := WriteHTML(&page, req.Plan, req.Source, req.Options); err != nil {
		return nil, err
	}
	dir, base := filepath.Split(req.Path)
	part := filepath.Join(dir, "."+base+".part")
	done := false
	defer func() {
		if !done {
			os.Remove(part)
		}
	}()
	if err := os.WriteFile(part, page.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("cannot write %s: %w", base, err)
	}
	// The file is read back from the disk, as the browser will.
	back, err := os.ReadFile(part)
	if err != nil {
		return nil, fmt.Errorf("cannot read back %s: %w", base, err)
	}
	if items := VerifyHTML(back, req.Plan, req.Source, req.Options.Cut); len(items) > 0 {
		return nil, &VerifyError{Items: items, Kind: "HTML"}
	}
	if err := os.Rename(part, req.Path); err != nil {
		return nil, fmt.Errorf("cannot replace %s (is it open in another program?): %w", base, err)
	}
	done = true
	return &Result{Pages: len(htmlPieces(req.Plan, req.Options.Cut)), Bytes: int64(len(back))}, nil
}

// maxRenderPixels is the most pixels a picture is drawn with to be written as
// a PNG: more is drawn smaller.
const maxRenderPixels = 64_000_000

// ImageSource chooses what a page carries of a picture. A JPEG that the PDF
// takes as it is, and a PNG that is not animated and not turned by a tag,
// are written as the file was: no loss, and the size of the file. Any other
// kind (a TIFF, a GIF, a WebP, a BMP, a PNG of several images) is drawn by the
// engine, which gives what the preview shows, turned upright and at its first
// image, and written as a PNG, which keeps its transparency.
func ImageSource(doc engine.Document, info engine.ImageInfo, original []byte) (HTMLSource, error) {
	switch {
	case info.Format == engine.FormatJPEG && info.EmbeddedJPEG && len(original) > 0:
		return HTMLSource{MIME: "image/jpeg", Data: original}, nil
	case info.Format == engine.FormatPNG && !info.Animated && !info.MultiPage && info.Orientation <= 1 && len(original) > 0:
		return HTMLSource{MIME: "image/png", Data: original}, nil
	}
	w, h := doc.Size()
	if !(w > 0) || !(h > 0) {
		return HTMLSource{}, errors.New("export: the picture has no size")
	}
	// The picture is drawn at the resolution of its file: as many pixels to
	// the unit as the file has along its longer side.
	ppu := 1.0
	if info.WidthPx > 0 && info.HeightPx > 0 {
		ppu = math.Max(float64(info.WidthPx), float64(info.HeightPx)) / math.Max(w, h)
	}
	if px := w * ppu * h * ppu; px > maxRenderPixels {
		ppu *= math.Sqrt(maxRenderPixels / px)
	}
	img, err := doc.Render(engine.Box{W: w, H: h}, ppu, nil)
	if err != nil {
		return HTMLSource{}, err
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return HTMLSource{}, err
	}
	return HTMLSource{MIME: "image/png", Data: out.Bytes()}, nil
}

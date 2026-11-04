// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"strings"

	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/i18n"
	"github.com/gillesmagneville/image-print-layout/internal/units"
)

// prepareRuntime must run first: it points the engine's libraries at the data
// shipped with the program (Windows packages).
func prepareRuntime() error { return engine.ConfigureRuntime() }

// libraries names the libraries the rendering engine runs with.
func libraries() string {
	rsvg, cairo := engine.Versions()
	return fmt.Sprintf("librsvg %s, cairo %s", rsvg, cairo)
}

// selfTestSVG is a 100 x 60 document with a background, a block, a line of
// text with accents and a small embedded PNG (a red square): enough to
// exercise loading, measuring, rendering, fonts, image decoding and PDF export.
const selfTestSVG = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="100" height="60" viewBox="0 0 100 60">
  <rect id="fond" width="100" height="60" fill="#f4f5f8"/>
  <rect id="bloc" x="10" y="10" width="40" height="20" fill="#2d6cdf"/>
  <text id="texte" x="10" y="48" font-family="sans-serif" font-size="8">Essai é à ç</text>
  <image id="image" x="60" y="10" width="20" height="20" xlink:href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAQAAAAECAIAAAAmkwkpAAAAEElEQVR42mO4aWAARwzEcQAHghORZScVxwAAAABJRU5ErkJggg=="/>
</svg>`

// Three images of the same four quadrants (red top left, green top right, blue
// bottom left, white bottom right), one for each format a package must be able
// to read besides SVG: a JPEG of 16 x 16 pixels, and a PNG and a TIFF of 4 x 4
// pixels at 300 dpi. They are read by the image loaders of gdk-pixbuf, which
// packages ship apart from the library itself.
const (
	selfTestJPEG = "/9j/2wCEAAMCAgMCAgMDAwMEAwMEBQgFBQQEBQoHBwYIDAoMDAsKCwsNDhIQDQ4RDgsLEBYQERMUFRUVDA8XGBYUGBIUFRQBAwQEBQQFCQUFCRQNCw0UFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFP/AABEIABAAEAMBIgACEQEDEQH/xAGiAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMFBQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+gEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoLEQACAQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/APnSvsavy8r+lujxM8DP9XPqf/Cl7T2ntP8Al1y25eT/AKeu97i8Ycz/AOI0/UPc+qfVPa9fa8/tfZ+VPl5fZ+d79La//9k="
	selfTestPNG  = "iVBORw0KGgoAAAANSUhEUgAAAAQAAAAECAIAAAAmkwkpAAAACXBIWXMAAC4jAAAuIwF4pT92AAAAIElEQVR4nGL5zwACjGCKiQEJsDAygAQh8igygAAAAP//ipEEC7T7bwUAAAAASUVORK5CYII="
	selfTestTIFF = "SUkqAAgAAAANAAABBAABAAAABAAAAAEBBAABAAAABAAAAAIBAwADAAAAqgAAAAMBAwABAAAAAQAAAAYBAwABAAAAAgAAABEBBAABAAAAwAAAABIBAwABAAAAAQAAABUBAwABAAAAAwAAABYBBAABAAAABAAAABcBBAABAAAAMAAAABoBBQABAAAAsAAAABsBBQABAAAAuAAAACgBAwABAAAAAgAAAAAAAAAIAAgACAAwdQAAZAAAADB1AABkAAAA/wAA/wAAAP8AAP8A/wAA/wAAAP8AAP8AAAD/AAD/////////AAD/AAD/////////"
)

// checkImages reads the three images above with the engine and checks what it
// sees: the size they have at their resolution, the red of the top left corner
// and the white of the bottom right one, and that a JPEG goes into a PDF as it
// is. It is what tells a package whose image loaders are missing.
func checkImages(out io.Writer, tr *i18n.Translator, ok func(string)) error {
	for _, c := range []struct {
		name   string
		format engine.Format
		data   string
		units  float64 // the size in units: pixels at 96 dpi, or at 300
	}{
		{"JPEG", engine.FormatJPEG, selfTestJPEG, 16},
		{"PNG", engine.FormatPNG, selfTestPNG, 4 * 96.0 / 300},
		{"TIFF", engine.FormatTIFF, selfTestTIFF, 4 * 96.0 / 300},
	} {
		raw, err := base64.StdEncoding.DecodeString(c.data)
		if err != nil {
			return err
		}
		if got := engine.Detect(raw); got != c.format {
			return errors.New(tr.T("selftest.error.image_detect", "name", c.name, "got", fmt.Sprintf("%q", got)))
		}
		d, err := engine.LoadImage(raw, "")
		if err != nil {
			return errors.New(tr.T("selftest.error.image_load", "name", c.name, "error", err.Error()))
		}
		defer d.Close()
		w, h := d.Size()
		if math.Abs(w-c.units) > 0.01 || math.Abs(h-c.units) > 0.01 {
			return errors.New(tr.T("selftest.error.image_size", "name", c.name, "expected", tr.Number(c.units, 2), "width", w, "height", h))
		}
		px := 32 / w
		img, err := d.Render(engine.Box{W: w, H: h}, px, nil)
		if err != nil {
			return err
		}
		red := color.RGBAModel.Convert(img.At(4, 4)).(color.RGBA)
		white := color.RGBAModel.Convert(img.At(28, 28)).(color.RGBA)
		if red.R < 200 || red.G > 80 || red.B > 80 || white.R < 200 || white.G < 200 || white.B < 200 {
			return errors.New(tr.T("selftest.error.image_pixels", "name", c.name, "red", red, "white", white))
		}
		if c.format == engine.FormatJPEG {
			var pdf bytes.Buffer
			page := engine.PageSpec{WidthPt: 100, HeightPt: 100, BandTop: 0, BandBottom: h, PtPerUnit: 1}
			if err := d.ExportPDF(&pdf, []engine.PageSpec{page}, engine.PDFOptions{WhitePage: true}); err != nil {
				return err
			}
			if !bytes.Contains(pdf.Bytes(), []byte("/DCTDecode")) {
				return errors.New(tr.T("selftest.error.jpeg_not_embedded"))
			}
		}
		ok(tr.T("selftest.ok.image", "name", c.name, "width", d.ImageInfo().WidthPx, "height", d.ImageInfo().HeightPx))
	}
	var have []string
	for _, f := range engine.SupportedImageFormats() {
		have = append(have, f.Label())
	}
	fmt.Fprintln(out, tr.T("selftest.formats", "formats", strings.Join(have, ", ")))
	return nil
}

// runSelfTest checks, on the machine and with the libraries it runs on, the
// whole chain an installed package depends on: the SVG library and its
// fonts, the drawing library, the PDF writer and the readers of images. It is what a package
// smoke test, or a user whose export fails, runs first. What it says is in the
// language of tr.
func runSelfTest(out io.Writer, tr *i18n.Translator) error {
	rsvg, cairo := engine.Versions()
	fmt.Fprintln(out, tr.T("selftest.engine", "rsvg", rsvg, "cairo", cairo))
	if conf := os.Getenv("FONTCONFIG_FILE"); conf != "" {
		fmt.Fprintln(out, tr.T("selftest.fontconfig", "path", conf))
	}
	if cache := os.Getenv("GDK_PIXBUF_MODULE_FILE"); cache != "" {
		fmt.Fprintln(out, tr.T("selftest.loaders", "path", cache))
	}
	ok := func(what string) { fmt.Fprintf(out, "  ok  %s\n", what) }

	d, err := engine.Load([]byte(selfTestSVG), "")
	if err != nil {
		return err
	}
	defer d.Close()
	w, h := d.Size()
	if math.Abs(w-100) > 1e-6 || math.Abs(h-60) > 1e-6 {
		return errors.New(tr.T("selftest.error.svg_size", "width", w, "height", h))
	}
	ok(tr.T("selftest.ok.svg"))

	box, err := d.Measure("bloc")
	if err != nil {
		return err
	}
	if math.Abs(box.X-10) > 0.01 || math.Abs(box.Y-10) > 0.01 || math.Abs(box.W-40) > 0.01 || math.Abs(box.H-20) > 0.01 {
		return errors.New(tr.T("selftest.error.box", "x", box.X, "y", box.Y, "width", box.W, "height", box.H))
	}
	ok(tr.T("selftest.ok.measure"))

	img, err := d.Render(engine.Box{X: 0, Y: 0, W: w, H: h}, 4, color.White)
	if err != nil {
		return err
	}
	blue := color.RGBAModel.Convert(img.At(4*20, 4*20)).(color.RGBA)
	paper := color.RGBAModel.Convert(img.At(4*2, 4*2)).(color.RGBA)
	if blue.B < 180 || blue.R > 100 || paper.R < 230 || paper.G < 230 || paper.B < 230 {
		return errors.New(tr.T("selftest.error.render", "block", blue, "paper", paper))
	}
	red := color.RGBAModel.Convert(img.At(4*70, 4*20)).(color.RGBA)
	if red.R < 180 || red.G > 110 || red.B > 110 {
		return errors.New(tr.T("selftest.error.embedded_png", "red", red))
	}
	ok(tr.T("selftest.ok.render"))

	a4, found := units.Lookup("A4")
	if !found {
		return errors.New(tr.T("selftest.error.no_a4"))
	}
	sheetW, sheetH := a4.Oriented(false)
	s, err := units.MMPerUnit(150, w)
	if err != nil {
		return err
	}
	page := engine.PageSpec{
		WidthPt: units.MMToPt(sheetW), HeightPt: units.MMToPt(sheetH),
		BandTop: 0, BandBottom: h,
		OriginXPt: units.MMToPt(30), OriginYPt: units.MMToPt(10),
		PtPerUnit: units.MMToPt(s),
	}
	var first, second bytes.Buffer
	for _, buf := range []*bytes.Buffer{&first, &second} {
		if err := d.ExportPDF(buf, []engine.PageSpec{page}, engine.PDFOptions{WhitePage: true}); err != nil {
			return err
		}
	}
	pdf := bytes.TrimRight(first.Bytes(), "\r\n ")
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || !bytes.HasSuffix(pdf, []byte("%%EOF")) {
		return errors.New(tr.T("selftest.error.pdf_incomplete"))
	}
	ok(tr.T("selftest.ok.pdf", "bytes", first.Len()))

	if !bytes.Contains(pdf, []byte("/FontFile")) {
		return errors.New(tr.T("selftest.error.no_font"))
	}
	ok(tr.T("selftest.ok.font"))

	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		return errors.New(tr.T("selftest.error.not_deterministic"))
	}
	ok(tr.T("selftest.ok.deterministic"))

	if err := checkImages(out, tr, ok); err != nil {
		return err
	}

	fmt.Fprintln(out, tr.T("selftest.success"))
	return nil
}

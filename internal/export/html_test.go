// SPDX-License-Identifier: GPL-3.0-only

package export

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/layout"
	"github.com/gillesmagneville/image-print-layout/internal/model"
)

var svgBytes = []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="5479"><rect width="10" height="10"/></svg>`)

func htmlOptions(cut bool) HTMLOptions {
	return HTMLOptions{
		Title: "mini <CAD> & co", Lang: "fr", Generator: "Image Print Layout test",
		Cut: cut, WhitePage: true,
		PageLabel: func(n, total int) string { return fmt.Sprintf("Page %d sur %d", n, total) },
	}
}

func writeHTML(t *testing.T, pl *layout.Plan, cut bool) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := WriteHTML(&b, pl, HTMLSource{MIME: "image/svg+xml", Data: svgBytes}, htmlOptions(cut)); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestCutPageHasOnePieceForEachPageOfThePlan(t *testing.T) {
	pl := planOf(t, acceptance(1380.5, 2760, 4120.25))
	page := writeHTML(t, pl, true)
	if got := len(htmlBoxRe.FindAll(page, -1)); got != 4 {
		t.Fatalf("%d pieces, want 4", got)
	}
	if items := VerifyHTML(page, pl, HTMLSource{MIME: "image/svg+xml", Data: svgBytes}, true); len(items) > 0 {
		t.Errorf("a page just written does not verify: %v", items)
	}
	// The picture is written once, whatever the number of pieces, and the
	// width is the one of the plan: 200 mm.
	if n := strings.Count(string(page), "base64,"); n != 1 {
		t.Errorf("the picture is written %d times, want once", n)
	}
	if !strings.Contains(string(page), "main{width:min(100%,200mm)") {
		t.Errorf("the page is not 200 mm wide")
	}
	// The captions are in the language asked for, and the title is escaped.
	for _, want := range []string{"Page 1 sur 4", "Page 4 sur 4", "<title>mini &lt;CAD&gt; &amp; co</title>", `<html lang="fr">`} {
		if !strings.Contains(string(page), want) {
			t.Errorf("the page does not say %q", want)
		}
	}
}

func TestUncutPageShowsThePictureWholeInOnePiece(t *testing.T) {
	// Cuts in the plan make no difference to a page that is not cut.
	pl := planOf(t, acceptance(1380.5, 2760, 4120.25))
	page := writeHTML(t, pl, false)
	boxes := htmlBoxRe.FindAllSubmatch(page, -1)
	if len(boxes) != 1 {
		t.Fatalf("%d pieces, want 1", len(boxes))
	}
	if string(boxes[0][3]) != "100" || string(boxes[0][4]) != "0" {
		t.Errorf("the piece shows the picture at %s%% from %s%%, want the whole of it", boxes[0][3], boxes[0][4])
	}
	if strings.Contains(string(page), `class="label"`) {
		t.Error("a page that is not cut has captions")
	}
	if items := VerifyHTML(page, pl, HTMLSource{MIME: "image/svg+xml", Data: svgBytes}, false); len(items) > 0 {
		t.Errorf("the whole page does not verify: %v", items)
	}
}

func TestPageIsTheSameForTheSamePlan(t *testing.T) {
	pl := planOf(t, acceptance(1380.5, 2760, 4120.25))
	if !bytes.Equal(writeHTML(t, pl, true), writeHTML(t, pl, true)) {
		t.Error("two exports of the same plan differ")
	}
}

// css is what a browser does with the style of a piece: the box is as wide as
// the page, as high as its aspect ratio says; the picture in it is as wide as
// the box and as high as a share of the box; its position in the box is a share
// of what the box has left over (negative: the picture is higher). It returns
// the part of the picture, in units, that the top and the bottom of the box show.
func css(t *testing.T, docW, docH float64, style string) (top, bottom float64) {
	t.Helper()
	m := regexp.MustCompile(`aspect-ratio:([0-9.]+) / ([0-9.]+);background-size:100% ([0-9.]+)%;background-position:0 ([0-9.]+)%`).FindStringSubmatch(style)
	if m == nil {
		t.Fatalf("unexpected style %q", style)
	}
	num := func(s string) float64 {
		var v float64
		fmt.Sscan(s, &v)
		return v
	}
	const boxW = 1000.0
	boxH := boxW * num(m[2]) / num(m[1])
	imgW, imgH := boxW, boxH*num(m[3])/100
	offset := num(m[4]) / 100 * (boxH - imgH)
	unitPx := imgW / docW // pixels to a unit, across; the picture has the shape of the document
	if got := imgH / docH; math.Abs(got-unitPx) > 1e-6*unitPx {
		t.Fatalf("the picture is stretched: %.6f px a unit down, %.6f across", got, unitPx)
	}
	return -offset / unitPx, (-offset + boxH) / unitPx
}

func TestPiecesShowTheBandsOfTheDocumentAsABrowserReadsTheirStyle(t *testing.T) {
	for _, d := range []layout.Doc{{W: 1280, H: 5479}, {W: 100, H: 100}, {W: 3000, H: 4000}} {
		for _, bands := range [][2]float64{{0, 1}, {0, 0.3}, {0.3, 0.55}, {0.55, 1}, {0.999, 1}} {
			p := htmlPiece{top: bands[0] * d.H, height: (bands[1] - bands[0]) * d.H}
			top, bottom := css(t, d.W, d.H, pieceStyle(d.W, d.H, p))
			if math.Abs(top-p.top) > 1e-3 || math.Abs(bottom-(p.top+p.height)) > 1e-3 {
				t.Errorf("%vx%v band %v-%v: the box shows %.4f to %.4f", d.W, d.H, p.top, p.top+p.height, top, bottom)
			}
		}
	}
}

func TestVerifyFindsAPageThatIsNotThePlans(t *testing.T) {
	pl := planOf(t, acceptance(1380.5, 2760, 4120.25))
	src := HTMLSource{MIME: "image/svg+xml", Data: svgBytes}
	page := writeHTML(t, pl, true)

	// A cut moved after the page was written.
	moved := planOf(t, acceptance(1380.5, 2800, 4120.25))
	items := VerifyHTML(page, moved, src, true)
	if !has(items, "export.html_band") {
		t.Errorf("a moved cut is not found: %v", codes(items))
	}
	// A cut added.
	more := planOf(t, acceptance(1380.5, 2760, 4120.25, 5000))
	if items := VerifyHTML(page, more, src, true); !has(items, "export.html_pieces") {
		t.Errorf("a missing piece is not found: %v", codes(items))
	}
	// Another scale: another width.
	wide := acceptance(1380.5, 2760, 4120.25)
	wide.Scale = model.Scale{Mode: model.Width, WidthMM: 150}
	if items := VerifyHTML(page, planOf(t, wide), src, true); !has(items, "export.html_width") {
		t.Errorf("another width is not found: %v", codes(items))
	}
	// Another picture.
	if items := VerifyHTML(page, pl, HTMLSource{MIME: "image/svg+xml", Data: []byte("<svg/>")}, true); !has(items, "export.html_picture") {
		t.Errorf("another picture is not found: %v", codes(items))
	}
	if items := VerifyHTML(page, pl, HTMLSource{MIME: "image/png", Data: svgBytes}, true); !has(items, "export.html_picture") {
		t.Errorf("another type is not found: %v", codes(items))
	}
	// A page that has lost its picture.
	broken := bytes.Replace(page, []byte("base64,"), []byte("base63,"), 1)
	if items := VerifyHTML(broken, pl, src, true); !has(items, "export.html_picture") {
		t.Errorf("a missing picture is not found: %v", codes(items))
	}
}

func TestPageHasTheBackgroundOfThePageAsked(t *testing.T) {
	pl := planOf(t, acceptance(2000))
	for _, white := range []bool{true, false} {
		opt := htmlOptions(true)
		opt.WhitePage = white
		var b bytes.Buffer
		if err := WriteHTML(&b, pl, HTMLSource{MIME: "image/png", Data: []byte{1, 2, 3}}, opt); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(b.String(), `class="piece doc white"`); got != white {
			t.Errorf("white page %v: the pieces are white: %v", white, got)
		}
	}
}

func TestWriteHTMLRefusesWhatItCannotWrite(t *testing.T) {
	pl := planOf(t, acceptance())
	var b bytes.Buffer
	if err := WriteHTML(&b, pl, HTMLSource{}, htmlOptions(true)); err == nil {
		t.Error("a page with no picture was written")
	}
	if err := WriteHTML(&b, nil, HTMLSource{MIME: "image/png", Data: []byte{1}}, htmlOptions(true)); err == nil {
		t.Error("a page with no plan was written")
	}
	huge := HTMLSource{MIME: "image/png", Data: make([]byte, MaxHTMLBytes)}
	err := WriteHTML(io.Discard, pl, huge, htmlOptions(true))
	if e, ok := diag.As(err); !ok || e.Code != "export.html_too_large" {
		t.Errorf("a page of more than the limit gave %v", err)
	}
}

func TestRunHTMLWritesTheFileOnlyOnceItIsVerified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.html")
	pl := planOf(t, acceptance(1380.5, 2760, 4120.25))
	res, err := RunHTML(HTMLRequest{Plan: pl, Source: HTMLSource{MIME: "image/svg+xml", Data: svgBytes}, Options: htmlOptions(true), Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if res.Pages != 4 || res.Bytes <= 0 {
		t.Errorf("result %+v, want 4 pieces", res)
	}
	data, err := os.ReadFile(path)
	if err != nil || int64(len(data)) != res.Bytes {
		t.Fatalf("the file has %d bytes (%v), the result says %d", len(data), err, res.Bytes)
	}
	left, _ := filepath.Glob(filepath.Join(dir, ".*"))
	if len(left) > 0 {
		t.Errorf("temporary files left: %v", left)
	}

	// A refused export leaves the good file where it was, and nothing beside it.
	_, err = RunHTML(HTMLRequest{Plan: pl, Source: HTMLSource{}, Options: htmlOptions(true), Path: path})
	if err == nil {
		t.Fatal("an export with no picture succeeded")
	}
	again, _ := os.ReadFile(path)
	if !bytes.Equal(again, data) {
		t.Error("a failed export changed the file")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".*")); len(left) > 0 {
		t.Errorf("temporary files left after a failure: %v", left)
	}
	// A plan of no page is no document.
	if _, err := RunHTML(HTMLRequest{Plan: &layout.Plan{}, Path: path}); err == nil {
		t.Error("an empty plan was exported")
	}
}

func TestCheckHTMLLeavesThePaperAlone(t *testing.T) {
	// A4 portrait at 400 mm wide: the pages are too wide and too tall for the
	// sheet. The PDF is refused; a page HTML does not care.
	p := acceptance(2000)
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 400}
	pl := planOf(t, p)
	in := Input{Project: p, Plan: pl}
	if !Blocked(Check(in)) {
		t.Fatal("the PDF of a sheet too small is not refused: the test says nothing")
	}
	for _, cut := range []bool{true, false} {
		if items := CheckHTML(in, cut); len(items) > 0 {
			t.Errorf("cut %v: the page HTML is refused for the paper: %v", cut, codes(items))
		}
	}
	// What is about the document stays: a background to replace that is not there.
	p.Background.Document = model.WhitenBackground
	in = Input{Project: p, Plan: pl}
	if items := CheckHTML(in, false); !has(items, "export.no_background") {
		t.Errorf("the background is not checked: %v", codes(items))
	}
	// A transparent page under a background that stays is a matter of the PDF.
	p.Background.Document = model.KeepBackground
	p.Background.Page = model.TransparentPage
	if items := CheckHTML(Input{Project: p, Plan: pl, HasBackground: true}, false); has(items, "export.background_covers") {
		t.Errorf("a page HTML is warned that the PDF is not transparent: %v", codes(items))
	}
	// The findings of the analysis are listed.
	warn := diag.New("svg.filter", 0)
	if items := CheckHTML(Input{Project: p, Plan: pl, Findings: []diag.Item{warn}}, true); !has(items, "svg.filter") {
		t.Errorf("the findings are not listed: %v", codes(items))
	}
	// A plan with no page is not exported.
	if items := CheckHTML(Input{Plan: &layout.Plan{}}, true); !has(items, "export.no_pages") {
		t.Errorf("an empty plan is not refused: %v", codes(items))
	}
}

// ---- the picture of a document -------------------------------------------------------------

// fakePicture is an engine document that is a picture of w x h units, drawn as
// a plain colour of the size it is asked for.
type fakePicture struct {
	fakeDoc
	w, h   float64
	asked  engine.Box
	pxUnit float64
	err    error
}

func (d *fakePicture) Size() (float64, float64) { return d.w, d.h }
func (d *fakePicture) Render(win engine.Box, ppu float64, _ color.Color) (*image.RGBA, error) {
	d.asked, d.pxUnit = win, ppu
	if d.err != nil {
		return nil, d.err
	}
	img := image.NewRGBA(image.Rect(0, 0, int(math.Ceil(win.W*ppu)), int(math.Ceil(win.H*ppu))))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 200, 255
	}
	return img, nil
}

func TestImageSourceKeepsTheFileWhenABrowserShowsItAsTheEngineDoes(t *testing.T) {
	file := []byte("the bytes of the file")
	doc := &fakePicture{w: 100, h: 50}
	for _, c := range []struct {
		name string
		info engine.ImageInfo
		mime string // "" when the picture is drawn
	}{
		{"jpeg", engine.ImageInfo{Format: engine.FormatJPEG, EmbeddedJPEG: true, Orientation: 1}, "image/jpeg"},
		{"jpeg turned by its tag (the browser turns it too)", engine.ImageInfo{Format: engine.FormatJPEG, EmbeddedJPEG: true, Orientation: 6}, "image/jpeg"},
		{"jpeg the PDF does not take as it is", engine.ImageInfo{Format: engine.FormatJPEG, Orientation: 1}, "image/png"},
		{"png", engine.ImageInfo{Format: engine.FormatPNG, Orientation: 1}, "image/png"},
		{"png with no tag", engine.ImageInfo{Format: engine.FormatPNG}, "image/png"},
		{"png turned by a tag", engine.ImageInfo{Format: engine.FormatPNG, Orientation: 6}, ""},
		{"animated png", engine.ImageInfo{Format: engine.FormatPNG, Animated: true}, ""},
		{"tiff", engine.ImageInfo{Format: engine.FormatTIFF, Orientation: 1}, ""},
		{"animated gif", engine.ImageInfo{Format: engine.FormatGIF, Animated: true}, ""},
		{"webp", engine.ImageInfo{Format: engine.FormatWebP}, ""},
		{"bmp", engine.ImageInfo{Format: engine.FormatBMP}, ""},
	} {
		src, err := ImageSource(doc, c.info, file)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		wantFile := c.mime != "" && c.mime != "image/png" || c.name == "png" || c.name == "png with no tag"
		if wantFile {
			if !bytes.Equal(src.Data, file) || src.MIME != c.mime {
				t.Errorf("%s: gives %s of %d bytes, want the file as it was (%s)", c.name, src.MIME, len(src.Data), c.mime)
			}
			continue
		}
		if src.MIME != "image/png" || bytes.Equal(src.Data, file) {
			t.Errorf("%s: gives %s of %d bytes, want a PNG drawn by the engine", c.name, src.MIME, len(src.Data))
			continue
		}
		if _, err := png.Decode(bytes.NewReader(src.Data)); err != nil {
			t.Errorf("%s: the PNG drawn is not one: %v", c.name, err)
		}
	}
}

func TestImageSourceDrawsAtTheResolutionOfTheFile(t *testing.T) {
	// A picture of 3000 x 2000 pixels at 300 dpi is 960 x 640 units: it is drawn
	// at 3.125 pixels to the unit, its own pixels.
	doc := &fakePicture{w: 960, h: 640}
	info := engine.ImageInfo{Format: engine.FormatTIFF, WidthPx: 3000, HeightPx: 2000, Orientation: 1}
	src, err := ImageSource(doc, info, []byte("tiff"))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(doc.pxUnit-3.125) > 1e-9 || doc.asked != (engine.Box{W: 960, H: 640}) {
		t.Errorf("drawn at %v px a unit, window %+v", doc.pxUnit, doc.asked)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(src.Data))
	if err != nil || cfg.Width != 3000 || cfg.Height != 2000 {
		t.Errorf("the PNG is %dx%d (%v), want 3000x2000", cfg.Width, cfg.Height, err)
	}

	// A turned picture (6: its file is 3000 x 2000, it is shown 2000 x 3000).
	doc = &fakePicture{w: 640, h: 960}
	info.Orientation = 6
	src, err = ImageSource(doc, info, []byte("tiff"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg, _ := png.DecodeConfig(bytes.NewReader(src.Data)); cfg.Width != 2000 || cfg.Height != 3000 {
		t.Errorf("the turned picture is %dx%d, want 2000x3000", cfg.Width, cfg.Height)
	}

	// A picture of more pixels than the limit is drawn smaller, in proportion.
	doc = &fakePicture{w: 20000, h: 10000}
	info = engine.ImageInfo{Format: engine.FormatBMP, WidthPx: 20000, HeightPx: 10000}
	src, err = ImageSource(doc, info, []byte("bmp"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ = png.DecodeConfig(bytes.NewReader(src.Data))
	if px := cfg.Width * cfg.Height; px > maxRenderPixels+maxRenderPixels/100 || math.Abs(float64(cfg.Width)/float64(cfg.Height)-2) > 0.01 {
		t.Errorf("200 megapixels are drawn as %dx%d", cfg.Width, cfg.Height)
	}
}

func TestImageSourceReportsWhatTheEngineSays(t *testing.T) {
	doc := &fakePicture{w: 10, h: 10, err: errors.New("no memory")}
	if _, err := ImageSource(doc, engine.ImageInfo{Format: engine.FormatTIFF}, []byte("x")); err == nil {
		t.Error("a failed drawing gave a picture")
	}
	if _, err := ImageSource(&fakePicture{}, engine.ImageInfo{Format: engine.FormatTIFF}, []byte("x")); err == nil {
		t.Error("a picture with no size gave a picture")
	}
}

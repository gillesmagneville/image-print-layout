// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"errors"
	"image"
	"image/color"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/layout"
	"github.com/gillesmagneville/image-print-layout/internal/model"
)

// pngBytes is enough of a PNG for the signature to say what it is: the
// fake engine does not decode.
var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

// fakeImage is an engine document that is a picture: it draws bands of ink on
// a background, the way a scanned page or a drawing on a sheet looks.
type fakeImage struct {
	fakeDoc
	info  engine.ImageInfo
	paper color.RGBA // the background; alpha 0 is a transparent one
	ink   color.RGBA
	bands [][2]float64 // in document units
	noise int          // the amplitude of the noise on every channel
	specs bool         // one stray dark pixel on every row of the background
	full  bool         // random pixels everywhere: a photograph
}

func (d *fakeImage) ImageInfo() engine.ImageInfo { return d.info }

func (d *fakeImage) Render(win engine.Box, px float64, _ color.Color) (*image.RGBA, error) {
	w, h := int(win.W*px), int(win.H*px)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(1))
	premult := func(c color.RGBA) color.RGBA {
		a := uint32(c.A)
		return color.RGBA{R: uint8(uint32(c.R) * a / 255), G: uint8(uint32(c.G) * a / 255), B: uint8(uint32(c.B) * a / 255), A: c.A}
	}
	jitter := func(c color.RGBA) color.RGBA {
		if d.noise == 0 || c.A == 0 {
			return c
		}
		j := func(v uint8) uint8 {
			n := int(v) + rng.Intn(2*d.noise+1) - d.noise
			return uint8(min(255, max(0, n)))
		}
		return color.RGBA{R: j(c.R), G: j(c.G), B: j(c.B), A: c.A}
	}
	for y := 0; y < h; y++ {
		inBand := false
		for _, b := range d.bands {
			if float64(y) >= b[0]*px && float64(y) < b[1]*px {
				inBand = true
			}
		}
		for x := 0; x < w; x++ {
			c := d.paper
			if inBand && x >= w/10 && x < w*9/10 {
				c = d.ink
			}
			if d.full {
				c = color.RGBA{R: uint8(rng.Intn(256)), G: uint8(rng.Intn(256)), B: uint8(rng.Intn(256)), A: 255}
			}
			img.SetRGBA(x, y, premult(jitter(c)))
		}
		if d.specs && !inBand && w > 4 {
			img.SetRGBA(w/2, y, color.RGBA{A: 255})
		}
	}
	return img, nil
}

func imageServices(doc *fakeImage) (Services, *[]byte) {
	var loaded []byte
	return Services{Load: func(src []byte, path string) (engine.Document, error) {
		loaded = src
		doc.loaded = src
		return doc, nil
	}}, &loaded
}

var (
	white = color.RGBA{255, 255, 255, 255}
	black = color.RGBA{0, 0, 0, 255}
)

func near(a, b float64) bool { return a-b < 1.01 && b-a < 1.01 }

func wantBlocks(t *testing.T, got []layout.Block, want [][2]float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d blocks %v, want %d %v", len(got), got, len(want), want)
	}
	for i, b := range got {
		if !near(b.Top, want[i][0]) || !near(b.Bottom, want[i][1]) {
			t.Errorf("block %d is %.1f to %.1f, want %.1f to %.1f", i+1, b.Top, b.Bottom, want[i][0], want[i][1])
		}
	}
}

func TestBlocksOfAPictureAreTheStretchesThatAreNotItsBackground(t *testing.T) {
	bands := [][2]float64{{50, 120}, {200, 260}, {300, 390}}
	for name, info := range map[string]engine.ImageInfo{
		"96 dpi":  {XDPI: 96, YDPI: 96},
		"192 dpi": {XDPI: 192, YDPI: 192}, // drawn at 2 pixels to the unit
		"no dpi":  {},
	} {
		doc := &fakeImage{fakeDoc: fakeDoc{w: 100, h: 400}, info: info, paper: white, ink: black, bands: bands}
		blocks, err := rasterBlocks(doc, &info)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(name, func(t *testing.T) { wantBlocks(t, blocks, bands) })
	}
}

func TestNoiseAndSpecksDoNotMakeInk(t *testing.T) {
	// The grain of a scan or of a JPEG on a flat background, and the dust mark.
	bands := [][2]float64{{50, 120}, {200, 260}}
	doc := &fakeImage{fakeDoc: fakeDoc{w: 1000, h: 400}, paper: white, ink: black, bands: bands, noise: 8, specs: true}
	blocks, err := rasterBlocks(doc, &engine.ImageInfo{})
	if err != nil {
		t.Fatal(err)
	}
	wantBlocks(t, blocks, bands)
}

func TestTheBackgroundMayBeDarkOrTransparent(t *testing.T) {
	bands := [][2]float64{{40, 90}, {150, 210}}
	for name, c := range map[string]struct{ paper, ink color.RGBA }{
		"dark paper, light ink": {color.RGBA{20, 20, 30, 255}, color.RGBA{240, 240, 240, 255}},
		"coloured paper":        {color.RGBA{250, 240, 200, 255}, color.RGBA{30, 60, 160, 255}},
		"transparent, as white": {color.RGBA{}, color.RGBA{0, 0, 0, 255}},
	} {
		doc := &fakeImage{fakeDoc: fakeDoc{w: 100, h: 300}, paper: c.paper, ink: c.ink, bands: bands}
		blocks, err := rasterBlocks(doc, &engine.ImageInfo{})
		if err != nil {
			t.Fatal(err)
		}
		t.Run(name, func(t *testing.T) { wantBlocks(t, blocks, bands) })
	}
}

func TestAPictureWithNoBackgroundOrNoVoidHasNoBlocks(t *testing.T) {
	// A photograph: nothing tells the picture from a background.
	photo := &fakeImage{fakeDoc: fakeDoc{w: 100, h: 300}, paper: white, ink: black, full: true}
	if blocks, err := rasterBlocks(photo, &engine.ImageInfo{}); err != nil || blocks != nil {
		t.Errorf("a photograph has blocks: %v, %v", blocks, err)
	}
	// Ink from top to bottom, with a white margin on each side: no stretch to
	// cut in.
	solid := &fakeImage{fakeDoc: fakeDoc{w: 100, h: 300}, paper: white, ink: black, bands: [][2]float64{{0, 300}}}
	if blocks, err := rasterBlocks(solid, &engine.ImageInfo{}); err != nil || blocks != nil {
		t.Errorf("a picture without a void has blocks: %v, %v", blocks, err)
	}
	// A blank picture.
	blank := &fakeImage{fakeDoc: fakeDoc{w: 100, h: 300}, paper: white, ink: black}
	if blocks, err := rasterBlocks(blank, &engine.ImageInfo{}); err != nil || blocks != nil {
		t.Errorf("a blank picture has blocks: %v, %v", blocks, err)
	}
}

func TestAPictureIsOpenedByItsContent(t *testing.T) {
	doc := &fakeImage{
		fakeDoc: fakeDoc{w: 800, h: 600}, paper: white, ink: black, bands: [][2]float64{{100, 200}, {300, 400}},
		info: engine.ImageInfo{Format: engine.FormatPNG, WidthPx: 800, HeightPx: 600, Truncated: true, MultiPage: true, Animated: true},
	}
	svc, loaded := imageServices(doc)
	// The name says SVG: the content decides.
	o, err := Prepare(svc, pngBytes, "/tmp/scan.svg", model.Background{Document: model.WhitenBackground})
	if err != nil {
		t.Fatal(err)
	}
	if string(*loaded) != string(pngBytes) {
		t.Error("the engine must receive the file as it is, not a working copy")
	}
	d := o.Document
	if !d.IsImage() || d.Format != engine.FormatPNG || d.Analysis != nil || d.Image == nil || d.Image.WidthPx != 800 {
		t.Errorf("document = %+v", d)
	}
	if d.Size != (layout.Doc{W: 800, H: 600}) || len(d.Blocks) != 2 {
		t.Errorf("size %v, %d blocks", d.Size, len(d.Blocks))
	}
	codes := map[string]diag.Level{}
	for _, it := range d.Findings {
		codes[it.Code] = diag.Registry[it.Code].Level
	}
	for _, want := range []string{"image.truncated", "image.multipage", "image.animated", "image.no_dpi"} {
		if _, ok := codes[want]; !ok {
			t.Errorf("no finding %s in %v", want, d.Findings)
		}
	}
	if codes["image.no_dpi"] != diag.Info || codes["image.truncated"] != diag.Warning {
		t.Errorf("levels: %v", codes)
	}

	// With a resolution, and a whole file, there is nothing to say.
	doc.info = engine.ImageInfo{Format: engine.FormatPNG, XDPI: 300, YDPI: 300}
	o, err = Prepare(svc, pngBytes, "/tmp/scan.png", model.Background{})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Document.Findings) != 0 {
		t.Errorf("findings = %v", o.Document.Findings)
	}
}

func TestAnImageHasNoBackgroundToChange(t *testing.T) {
	doc := &fakeImage{fakeDoc: fakeDoc{w: 800, h: 600}, paper: white, ink: black, info: engine.ImageInfo{XDPI: 96, YDPI: 96}}
	svc, loaded := imageServices(doc)
	o, err := Prepare(svc, pngBytes, "/tmp/scan.png", model.Background{})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if err := s.Open(o.Document); err != nil {
		t.Fatal(err)
	}
	if _, found := s.DocumentBackground(); found {
		t.Error("an image has no background rectangle")
	}
	for _, mode := range []model.DocBackground{model.WhitenBackground, model.RemoveBackground} {
		if err := s.SetDocBackground(mode); diag.CodeOf(err) != "background.none" {
			t.Errorf("SetDocBackground(%s) = %v, want background.none", mode, err)
		}
	}
	// Loading it again gives the same file back.
	*loaded = nil
	if _, err := Rebuild(svc, o.Document, model.Background{}); err != nil || string(*loaded) != string(pngBytes) {
		t.Errorf("Rebuild: %v, loaded %q", err, *loaded)
	}
	if len(s.Cuts()) != 0 || s.Plan() == nil {
		t.Error("the plan of an image is computed like any other")
	}
}

func TestImageErrorsAreDiagnostics(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		code string
		args []any
	}{
		"too large":   {&engine.ImageError{Kind: engine.ImageTooLarge, W: 20000, H: 20000}, "image.too_large", []any{"w", 20000, "h", 20000, "mp", 400.0, "max", 100}},
		"unsupported": {&engine.ImageError{Kind: engine.ImageUnsupported, Format: engine.FormatWebP}, "image.unsupported", []any{"format", "WebP"}},
		"unreadable":  {&engine.ImageError{Kind: engine.ImageUnreadable, Reason: "bad data"}, "image.unreadable", []any{"reason", "bad data"}},
	} {
		svc := Services{Load: func([]byte, string) (engine.Document, error) { return nil, c.err }}
		_, err := Prepare(svc, pngBytes, "/tmp/x.png", model.Background{})
		e, ok := diag.As(err)
		if !ok || e.Code != c.code {
			t.Errorf("%s: err = %v, want code %s", name, err, c.code)
			continue
		}
		if len(e.Args) != len(c.args) {
			t.Errorf("%s: args = %v, want %v", name, e.Args, c.args)
			continue
		}
		for i := range c.args {
			if e.Args[i] != c.args[i] {
				t.Errorf("%s: arg %d = %v, want %v", name, i, e.Args[i], c.args[i])
			}
		}
		if _, known := diag.Registry[c.code]; !known || diag.Registry[c.code].Level != diag.Failure {
			t.Errorf("%s: the code must be a registered failure", name)
		}
	}
	// An error that is not about an image is kept as it is.
	other := errors.New("engine: not built with cgo")
	svc := Services{Load: func([]byte, string) (engine.Document, error) { return nil, other }}
	if _, err := Prepare(svc, pngBytes, "/tmp/x.png", model.Background{}); err != other {
		t.Errorf("err = %v", err)
	}
}

func TestAFileThatIsNeitherAnImageNorAnSVGIsSaidSo(t *testing.T) {
	svc := Services{Load: func([]byte, string) (engine.Document, error) {
		return nil, errors.New("engine: cannot read the SVG: bad")
	}}
	_, err := Prepare(svc, []byte("just some text"), "/tmp/notes.txt", model.Background{})
	if diag.CodeOf(err) != "file.unrecognised" {
		t.Errorf("err = %v, want file.unrecognised", err)
	}
	// An SVG that the engine refuses keeps the message of the engine.
	_, err = Prepare(svc, []byte(`<svg xmlns="http://www.w3.org/2000/svg"`), "/tmp/broken.svg", model.Background{})
	if err == nil || diag.CodeOf(err) != "" {
		t.Errorf("err = %v, want the engine's own", err)
	}
}

func TestAProjectOnAnImageIsSavedAndOpenedAgain(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "scan.png")
	if err := os.WriteFile(imgPath, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	doc := &fakeImage{fakeDoc: fakeDoc{w: 800, h: 3000}, paper: white, ink: black, info: engine.ImageInfo{XDPI: 96, YDPI: 96}}
	svc, _ := imageServices(doc)
	o, err := Open(svc, imgPath, model.Background{})
	if err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if err := s.Open(o.Document); err != nil {
		t.Fatal(err)
	}
	for _, y := range []float64{1000, 2000} {
		if _, err := s.AddCut(y, 0); err != nil {
			t.Fatal(err)
		}
	}
	projectPath := filepath.Join(dir, "scan.imgpl")
	if err := s.SaveProject(projectPath); err != nil {
		t.Fatal(err)
	}
	pf, err := ReadProject(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if pf.SourcePath != imgPath {
		t.Fatalf("the document of the project is %q, want %q", pf.SourcePath, imgPath)
	}
	o2, err := Open(svc, pf.SourcePath, pf.Project.Background)
	if err != nil {
		t.Fatal(err)
	}
	r := NewSession()
	items, err := r.OpenProject(o2.Document, pf)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("a project on the same file reports %v", items)
	}
	if cuts := r.Cuts(); len(cuts) != 2 || cuts[0].Position != 1000 || cuts[1].Position != 2000 {
		t.Errorf("cuts = %+v", cuts)
	}

	// A project that asks to whiten the background of an SVG, opened on an
	// image: the instruction cannot apply, and the project says what is shown.
	pf.Project.Background.Document = model.WhitenBackground
	pf.Project.Background.Element = "bg"
	r = NewSession()
	items, err = r.OpenProject(o2.Document, pf)
	if err != nil {
		t.Fatal(err)
	}
	lost := false
	for _, it := range items {
		lost = lost || it.Code == "project.background_lost"
	}
	if !lost || r.Project().Background.Document != model.KeepBackground || r.Project().Background.Element != "" {
		t.Errorf("items %v, background %+v", items, r.Project().Background)
	}
}

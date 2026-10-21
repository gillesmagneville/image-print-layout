// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package engine

import (
	"bytes"
	"image"
	"image/color"
	stdpng "image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gillesmagneville/pageslice/internal/testsvg"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func load(t *testing.T, svg []byte) *Doc {
	t.Helper()
	d, err := Load(svg, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(d.Close)
	return d
}

func TestLoadReadsTheDocumentSize(t *testing.T) {
	d := load(t, testsvg.Generate(testsvg.Options{}))
	if w, h := d.Size(); w != 1280 || h != 5479 {
		t.Errorf("Size = %v x %v, want 1280 x 5479", w, h)
	}
}

// The DPI is set explicitly: a document sized in millimetres must come out at
// 96 pixels per inch, whatever librsvg's own default is.
func TestPhysicalLengthsUseNinetySixDpi(t *testing.T) {
	d := load(t, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="210mm" height="297mm" viewBox="0 0 210 297"/>`))
	w, h := d.Size()
	if !near(w, 210/25.4*96, 0.01) || !near(h, 297/25.4*96, 0.01) {
		t.Errorf("Size = %v x %v px, want %v x %v", w, h, 210/25.4*96, 297/25.4*96)
	}
}

func TestLoadErrorsAreExplicit(t *testing.T) {
	cases := map[string][]byte{
		"empty":          nil,
		"not xml":        []byte("this is not an SVG"),
		"no size at all": []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>`),
	}
	for name, data := range cases {
		d, err := Load(data, "")
		if err == nil {
			d.Close()
			t.Errorf("%s: Load should fail", name)
			continue
		}
		if !strings.HasPrefix(err.Error(), "engine: ") {
			t.Errorf("%s: error %q is not an engine error", name, err)
		}
	}
}

// Measure gives the position in the drawing, ancestors' transformations
// included -- not the box of the element extracted alone.
func TestMeasureIsInDocumentCoordinates(t *testing.T) {
	d := load(t, testsvg.Generate(testsvg.Options{}))

	b, err := d.Measure(testsvg.TransformedID)
	if err != nil {
		t.Fatal(err)
	}
	want := testsvg.TransformedBox
	if !near(b.X, want[0], 0.01) || !near(b.Y, want[1], 0.01) || !near(b.W, want[2], 0.01) || !near(b.H, want[3], 0.01) {
		t.Errorf("Measure(%s) = %+v, want %v", testsvg.TransformedID, b, want)
	}

	banner, err := d.Measure(testsvg.BannerID)
	if err != nil {
		t.Fatal(err)
	}
	if banner.X != 0 || banner.Y != 0 || !near(banner.W, 1280, 0.01) || !near(banner.H, 84, 0.01) {
		t.Errorf("Measure(banner) = %+v", banner)
	}

	block, err := d.Measure(testsvg.BlockID(2, 3))
	if err != nil {
		t.Fatal(err)
	}
	top := testsvg.BlockTop(testsvg.Options{}, 2, 3)
	// The block has a 1.2-unit stroke: its ink box is 0.6 larger on each side.
	if !near(block.Y, top-0.6, 0.01) || !near(block.H, testsvg.BlockHeight+1.2, 0.01) {
		t.Errorf("Measure(block 2.3) = %+v, want top %v height %v", block, top-0.6, testsvg.BlockHeight+1.2)
	}

	if _, err := d.Measure("does-not-exist"); err == nil {
		t.Error("measuring an unknown id should fail")
	}
}

func pixel(t *testing.T, img interface{ At(x, y int) color.Color }, x, y int) color.RGBA {
	t.Helper()
	r, g, b, a := img.At(x, y).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func TestRenderWindow(t *testing.T) {
	d := load(t, testsvg.Generate(testsvg.Options{}))

	// A window inside the banner (#2b3a55), at 2 pixels per unit.
	img, err := d.Render(Box{X: 900, Y: 10, W: 20, H: 20}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 40 || b.Dy() != 40 {
		t.Errorf("image is %v, want 40 x 40", b)
	}
	if got, want := pixel(t, img, 20, 20), (color.RGBA{0x2b, 0x3a, 0x55, 0xff}); got != want {
		t.Errorf("banner pixel = %v, want %v", got, want)
	}

	// A window in the grey background, between two blocks.
	img, err = d.Render(Box{X: 1250, Y: 2000, W: 10, H: 10}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pixel(t, img, 5, 5), (color.RGBA{0xf4, 0xf5, 0xf8, 0xff}); got != want {
		t.Errorf("background pixel = %v, want %v", got, want)
	}

	// The window origin matters: the same pixel read from another window.
	img2, err := d.Render(Box{X: 900, Y: 10, W: 20, H: 20}, 2, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pixel(t, img2, 20, 20), (color.RGBA{0x2b, 0x3a, 0x55, 0xff}); got != want {
		t.Errorf("banner pixel over a white background = %v, want %v", got, want)
	}

	if _, err := d.Render(Box{W: 0, H: 10}, 1, nil); err == nil {
		t.Error("an empty window should fail")
	}
}

// Both versions must look like versions: on Windows, reading librsvg's version
// variables through cgo once gave 1257383423.1259480575.1258431999.
func TestVersionsAreReported(t *testing.T) {
	rsvg, cairo := Versions()
	version := regexp.MustCompile(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`)
	if !version.MatchString(rsvg) || !version.MatchString(cairo) {
		t.Errorf("Versions() = %q, %q", rsvg, cairo)
	}
	t.Logf("librsvg %s, cairo %s", rsvg, cairo)
}

func TestMeasureBlocks(t *testing.T) {
	d := load(t, testsvg.Generate(testsvg.Options{}))
	ids := []string{testsvg.BlockID(1, 1), testsvg.BlockID(2, 3), "does-not-exist", testsvg.BannerID}
	got := d.MeasureBlocks(ids)
	if len(got) != 3 {
		t.Fatalf("%d boxes, want 3: %v", len(got), got)
	}
	if _, ok := got["does-not-exist"]; ok {
		t.Error("an unknown id is measured")
	}
	top := testsvg.BlockTop(testsvg.Options{}, 2, 3)
	if b := got[testsvg.BlockID(2, 3)]; !near(b.Y, top-0.6, 0.01) {
		t.Errorf("block 2.3 = %+v", b)
	}
	var _ Document = d
}

// A linked image is looked for next to the document it is linked from.
func TestLinkedImagesAreResolvedFromThePathOfTheDocument(t *testing.T) {
	dir := t.TempDir()
	pic := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := 0; i < len(pic.Pix); i += 4 {
		pic.Pix[i], pic.Pix[i+3] = 255, 255 // opaque red
	}
	var png bytes.Buffer
	if err := stdpng.Encode(&png, pic); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pics"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pics", "red.png"), png.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="20" height="20" viewBox="0 0 20 20"><image x="0" y="0" width="20" height="20" xlink:href="pics/red.png"/></svg>`)

	with, err := Load(svg, filepath.Join(dir, "doc.svg"))
	if err != nil {
		t.Fatal(err)
	}
	defer with.Close()
	img, err := with.Render(Box{W: 20, H: 20}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := pixel(t, img, 10, 10); got.R < 200 || got.A < 250 {
		t.Errorf("the linked image is not drawn: pixel %v", got)
	}

	without := load(t, svg)
	img, err = without.Render(Box{W: 20, H: 20}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := pixel(t, img, 10, 10); got.A != 0 {
		t.Errorf("an image that cannot be found is drawn anyway: pixel %v", got)
	}
}

func TestMatchFont(t *testing.T) {
	generic := MatchFont("sans-serif")
	if generic == "" {
		t.Skip("fontconfig has no font to offer on this machine")
	}
	if got := MatchFont(generic); !strings.EqualFold(got, generic) {
		t.Errorf("an installed family is not found as itself: %q gives %q", generic, got)
	}
	if got := MatchFont("Famille Qui N'Existe Pas 4711"); got == "" || strings.EqualFold(got, "Famille Qui N'Existe Pas 4711") {
		t.Errorf("a missing family gives %q: it should give the one that replaces it", got)
	}
}

// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package engine

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"regexp"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/testimg"
)

func loadImage(t *testing.T, data []byte) *ImageDoc {
	t.Helper()
	d, err := LoadImage(data, "")
	if err != nil {
		t.Fatalf("LoadImage: %v", err)
	}
	t.Cleanup(d.Close)
	return d
}

func TestImageSizeIsItsPixelsAtTheResolutionOfTheFile(t *testing.T) {
	img := testimg.Quadrants(96, 48)
	for _, c := range []struct {
		name         string
		data         []byte
		wantW, wantH float64
	}{
		{"no resolution: 96 to the inch", testimg.PNG(img, 0, 0), 96, 48},
		{"192 to the inch", testimg.PNG(img, 192, 192), 48, 24},
		{"72 to the inch", testimg.PNG(img, 72, 72), 128, 64},
		{"across and down differ", testimg.PNG(img, 96, 192), 96, 24},
		{"a tiff of 300 to the inch", testimg.TIFF(img, 300, 300, 1), 30.72, 15.36},
	} {
		d := loadImage(t, c.data)
		if w, h := d.Size(); !near(w, c.wantW, 0.01) || !near(h, c.wantH, 0.01) {
			t.Errorf("%s: Size = %v x %v, want %v x %v", c.name, w, h, c.wantW, c.wantH)
		}
	}
}

func TestImageInfoTellsWhatTheFileWas(t *testing.T) {
	d := loadImage(t, testimg.PNG(testimg.Quadrants(30, 20), 150, 150))
	info := d.ImageInfo()
	if info.Format != FormatPNG || info.WidthPx != 30 || info.HeightPx != 20 || info.Orientation != 1 {
		t.Errorf("png: %+v", info)
	}
	if !near(info.XDPI, 150, 0.5) || !near(info.YDPI, 150, 0.5) || !info.HasResolution() {
		t.Errorf("png resolution: %v x %v", info.XDPI, info.YDPI)
	}
	if info.HasAlpha || info.MultiPage || info.Animated || info.EmbeddedJPEG {
		t.Errorf("png flags: %+v", info)
	}

	plain := loadImage(t, testimg.PNG(testimg.Quadrants(30, 20), 0, 0)).ImageInfo()
	if plain.HasResolution() {
		t.Errorf("a picture without resolution reports %v x %v", plain.XDPI, plain.YDPI)
	}

	// The TIFF decoder of gdk-pixbuf has an alpha channel for every picture,
	// and turns the pictures of some orientations itself: what counts is the
	// picture seen (TestOrientationTurnsThePictureAsItsTagSays).
	tif := loadImage(t, testimg.TIFF(testimg.Quadrants(30, 20), 300, 300, 1)).ImageInfo()
	if tif.Format != FormatTIFF || tif.HasAlpha || tif.Orientation != 1 {
		t.Errorf("tiff: %+v", tif)
	}
}

func TestAnImageIsOpenedByItsContentNotItsName(t *testing.T) {
	data := testimg.PNG(testimg.Quadrants(8, 8), 0, 0)
	for _, path := range []string{"", "photo.jpg", "photo.png", "no-extension"} {
		d, err := LoadDocument(data, path)
		if err != nil {
			t.Fatalf("LoadDocument(%q): %v", path, err)
		}
		if _, ok := d.(ImageDocument); !ok {
			t.Errorf("LoadDocument(%q) is not an image document", path)
		}
		d.(interface{ Close() }).Close()
	}
	svg, err := LoadDocument([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="5"/>`), "")
	if err != nil {
		t.Fatal(err)
	}
	defer svg.(interface{ Close() }).Close()
	if _, ok := svg.(ImageDocument); ok {
		t.Error("an SVG is not an image document")
	}
}

func TestOrientationTurnsThePictureAsItsTagSays(t *testing.T) {
	// The corners of the picture as it is seen: top left, top right, bottom
	// left, bottom right.
	const (
		tl = iota
		tr
		bl
		br
	)
	// For an upright 40 x 20 picture (red, green / blue, white), where each
	// colour is seen for each of the eight orientations of EXIF.
	cases := []struct {
		orientation int
		turned      bool // width and height swapped
		red, green  int  // the corner where red and green end up
		blue, white int
	}{
		{1, false, tl, tr, bl, br},
		{2, false, tr, tl, br, bl}, // mirrored left to right
		{3, false, br, bl, tr, tl}, // turned half a turn
		{4, false, bl, br, tl, tr}, // mirrored top to bottom
		{5, true, tl, bl, tr, br},  // transposed
		{6, true, tr, br, tl, bl},  // a quarter turn clockwise
		{7, true, br, tr, bl, tl},  // transposed on the other diagonal
		{8, true, bl, tl, br, tr},  // a quarter turn anti-clockwise
	}
	for _, c := range cases {
		d := loadImage(t, testimg.TIFF(testimg.Quadrants(40, 20), 96, 96, c.orientation))
		w, h := d.Size()
		wantW, wantH := 40.0, 20.0
		if c.turned {
			wantW, wantH = 20, 40
		}
		if w != wantW || h != wantH {
			t.Errorf("orientation %d: Size = %v x %v, want %v x %v", c.orientation, w, h, wantW, wantH)
			continue
		}
		img, err := d.Render(Box{W: w, H: h}, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		corner := func(i int) color.RGBA {
			x, y := 2, 2
			if i == tr || i == br {
				x = int(w) - 3
			}
			if i == bl || i == br {
				y = int(h) - 3
			}
			return img.RGBAAt(x, y)
		}
		for _, w := range []struct {
			name string
			at   int
			want color.RGBA
		}{{"red", c.red, testimg.Red}, {"green", c.green, testimg.Green}, {"blue", c.blue, testimg.Blue}, {"white", c.white, testimg.White}} {
			if got := corner(w.at); got != w.want {
				t.Errorf("orientation %d: the corner %d of %s shows %v, want %v", c.orientation, w.at, w.name, got, w.want)
			}
		}
	}
}

func TestTransparentPartsStayTransparent(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 10; x < 20; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	d := loadImage(t, testimg.PNG(img, 0, 0))
	if !d.ImageInfo().HasAlpha {
		t.Fatal("a PNG with an alpha channel must say so")
	}
	out, err := d.Render(Box{W: 20, H: 10}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a := out.RGBAAt(3, 5).A; a != 0 {
		t.Errorf("the transparent half has alpha %d on a transparent canvas", a)
	}
	if got := out.RGBAAt(15, 5); got != testimg.Red {
		t.Errorf("the opaque half shows %v", got)
	}
	onWhite, err := d.Render(Box{W: 20, H: 10}, 1, testimg.White)
	if err != nil {
		t.Fatal(err)
	}
	if got := onWhite.RGBAAt(3, 5); got != testimg.White {
		t.Errorf("the transparent half on white shows %v", got)
	}
}

func TestRenderWindowIsInDocumentUnits(t *testing.T) {
	d := loadImage(t, testimg.PNG(testimg.Quadrants(40, 20), 0, 0))
	// The right half of the picture, at twice the size.
	img, err := d.Render(Box{X: 20, Y: 0, W: 20, H: 20}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 40 || b.Dy() != 40 {
		t.Fatalf("size %v", b)
	}
	if got := img.RGBAAt(5, 5); got != testimg.Green {
		t.Errorf("top left of the right half shows %v, want green", got)
	}
	if got := img.RGBAAt(35, 35); got != testimg.White {
		t.Errorf("bottom right shows %v, want white", got)
	}
	if _, err := d.Render(Box{W: 0, H: 5}, 1, nil); err == nil {
		t.Error("an empty window must be refused")
	}
}

func TestAnImageHasNoElementsToMeasure(t *testing.T) {
	d := loadImage(t, testimg.PNG(testimg.Quadrants(8, 8), 0, 0))
	if _, err := d.Measure("anything"); err == nil {
		t.Error("Measure must fail")
	}
	if got := d.MeasureBlocks([]string{"a", "b"}); len(got) != 0 {
		t.Errorf("MeasureBlocks found %v", got)
	}
}

func TestUnreadableImagesAreReportedByKind(t *testing.T) {
	good := testimg.PNG(testimg.Quadrants(64, 64), 0, 0)
	var ie *ImageError
	for name, data := range map[string][]byte{
		"empty":   nil,
		"garbage": []byte("this is not a picture at all"),
		"corrupt": append(append([]byte{}, good[:40]...), bytes.Repeat([]byte{0x55}, 64)...),
	} {
		_, err := LoadImage(data, "")
		if !errors.As(err, &ie) || ie.Kind != ImageUnreadable {
			t.Errorf("%s: err = %v, want an unreadable image", name, err)
		}
	}
}

func TestAFileThatStopsEarlyIsOpenedAndSaidTruncated(t *testing.T) {
	// The decoders give the part of the picture they have and no error; the
	// program must be able to tell the user.
	for name, data := range map[string][]byte{
		"png":  testimg.PNG(testimg.Quadrants(300, 300), 0, 0),
		"jpeg": testimg.JPEG(testimg.Quadrants(300, 300)),
	} {
		whole := loadImage(t, data)
		if whole.ImageInfo().Truncated {
			t.Errorf("%s: a whole file is said truncated", name)
		}
		d, err := LoadImage(data[:len(data)*6/10], "")
		if err != nil {
			t.Errorf("%s: a truncated file is opened as far as it goes, got %v", name, err)
			continue
		}
		if !d.ImageInfo().Truncated {
			t.Errorf("%s: a file cut at 60%% is not said truncated", name)
		}
		d.Close()
	}
}

func TestAnImageTooLargeIsRefusedBeforeItIsDecoded(t *testing.T) {
	var ie *ImageError
	_, err := LoadImage(testimg.HugePNG(20000, 20000), "")
	if !errors.As(err, &ie) || ie.Kind != ImageTooLarge {
		t.Fatalf("err = %v, want an image too large", err)
	}
	if ie.W != 20000 || ie.H != 20000 {
		t.Errorf("the error says %d x %d", ie.W, ie.H)
	}
}

func TestSupportedImageFormatsAreTheOnesTheDecoderHas(t *testing.T) {
	got := map[Format]bool{}
	for _, f := range SupportedImageFormats() {
		got[f] = true
		if !f.IsImage() {
			t.Errorf("%q is listed as supported but is not an image format", f)
		}
	}
	for _, f := range []Format{FormatJPEG, FormatPNG, FormatTIFF, FormatGIF, FormatBMP} {
		if !got[f] {
			t.Errorf("%q comes with gdk-pixbuf and must be supported", f)
		}
	}
}

func TestResolutionIgnoresValuesNoDeviceWrites(t *testing.T) {
	for _, c := range []struct {
		x, y         string
		wantX, wantY float64
	}{
		{"300", "300", 300, 300},
		{"300", "", 300, 300},
		{"", "150", 150, 150},
		{"", "", 0, 0},
		{"0", "0", 0, 0},
		{"1", "1", 0, 0},          // below any scanner
		{"1000000", "96", 96, 96}, // above any scanner: only the good one is kept
		{"nan", "inf", 0, 0},
		{" 72 ", "abc", 72, 72},
	} {
		if x, y := resolution(c.x, c.y); x != c.wantX || y != c.wantY {
			t.Errorf("resolution(%q, %q) = %v, %v; want %v, %v", c.x, c.y, x, y, c.wantX, c.wantY)
		}
	}
}

func TestJPEGPhotographsKeepTheirOrientationAndResolution(t *testing.T) {
	data := testimg.WithEXIFOrientation(testimg.WithJFIFDensity(testimg.JPEG(testimg.Quadrants(64, 32)), 200, 200), 6)
	d := loadImage(t, data)
	info := d.ImageInfo()
	if info.Format != FormatJPEG || info.Orientation != 6 {
		t.Errorf("info %+v", info)
	}
	if !near(info.XDPI, 200, 0.5) || !near(info.YDPI, 200, 0.5) {
		t.Errorf("resolution %v x %v, want 200 x 200", info.XDPI, info.YDPI)
	}
	// 64 x 32 pixels at 200 dpi, a quarter turn: 15.36 wide, 30.72 high.
	if w, h := d.Size(); !near(w, 15.36, 0.01) || !near(h, 30.72, 0.01) {
		t.Errorf("Size = %v x %v", w, h)
	}
	if !info.EmbeddedJPEG {
		t.Error("a plain JPEG is carried into the PDF as it is, turned by the matrix of the page")
	}
}

// pdfOf exports the document on the given pages.
func pdfOf(t *testing.T, d Document, pages []PageSpec, opt PDFOptions) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := d.ExportPDF(&buf, pages, opt); err != nil {
		t.Fatalf("ExportPDF: %v", err)
	}
	return buf.Bytes()
}

// threeBands cuts a document w x h units wide into three bands at the given
// heights, on A4 sheets, at 0.5 pt a unit, 10 mm from the top and 10 mm from
// the left.
func threeBands(h, c1, c2 float64) []PageSpec {
	var pages []PageSpec
	for _, band := range [][2]float64{{0, c1}, {c1, c2}, {c2, h}} {
		pages = append(pages, PageSpec{
			WidthPt: 595.2756, HeightPt: 841.8898,
			BandTop: band[0], BandBottom: band[1],
			OriginXPt: 28.3465, OriginYPt: 28.3465,
			PtPerUnit: 0.5,
		})
	}
	return pages
}

func TestAnImageIsWrittenOnceAndShownOnEveryPage(t *testing.T) {
	jpeg := testimg.WithJFIFDensity(testimg.JPEG(testimg.Quadrants(120, 90)), 96, 96)
	for name, data := range map[string][]byte{
		"jpeg": jpeg,
		"png":  testimg.PNG(testimg.Quadrants(120, 90), 0, 0),
		"tiff": testimg.TIFF(testimg.Quadrants(120, 90), 96, 96, 1),
	} {
		d := loadImage(t, data)
		w, h := d.Size()
		out := pdfOf(t, d, threeBands(h, 30, 60), PDFOptions{WhitePage: true, FixedDate: DefaultFixedDate})
		if n := len(regexp.MustCompile(`/Subtype /Image\b`).FindAll(out, -1)); n != 1 {
			t.Errorf("%s: the PDF holds %d image objects for three pages, want 1", name, n)
		}
		if n := len(contentStreams(t, out)); n != 3 {
			t.Errorf("%s: %d pages", name, n)
		}
		isJPEG := bytes.Contains(out, []byte("/DCTDecode"))
		if isJPEG != (name == "jpeg") {
			t.Errorf("%s: DCTDecode present = %v", name, isJPEG)
		}
		if name == "jpeg" && !bytes.Contains(out, jpeg[len(jpeg)/2:len(jpeg)/2+64]) {
			t.Error("jpeg: the file is not in the PDF as it was")
		}
		if w != 120 || h != 90 {
			t.Errorf("%s: Size %v x %v", name, w, h)
		}
	}
}

func TestEachSheetShowsItsBandOfTheImage(t *testing.T) {
	// Red above, blue below, cut in the middle: the first sheet shows red only
	// and the second blue only.
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if y < 50 {
				img.SetRGBA(x, y, testimg.Red)
			} else {
				img.SetRGBA(x, y, testimg.Blue)
			}
		}
	}
	d := loadImage(t, testimg.PNG(img, 0, 0))
	pages := []PageSpec{
		{WidthPt: 200, HeightPt: 200, BandTop: 0, BandBottom: 50, OriginXPt: 20, OriginYPt: 10, PtPerUnit: 1},
		{WidthPt: 200, HeightPt: 200, BandTop: 50, BandBottom: 100, OriginXPt: 20, OriginYPt: 10, PtPerUnit: 1},
	}
	for i, want := range []color.RGBA{testimg.Red, testimg.Blue} {
		sheet, err := d.RenderPage(pages[i], 1, true)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range []struct{ x, y int }{{21, 11}, {119, 11}, {21, 59}, {119, 59}, {70, 35}} {
			if got := sheet.RGBAAt(p.x, p.y); got != want {
				t.Errorf("sheet %d at %v shows %v, want %v", i+1, p, got, want)
			}
		}
		// Outside the band: the white of the page, whatever the picture has there.
		for _, p := range []struct{ x, y int }{{5, 100}, {150, 100}, {70, 5}, {70, 65}, {70, 150}} {
			if got := sheet.RGBAAt(p.x, p.y); got != testimg.White {
				t.Errorf("sheet %d at %v shows %v outside the band, want white", i+1, p, got)
			}
		}
	}
}

func TestImageExportIsDeterministic(t *testing.T) {
	for name, data := range map[string][]byte{
		"jpeg": testimg.JPEG(testimg.Quadrants(64, 48)),
		"png":  testimg.PNG(testimg.Quadrants(64, 48), 0, 0),
	} {
		d := loadImage(t, data)
		_, h := d.Size()
		pages := threeBands(h, 10, 30)
		first := pdfOf(t, d, pages, PDFOptions{WhitePage: true, FixedDate: DefaultFixedDate})
		second := pdfOf(t, d, pages, PDFOptions{WhitePage: true, FixedDate: DefaultFixedDate})
		again := loadImage(t, data)
		third := pdfOf(t, again, pages, PDFOptions{WhitePage: true, FixedDate: DefaultFixedDate})
		if !bytes.Equal(first, second) || !bytes.Equal(first, third) {
			t.Errorf("%s: two exports of the same picture differ", name)
		}
	}
}

func TestImageExportCanBeStopped(t *testing.T) {
	d := loadImage(t, testimg.PNG(testimg.Quadrants(64, 48), 0, 0))
	_, h := d.Size()
	calls := 0
	err := d.ExportPDF(&bytes.Buffer{}, threeBands(h, 10, 30), PDFOptions{Progress: func(done, total int) bool {
		calls++
		return done < 1
	}})
	if !errors.Is(err, ErrCancelled) || calls != 1 {
		t.Errorf("err = %v after %d calls", err, calls)
	}
}

func TestAnOrientedImageIsTurnedInThePDFToo(t *testing.T) {
	need(t, "pdftoppm")
	// 40 x 20 pixels at 96 dpi and orientation 6: seen as 20 x 40 units, red
	// top right, green bottom right, white bottom left, blue top left.
	for name, data := range map[string][]byte{
		"tiff": testimg.TIFF(testimg.Quadrants(40, 20), 96, 96, 6),
		"jpeg": testimg.WithEXIFOrientation(testimg.WithJFIFDensity(testimg.JPEG(testimg.Quadrants(40, 20)), 96, 96), 6),
	} {
		d := loadImage(t, data)
		w, h := d.Size()
		if w != 20 || h != 40 {
			t.Fatalf("%s: Size %v x %v", name, w, h)
		}
		page := PageSpec{WidthPt: 20 * 0.75, HeightPt: 40 * 0.75, BandTop: 0, BandBottom: h, PtPerUnit: 0.75}
		pdf := writeTemp(t, name+".pdf", pdfOf(t, d, []PageSpec{page}, PDFOptions{WhitePage: true, FixedDate: DefaultFixedDate}))
		files := rasterise(t, pdf, 288) // 4 pixels to the unit
		if len(files) != 1 {
			t.Fatalf("%s: %d pages rasterised", name, len(files))
		}
		l := readLuma(t, files[0])
		// Grey levels of the colours: red 76, green 150, blue 29, white 255.
		for _, c := range []struct {
			corner string
			x, y   int
			want   int
		}{
			{"top left", 8, 8, 29},
			{"top right", l.w - 9, 8, 76},
			{"bottom left", 8, l.h - 9, 255},
			{"bottom right", l.w - 9, l.h - 9, 150},
		} {
			if got := l.at(c.x, c.y); abs(got-c.want) > 12 {
				t.Errorf("%s: %s has grey level %d, want about %d", name, c.corner, got, c.want)
			}
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

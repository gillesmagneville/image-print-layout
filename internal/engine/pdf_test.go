// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package engine

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gillesmagneville/pageslice/internal/testsvg"
	"github.com/gillesmagneville/pageslice/internal/units"
)

// The acceptance scenario of the specification: A4 portrait, margins
// 10/10/5/5 mm, content 200 mm wide, three cuts, every page 10 mm from the
// top. The plan is computed here with the simplest possible code: the real
// layout package is milestone M1.
const (
	targetWidthMM = 200.0
	marginTopMM   = 10.0
	marginLeftMM  = 5.0
)

var scenarioCuts = []float64{1380.5, 2760, 4120.25}

// scenarioPlan returns one PageSpec per band, and the scale in mm per unit.
func scenarioPlan(t *testing.T, docW, docH float64, cuts []float64) ([]PageSpec, float64) {
	t.Helper()
	a4, _ := units.Lookup("A4")
	sheetW, sheetH := a4.Oriented(false)
	s, err := units.MMPerUnit(targetWidthMM, docW)
	if err != nil {
		t.Fatal(err)
	}
	tops := append([]float64{0}, cuts...)
	bottoms := append(append([]float64{}, cuts...), docH)
	var pages []PageSpec
	for i := range tops {
		pages = append(pages, PageSpec{
			WidthPt: units.MMToPt(sheetW), HeightPt: units.MMToPt(sheetH),
			BandTop: tops[i], BandBottom: bottoms[i],
			OriginXPt: units.MMToPt(marginLeftMM), OriginYPt: units.MMToPt(marginTopMM),
			PtPerUnit: units.MMToPt(s),
		})
	}
	return pages, s
}

// referencePlan is the whole document on one tall sheet of exactly its size,
// at the same scale and without margin: the picture the cuts must reproduce.
func referencePlan(docW, docH, s float64) []PageSpec {
	return []PageSpec{{
		WidthPt: units.MMToPt(docW * s), HeightPt: units.MMToPt(docH * s),
		BandTop: 0, BandBottom: docH, PtPerUnit: units.MMToPt(s),
	}}
}

// document is a test input: the synthetic one always, the real Mini-CAD when
// SVGPL_MINICAD points to it (it is not in the public repository).
type document struct {
	name string
	svg  []byte
}

func documents(t *testing.T) []document {
	docs := []document{{"synthetic", testsvg.Generate(testsvg.Options{})}}
	if p := os.Getenv("SVGPL_MINICAD"); p != "" {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("SVGPL_MINICAD: %v", err)
		}
		docs = append(docs, document{"mini-cad", data})
	}
	return docs
}

// need skips the test when a poppler tool is missing, except on CI where a
// missing tool must not turn a check into a silent skip.
func need(t *testing.T, tool string) {
	t.Helper()
	if _, err := exec.LookPath(tool); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s is required on CI", tool)
		}
		t.Skipf("%s not installed", tool)
	}
}

func export(t *testing.T, d *Doc, pages []PageSpec, opt PDFOptions) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := d.ExportPDF(&buf, pages, opt); err != nil {
		t.Fatalf("ExportPDF: %v", err)
	}
	return buf.Bytes()
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func run(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

var pageSize = regexp.MustCompile(`Page\s+(\d+) size:\s+([\d.]+) x ([\d.]+) pts`)

func TestExportedPagesHaveTheSheetSize(t *testing.T) {
	need(t, "pdfinfo")
	for _, doc := range documents(t) {
		t.Run(doc.name, func(t *testing.T) {
			d := load(t, doc.svg)
			w, h := d.Size()
			pages, _ := scenarioPlan(t, w, h, scenarioCuts)
			pdf := writeTemp(t, "out.pdf", export(t, d, pages, PDFOptions{WhitePage: true}))

			info := run(t, "pdfinfo", "-f", "1", "-l", "99", pdf)
			if !strings.Contains(info, fmt.Sprintf("Pages:           %d", len(pages))) {
				t.Errorf("expected %d pages, pdfinfo says:\n%s", len(pages), info)
			}
			got := pageSize.FindAllStringSubmatch(info, -1)
			if len(got) != len(pages) {
				t.Fatalf("found sizes for %d pages, want %d", len(got), len(pages))
			}
			// pdfinfo prints three decimals; 0.02 mm is 0.057 pt.
			tol := units.MMToPt(0.02)
			for i, m := range got {
				pw, _ := strconv.ParseFloat(m[2], 64)
				ph, _ := strconv.ParseFloat(m[3], 64)
				if math.Abs(pw-595.276) > tol || math.Abs(ph-841.89) > tol {
					t.Errorf("page %d is %v x %v pt, want 595.276 x 841.89 (A4)", i+1, pw, ph)
				}
			}
		})
	}
}

func TestExportIsDeterministic(t *testing.T) {
	for _, doc := range documents(t) {
		t.Run(doc.name, func(t *testing.T) {
			d := load(t, doc.svg)
			w, h := d.Size()
			pages, _ := scenarioPlan(t, w, h, scenarioCuts)
			first := export(t, d, pages, PDFOptions{WhitePage: true})
			time.Sleep(1100 * time.Millisecond) // a PDF date has a one-second resolution
			second := export(t, d, pages, PDFOptions{WhitePage: true})
			// A second document loaded from the same bytes, as after a restart.
			d2 := load(t, doc.svg)
			third := export(t, d2, pages, PDFOptions{WhitePage: true})

			sum := func(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b))[:16] }
			if !bytes.Equal(first, second) || !bytes.Equal(first, third) {
				t.Errorf("exports differ: %s %s %s", sum(first), sum(second), sum(third))
			}
			t.Logf("%d bytes, sha256 %s", len(first), sum(first))
		})
	}
}

func TestTextStaysTextAndFontsAreEmbedded(t *testing.T) {
	need(t, "pdffonts")
	need(t, "pdftotext")
	d := load(t, testsvg.Generate(testsvg.Options{}))
	w, h := d.Size()
	pages, _ := scenarioPlan(t, w, h, scenarioCuts)
	pdf := writeTemp(t, "out.pdf", export(t, d, pages, PDFOptions{WhitePage: true}))

	text := run(t, "pdftotext", "-layout", pdf, "-")
	for _, want := range []string{"Flux réseau — document de test", "équipement é à ç ü", "VLAN"} {
		if !strings.Contains(text, want) {
			t.Errorf("selectable text lacks %q", want)
		}
	}

	fonts := run(t, "pdffonts", pdf)
	t.Logf("fonts:\n%s", fonts)
	lines := strings.Split(strings.TrimSpace(fonts), "\n")
	if len(lines) < 3 {
		t.Fatalf("no font listed:\n%s", fonts)
	}
	for _, l := range lines[2:] {
		f := strings.Fields(l)
		// name type encoding emb sub uni object-ID: "emb" is the 4th column from the end - 3.
		if len(f) < 7 || f[len(f)-5] != "yes" {
			t.Errorf("font not embedded: %q", l)
		}
	}
}

// The document is rendered on every page: measure whether the PDF
// then holds the drawing once or once per page.
func TestFileSizeDoesNotGrowWithThePageCount(t *testing.T) {
	for _, doc := range documents(t) {
		t.Run(doc.name, func(t *testing.T) {
			d := load(t, doc.svg)
			w, h := d.Size()
			pages, s := scenarioPlan(t, w, h, scenarioCuts)
			oneTall := len(export(t, d, referencePlan(w, h, s), PDFOptions{WhitePage: true}))
			four := len(export(t, d, pages, PDFOptions{WhitePage: true}))
			single := len(export(t, d, pages[:1], PDFOptions{WhitePage: true}))
			t.Logf("one tall page: %d bytes; one band of %d: %d bytes; %d pages: %d bytes (x%.2f of one tall page)",
				oneTall, len(pages), single, len(pages), four, float64(four)/float64(oneTall))
			if float64(four) > 1.5*float64(oneTall) {
				t.Errorf("the %d-page PDF is %.2f times the size of the whole drawing once: the drawing is repeated on every page",
					len(pages), float64(four)/float64(oneTall))
			}
		})
	}
}

// ----- the seam tests -------------------------------------------------------------------------
//
// "The cuts match the preview exactly" is checked twice, because no single
// method is both exact and robust:
//
//   - the geometry written in the PDF itself: the clip rectangle and the offset
//     of the drawing on every page, read from the content stream and compared to
//     the plan, exact to a few thousandths of a point. This is the exact check,
//     and it holds for any cut position;
//   - the pixels: the pages rasterised and compared with the uncut drawing, to
//     catch content lost or repeated whatever the cause. This one is coarse by
//     necessity: a rasteriser does not draw the same picture twice to the pixel.
//     Poppler places a glyph on a quarter-pixel grid and snaps thin strokes to
//     the pixel grid, so a rounding error of a hundredth of a pixel in the
//     origin of a page moves a hairline by a whole pixel or changes the grey
//     level of an edge by a fifth. Comparing pixel by pixel therefore reports
//     differences that are not in the PDF. Comparing the mean grey level of
//     16 x 16 squares does not see them (a line one pixel off moves a square by
//     less than 14 levels, measured on the real Mini-CAD) and still sees a text
//     line, a block or a stretch of drawing that is missing, repeated or moved.

func readLuma(t *testing.T, path string) luma {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	pix := make([]uint8, w*h)
	if nrgba, ok := img.(*image.NRGBA); ok {
		for y := 0; y < h; y++ {
			row := nrgba.Pix[y*nrgba.Stride:]
			for x := 0; x < w; x++ {
				r, g, bl := int(row[x*4]), int(row[x*4+1]), int(row[x*4+2])
				pix[y*w+x] = uint8((299*r + 587*g + 114*bl) / 1000)
			}
		}
		return luma{pix, w, h}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			pix[y*w+x] = uint8((299*(r>>8) + 587*(g>>8) + 114*(bl>>8)) / 1000)
		}
	}
	return luma{pix, w, h}
}

// rasterise renders every page of a PDF with pdftoppm and returns the PNG
// files in page order. Extra arguments go to pdftoppm.
func rasterise(t *testing.T, pdf string, dpi int, extra ...string) []string {
	t.Helper()
	prefix := filepath.Join(filepath.Dir(pdf), strings.TrimSuffix(filepath.Base(pdf), ".pdf"))
	args := append([]string{"-r", strconv.Itoa(dpi), "-png"}, extra...)
	run(t, "pdftoppm", append(args, pdf, prefix)...)
	files, _ := filepath.Glob(prefix + "-*.png")
	sort.Strings(files)
	return files
}

// luma is a grey raster.
type luma struct {
	pix  []uint8
	w, h int
}

func (l luma) at(x, y int) int { return int(l.pix[y*l.w+x]) }

func (l luma) crop(x, y, w, h int) luma {
	out := luma{pix: make([]uint8, w*h), w: w, h: h}
	for j := 0; j < h; j++ {
		copy(out.pix[j*w:(j+1)*w], l.pix[(y+j)*l.w+x:(y+j)*l.w+x+w])
	}
	return out
}

// blocks replaces every n x n square by its mean grey level.
func (l luma) blocks(n int) luma {
	w, h := l.w/n, l.h/n
	out := luma{pix: make([]uint8, w*h), w: w, h: h}
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			sum := 0
			for dy := 0; dy < n; dy++ {
				for dx := 0; dx < n; dx++ {
					sum += l.at(i*n+dx, j*n+dy)
				}
			}
			out.pix[j*w+i] = uint8(sum / (n * n))
		}
	}
	return out
}

// differs counts the pixels of a that differ from the same pixel of b (same
// size) by more than tol grey levels. It also returns the worst share of
// differing pixels over one row.
func differs(a, b luma, tol int) (bad int, worstRow float64) {
	if a.w != b.w || a.h != b.h {
		panic("differs: rasters of different sizes")
	}
	for y := 0; y < a.h; y++ {
		rowBad := 0
		for x := 0; x < a.w; x++ {
			if d := a.at(x, y) - b.at(x, y); d > tol || d < -tol {
				bad++
				rowBad++
			}
		}
		if f := float64(rowBad) / float64(a.w); f > worstRow {
			worstRow = f
		}
	}
	return bad, worstRow
}

// seam sets how the pages are compared with the uncut drawing.
type seam struct {
	dpi   int // resolution of both rasterisations
	block int // the squares of block x block pixels whose mean grey level is compared
	tol   int // tolerated difference of the means, in grey levels (of 255)
}

// compareToUncut rasterises the exported pages and the whole drawing on one
// tall sheet at the same scale, then compares, in both directions, the content
// window of each page (taken from expected) with the same part of the tall
// sheet. Both go through the same rasteriser, so what differs is the cutting
// alone: content lost, repeated or moved at a seam. It returns the share of
// differing pixels and the worst share over one row.
func compareToUncut(t *testing.T, d *Doc, exported, expected []PageSpec, s float64, o seam) (frac, worstRow float64) {
	t.Helper()
	pxPerPt := float64(o.dpi) / 72
	w, h := d.Size()
	cutPDF := writeTemp(t, "cut.pdf", export(t, d, exported, PDFOptions{WhitePage: true}))
	refPDF := writeTemp(t, "ref.pdf", export(t, d, referencePlan(w, h, s), PDFOptions{WhitePage: true}))
	cutPNG, refPNG := rasterise(t, cutPDF, o.dpi), rasterise(t, refPDF, o.dpi)
	if len(cutPNG) != len(exported) || len(refPNG) != 1 {
		t.Fatalf("got %d cut pages and %d reference pages", len(cutPNG), len(refPNG))
	}
	ref := readLuma(t, refPNG[0])

	var total, bad int
	for i, p := range expected {
		pg := readLuma(t, cutPNG[i])
		x0 := int(math.Round(p.OriginXPt * pxPerPt))
		y0 := int(math.Round(p.OriginYPt * pxPerPt))
		cw := int(math.Round(w * p.PtPerUnit * pxPerPt))
		ch := int(math.Round((p.BandBottom - p.BandTop) * p.PtPerUnit * pxPerPt))
		refTop := int(math.Round(p.BandTop * p.PtPerUnit * pxPerPt))
		if x0+cw > pg.w || y0+ch > pg.h || refTop+ch > ref.h || cw > ref.w {
			t.Fatalf("page %d: window %dx%d at (%d,%d) outside the %dx%d raster", i+1, cw, ch, x0, y0, pg.w, pg.h)
		}
		a, b := pg.crop(x0, y0, cw, ch).blocks(o.block), ref.crop(0, refTop, cw, ch).blocks(o.block)
		n, r := differs(a, b, o.tol)
		bad += n
		total += a.w * a.h
		worstRow = math.Max(worstRow, r)
	}
	return float64(bad) / float64(total), worstRow
}

func seamTest(t *testing.T, d *Doc, cuts []float64, o seam) (frac, worstRow float64) {
	t.Helper()
	w, h := d.Size()
	pages, s := scenarioPlan(t, w, h, cuts)
	return compareToUncut(t, d, pages, pages, s, o)
}

// Cuts and sheet positions on the pixel grid (at 254 dpi one millimetre is ten
// pixels, and a multiple of 16 units is a whole number of pixels at 200 mm for
// 1280 units): the pages must reproduce the uncut drawing.
func TestSeamPixelsOnThePixelGrid(t *testing.T) {
	need(t, "pdftoppm")
	for _, doc := range documents(t) {
		t.Run(doc.name, func(t *testing.T) {
			d := load(t, doc.svg)
			if w, _ := d.Size(); w != 1280 {
				t.Skipf("the grid cuts assume a 1280-unit width, got %v", w)
			}
			frac, worst := seamTest(t, d, []float64{1376, 2752, 4128}, seam{dpi: 254, block: 16, tol: 24})
			t.Logf("pixel grid, 254 dpi: %.5f%% of the squares differ, worst row %.2f%%", 100*frac, 100*worst)
			if frac > 0.00002 || worst > 0.02 {
				t.Errorf("the pages do not reproduce the uncut drawing: %.5f%% of the squares differ, worst row %.2f%%", 100*frac, 100*worst)
			}
		})
	}
}

// The cuts of the acceptance scenario fall between pixels.
func TestSeamPixelsAtTheAcceptanceCuts(t *testing.T) {
	need(t, "pdftoppm")
	for _, doc := range documents(t) {
		t.Run(doc.name, func(t *testing.T) {
			d := load(t, doc.svg)
			frac, worst := seamTest(t, d, scenarioCuts, seam{dpi: 150, block: 16, tol: 24})
			t.Logf("acceptance cuts, 150 dpi: %.5f%% of the squares differ, worst row %.2f%%", 100*frac, 100*worst)
			if frac > 0.00002 || worst > 0.02 {
				t.Errorf("%.5f%% of the squares differ from the uncut drawing, worst row %.2f%%", 100*frac, 100*worst)
			}
		})
	}
}

// The pixel comparison must be able to fail: a page whose band is the wrong
// one is what it exists to catch. It is a coarse check (see above): it must see
// a shift of a few millimetres, and the geometry check below sees the rest.
func TestSeamPixelsDetectMissingContent(t *testing.T) {
	need(t, "pdftoppm")
	d := load(t, testsvg.Generate(testsvg.Options{}))
	w, h := d.Size()
	expected, s := scenarioPlan(t, w, h, scenarioCuts)
	for _, shift := range []float64{60, 12} { // 26 mm, and 5 mm at 0.156 mm per unit
		shifted := append([]PageSpec(nil), expected...)
		shifted[2].BandTop += shift // page 3 starts too low, as a lost or repeated stretch would
		shifted[2].BandBottom += shift
		frac, _ := compareToUncut(t, d, shifted, expected, s, seam{dpi: 150, block: 16, tol: 24})
		t.Logf("page 3 shifted by %v units: %.4f%% of the squares differ", shift, 100*frac)
		if frac < 0.001 {
			t.Errorf("a page shifted by %v units differs in only %.5f%% of its squares: the comparison is too lenient", shift, 100*frac)
		}
	}
}

// ----- the geometry written in the PDF ---------------------------------------------------------

// contentStreams returns the decoded streams of a PDF written by cairo that
// look like a page content (they clip to the content window), in file order,
// which is page order.
func contentStreams(t *testing.T, pdf []byte) []string {
	t.Helper()
	var out []string
	for rest := pdf; ; {
		i := bytes.Index(rest, []byte(">>\nstream\n"))
		if i < 0 {
			break
		}
		rest = rest[i+len(">>\nstream\n"):]
		end := bytes.Index(rest, []byte("\nendstream"))
		if end < 0 {
			break
		}
		zr, err := zlib.NewReader(bytes.NewReader(rest[:end]))
		rest = rest[end:]
		if err != nil {
			continue
		}
		data, err := io.ReadAll(zr)
		if err != nil || !bytes.Contains(data, []byte(" re W n")) {
			continue
		}
		out = append(out, string(data))
	}
	return out
}

const nUnit = `(-?\d+(?:\.\d+)?)`

var (
	clipRe = regexp.MustCompile(nUnit + ` ` + nUnit + ` ` + nUnit + ` ` + nUnit + ` re W n`)
	fillRe = regexp.MustCompile(nUnit + ` ` + nUnit + ` ` + nUnit + ` ` + nUnit + ` re f`)
)

func floats(m []string) []float64 {
	v := make([]float64, len(m))
	for i, s := range m {
		v[i], _ = strconv.ParseFloat(s, 64)
	}
	return v
}

// pageGeometry compares what the PDF says with the plan, page by page: the
// clip rectangle (the content window) and, when a rectangle covering the whole
// document is painted (the background), where it is drawn, which gives the
// offset of the drawing on the page. Cairo keeps clip and path coordinates in
// 24.8 fixed point, so they are exact to 1/256 pt (0.0014 mm); the tolerance
// is that plus the three decimals written. It returns the problems found and the number of pages whose offset could be
// checked.
func pageGeometry(streams []string, pages []PageSpec, docW, docH float64) (problems []string, offsetChecked int) {
	const tol = 1.0/256 + 0.001 // pt
	if len(streams) != len(pages) {
		return []string{fmt.Sprintf("the PDF has %d page contents, the plan %d pages", len(streams), len(pages))}, 0
	}
	near := func(got, want float64) bool { return math.Abs(got-want) <= tol }
	for i, p := range pages {
		st := streams[i]
		loc := clipRe.FindStringSubmatchIndex(st)
		if loc == nil {
			problems = append(problems, fmt.Sprintf("page %d: no clip rectangle", i+1))
			continue
		}
		c := floats(clipRe.FindStringSubmatch(st[loc[0]:])[1:])
		wantW, wantH := docW*p.PtPerUnit, (p.BandBottom-p.BandTop)*p.PtPerUnit
		if !near(c[0], p.OriginXPt) || !near(c[1], p.OriginYPt) || !near(c[2], wantW) || !near(c[3], wantH) {
			problems = append(problems, fmt.Sprintf("page %d: clip is (%.3f, %.3f, %.3f x %.3f), the plan says (%.3f, %.3f, %.3f x %.3f)",
				i+1, c[0], c[1], c[2], c[3], p.OriginXPt, p.OriginYPt, wantW, wantH))
		}
		for _, m := range fillRe.FindAllStringSubmatch(st[loc[1]:], -1) {
			f := floats(m[1:])
			if math.Abs(f[2]-docW*p.PtPerUnit) > 0.01 || math.Abs(f[3]-docH*p.PtPerUnit) > 0.01 {
				continue
			}
			wantX, wantY := p.OriginXPt, p.OriginYPt-p.BandTop*p.PtPerUnit
			if !near(f[0], wantX) || !near(f[1], wantY) {
				problems = append(problems, fmt.Sprintf("page %d: the whole-document rectangle is drawn at (%.3f, %.3f), the plan says (%.3f, %.3f)",
					i+1, f[0], f[1], wantX, wantY))
			}
			offsetChecked++
			break
		}
	}
	return problems, offsetChecked
}

func TestGeometryInThePDFMatchesThePlan(t *testing.T) {
	for _, doc := range documents(t) {
		t.Run(doc.name, func(t *testing.T) {
			d := load(t, doc.svg)
			w, h := d.Size()
			for _, cuts := range [][]float64{scenarioCuts, {1376, 2752, 4128}, {1000, 2000, 3000, 4000, 5000}} {
				pages, _ := scenarioPlan(t, w, h, cuts)
				problems, checked := pageGeometry(contentStreams(t, export(t, d, pages, PDFOptions{WhitePage: true})), pages, w, h)
				for _, p := range problems {
					t.Errorf("cuts %v: %s", cuts, p)
				}
				if checked != len(pages) {
					t.Logf("cuts %v: the offset of the drawing could be checked on %d of %d pages (no rectangle covers the whole document on the others)", cuts, checked, len(pages))
				}
				if doc.name == "synthetic" && checked != len(pages) {
					t.Errorf("cuts %v: the synthetic document has a background on every page, only %d of %d were found", cuts, checked, len(pages))
				}
			}
		})
	}
}

// A tenth of a unit is 0.044 pt at this scale, ten times the tolerance: the
// geometry check sees what no picture could.
func TestGeometryCheckDetectsASlightShift(t *testing.T) {
	d := load(t, testsvg.Generate(testsvg.Options{}))
	w, h := d.Size()
	expected, _ := scenarioPlan(t, w, h, scenarioCuts)
	shifted := append([]PageSpec(nil), expected...)
	shifted[1].BandTop += 0.1 // the content moves by 0.1 unit and the window grows by as much
	problems, _ := pageGeometry(contentStreams(t, export(t, d, shifted, PDFOptions{WhitePage: true})), expected, w, h)
	if len(problems) < 2 {
		t.Errorf("a band moved by 0.1 unit produced %d problems %v, want the clip and the offset", len(problems), problems)
	}
}

// ----- the preview of a sheet and the progress of an export ------------------------------------

func TestRenderPageIsTheSheetDrawnLikeThePDF(t *testing.T) {
	d := load(t, testsvg.Generate(testsvg.Options{}))
	w, h := d.Size()
	pages, _ := scenarioPlan(t, w, h, scenarioCuts)
	p := pages[1]
	const pxPerPt = 0.5

	white, err := d.RenderPage(p, pxPerPt, true)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := white.Bounds().Dx(), int(math.Ceil(p.WidthPt*pxPerPt)); got != want {
		t.Errorf("width %d px, want %d", got, want)
	}
	if got, want := white.Bounds().Dy(), int(math.Ceil(p.HeightPt*pxPerPt)); got != want {
		t.Errorf("height %d px, want %d", got, want)
	}
	// The corner is in the margin: opaque white on a white page.
	if c := pixel(t, white, 1, 1); c != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("corner of a white page = %v, want opaque white", c)
	}
	clear, err := d.RenderPage(p, pxPerPt, false)
	if err != nil {
		t.Fatal(err)
	}
	if c := pixel(t, clear, 1, 1); c.A != 0 {
		t.Errorf("corner of a transparent page = %v, want alpha 0", c)
	}
	// Something is drawn inside the content window.
	ink := 0
	for y := 0; y < white.Bounds().Dy(); y += 3 {
		for x := 0; x < white.Bounds().Dx(); x += 3 {
			if c := pixel(t, white, x, y); c != (color.RGBA{255, 255, 255, 255}) {
				ink++
			}
		}
	}
	if ink == 0 {
		t.Error("nothing is drawn on the sheet")
	}
	// A sheet without a band is refused, not drawn.
	if _, err := d.RenderPage(PageSpec{WidthPt: 100, HeightPt: 100, PtPerUnit: 1}, 1, true); err == nil {
		t.Error("an empty band was drawn")
	}
}

func TestExportReportsItsProgressAndCanBeStopped(t *testing.T) {
	d := load(t, testsvg.Generate(testsvg.Options{}))
	w, h := d.Size()
	pages, _ := scenarioPlan(t, w, h, scenarioCuts)

	var seen [][2]int
	var out bytes.Buffer
	err := d.ExportPDF(&out, pages, PDFOptions{Progress: func(done, total int) bool {
		seen = append(seen, [2]int{done, total})
		return true
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != len(pages) || seen[0] != [2]int{1, len(pages)} || seen[len(seen)-1] != [2]int{len(pages), len(pages)} {
		t.Errorf("progress = %v, want 1..%d of %d", seen, len(pages), len(pages))
	}

	calls := 0
	err = d.ExportPDF(&bytes.Buffer{}, pages, PDFOptions{Progress: func(done, total int) bool {
		calls++
		return done < 2
	}})
	if !errors.Is(err, ErrCancelled) {
		t.Errorf("a stopped export returned %v, want ErrCancelled", err)
	}
	if calls != 2 {
		t.Errorf("the export went on after being stopped: %d calls", calls)
	}
}

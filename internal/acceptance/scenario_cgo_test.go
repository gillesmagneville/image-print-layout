// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package acceptance

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/app"
	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/export"
	"github.com/gillesmagneville/image-print-layout/internal/layout"
	"github.com/gillesmagneville/image-print-layout/internal/model"
	"github.com/gillesmagneville/image-print-layout/internal/testsvg"
	"github.com/gillesmagneville/image-print-layout/internal/units"
)

// The acceptance scenario: the Mini-CAD, A4 portrait, a drawing 200 mm wide,
// cuts placed by hand, every page 10 mm from the top, the grey background
// replaced by white, a multipage PDF.
var scenarioCuts = []float64{1380.5, 2760, 4120.25}

type fixture struct {
	name string
	svg  []byte
}

// fixtures are the documents the scenario runs on: the synthetic Mini-CAD
// always, the real one when SVGPL_MINICAD points to it (it is not in the
// public repository).
func fixtures(t *testing.T) []fixture {
	t.Helper()
	list := []fixture{{"synthetic", testsvg.Generate(testsvg.Options{})}}
	if p := os.Getenv("SVGPL_MINICAD"); p != "" {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("SVGPL_MINICAD: %v", err)
		}
		list = append(list, fixture{"mini-cad", data})
	}
	return list
}

func services() app.Services {
	return app.Services{Load: engine.LoadDocument, Match: engine.MatchFont}
}

// scenario is the work of a user: a session on a document, and the engine
// document the preview shows.
type scenario struct {
	t    *testing.T
	dir  string
	path string
	sess *app.Session
	eng  engine.Document
}

func newScenario(t *testing.T, svg []byte) *scenario {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mini-cad.svg")
	if err := os.WriteFile(path, svg, 0o644); err != nil {
		t.Fatal(err)
	}
	o, err := app.Open(services(), path, model.Background{Document: model.KeepBackground, Page: model.WhitePage})
	if err != nil {
		t.Fatalf("test 1: the document does not open: %v", err)
	}
	t.Cleanup(func() { o.Engine.Close() })
	s := app.NewSession()
	if err := s.Open(o.Document); err != nil {
		t.Fatal(err)
	}
	return &scenario{t: t, dir: dir, path: path, sess: s, eng: o.Engine}
}

func (sc *scenario) must(err error) {
	sc.t.Helper()
	if err != nil {
		sc.t.Fatal(err)
	}
}

// layOut does the steps 2 to 8 of the scenario.
func (sc *scenario) layOut(cuts []float64) {
	sc.t.Helper()
	sc.must(sc.sess.Apply("undo.paper", "", app.Edit(func(p *model.Project) {
		p.Paper = model.Paper{Format: "A4", Orientation: model.Portrait}
		p.Scale = model.Scale{Mode: model.Width, WidthMM: 200}
		p.Margins = model.Margins{Top: 10, Bottom: 10, Left: 5, Right: 5}
		p.Placement = model.Placement{Horizontal: model.CenterH, Vertical: model.Top}
	})))
	for _, y := range cuts {
		if _, err := sc.sess.AddCut(y, 0); err != nil {
			sc.t.Fatalf("cut at %v: %v", y, err)
		}
	}
}

// exportWith writes the PDF with a background for the document, the way the
// window does: the working copy is loaded again with the choice, then the
// export is run on it.
func (sc *scenario) exportWith(mode model.DocBackground, name string) (path string, res *export.Result) {
	sc.t.Helper()
	sc.must(sc.sess.SetDocBackground(mode))
	doc, err := app.Rebuild(services(), sc.sess.Document(), sc.sess.Project().Background)
	sc.must(err)
	defer doc.Close()
	path = filepath.Join(sc.dir, name)
	res, err = export.Run(export.Request{Doc: doc, Plan: sc.sess.Plan(), Project: sc.sess.Project(), Path: path})
	if err != nil {
		sc.t.Fatalf("export: %v", err)
	}
	return path, res
}

func fileHash(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}

var pageSizeRe = regexp.MustCompile(`Page\s+(\d+) size:\s+([\d.]+) x ([\d.]+) pts`)

const dpi = 300

func TestAcceptance8To13OnTheScenario(t *testing.T) {
	for _, fx := range fixtures(t) {
		t.Run(fx.name, func(t *testing.T) {
			sc := newScenario(t, fx.svg)
			sourceBefore := fileHash(t, sc.path)
			sc.layOut(scenarioCuts)
			plan := sc.sess.Plan()
			if len(plan.Pages) != 4 {
				t.Fatalf("%d pages, want 4", len(plan.Pages))
			}
			// The scale in force when the cuts were added is the one of the
			// export: nothing resized the drawing on the way.
			scale := plan.Scale
			if !near(scale, 200/1280.0, 1e-9) {
				t.Fatalf("scale = %v mm per unit, want 200/1280", scale)
			}

			// 9, the background: found, and white after the replacement.
			info, found := sc.sess.DocumentBackground()
			if !found {
				t.Fatalf("test 9: no background detected in the document")
			}
			t.Logf("background detected: %s %s", info.Element, info.Fill)

			whiten, res := sc.exportWith(model.WhitenBackground, "whiten.pdf")
			keep, _ := sc.exportWith(model.KeepBackground, "keep.pdf")
			remove, _ := sc.exportWith(model.RemoveBackground, "remove.pdf")

			// 10, a multipage PDF: one page per band.
			t.Run("10 page count", func(t *testing.T) {
				if res.Pages != 4 {
					t.Errorf("the export says %d pages, want 4", res.Pages)
				}
				data, _ := os.ReadFile(whiten)
				pdf, err := export.ReadPDF(data)
				if err != nil || len(pdf.Pages) != len(plan.Pages) {
					t.Errorf("the PDF read back has %d pages (%v), want %d", len(pdf.Pages), err, len(plan.Pages))
				}
				if got := len(rasterise(t, whiten, 20)); got != len(plan.Pages) {
					t.Errorf("poppler draws %d pages, want %d", got, len(plan.Pages))
				}
			})

			// 11, the physical size of the pages.
			t.Run("11 page sizes", func(t *testing.T) {
				need(t, "pdfinfo")
				out := run(t, "pdfinfo", "-f", "1", "-l", "99", whiten)
				sizes := pageSizeRe.FindAllStringSubmatch(out, -1)
				if len(sizes) != 4 {
					t.Fatalf("pdfinfo lists %d page sizes:\n%s", len(sizes), out)
				}
				wantW, wantH := units.MMToPt(210), units.MMToPt(297)
				for i, m := range sizes {
					w, _ := strconv.ParseFloat(m[2], 64)
					h, _ := strconv.ParseFloat(m[3], 64)
					// pdfinfo rounds to three decimals: its own error is 0.0005 pt.
					if math.Abs(units.PtToMM(w)-210) > 0.02 || math.Abs(units.PtToMM(h)-297) > 0.02 {
						t.Errorf("page %d is %.3f x %.3f pt, want %.3f x %.3f (0.02 mm)", i+1, w, h, wantW, wantH)
					}
				}
			})

			// 8, every page 10 mm from the top.
			t.Run("8 ten millimetres from the top", func(t *testing.T) {
				for _, pg := range plan.Pages {
					if !near(pg.Y, 10, units.ToleranceMM) {
						t.Errorf("page %d: y = %v mm in the plan, want 10", pg.Number, pg.Y)
					}
				}
				// The kept background is drawn from the top of the band, so its
				// first row is where the band starts, whatever the document
				// draws first (the real Mini-CAD starts 43 units down).
				first := rasterise(t, keep, dpi)[0]
				row := -1
			scan:
				for y := 0; y < first.h; y++ {
					for x := 0; x < first.w; x++ {
						if !first.white(x, y) {
							row = y
							break scan
						}
					}
				}
				if row < 0 {
					t.Fatal("page 1 is blank")
				}
				mm := float64(row) / dpi * 25.4
				t.Logf("first non-white row of page 1: %d px = %.2f mm", row, mm)
				if !near(mm, 10, 0.2) {
					t.Errorf("the drawing starts %.2f mm from the top, want 10 mm (0.2 mm)", mm)
				}
			})

			// 9, the grey background of the document is white, on every page,
			// whether it is replaced or removed. The pixels that only the
			// background paints are the ones of the fill colour with all eight
			// neighbours of that colour too: the edges of the drawing blend
			// other colours into the same grey.
			t.Run("9 background replaced by white", func(t *testing.T) {
				var fill [3]int
				if _, err := fmt.Sscanf(info.Fill, "#%02x%02x%02x", &fill[0], &fill[1], &fill[2]); err != nil {
					t.Skipf("the fill %q is not a #rrggbb colour", info.Fill)
				}
				isFill := func(r raster, x, y int) bool {
					c := r.at(x, y)
					return abs(c[0]-fill[0]) <= 1 && abs(c[1]-fill[1]) <= 1 && abs(c[2]-fill[2]) <= 1
				}
				kept := rasterise(t, keep, 100)
				for _, choice := range []struct {
					name string
					pdf  string
				}{{"replaced", whiten}, {"removed", remove}} {
					got := rasterise(t, choice.pdf, 100)
					if len(got) != len(kept) {
						t.Fatalf("%s: %d pages, %d when kept", choice.name, len(got), len(kept))
					}
					for i := range kept {
						k, g := kept[i], got[i]
						grey, turned := 0, 0
						for y := 1; y < k.h-1; y++ {
							for x := 1; x < k.w-1; x++ {
								inside := true
								for dy := -1; dy <= 1 && inside; dy++ {
									for dx := -1; dx <= 1; dx++ {
										if !isFill(k, x+dx, y+dy) {
											inside = false
											break
										}
									}
								}
								if !inside {
									continue
								}
								grey++
								if g.white(x, y) {
									turned++
								}
							}
						}
						if grey == 0 {
							t.Errorf("%s, page %d: the kept background is nowhere to be seen", choice.name, i+1)
							continue
						}
						if frac := float64(turned) / float64(grey); frac < 0.999 {
							t.Errorf("%s, page %d: only %.2f%% of the grey pixels are white", choice.name, i+1, 100*frac)
						}
						// The margin is the page: opaque white either way.
						if !k.white(2, 2) || !g.white(2, 2) {
							t.Errorf("%s, page %d: the corner of the sheet is not white", choice.name, i+1)
						}
					}
				}
			})

			// The export is checked against the plan (export.Verify) whatever the
			// document background and the page background: a document without
			// its background rectangle leaves no clip of the band in the PDF
			// unless the engine writes one, and a transparent page cannot.
			t.Run("export with every background", func(t *testing.T) {
				defer func() { sc.must(sc.sess.SetPageBackground(model.WhitePage)) }()
				for _, page := range []model.PageBackground{model.WhitePage, model.TransparentPage} {
					sc.must(sc.sess.SetPageBackground(page))
					for _, doc := range []model.DocBackground{model.KeepBackground, model.WhitenBackground, model.RemoveBackground} {
						_, res := sc.exportWith(doc, fmt.Sprintf("every-%s-%s.pdf", page, doc))
						if res.Pages != len(plan.Pages) {
							t.Errorf("page %s, document %s: %d pages, want %d", page, doc, res.Pages, len(plan.Pages))
						}
					}
				}
			})

			// 12, the drawing is as wide on every page.
			t.Run("12 same width on every page", func(t *testing.T) {
				pages := rasterise(t, keep, dpi)
				var fill [3]int
				if _, err := fmt.Sscanf(info.Fill, "#%02x%02x%02x", &fill[0], &fill[1], &fill[2]); err != nil {
					t.Skipf("the fill %q is not a #rrggbb colour", info.Fill)
				}
				for i, pg := range pages {
					// A row of the content window: the background covers it
					// from one edge of the drawing to the other.
					y := int(mmToPx(plan.Pages[i].Y+plan.Pages[i].ContentH/2, dpi))
					left, right := -1, -1
					for x := 0; x < pg.w; x++ {
						if !pg.white(x, y) {
							if left < 0 {
								left = x
							}
							right = x
						}
					}
					if left < 0 {
						t.Fatalf("page %d: nothing on the middle row", i+1)
					}
					width := float64(right-left+1) / dpi * 25.4
					x0 := float64(left) / dpi * 25.4
					if !near(width, 200, 0.2) || !near(x0, 5, 0.2) {
						t.Errorf("page %d: the drawing is %.2f mm wide from x = %.2f mm, want 200 mm from 5 mm", i+1, width, x0)
					}
				}
			})

			// 13, the cuts are where the preview showed them: the PDF says so
			// (geometry), and the pages put side by side are the drawing
			// (pixels).
			t.Run("13 cuts match the preview", func(t *testing.T) {
				data, _ := os.ReadFile(whiten)
				pdf, err := export.ReadPDF(data)
				if err != nil {
					t.Fatal(err)
				}
				specs := export.Specs(plan)
				if items := export.Verify(pdf, specs, plan.Doc.W, plan.Doc.H, true); len(items) > 0 {
					t.Errorf("the PDF does not say what the plan says: %v", items)
				}
				for i, pg := range plan.Pages {
					wantTop := 0.0
					if i > 0 {
						wantTop = scenarioCuts[i-1]
					}
					if pg.Top != wantTop {
						t.Errorf("page %d starts at %v, the cut was placed at %v", i+1, pg.Top, wantTop)
					}
				}
				seamOK(t, sc, whiten, plan.Pages)
			})

			t.Run("export is deterministic", func(t *testing.T) {
				again, _ := sc.exportWith(model.WhitenBackground, "whiten-again.pdf")
				if fileHash(t, whiten) != fileHash(t, again) {
					t.Error("two exports of the same project differ")
				}
			})

			t.Run("source is intact", func(t *testing.T) {
				if sourceBefore != fileHash(t, sc.path) {
					t.Error("the SVG was modified")
				}
			})

			// The scale was never touched by the export, nor by the choice of
			// the background.
			if sc.sess.Plan().Scale != scale {
				t.Errorf("the scale changed from %v to %v", scale, sc.sess.Plan().Scale)
			}
		})
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// seamOK compares the content window of each page with the same stretch of
// the whole drawing exported on one tall sheet at the same scale: both go
// through the same rasteriser, so what differs is the cutting alone.
func seamOK(t *testing.T, sc *scenario, pdf string, pages []layout.Page) {
	t.Helper()
	plan := sc.sess.Plan()
	const rdpi, block, tol = 150, 16, 24
	pxPerPt := float64(rdpi) / 72
	doc, err := app.Rebuild(services(), sc.sess.Document(), sc.sess.Project().Background)
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	ptPerUnit := units.MMToPt(plan.Scale)
	ref := filepath.Join(sc.dir, "ref.pdf")
	var buf bytes.Buffer
	whole := engine.PageSpec{
		WidthPt: units.MMToPt(plan.Doc.W * plan.Scale), HeightPt: units.MMToPt(plan.Doc.H * plan.Scale),
		BandTop: 0, BandBottom: plan.Doc.H, PtPerUnit: ptPerUnit,
	}
	if err := doc.ExportPDF(&buf, []engine.PageSpec{whole}, engine.PDFOptions{WhitePage: true}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ref, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	cut, whole1 := rasterise(t, pdf, rdpi), rasterise(t, ref, rdpi)
	if len(cut) != len(pages) || len(whole1) != 1 {
		t.Fatalf("%d cut pages and %d reference pages", len(cut), len(whole1))
	}
	total, bad := 0, 0
	for i, p := range export.Specs(plan) {
		x0 := int(math.Round(p.OriginXPt * pxPerPt))
		y0 := int(math.Round(p.OriginYPt * pxPerPt))
		cw := int(math.Round(plan.Doc.W * p.PtPerUnit * pxPerPt))
		ch := int(math.Round((p.BandBottom - p.BandTop) * p.PtPerUnit * pxPerPt))
		refTop := int(math.Round(p.BandTop * p.PtPerUnit * pxPerPt))
		if x0+cw > cut[i].w || y0+ch > cut[i].h || refTop+ch > whole1[0].h || cw > whole1[0].w {
			t.Fatalf("page %d: window %dx%d at (%d,%d) is outside the raster", i+1, cw, ch, x0, y0)
		}
		a := cut[i].crop(x0, y0, cw, ch).blocks(block)
		b := whole1[0].crop(0, refTop, cw, ch).blocks(block)
		bad += differing(a, b, tol)
		total += len(a)
	}
	frac := float64(bad) / float64(total)
	t.Logf("seam: %.5f%% of the squares differ from the uncut drawing", 100*frac)
	if frac > 0.00002 {
		t.Errorf("the pages do not reproduce the uncut drawing: %.5f%% of the squares differ", 100*frac)
	}
}

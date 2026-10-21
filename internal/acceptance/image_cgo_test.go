// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package acceptance

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/app"
	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/export"
	"github.com/gillesmagneville/image-print-layout/internal/layout"
	"github.com/gillesmagneville/image-print-layout/internal/model"
	"github.com/gillesmagneville/image-print-layout/internal/testimg"
	"github.com/gillesmagneville/image-print-layout/internal/testsvg"
	"github.com/gillesmagneville/image-print-layout/internal/units"
)

// The scenario of the specification on a picture: the Mini-CAD drawn as a PNG
// of 1 280 × 5 479 pixels, which the program opens as it does an SVG.

// miniCADPicture is the synthetic Mini-CAD as a PNG, one pixel to the unit.
func miniCADPicture(t *testing.T) []byte {
	t.Helper()
	doc, err := engine.Load(testsvg.Generate(testsvg.Options{}), "")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	w, h := doc.Size()
	img, err := doc.Render(engine.Box{W: w, H: h}, 1, color.White)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestTheScenarioOnAPicture(t *testing.T) {
	need(t, "pdfinfo")
	sc := newScenarioFile(t, "mini-cad.png", miniCADPicture(t))
	before := fileHash(t, sc.path)

	// 1. The picture is opened as a document of 1 280 × 5 479 units.
	doc := sc.sess.Document()
	if !doc.IsImage() || doc.Analysis != nil || doc.Image == nil {
		t.Fatalf("the picture was not opened as an image: %+v", doc)
	}
	if doc.Size != (layout.Doc{W: 1280, H: 5479}) {
		t.Fatalf("size = %v, want 1280 x 5479", doc.Size)
	}
	if doc.Image.Format != engine.FormatPNG || doc.Image.WidthPx != 1280 || doc.Image.HeightPx != 5479 {
		t.Errorf("image = %+v", doc.Image)
	}
	if len(doc.Blocks) == 0 {
		t.Fatal("the picture has no blocks: the cuts could not be proposed between them")
	}

	// 2 to 8. A4 portrait, 200 mm, cuts by hand, 10 mm from the top.
	sc.layOut(scenarioCuts)
	plan := sc.sess.Plan()
	if len(plan.Pages) != 4 {
		t.Fatalf("%d pages, want 4", len(plan.Pages))
	}
	if !near(plan.Scale, 200/1280.0, 1e-9) {
		t.Fatalf("scale = %v mm per unit, want 200/1280", plan.Scale)
	}
	scale := plan.Scale

	// Adding a cut and moving one never change the scale.
	more, err := sc.sess.AddCut(600, 0)
	sc.must(err)
	sameWidths(t, "adding a cut", plan, sc.sess.Plan())
	if _, err := sc.sess.DragCut(more, 640, 0); err != nil {
		t.Fatal(err)
	}
	sc.sess.EndDrag()
	sameWidths(t, "moving a cut", plan, sc.sess.Plan())
	sc.must(sc.sess.DeleteCut(more))
	plan = sc.sess.Plan()

	// 9. A picture has no background rectangle: nothing to whiten.
	if _, found := sc.sess.DocumentBackground(); found {
		t.Error("a picture has no background rectangle")
	}

	// 10. Export.
	keep, res := sc.exportWith(model.KeepBackground, "picture.pdf")
	if res.Pages != 4 {
		t.Fatalf("the export says %d pages, want 4", res.Pages)
	}

	// 11. The pages have the size of the sheet.
	out := run(t, "pdfinfo", "-f", "1", "-l", "99", keep)
	sizes := pageSizeRe.FindAllStringSubmatch(out, -1)
	if len(sizes) != 4 {
		t.Fatalf("pdfinfo lists %d page sizes:\n%s", len(sizes), out)
	}
	for i, m := range sizes {
		w, _ := strconv.ParseFloat(m[2], 64)
		h, _ := strconv.ParseFloat(m[3], 64)
		if math.Abs(units.PtToMM(w)-210) > 0.02 || math.Abs(units.PtToMM(h)-297) > 0.02 {
			t.Errorf("page %d is %.3f x %.3f pt, want 210 x 297 mm", i+1, w, h)
		}
	}

	pages := rasterise(t, keep, dpi)
	// 8. The drawing starts 10 mm from the top of the first page.
	row := -1
scan:
	for y := 0; y < pages[0].h; y++ {
		for x := 0; x < pages[0].w; x++ {
			if !pages[0].white(x, y) {
				row = y
				break scan
			}
		}
	}
	if row < 0 {
		t.Fatal("page 1 is blank")
	}
	if mm := float64(row) / dpi * 25.4; !near(mm, 10, 0.2) {
		t.Errorf("the drawing starts %.2f mm from the top, want 10 mm", mm)
	}

	// 12. The drawing is as wide on every page: 200 mm from 5 mm.
	for i, pg := range pages {
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
		width, x0 := float64(right-left+1)/dpi*25.4, float64(left)/dpi*25.4
		if !near(width, 200, 0.2) || !near(x0, 5, 0.2) {
			t.Errorf("page %d: the drawing is %.2f mm wide from x = %.2f mm, want 200 mm from 5 mm", i+1, width, x0)
		}
	}

	// 13. The cuts are where the preview put them, and the pages put side by
	// side are the whole picture.
	data, _ := os.ReadFile(keep)
	pdf, err := export.ReadPDF(data)
	if err != nil {
		t.Fatal(err)
	}
	if items := export.Verify(pdf, export.Specs(plan), plan.Doc.W, plan.Doc.H, true); len(items) > 0 {
		t.Errorf("the PDF does not say what the plan says: %v", items)
	}
	seamOK(t, sc, keep, plan.Pages)

	// The export is deterministic and the source is intact.
	again, _ := sc.exportWith(model.KeepBackground, "picture-again.pdf")
	if fileHash(t, keep) != fileHash(t, again) {
		t.Error("two exports of the same project differ")
	}
	if before != fileHash(t, sc.path) {
		t.Error("the picture was modified")
	}
	if sc.sess.Plan().Scale != scale {
		t.Errorf("the scale changed from %v to %v", scale, sc.sess.Plan().Scale)
	}
}

func TestTheCutsOfAPictureAreProposedBetweenItsBlocks(t *testing.T) {
	sc := newScenarioFile(t, "mini-cad.png", miniCADPicture(t))
	sc.layOut(nil)
	if len(sc.sess.Voids()) == 0 {
		t.Fatal("the picture has no void to cut in")
	}
	prop, err := sc.sess.AutoCuts()
	if err != nil {
		t.Fatal(err)
	}
	if len(prop.Positions) == 0 {
		t.Fatal("no cut is proposed for a picture 856 mm high on A4")
	}
	crossing := 0
	for _, y := range prop.Positions {
		if len(layout.Crossed(y, sc.sess.Blocks())) > 0 {
			crossing++
		}
	}
	t.Logf("%d cuts proposed, %d cross a block", len(prop.Positions), crossing)
	if crossing > 0 {
		t.Errorf("%d of the proposed cuts cross a block that the picture has a void to avoid", crossing)
	}
}

// A picture that fits on one page and is exported on a transparent page: the
// check of the PDF must find the band of the picture by itself.
func TestAPictureOnOnePageAndATransparentPage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 300, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 300; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	sc := newScenarioFile(t, "gradient.png", buf.Bytes())
	sc.must(sc.sess.Apply("undo.paper", "", app.Edit(func(p *model.Project) {
		p.Paper = model.Paper{Format: "A4", Orientation: model.Portrait}
		p.Scale = model.Scale{Mode: model.Width, WidthMM: 100}
		p.Margins = model.Margins{Top: 10, Bottom: 10, Left: 5, Right: 5}
		p.Placement = model.Placement{Horizontal: model.CenterH, Vertical: model.Top}
	})))
	if n := len(sc.sess.Plan().Pages); n != 1 {
		t.Fatalf("%d pages, want 1", n)
	}
	for _, page := range []model.PageBackground{model.WhitePage, model.TransparentPage} {
		sc.must(sc.sess.SetPageBackground(page))
		path := filepath.Join(sc.dir, "gradient-"+string(page)+".pdf")
		doc, err := app.Rebuild(services(), sc.sess.Document(), sc.sess.Project().Background)
		sc.must(err)
		_, err = export.Run(export.Request{Doc: doc, Plan: sc.sess.Plan(), Project: sc.sess.Project(), Path: path})
		doc.Close()
		if err != nil {
			t.Errorf("page %s: %v", page, err)
		}
	}
}

// Pictures of every format, turned by their orientation tag or at another
// resolution, are placed and cut like the others: the check of the PDF finds
// them where the plan puts them, and the drawing keeps the width asked for.
// Every picture is at most 200 mm high at 100 mm wide, so it fits on a sheet.
func TestPicturesOfEveryKindAreExportedWhereThePlanPutsThem(t *testing.T) {
	need(t, "pdftoppm")
	quads := testimg.Quadrants(120, 60)
	for name, data := range map[string][]byte{
		"png":                 testimg.PNG(quads, 0, 0),
		"png at 300 dpi":      testimg.PNG(quads, 300, 300),
		"jpeg":                testimg.JPEG(quads),
		"jpeg turned":         testimg.WithEXIFOrientation(testimg.WithJFIFDensity(testimg.JPEG(quads), 150, 150), 6),
		"tiff":                testimg.TIFF(quads, 200, 200, 1),
		"tiff turned":         testimg.TIFF(quads, 200, 200, 8),
		"tiff mirrored":       testimg.TIFF(quads, 200, 200, 2),
		"tiff transposed":     testimg.TIFF(quads, 96, 96, 5),
		"tiff other diagonal": testimg.TIFF(quads, 96, 96, 7),
	} {
		t.Run(name, func(t *testing.T) {
			sc := newScenarioFile(t, "picture", data)
			if !sc.sess.Document().IsImage() {
				t.Fatal("not opened as an image")
			}
			sc.must(sc.sess.Apply("undo.paper", "", app.Edit(func(p *model.Project) {
				p.Paper = model.Paper{Format: "A4", Orientation: model.Portrait}
				p.Scale = model.Scale{Mode: model.Width, WidthMM: 100}
				p.Margins = model.Margins{Top: 10, Bottom: 10, Left: 5, Right: 5}
				p.Placement = model.Placement{Horizontal: model.CenterH, Vertical: model.Top}
			})))
			plan := sc.sess.Plan()
			if !near(plan.ContentWidthMM, 100, 1e-9) {
				t.Fatalf("content is %v mm wide, want 100", plan.ContentWidthMM)
			}
			// The export runs the check of the PDF against the plan.
			path, _ := sc.exportWith(model.KeepBackground, "picture.pdf")
			page := rasterise(t, path, 100)[0]
			// Where the plan puts the picture, in pixels at 100 dpi.
			pg := plan.Pages[0]
			x0, y0 := int(mmToPx(pg.X, 100)), int(mmToPx(pg.Y, 100))
			w, h := int(mmToPx(plan.ContentWidthMM, 100)), int(mmToPx(pg.ContentH, 100))
			if page.white(x0+w/4, y0+h/4) && page.white(x0+3*w/4, y0+h/4) && page.white(x0+w/4, y0+3*h/4) {
				t.Errorf("the picture is not where the plan puts it")
			}
			// The corner outside the picture is the page.
			if !page.white(x0/2, y0/2) {
				t.Errorf("the margin of the sheet is not white")
			}
		})
	}
}

// SPDX-License-Identifier: GPL-3.0-only

package export

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/engine"
	"github.com/gillesmagneville/pageslice/internal/layout"
	"github.com/gillesmagneville/pageslice/internal/model"
	"github.com/gillesmagneville/pageslice/internal/units"
)

var miniCAD = layout.Doc{W: 1280, H: 5479}

// acceptance is the project of the acceptance scenario: A4 portrait, the
// drawing 200 mm wide, three cuts, every page 10 mm from the top.
func acceptance(cuts ...float64) model.Project {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 200}
	for _, c := range cuts {
		p.Cuts = append(p.Cuts, model.Cut{ID: p.NextCutID, Axis: model.AxisY, Position: c})
		p.NextCutID++
	}
	return p
}

func planOf(t *testing.T, p model.Project) *layout.Plan {
	t.Helper()
	pl, err := layout.Compute(p, miniCAD, nil)
	if err != nil {
		t.Fatal(err)
	}
	return pl
}

func codes(items []diag.Item) []string {
	var out []string
	for _, i := range items {
		out = append(out, i.Code)
	}
	return out
}

func has(items []diag.Item, code string) bool {
	for _, i := range items {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestSpecsAreThePlanInPoints(t *testing.T) {
	p := acceptance(1380.5, 2760, 4120.25)
	pl := planOf(t, p)
	specs := Specs(pl)
	if len(specs) != 4 {
		t.Fatalf("%d specs, want 4", len(specs))
	}
	for i, s := range specs {
		page := pl.Pages[i]
		if math.Abs(units.PtToMM(s.WidthPt)-210) > 1e-9 || math.Abs(units.PtToMM(s.HeightPt)-297) > 1e-9 {
			t.Errorf("page %d: sheet %.4f x %.4f mm", i+1, units.PtToMM(s.WidthPt), units.PtToMM(s.HeightPt))
		}
		if s.BandTop != page.Top || s.BandBottom != page.Bottom {
			t.Errorf("page %d: band %v-%v, plan %v-%v", i+1, s.BandTop, s.BandBottom, page.Top, page.Bottom)
		}
		if math.Abs(units.PtToMM(s.OriginYPt)-10) > 1e-9 {
			t.Errorf("page %d is %.4f mm from the top, want 10", i+1, units.PtToMM(s.OriginYPt))
		}
		// The drawing is as wide on every page: 200 mm.
		if got := units.PtToMM(s.PtPerUnit * miniCAD.W); math.Abs(got-200) > 1e-6 {
			t.Errorf("page %d: the drawing is %.6f mm wide, want 200", i+1, got)
		}
		if s.PtPerUnit != specs[0].PtPerUnit {
			t.Errorf("page %d has its own scale", i+1)
		}
	}
}

func TestOnlyAPageThatAskedForTransparencyIsNotPaintedWhite(t *testing.T) {
	p := model.Default()
	if !WhitePage(p) {
		t.Error("the default page is not white")
	}
	p.Background.Page = model.TransparentPage
	if WhitePage(p) {
		t.Error("a transparent page is painted white")
	}
}

func TestCheckPassesOnTheAcceptanceScenario(t *testing.T) {
	p := acceptance(1380.5, 2760, 4120.25)
	if items := Check(Input{Project: p, Plan: planOf(t, p), HasBackground: true}); len(items) != 0 {
		t.Errorf("the acceptance scenario raises %v", codes(items))
	}
}

func TestCheckStopsAPageThatLeavesTheSheet(t *testing.T) {
	p := acceptance() // one page, 855 mm tall
	items := Check(Input{Project: p, Plan: planOf(t, p)})
	if !has(items, "export.outside_sheet") || !Blocked(items) {
		t.Errorf("items = %v", codes(items))
	}
	if NeedsConfirmation(items) {
		t.Error("a failure is not a warning to confirm")
	}
}

func TestCheckWarnsOfAPageThatGoesIntoTheMargins(t *testing.T) {
	p := acceptance(1380.5, 2760, 4120.25)
	p.Margins = model.Margins{Top: 10, Bottom: 10, Left: 5, Right: 5}
	// Pushed 66 mm down, the pages (about 215 mm tall) end at about 291 mm:
	// inside the 297 mm of the sheet, but over the bottom margin, which starts
	// at 287 mm.
	p.Placement.OffsetYMM = 66
	pl := planOf(t, p)
	items := Check(Input{Project: p, Plan: pl})
	if !has(items, "export.outside_margins") {
		t.Fatalf("items = %v, pages %+v", codes(items), pl.Pages)
	}
	if Blocked(items) || !NeedsConfirmation(items) {
		t.Errorf("a warning must ask for a confirmation, not forbid: %v", codes(items))
	}
}

func TestCheckCatchesAPlanThatLosesOrRepeatsPartOfTheDrawing(t *testing.T) {
	p := acceptance(1380.5, 2760, 4120.25)

	gap := planOf(t, p)
	gap.Pages[2].Top += 3 // a strip of the drawing falls between two pages
	if items := Check(Input{Project: p, Plan: gap}); !has(items, "export.cuts_invalid") {
		t.Errorf("a gap between two bands: %v", codes(items))
	}

	short := planOf(t, p)
	short.Pages[3].Bottom -= 3
	short.Pages[3].ContentH -= 3 * short.Scale
	if items := Check(Input{Project: p, Plan: short}); !has(items, "export.bands_sum") {
		t.Errorf("bands that do not add up: %v", codes(items))
	}

	narrow := planOf(t, p)
	narrow.Pages[1].ContentW -= 2
	if items := Check(Input{Project: p, Plan: narrow}); !has(items, "export.width_differs") {
		t.Errorf("a page narrower than the others: %v", codes(items))
	}

	reversed := planOf(t, p)
	reversed.Pages[1].Top, reversed.Pages[1].Bottom = reversed.Pages[1].Bottom, reversed.Pages[1].Top
	if items := Check(Input{Project: p, Plan: reversed}); !has(items, "export.cuts_invalid") {
		t.Errorf("an upside-down band: %v", codes(items))
	}

	close := planOf(t, p)
	close.Pages[0].Bottom = 1
	close.Pages[1].Top = 1
	if items := Check(Input{Project: p, Plan: close}); !has(items, "export.cuts_invalid") {
		t.Errorf("a page thinner than the smallest gap: %v", codes(items))
	}
}

func TestCheckComparesTheBackgroundAskedForWithTheOneFound(t *testing.T) {
	p := acceptance(1380.5, 2760, 4120.25)
	pl := planOf(t, p)

	p.Background.Document = model.WhitenBackground
	if items := Check(Input{Project: p, Plan: pl, HasBackground: false}); !has(items, "export.no_background") {
		t.Errorf("a replacement with nothing to replace: %v", codes(items))
	}
	if items := Check(Input{Project: p, Plan: pl, HasBackground: true}); has(items, "export.no_background") {
		t.Errorf("a replacement of a background that exists: %v", codes(items))
	}

	p.Background = model.Background{Document: model.KeepBackground, Page: model.TransparentPage}
	if items := Check(Input{Project: p, Plan: pl, HasBackground: true}); !has(items, "export.background_covers") {
		t.Errorf("a transparent page under a kept background: %v", codes(items))
	}
	p.Background.Document = model.RemoveBackground
	if items := Check(Input{Project: p, Plan: pl, HasBackground: true}); has(items, "export.background_covers") {
		t.Errorf("a transparent page with the background removed: %v", codes(items))
	}
}

func TestCheckListsWhatTheEngineWillNotDraw(t *testing.T) {
	p := acceptance(1380.5, 2760, 4120.25)
	findings := []diag.Item{diag.New("svg.filter", 0), diag.New("svg.images_embedded", 0, "count", 2)}
	items := Check(Input{Project: p, Plan: planOf(t, p), Findings: findings})
	if !has(items, "svg.filter") || has(items, "svg.images_embedded") {
		t.Errorf("only the warnings are listed: %v", codes(items))
	}
	if !NeedsConfirmation(items) {
		t.Error("an unsupported feature asks for a confirmation")
	}
}

// ---- reading a PDF ------------------------------------------------------------------------

type synthPage struct {
	w, h    float64
	content string
}

func flate(data string) []byte {
	var b bytes.Buffer
	zw := zlib.NewWriter(&b)
	zw.Write([]byte(data))
	zw.Close()
	return b.Bytes()
}

// synthetic writes a PDF of the two shapes the reader knows: classic objects,
// or, as cairo 1.18 does, the page tree packed in an object stream and the
// content in flate streams.
func synthetic(pages []synthPage, packed bool) []byte {
	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n")
	n := len(pages)
	kids := make([]string, n)
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", 10+2*i)
	}
	catalog := "<< /Type /Catalog /Pages 2 0 R >>"
	tree := fmt.Sprintf("<< /Type /Pages /Count %d /Kids [ %s ] >>", n, strings.Join(kids, " "))
	pageDict := func(i int) string {
		return fmt.Sprintf("<< /Type /Page /Parent 2 0 R /Contents %d 0 R /MediaBox [ 0 0 %.6f %.6f ] >>", 11+2*i, pages[i].w, pages[i].h)
	}
	object := func(num int, dict string, stream []byte) {
		fmt.Fprintf(&out, "%d 0 obj\n%s\n", num, dict)
		if stream != nil {
			out.WriteString("stream\n")
			out.Write(stream)
			out.WriteString("\nendstream\n")
		}
		out.WriteString("endobj\n")
	}
	if packed {
		inner := []string{catalog, tree}
		nums := []int{1, 2}
		for i := range pages {
			inner = append(inner, pageDict(i))
			nums = append(nums, 10+2*i)
		}
		var header, body strings.Builder
		for i, d := range inner {
			fmt.Fprintf(&header, "%d %d ", nums[i], body.Len())
			body.WriteString(d + "\n")
		}
		data := header.String() + body.String()
		z := flate(data)
		object(3, fmt.Sprintf("<< /Type /ObjStm /N %d /First %d /Filter /FlateDecode /Length %d >>", len(inner), header.Len(), len(z)), z)
	} else {
		object(1, catalog, nil)
		object(2, tree, nil)
		for i := range pages {
			object(10+2*i, pageDict(i), nil)
		}
	}
	for i, p := range pages {
		if packed {
			z := flate(p.content)
			object(11+2*i, fmt.Sprintf("<< /Filter /FlateDecode /Length %d >>", len(z)), z)
		} else {
			object(11+2*i, fmt.Sprintf("<< /Length %d >>", len(p.content)), []byte(p.content))
		}
	}
	out.WriteString("trailer\n<< /Root 1 0 R >>\nstartxref\n0\n%%EOF\n")
	return out.Bytes()
}

func TestReadPDFReadsPagesInOrderInBothShapes(t *testing.T) {
	pages := []synthPage{
		{595.275591, 841.889764, "1 0 0 -1 0 841.89 cm\nq\n14.172 28.348 566.93 611.441 re W n\nQ"},
		{841.889764, 595.275591, "second"},
		{595.275591, 841.889764, "third"},
	}
	for _, packed := range []bool{false, true} {
		pdf, err := ReadPDF(synthetic(pages, packed))
		if err != nil {
			t.Fatalf("packed=%v: %v", packed, err)
		}
		if len(pdf.Pages) != 3 {
			t.Fatalf("packed=%v: %d pages", packed, len(pdf.Pages))
		}
		for i, want := range pages {
			got := pdf.Pages[i]
			if math.Abs(got.WidthPt-want.w) > 1e-6 || math.Abs(got.HeightPt-want.h) > 1e-6 {
				t.Errorf("packed=%v page %d: %.4f x %.4f, want %.4f x %.4f", packed, i+1, got.WidthPt, got.HeightPt, want.w, want.h)
			}
			if !strings.Contains(got.Content, want.content) {
				t.Errorf("packed=%v page %d: content %q does not hold %q", packed, i+1, got.Content, want.content)
			}
		}
	}
}

func TestReadPDFRefusesWhatIsNotACompletePDF(t *testing.T) {
	good := synthetic([]synthPage{{100, 100, "x"}}, true)
	cases := map[string][]byte{
		"empty":     nil,
		"text":      []byte("hello"),
		"truncated": good[:len(good)/2],
		"no pages":  []byte("%PDF-1.7\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\ntrailer\n%%EOF\n"),
	}
	for name, data := range cases {
		if _, err := ReadPDF(data); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// ---- comparing with the plan -------------------------------------------------------------------

// pdfOf draws what the engine would write for the specs, without the engine:
// the clip of each band, then a rectangle that covers the whole document.
func pdfOf(specs []engine.PageSpec, docW, docH float64) *PDF {
	pdf := &PDF{}
	for _, s := range specs {
		content := fmt.Sprintf("1 0 0 -1 0 %.6f cm\nq\n1 1 1 rg\n0 0 596 842 re f\nQ q\n%.3f %.3f %.3f %.3f re W n\n0.9 g\n%.3f %.3f %.3f %.3f re f\nQ\n",
			s.HeightPt,
			s.OriginXPt, s.OriginYPt, docW*s.PtPerUnit, (s.BandBottom-s.BandTop)*s.PtPerUnit,
			s.OriginXPt, s.OriginYPt-s.BandTop*s.PtPerUnit, docW*s.PtPerUnit, docH*s.PtPerUnit)
		pdf.Pages = append(pdf.Pages, PDFPage{WidthPt: s.WidthPt, HeightPt: s.HeightPt, Content: content})
	}
	return pdf
}

func TestVerifyAcceptsAPDFThatMatchesThePlan(t *testing.T) {
	specs := Specs(planOf(t, acceptance(1380.5, 2760, 4120.25)))
	if items := Verify(pdfOf(specs, miniCAD.W, miniCAD.H), specs, miniCAD.W, miniCAD.H, true); len(items) != 0 {
		t.Errorf("a matching PDF raises %v", items)
	}
}

func TestVerifyCatchesEveryWayAPDFCanDifferFromThePlan(t *testing.T) {
	specs := Specs(planOf(t, acceptance(1380.5, 2760, 4120.25)))
	fresh := func() *PDF { return pdfOf(specs, miniCAD.W, miniCAD.H) }

	t.Run("pages", func(t *testing.T) {
		pdf := fresh()
		pdf.Pages = pdf.Pages[:3]
		if items := Verify(pdf, specs, miniCAD.W, miniCAD.H, true); !has(items, "export.pdf_pages") {
			t.Errorf("items = %v", codes(items))
		}
	})
	t.Run("size", func(t *testing.T) {
		pdf := fresh()
		pdf.Pages[2].WidthPt = units.MMToPt(210.5) // half a millimetre wider
		if items := Verify(pdf, specs, miniCAD.W, miniCAD.H, true); !has(items, "export.pdf_size") {
			t.Errorf("items = %v", codes(items))
		}
	})
	t.Run("size within the tolerance", func(t *testing.T) {
		pdf := fresh()
		pdf.Pages[2].WidthPt = units.MMToPt(210.015)
		if items := Verify(pdf, specs, miniCAD.W, miniCAD.H, true); len(items) != 0 {
			t.Errorf("0.015 mm is within the tolerance of 0.02 mm: %v", items)
		}
	})
	t.Run("clip", func(t *testing.T) {
		moved := append([]engine.PageSpec(nil), specs...)
		moved[1].BandBottom += 0.1 // a tenth of a unit: 0.04 pt, ten times the tolerance
		items := Verify(pdfOf(moved, miniCAD.W, miniCAD.H), specs, miniCAD.W, miniCAD.H, true)
		if !has(items, "export.pdf_clip") {
			t.Errorf("items = %v", codes(items))
		}
	})
	t.Run("offset", func(t *testing.T) {
		moved := append([]engine.PageSpec(nil), specs...)
		moved[2].BandTop += 0.1
		moved[2].BandBottom += 0.1 // same window height, drawing shifted
		items := Verify(pdfOf(moved, miniCAD.W, miniCAD.H), specs, miniCAD.W, miniCAD.H, true)
		if !has(items, "export.pdf_offset") {
			t.Errorf("items = %v", codes(items))
		}
	})
	t.Run("no clip", func(t *testing.T) {
		pdf := fresh()
		pdf.Pages[0].Content = "q Q"
		if items := Verify(pdf, specs, miniCAD.W, miniCAD.H, true); !has(items, "export.pdf_clip") {
			t.Errorf("items = %v", codes(items))
		}
	})
}

// reducedPDF draws what cairo writes when the page is not white and the document
// has no rectangle that covers it: no clip for the band, but, for each element
// that crosses a cut, its own clip cut down to the band.
func reducedPDF(specs []engine.PageSpec) *PDF {
	pdf := &PDF{}
	for _, s := range specs {
		bandH := (s.BandBottom - s.BandTop) * s.PtPerUnit
		content := fmt.Sprintf("1 0 0 -1 0 %.6f cm\nq\n%.3f %.3f %.3f %.3f re W n\n0.9 g\n%.3f %.3f 80 20 re f\nQ\nq\n%.3f %.3f %.3f %.3f re W n\n0.8 g\n%.3f %.3f 120 40 re f\nQ\n",
			s.HeightPt,
			s.OriginXPt+20, s.OriginYPt, 90.0, 12.5, // an element across the top cut
			s.OriginXPt+20, s.OriginYPt-8,
			s.OriginXPt+40, s.OriginYPt+bandH-15, 200.0, 15.0, // one across the bottom cut
			s.OriginXPt+40, s.OriginYPt+bandH-35)
		pdf.Pages = append(pdf.Pages, PDFPage{WidthPt: s.WidthPt, HeightPt: s.HeightPt, Content: content})
	}
	return pdf
}

// With a white page the engine writes the clip of the band whole; without one,
// and without a rectangle that covers the document, the file holds only the
// clips of the elements that cross a cut (ARCHITECTURE §9).
func TestVerifyWithoutAWholeClipChecksTheClipsOfTheElements(t *testing.T) {
	specs := Specs(planOf(t, acceptance(1380.5, 2760, 4120.25)))

	t.Run("a page that is not white accepts them", func(t *testing.T) {
		if items := Verify(reducedPDF(specs), specs, miniCAD.W, miniCAD.H, false); len(items) != 0 {
			t.Errorf("the clips of the elements are inside the band: %v", items)
		}
	})
	t.Run("a white page needs the whole clip", func(t *testing.T) {
		if items := Verify(reducedPDF(specs), specs, miniCAD.W, miniCAD.H, true); !has(items, "export.pdf_clip") {
			t.Errorf("items = %v", codes(items))
		}
	})
	t.Run("a page with no clip at all is fine when nothing crosses a cut", func(t *testing.T) {
		pdf := pdfOf(specs, miniCAD.W, miniCAD.H)
		pdf.Pages[0].Content = "1 0 0 -1 0 842 cm\nq\n0.9 g\n20 40 80 20 re f\nQ\n"
		if items := Verify(pdf, specs, miniCAD.W, miniCAD.H, false); len(items) != 0 {
			t.Errorf("items = %v", codes(items))
		}
	})
	t.Run("a clip that leaves the band is caught", func(t *testing.T) {
		pdf := reducedPDF(specs)
		bandH := (specs[1].BandBottom - specs[1].BandTop) * specs[1].PtPerUnit
		pdf.Pages[1].Content += fmt.Sprintf("q\n%.3f %.3f 90 20 re W n\n0 g\n1 1 1 1 re f\nQ\n", specs[1].OriginXPt+20, specs[1].OriginYPt+bandH-5) // 15 pt too low
		if items := Verify(pdf, specs, miniCAD.W, miniCAD.H, false); !has(items, "export.pdf_clip") {
			t.Errorf("items = %v", codes(items))
		}
	})
	t.Run("a whole clip that is not the plan's is caught", func(t *testing.T) {
		moved := append([]engine.PageSpec(nil), specs...)
		moved[1].BandBottom += 0.1
		if items := Verify(pdfOf(moved, miniCAD.W, miniCAD.H), specs, miniCAD.W, miniCAD.H, false); !has(items, "export.pdf_clip") {
			t.Errorf("items = %v", codes(items))
		}
	})
}

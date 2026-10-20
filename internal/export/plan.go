// SPDX-License-Identifier: GPL-3.0-only

package export

import (
	"math"

	"github.com/gillesmagneville/svg-print-layout/internal/diag"
	"github.com/gillesmagneville/svg-print-layout/internal/engine"
	"github.com/gillesmagneville/svg-print-layout/internal/layout"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
	"github.com/gillesmagneville/svg-print-layout/internal/units"
)

// Specs is what the engine draws for a plan: one sheet per page, in points.
// The scale is read from the plan and never recomputed here, and the same
// specs serve the preview of a sheet and the PDF.
func Specs(pl *layout.Plan) []engine.PageSpec {
	specs := make([]engine.PageSpec, len(pl.Pages))
	for i, p := range pl.Pages {
		specs[i] = engine.PageSpec{
			WidthPt: units.MMToPt(pl.Sheet.WidthMM), HeightPt: units.MMToPt(pl.Sheet.HeightMM),
			BandTop: p.Top, BandBottom: p.Bottom,
			OriginXPt: units.MMToPt(p.X), OriginYPt: units.MMToPt(p.Y),
			PtPerUnit: units.MMToPt(pl.Scale),
		}
	}
	return specs
}

// WhitePage tells whether the sheets are painted white before the drawing.
func WhitePage(p model.Project) bool { return p.Background.Page != model.TransparentPage }

// Input is what the checks before the export look at.
type Input struct {
	Project model.Project
	Plan    *layout.Plan
	// HasBackground tells whether the analysis found a background rectangle
	// in the document.
	HasBackground bool
	// Findings are what the analysis said of the document: unsupported
	// features, substituted fonts.
	Findings []diag.Item
}

// tol is the tolerance of the position checks, in millimetres.
const tol = units.ToleranceMM

// Check runs the checks before the export (ARCHITECTURE §11, 1 to 7). A
// Failure forbids the export; a Warning asks the user to confirm it. The
// checks recompute from the plan what the plan is meant to guarantee, rather
// than trusting its states.
func Check(in Input) []diag.Item {
	var out []diag.Item
	pl := in.Plan
	if pl == nil || len(pl.Pages) == 0 {
		return []diag.Item{diag.New("export.no_pages", 0)}
	}

	// 1. The cuts are in strict order, inside the document, far enough apart.
	minGap := layout.MinCutGapMM / pl.Scale
	for i, p := range pl.Pages {
		wantTop := 0.0
		if i > 0 {
			wantTop = pl.Pages[i-1].Bottom
		}
		switch {
		case p.Top != wantTop && math.Abs(p.Top-wantTop) > 1e-9:
			out = append(out, diag.New("export.cuts_invalid", p.Number))
		case p.Bottom <= p.Top, p.Bottom > pl.Doc.H+1e-9, p.Top < -1e-9:
			out = append(out, diag.New("export.cuts_invalid", p.Number))
		case p.Bottom-p.Top < minGap-1e-9 && len(pl.Pages) > 1:
			out = append(out, diag.New("export.cuts_invalid", p.Number))
		}
	}

	// 2. The bands add up to the document: nothing is lost, nothing is drawn
	// twice.
	sum := 0.0
	for _, p := range pl.Pages {
		sum += p.Bottom - p.Top
	}
	if math.Abs(sum-pl.Doc.H) > 1e-9*math.Max(1, pl.Doc.H) {
		out = append(out, diag.New("export.bands_sum", 0, "sum", sum, "height", pl.Doc.H))
	}

	// 3. The drawing is as wide on every page.
	wantW := pl.Doc.W * pl.Scale
	for _, p := range pl.Pages {
		if math.Abs(p.ContentW-wantW) > 1e-9*math.Max(1, wantW) {
			out = append(out, diag.New("export.width_differs", p.Number, "w", p.ContentW, "want", wantW))
		}
	}

	// 4 and 5. The content is on the sheet, and inside the margins.
	sh := pl.Sheet
	for _, p := range pl.Pages {
		r := p.Rect()
		onSheet := r.X >= -tol && r.Y >= -tol && r.X+r.W <= sh.WidthMM+tol && r.Y+r.H <= sh.HeightMM+tol
		inMargins := r.X >= sh.UsableLeft-tol && r.Y >= sh.UsableTop-tol &&
			r.X+r.W <= sh.UsableRight()+tol && r.Y+r.H <= sh.UsableBottom()+tol
		switch {
		case !onSheet:
			out = append(out, diag.New("export.outside_sheet", p.Number))
		case !inMargins:
			out = append(out, diag.New("export.outside_margins", p.Number))
		}
	}

	// 6. The background asked for matches the one found.
	bg := in.Project.Background
	if bg.Document != model.KeepBackground && bg.Document != "" && !in.HasBackground {
		out = append(out, diag.New("export.no_background", 0))
	}
	if bg.Page == model.TransparentPage && in.HasBackground && (bg.Document == model.KeepBackground || bg.Document == model.WhitenBackground || bg.Document == "") {
		out = append(out, diag.New("export.background_covers", 0))
	}

	// 7. What the engine will not draw as the document says: listed as the
	// analysis found it.
	for _, f := range in.Findings {
		if f.Level == diag.Warning {
			out = append(out, f)
		}
	}
	return out
}

// Blocked tells whether a list of items forbids the export.
func Blocked(items []diag.Item) bool {
	for _, i := range items {
		if i.Level == diag.Failure {
			return true
		}
	}
	return false
}

// NeedsConfirmation tells whether a list of items, with no failure in it,
// holds warnings that the user must accept.
func NeedsConfirmation(items []diag.Item) bool {
	if Blocked(items) {
		return false
	}
	for _, i := range items {
		if i.Level == diag.Warning {
			return true
		}
	}
	return false
}

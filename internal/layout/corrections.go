// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"math"
	"sort"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/model"
	"github.com/gillesmagneville/pageslice/internal/units"
)

// CorrectionKind is the way a proposal fixes an overflow.
type CorrectionKind string

const (
	// CorrectionAddCut cuts the page that is too tall in two.
	CorrectionAddCut CorrectionKind = "add_cut"
	// CorrectionMoveCut moves the cut next to the page that is too tall.
	CorrectionMoveCut CorrectionKind = "move_cut"
	// CorrectionReduceScale lowers the scale, on purpose and by the user's click, to the
	// value at which the tallest page fits.
	CorrectionReduceScale CorrectionKind = "reduce_scale"
	// CorrectionChangePaper takes the first format in which every page fits.
	CorrectionChangePaper CorrectionKind = "change_paper"
)

// Correction is a proposal made when a page overflows. It carries the
// figures to show on its button, and Apply turns it into a new project. The
// program never applies one by itself: a drawing is never made smaller without
// the user having asked for it.
type Correction struct {
	Kind CorrectionKind
	// Page is the page the proposal is about, 0 for the whole document.
	Page int
	// CutID and Position, in document units, for CorrectionAddCut (no
	// CutID) and CorrectionMoveCut.
	CutID    int
	Position float64
	// Scale (mm per unit), Percent and WidthMM, for CorrectionReduceScale.
	Scale, Percent, WidthMM float64
	// Paper and Orientation, for CorrectionChangePaper.
	Paper       string
	Orientation model.Orientation
	// Resolves tells whether, once applied, every page fits.
	Resolves bool
}

// corrections lists the proposals for the overflowing pages of a plan.
func corrections(p model.Project, pl *Plan, blocks []Block) []Correction {
	var out []Correction
	anyOverflow := false
	maxBand := 0.0
	for _, page := range pl.Pages {
		tall, wide := page.overflowsSize(pl.Sheet)
		if !tall && !wide {
			continue
		}
		anyOverflow = true
		maxBand = max(maxBand, page.Units())
		if !tall {
			continue
		}
		if c, ok := proposeAddCut(p, pl, page, blocks); ok {
			out = append(out, c)
		}
		if c, ok := proposeMoveCut(p, pl, page, blocks); ok {
			out = append(out, c)
		}
	}
	if !anyOverflow {
		return nil
	}
	if c, ok := proposeReduceScale(p, pl, maxBand); ok {
		out = append(out, c)
	}
	if c, ok := proposeChangePaper(p, pl); ok {
		out = append(out, c)
	}
	return out
}

// fits reports whether no page of the project is larger than its sheet.
func fits(p model.Project, doc Doc) bool {
	pl, err := build(p, doc, nil)
	if err != nil {
		return false
	}
	for _, page := range pl.Pages {
		if tall, wide := page.overflowsSize(pl.Sheet); tall || wide {
			return false
		}
	}
	return true
}

// maxBandUnits is the tallest band, in units, that the usable height holds.
func maxBandUnits(pl *Plan) float64 { return pl.Sheet.UsableHeight / pl.Scale }

func proposeAddCut(p model.Project, pl *Plan, page Page, blocks []Block) (Correction, bool) {
	hMax := maxBandUnits(pl)
	height := page.Units()
	pieces := max(int(math.Ceil(height/hMax-1e-9)), 2)
	ideal := page.Top + height/float64(pieces)
	lo := page.Top + 0.4*height/float64(pieces)
	hi := page.Top + hMax

	y := ideal
	if blocks != nil {
		best, bestDist := ideal, math.Inf(1)
		found := false
		for _, v := range Voids(blocks, pl.Doc.H) {
			c := v.Center()
			if c < lo || c > hi {
				continue
			}
			if d := abs(c - ideal); d < bestDist {
				best, bestDist, found = c, d, true
			}
		}
		if found {
			y = best
		} else if snapped, ok := SnapToVoid(ideal, Voids(blocks, pl.Doc.H), (hi-lo)/2); ok && snapped >= lo && snapped <= hi {
			y = snapped
		}
	}
	q, _, err := AddCut(p, pl.Doc, y)
	if err != nil {
		return Correction{}, false
	}
	return Correction{Kind: CorrectionAddCut, Page: page.Number, Position: y, Resolves: fits(q, pl.Doc)}, true
}

func proposeMoveCut(p model.Project, pl *Plan, page Page, blocks []Block) (Correction, bool) {
	hMax := maxBandUnits(pl)
	var id int
	var y float64
	switch {
	case page.CutBelow != 0:
		id, y = page.CutBelow, page.Top+hMax
		if blocks != nil {
			y = lastFreePointBefore(y, page.Top+0.4*hMax, Voids(blocks, pl.Doc.H))
		}
	case page.CutAbove != 0:
		id, y = page.CutAbove, page.Bottom-hMax
		if blocks != nil {
			y = firstFreePointAfter(y, page.Bottom-0.4*hMax, Voids(blocks, pl.Doc.H))
		}
	default:
		return Correction{}, false // the only page: there is no cut to move
	}
	q, err := MoveCut(p, pl.Doc, id, y)
	if err != nil {
		return Correction{}, false
	}
	return Correction{Kind: CorrectionMoveCut, Page: page.Number, CutID: id, Position: y, Resolves: fits(q, pl.Doc)}, true
}

// lastFreePointBefore is the greatest position at most y, and at least floor,
// that lies in a void; y itself when there is none.
func lastFreePointBefore(y, floor float64, voids []Interval) float64 {
	best, found := floor, false
	for _, v := range voids {
		inset := min(snapInset, v.Len()/2)
		pt := min(y, v.End-inset)
		if pt >= v.Start+inset-cutEpsilon && pt >= floor && pt > best {
			best, found = pt, true
		}
	}
	if !found {
		return y
	}
	return best
}

// firstFreePointAfter is the smallest position at least y, and at most
// ceiling, that lies in a void; y itself when there is none.
func firstFreePointAfter(y, ceiling float64, voids []Interval) float64 {
	best, found := ceiling, false
	for _, v := range voids {
		inset := min(snapInset, v.Len()/2)
		pt := max(y, v.Start+inset)
		if pt <= v.End-inset+cutEpsilon && pt <= ceiling && pt < best {
			best, found = pt, true
		}
	}
	if !found {
		return y
	}
	return best
}

func proposeReduceScale(p model.Project, pl *Plan, maxBand float64) (Correction, bool) {
	s := pl.Scale
	if maxBand > 0 {
		s = min(s, pl.Sheet.UsableHeight/maxBand)
	}
	s = min(s, pl.Sheet.UsableWidth/pl.Doc.W)
	// The width is rounded down to a tenth of a millimetre: "156 mm" rather
	// than a figure that only a computer could type back.
	width := math.Floor(pl.Doc.W*s*10+1e-6) / 10
	if width <= 0 || width >= pl.ContentWidthMM-units.ToleranceMM {
		return Correction{}, false
	}
	q := p.Clone()
	q.Scale = model.Scale{Mode: model.Width, WidthMM: width}
	s2 := width / pl.Doc.W
	return Correction{
		Kind: CorrectionReduceScale, Scale: s2, Percent: s2 / NativeMMPerUnit * 100, WidthMM: width,
		Resolves: fits(q, pl.Doc),
	}, true
}

func proposeChangePaper(p model.Project, pl *Plan) (Correction, bool) {
	type candidate struct {
		name string
		o    model.Orientation
		area float64
	}
	var all []candidate
	for _, paper := range units.Catalog() {
		for _, o := range []model.Orientation{model.Portrait, model.Landscape} {
			w, h := paper.Oriented(o == model.Landscape)
			all = append(all, candidate{paper.Name, o, w * h})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].area < all[j].area })
	for _, c := range all {
		if c.name == p.Paper.Format && c.o == p.Paper.Orientation {
			continue
		}
		q := p.Clone()
		q.Paper = model.Paper{Format: c.name, Orientation: c.o}
		if fits(q, pl.Doc) {
			return Correction{Kind: CorrectionChangePaper, Paper: c.name, Orientation: c.o, Resolves: true}, true
		}
	}
	return Correction{}, false
}

// Apply carries out a proposal the user chose, and returns the new project.
func Apply(p model.Project, doc Doc, c Correction) (model.Project, error) {
	switch c.Kind {
	case CorrectionAddCut:
		q, _, err := AddCut(p, doc, c.Position)
		return q, err
	case CorrectionMoveCut:
		return MoveCut(p, doc, c.CutID, c.Position)
	case CorrectionReduceScale:
		q := p.Clone()
		q.Scale = model.Scale{Mode: model.Width, WidthMM: c.WidthMM}
		return q, nil
	case CorrectionChangePaper:
		q := p.Clone()
		q.Paper = model.Paper{Format: c.Paper, Orientation: c.Orientation}
		return q, nil
	}
	return p, diag.Fail("correction.none")
}

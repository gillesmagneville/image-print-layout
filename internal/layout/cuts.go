// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"math"
	"sort"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/model"
)

// MinCutGapMM is the smallest physical distance between two cuts, and
// between a cut and an edge of the document.
const MinCutGapMM = 0.5

// cutEpsilon absorbs the rounding of float64 in the comparisons of positions.
const cutEpsilon = 1e-9

// A cut is a position on the vertical axis of the document, in document
// units. The commands below take a project and return a new one; none of them
// reads or writes the scale, which is computed from the paper, the margins and
// the choice of scale only.

// checkStructure verifies what every project must satisfy to have a plan:
// cuts on the vertical axis, strictly inside the document, in strictly
// increasing order. The minimal gap is not part of it: a change of scale can
// make an existing gap shorter than the minimum, and the project must still
// open; only the commands that place a cut apply the gap.
func checkStructure(cuts []model.Cut, doc Doc) error {
	prev := 0.0
	for _, c := range cuts {
		if c.Axis != model.AxisY && c.Axis != "" {
			return diag.Fail("cut.invalid")
		}
		if math.IsNaN(c.Position) || c.Position <= 0 || c.Position >= doc.H {
			return diag.Fail("cut.out_of_range", "min", 0.0, "max", doc.H)
		}
		if c.Position <= prev {
			return diag.Fail("cut.invalid")
		}
		prev = c.Position
	}
	return nil
}

// gapUnits is MinCutGapMM in document units for the scale s.
func gapUnits(s float64) float64 { return MinCutGapMM / s }

// CutBounds gives the stretch in which the cut with that id may be placed:
// between its neighbours (or the edges of the document), each at the minimal
// gap. A drag clamps to it.
func CutBounds(p model.Project, doc Doc, id int) (lo, hi float64, err error) {
	_, s, err := Resolve(p, doc)
	if err != nil {
		return 0, 0, err
	}
	i := cutIndex(p.Cuts, id)
	if i < 0 {
		return 0, 0, diag.Fail("cut.unknown")
	}
	g := gapUnits(s)
	lo, hi = g, doc.H-g
	if i > 0 {
		lo = p.Cuts[i-1].Position + g
	}
	if i+1 < len(p.Cuts) {
		hi = p.Cuts[i+1].Position - g
	}
	return lo, hi, nil
}

func cutIndex(cuts []model.Cut, id int) int {
	for i, c := range cuts {
		if c.ID == id {
			return i
		}
	}
	return -1
}

// pageOf is the number of the page (from 1) that contains position y.
func pageOf(cuts []model.Cut, y float64) int {
	return sort.Search(len(cuts), func(i int) bool { return cuts[i].Position >= y }) + 1
}

// checkPlaceable verifies that a cut at y respects the minimal gap with the
// edges and with every cut except the one with the id skip (0 for none).
func checkPlaceable(cuts []model.Cut, doc Doc, s, y float64, skip int) error {
	g := gapUnits(s)
	if math.IsNaN(y) || y <= 0 || y >= doc.H {
		return diag.Fail("cut.out_of_range", "min", 0.0, "max", doc.H)
	}
	if y < g-cutEpsilon || doc.H-y < g-cutEpsilon {
		return diag.Fail("cut.too_close", "min", MinCutGapMM)
	}
	for _, c := range cuts {
		if c.ID != skip && abs(c.Position-y) < g-cutEpsilon {
			return diag.Fail("cut.too_close", "min", MinCutGapMM)
		}
	}
	return nil
}

func nextID(p model.Project) int {
	id := max(p.NextCutID, 1)
	for _, c := range p.Cuts {
		id = max(id, c.ID+1)
	}
	return id
}

// AddCut adds a cut at y and returns the new project and the id of the cut.
// The page it cuts keeps its placement on both halves.
func AddCut(p model.Project, doc Doc, y float64) (model.Project, int, error) {
	_, s, err := Resolve(p, doc)
	if err != nil {
		return p, 0, err
	}
	if err := checkPlaceable(p.Cuts, doc, s, y, 0); err != nil {
		return p, 0, err
	}
	q := p.Clone()
	id := nextID(p)
	at := pageOf(q.Cuts, y) // the page the cut falls in, before the cut exists
	i := at - 1
	q.Cuts = append(q.Cuts, model.Cut{})
	copy(q.Cuts[i+1:], q.Cuts[i:])
	q.Cuts[i] = model.Cut{ID: id, Axis: model.AxisY, Position: y}
	q.NextCutID = id + 1
	q.PageOverrides = overridesAfterAdd(q.PageOverrides, at)
	return q, id, nil
}

// MoveCut moves the cut with that id to y.
func MoveCut(p model.Project, doc Doc, id int, y float64) (model.Project, error) {
	_, s, err := Resolve(p, doc)
	if err != nil {
		return p, err
	}
	i := cutIndex(p.Cuts, id)
	if i < 0 {
		return p, diag.Fail("cut.unknown")
	}
	if err := checkPlaceable(p.Cuts, doc, s, y, id); err != nil {
		return p, err
	}
	// The order must not change: a cut does not jump over another.
	if (i > 0 && y <= p.Cuts[i-1].Position) || (i+1 < len(p.Cuts) && y >= p.Cuts[i+1].Position) {
		return p, diag.Fail("cut.too_close", "min", MinCutGapMM)
	}
	q := p.Clone()
	q.Cuts[i].Position = y
	return q, nil
}

// DeleteCut removes the cut with that id. The page above it keeps its
// placement for the page the two halves become.
func DeleteCut(p model.Project, id int) (model.Project, error) {
	i := cutIndex(p.Cuts, id)
	if i < 0 {
		return p, diag.Fail("cut.unknown")
	}
	q := p.Clone()
	q.Cuts = append(q.Cuts[:i], q.Cuts[i+1:]...)
	if len(q.Cuts) == 0 {
		q.Cuts = nil
	}
	q.PageOverrides = overridesAfterDelete(q.PageOverrides, i+1)
	return q, nil
}

// SetCuts replaces all the cuts by the given positions (an automatic
// proposal, for instance). The placements of single pages are dropped: the
// pages they belonged to no longer exist.
func SetCuts(p model.Project, doc Doc, positions []float64) (model.Project, error) {
	_, s, err := Resolve(p, doc)
	if err != nil {
		return p, err
	}
	sorted := append([]float64(nil), positions...)
	sort.Float64s(sorted)
	q := p.Clone()
	q.Cuts = nil
	q.PageOverrides = nil
	id := nextID(p)
	for _, y := range sorted {
		if err := checkPlaceable(q.Cuts, doc, s, y, 0); err != nil {
			return p, err
		}
		q.Cuts = append(q.Cuts, model.Cut{ID: id, Axis: model.AxisY, Position: y})
		id++
	}
	q.NextCutID = id
	return q, nil
}

// overridesAfterAdd renumbers the overrides when page `at` is cut in two: the
// pages after it move down by one, and both halves take the override of the
// page that was cut.
func overridesAfterAdd(ovs []model.PageOverride, at int) []model.PageOverride {
	var out []model.PageOverride
	for _, o := range ovs {
		switch {
		case o.Page < at:
			out = append(out, o)
		case o.Page == at:
			out = append(out, o)
			second := o
			second.Page = at + 1
			second.OffsetXMM, second.OffsetYMM = copyFloat(o.OffsetXMM), copyFloat(o.OffsetYMM)
			out = append(out, second)
		default:
			o.Page++
			out = append(out, o)
		}
	}
	return out
}

// overridesAfterDelete renumbers the overrides when pages `upper` and
// `upper`+1 become one: the page above keeps its override, the one below
// loses it, and the following pages move up by one.
func overridesAfterDelete(ovs []model.PageOverride, upper int) []model.PageOverride {
	var out []model.PageOverride
	for _, o := range ovs {
		switch {
		case o.Page <= upper:
			out = append(out, o)
		case o.Page == upper+1:
			// dropped
		default:
			o.Page--
			out = append(out, o)
		}
	}
	return out
}

func copyFloat(f *float64) *float64 {
	if f == nil {
		return nil
	}
	v := *f
	return &v
}

// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"math"
	"sort"

	"github.com/gillesmagneville/svg-print-layout/internal/diag"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
	"github.com/gillesmagneville/svg-print-layout/internal/units"
)

// NearMM is the distance, in millimetres on the sheet, under which a cut is
// said to be near a block: closer than that, the cut is flagged.
const NearMM = 2.0

// minBandShare is the smallest share of the ideal band that a page of an
// automatic proposal may have.
const minBandShare = 0.4

// Proposal is a set of cuts the program suggests. It is applied by the user,
// with SetCuts, and each cut can then be moved by hand.
type Proposal struct {
	// Positions are the cuts, in document units, in increasing order.
	Positions []float64
	// Pages is the number of pages they make.
	Pages int
	// Crossings is the number of (cut, block) pairs where the cut crosses the
	// block.
	Crossings int
	Items     []diag.Item
}

// cost compares candidate solutions: first the blocks crossed, then the cuts
// too near a block, then the balance of the pages.
type cost struct {
	cross, near int
	balance     float64
}

func (c cost) plus(o cost) cost {
	return cost{c.cross + o.cross, c.near + o.near, c.balance + o.balance}
}

func (c cost) less(o cost) bool {
	if c.cross != o.cross {
		return c.cross < o.cross
	}
	if c.near != o.near {
		return c.near < o.near
	}
	return c.balance < o.balance-1e-12
}

type candidate struct {
	y     float64
	cross int
	near  bool
}

// AutoCuts proposes cuts for a project (ARCHITECTURE §7): the fewest pages
// the usable height allows, or up to two more when that makes the cuts cross
// fewer blocks; cuts in the voids between blocks when there are some, far
// from the nearest block; pages as balanced as the voids allow. The cuts the
// project already has are ignored. blocks may be nil: the cuts are then
// evenly spaced.
func AutoCuts(p model.Project, doc Doc, blocks []Block) (Proposal, error) {
	sheet, s, err := Resolve(p, doc)
	if err != nil {
		return Proposal{}, err
	}
	nMin := ceilPages(doc.H*s, sheet.UsableHeight)
	if nMin == 1 {
		return Proposal{Pages: 1, Items: []diag.Item{diag.New("auto.no_cut_needed", 0)}}, nil
	}

	hMax := (sheet.UsableHeight + units.ToleranceMM/2) / s
	g := gapUnits(s)
	voids := Voids(blocks, doc.H)

	var best *solution
	for n := nMin; n <= nMin+2; n++ {
		sol := solve(n, doc.H, hMax, g, s, blocks, voids)
		if sol == nil {
			continue
		}
		if best == nil || sol.better(best) {
			best = sol
		}
	}
	if best == nil {
		// Cannot happen when a band can hold the evenly spaced cuts, which
		// nMin guarantees; kept so that a rounding accident never leaves the
		// user without a proposal.
		best = evenly(nMin, doc.H, g, s, blocks)
	}

	prop := Proposal{Positions: best.positions, Pages: best.n, Crossings: best.cost.cross}
	if best.n > nMin {
		prop.Items = append(prop.Items, diag.New("auto.more_pages", 0, "count", best.n, "min", nMin))
	}
	for i, c := range best.cands {
		if c.cross > 0 {
			prop.Items = append(prop.Items, diag.New("auto.cut_crosses", 0, "n", i+1, "count", c.cross))
		} else if c.near {
			prop.Items = append(prop.Items, diag.New("auto.cut_near", 0, "n", i+1, "mm", distanceToBlocks(c.y, blocks)*s))
		}
	}
	return prop, nil
}

type solution struct {
	n         int
	positions []float64
	cands     []candidate
	cost      cost
}

// better: fewer blocks crossed, then fewer cuts near a block, then fewer pages,
// then the more balanced pages (balance is the mean over the pages, so that
// solutions of different lengths compare).
func (a *solution) better(b *solution) bool {
	if a.cost.cross != b.cost.cross {
		return a.cost.cross < b.cost.cross
	}
	if a.cost.near != b.cost.near {
		return a.cost.near < b.cost.near
	}
	if a.n != b.n {
		return a.n < b.n
	}
	return a.cost.balance < b.cost.balance-1e-12
}

// solve finds the best n-page split by dynamic programming over the candidate
// positions: the state is the last cut, and a transition adds one page whose
// height must be between the minimal share of the ideal and the usable height.
func solve(n int, docH, hMax, g, s float64, blocks []Block, voids []Interval) *solution {
	ideal := docH / float64(n)
	minBand := max(minBandShare*ideal, g)
	cands := candidates(n, docH, g, s, blocks, voids)
	if len(cands) == 0 {
		return nil
	}

	const none = -1
	type cell struct {
		c    cost
		prev int
		ok   bool
	}
	layers := make([][]cell, n) // layers[k] is the cell of the cut k (1..n-1)
	bandCost := func(h float64) cost {
		d := (h - ideal) / ideal
		return cost{balance: d * d}
	}
	candCost := func(c candidate) cost {
		cs := cost{cross: c.cross}
		if c.near {
			cs.near = 1
		}
		return cs
	}

	for k := 1; k < n; k++ {
		layers[k] = make([]cell, len(cands))
		for i, c := range cands {
			if k == 1 {
				if c.y <= hMax && c.y >= minBand {
					layers[k][i] = cell{candCost(c).plus(bandCost(c.y)), none, true}
				}
				continue
			}
			lo := sort.Search(len(cands), func(j int) bool { return cands[j].y >= c.y-hMax })
			for j := lo; j < len(cands) && cands[j].y <= c.y-minBand; j++ {
				prev := layers[k-1][j]
				if !prev.ok {
					continue
				}
				total := prev.c.plus(candCost(c)).plus(bandCost(c.y - cands[j].y))
				if cur := layers[k][i]; !cur.ok || total.less(cur.c) {
					layers[k][i] = cell{total, j, true}
				}
			}
		}
	}

	// The last page closes the document.
	bestI, bestC, found := none, cost{}, false
	for i, c := range cands {
		cell := layers[n-1][i]
		if !cell.ok {
			continue
		}
		h := docH - c.y
		if h > hMax || h < minBand {
			continue
		}
		total := cell.c.plus(bandCost(h))
		if !found || total.less(bestC) {
			bestI, bestC, found = i, total, true
		}
	}
	if !found {
		return nil
	}
	sol := &solution{n: n, cost: bestC}
	sol.cost.balance /= float64(n)
	idx := bestI
	for k := n - 1; k >= 1; k-- {
		sol.cands = append([]candidate{cands[idx]}, sol.cands...)
		sol.positions = append([]float64{cands[idx].y}, sol.positions...)
		idx = layers[k][idx].prev
	}
	return sol
}

// candidates are the positions the search considers: the centre of every void
// (as far as possible from the blocks around it), the evenly spaced positions
// of an n-page split, and, near each of those, the closest points that lie in
// a void.
func candidates(n int, docH, g, s float64, blocks []Block, voids []Interval) []candidate {
	var ys []float64
	for _, v := range voids {
		ys = append(ys, v.Center())
	}
	ideal := docH / float64(n)
	for j := 1; j < n; j++ {
		y := float64(j) * ideal
		ys = append(ys, y)
		ys = append(ys, lastFreePointBefore(y, y-0.35*ideal, voids), firstFreePointAfter(y, y+0.35*ideal, voids))
	}
	sort.Float64s(ys)

	near := NearMM / s
	var out []candidate
	for _, y := range ys {
		if y < g || y > docH-g {
			continue
		}
		if len(out) > 0 && y-out[len(out)-1].y < 1e-6 {
			continue
		}
		c := candidate{y: y, cross: len(Crossed(y, blocks))}
		if c.cross == 0 && len(blocks) > 0 {
			c.near = distanceToBlocks(y, blocks) < near
		}
		out = append(out, c)
	}
	return out
}

// distanceToBlocks is the distance from y to the nearest block, 0 inside one.
func distanceToBlocks(y float64, blocks []Block) float64 {
	d := math.Inf(1)
	for _, b := range blocks {
		switch {
		case y < b.Top:
			d = min(d, b.Top-y)
		case y > b.Bottom:
			d = min(d, y-b.Bottom)
		default:
			return 0
		}
	}
	return d
}

// evenly is the split into n equal pages, whatever they cross.
func evenly(n int, docH, g, s float64, blocks []Block) *solution {
	sol := &solution{n: n}
	near := NearMM / s
	for j := 1; j < n; j++ {
		y := float64(j) * docH / float64(n)
		c := candidate{y: y, cross: len(Crossed(y, blocks))}
		if c.cross == 0 && len(blocks) > 0 {
			c.near = distanceToBlocks(y, blocks) < near
		}
		sol.cands = append(sol.cands, c)
		sol.positions = append(sol.positions, y)
		sol.cost.cross += c.cross
	}
	return sol
}

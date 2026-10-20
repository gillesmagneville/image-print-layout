// SPDX-License-Identifier: GPL-3.0-only

package layout

import "sort"

// Block is the vertical extent, in document units, of a block of the
// drawing: a group that carries visible content, as the engine measured it.
// The layout reads nothing else of the drawing.
type Block struct {
	ID          string
	Top, Bottom float64
}

// CrossTolerance is how far, in document units, a cut may enter a block
// before it is said to cross it (ARCHITECTURE §7).
const CrossTolerance = 0.05

// Interval is a stretch of the vertical axis, in document units.
type Interval struct {
	Start, End float64
}

// Len is the length of the interval.
func (i Interval) Len() float64 { return i.End - i.Start }

// Center is the middle of the interval.
func (i Interval) Center() float64 { return (i.Start + i.End) / 2 }

// Crossed lists the blocks that a cut at y crosses: those that contain y
// strictly inside, tolerance included. The order is that of blocks.
func Crossed(y float64, blocks []Block) []string {
	var ids []string
	for _, b := range blocks {
		if y > b.Top+CrossTolerance && y < b.Bottom-CrossTolerance {
			ids = append(ids, b.ID)
		}
	}
	return ids
}

// Voids returns the stretches of [0, docH] that no block touches: the union
// of the intervals of the blocks, complemented. A cut in a void crosses
// nothing. Stretches too short to hold a cut (twice the tolerance) are left
// out. The result is in increasing order.
func Voids(blocks []Block, docH float64) []Interval {
	spans := make([]Interval, 0, len(blocks))
	for _, b := range blocks {
		top, bottom := max(b.Top, 0), min(b.Bottom, docH)
		if bottom > top {
			spans = append(spans, Interval{top, bottom})
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].Start < spans[j].Start })

	var voids []Interval
	add := func(start, end float64) {
		if end-start > 2*CrossTolerance {
			voids = append(voids, Interval{start, end})
		}
	}
	reached := 0.0
	for _, sp := range spans {
		if sp.Start > reached {
			add(reached, sp.Start)
		}
		reached = max(reached, sp.End)
	}
	if reached < docH {
		add(reached, docH)
	}
	return voids
}

// snapInset is how far, in document units, inside a void a snapped cut sits:
// not on the very edge of the neighbouring block.
const snapInset = 1.0

// SnapToVoid moves y to the nearest position that lies in a void, when one is
// at most radius units away (the radius is 8 screen pixels, converted to
// units by the caller). A position already inside a void is kept. The second
// result is false when no void is that close, and y comes back unchanged.
func SnapToVoid(y float64, voids []Interval, radius float64) (float64, bool) {
	best, found := y, false
	bestDist := radius + 1
	for _, v := range voids {
		inset := min(snapInset, v.Len()/2)
		target := min(max(y, v.Start+inset), v.End-inset)
		if d := abs(target - y); d <= radius && d < bestDist {
			best, bestDist, found = target, d, true
		}
	}
	return best, found
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

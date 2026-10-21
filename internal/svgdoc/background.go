// SPDX-License-Identifier: GPL-3.0-only

package svgdoc

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/gillesmagneville/pageslice/internal/model"
	"github.com/gillesmagneville/pageslice/internal/units"
)

// edit replaces the bytes [start, end) of the source by text.
type edit struct {
	start, end int
	text       string
}

// apply returns the source with the edits applied. The edits must not overlap;
// they are applied in the order of their position, so the result does not
// depend on the order they are given in.
func apply(src []byte, edits []edit) ([]byte, error) {
	sorted := append([]edit(nil), edits...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].start != sorted[j].start {
			return sorted[i].start < sorted[j].start
		}
		return sorted[i].end < sorted[j].end
	})
	out := make([]byte, 0, len(src)+64*len(edits))
	at := 0
	for _, e := range sorted {
		if e.start < at || e.end < e.start || e.end > len(src) {
			return nil, fmt.Errorf("svgdoc: overlapping or invalid edit [%d, %d) after %d", e.start, e.end, at)
		}
		out = append(out, src[at:e.start]...)
		out = append(out, e.text...)
		at = e.end
	}
	out = append(out, src[at:]...)
	return out, nil
}

// bgTolerance is how far, in user units, the rectangle may fall short of an
// edge of the viewBox and still cover the document (ARCHITECTURE §8).
const bgTolerance = 0.5

// background tests an element as the background rectangle: a rect, or a path
// of four orthogonal segments, whose box covers the viewBox, that is filled,
// and that is the first to be painted among such elements. Called for elements
// that are drawn, in document order, until one is found.
func (r *report) background(e *element, p props) {
	if (e.Local != "rect" && e.Local != "path") || r.vb[2] <= 0 {
		return
	}
	fill := e.fill
	if strings.EqualFold(fill, "none") || p.visibility == "hidden" || p.fillOpacity == "0" {
		return
	}
	var minX, minY, maxX, maxY float64
	var ok bool
	switch e.Local {
	case "rect":
		minX, minY, maxX, maxY, ok = rectBox(e, r.vb)
	case "path":
		d, _ := e.attr("d")
		minX, minY, maxX, maxY, ok = orthogonalBox(d)
	}
	if !ok {
		return
	}
	// The box in the user space of the root, whatever the transformations of
	// the ancestors.
	x0, y0 := math.Inf(1), math.Inf(1)
	x1, y1 := math.Inf(-1), math.Inf(-1)
	for _, c := range [4][2]float64{{minX, minY}, {maxX, minY}, {maxX, maxY}, {minX, maxY}} {
		x, y := e.m.apply(c[0], c[1])
		x0, y0, x1, y1 = math.Min(x0, x), math.Min(y0, y), math.Max(x1, x), math.Max(y1, y)
	}
	vb := r.vb
	if x0 > vb[0]+bgTolerance || y0 > vb[1]+bgTolerance || x1 < vb[0]+vb[2]-bgTolerance || y1 < vb[1]+vb[3]-bgTolerance {
		return
	}

	r.foundBG = true
	bg := &Background{
		Ref: refOf(e), Element: e.Local, Fill: fill,
		start: e.Start, nameEnd: e.NameEnd, tagEnd: e.TagEnd,
		styleVal: [2]int{-1, -1},
	}
	for _, a := range e.Attrs {
		if a.Name == "style" {
			bg.styleVal = [2]int{e.Start + a.ValStart, e.Start + a.ValEnd}
		}
	}
	r.a.Background = bg
	r.bgAncestors = map[int]bool{}
	for n := e.Parent; n != nil; n = n.Parent {
		r.bgAncestors[n.Start] = true
	}
	r.bgStart = e.Start
}

// rectBox reads the box of a rect element, in its own user space.
func rectBox(e *element, vb [4]float64) (minX, minY, maxX, maxY float64, ok bool) {
	get := func(name string, ref float64, def float64) (float64, bool) {
		s, present := e.attr(name)
		if !present {
			return def, true
		}
		l, err := units.ParseLength(s)
		if err != nil {
			return 0, false
		}
		if l.Unit == units.UnitPercent {
			return l.Value / 100 * ref, true
		}
		v, err := l.ToPx()
		return v, err == nil
	}
	x, ok1 := get("x", vb[2], 0)
	y, ok2 := get("y", vb[3], 0)
	w, ok3 := get("width", vb[2], 0)
	h, ok4 := get("height", vb[3], 0)
	if !ok1 || !ok2 || !ok3 || !ok4 || w <= 0 || h <= 0 {
		return 0, 0, 0, 0, false
	}
	return x, y, x + w, y + h, true
}

// orthogonalBox reads a path made only of horizontal and vertical segments
// that closes into a rectangle (M L H V Z, absolute or relative) and returns
// its box.
func orthogonalBox(d string) (minX, minY, maxX, maxY float64, ok bool) {
	type pt struct{ x, y float64 }
	var pts []pt
	var cur, start pt
	cmd := byte(0)
	i := 0
	number := func() (float64, bool) {
		for i < len(d) && (isSpace(d[i]) || d[i] == ',') {
			i++
		}
		j := i
		if j < len(d) && (d[j] == '+' || d[j] == '-') {
			j++
		}
		digits := 0
		for j < len(d) && d[j] >= '0' && d[j] <= '9' {
			j++
			digits++
		}
		if j < len(d) && d[j] == '.' {
			j++
			for j < len(d) && d[j] >= '0' && d[j] <= '9' {
				j++
				digits++
			}
		}
		if digits == 0 {
			return 0, false
		}
		if j < len(d) && (d[j] == 'e' || d[j] == 'E') {
			k := j + 1
			if k < len(d) && (d[k] == '+' || d[k] == '-') {
				k++
			}
			if k < len(d) && d[k] >= '0' && d[k] <= '9' {
				for k < len(d) && d[k] >= '0' && d[k] <= '9' {
					k++
				}
				j = k
			}
		}
		v, ok := parseFloat(d[i:j])
		i = j
		return v, ok
	}
	moveTo := func(p pt) {
		pts = append(pts, p)
		cur = p
	}
	for {
		for i < len(d) && (isSpace(d[i]) || d[i] == ',') {
			i++
		}
		if i >= len(d) {
			break
		}
		c := d[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			cmd = c
			i++
			if c == 'z' || c == 'Z' {
				cur = start
				continue
			}
		} else if cmd == 0 || cmd == 'z' || cmd == 'Z' {
			return 0, 0, 0, 0, false
		}
		rel := cmd >= 'a'
		switch cmd {
		case 'M', 'm', 'L', 'l':
			x, okx := number()
			y, oky := number()
			if !okx || !oky {
				return 0, 0, 0, 0, false
			}
			if rel {
				x, y = cur.x+x, cur.y+y
			}
			first := cmd == 'M' || cmd == 'm'
			if first && len(pts) == 0 {
				start = pt{x, y}
			} else if first {
				return 0, 0, 0, 0, false // a second subpath
			}
			moveTo(pt{x, y})
			if cmd == 'M' {
				cmd = 'L'
			} else if cmd == 'm' {
				cmd = 'l'
			}
		case 'H', 'h':
			x, okx := number()
			if !okx {
				return 0, 0, 0, 0, false
			}
			if rel {
				x += cur.x
			}
			moveTo(pt{x, cur.y})
		case 'V', 'v':
			y, oky := number()
			if !oky {
				return 0, 0, 0, 0, false
			}
			if rel {
				y += cur.y
			}
			moveTo(pt{cur.x, y})
		default:
			return 0, 0, 0, 0, false // a curve or an arc: not a rectangle
		}
	}
	// Close the outline back to the start, if it was not drawn to it.
	if len(pts) < 4 {
		return 0, 0, 0, 0, false
	}
	if pts[len(pts)-1] == pts[0] {
		pts = pts[:len(pts)-1]
	}
	if len(pts) != 4 {
		return 0, 0, 0, 0, false
	}
	minX, minY, maxX, maxY = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range pts {
		minX, minY, maxX, maxY = math.Min(minX, p.x), math.Min(minY, p.y), math.Max(maxX, p.x), math.Max(maxY, p.y)
	}
	const eps = 1e-6
	if maxX-minX < eps || maxY-minY < eps {
		return 0, 0, 0, 0, false
	}
	for k, p := range pts {
		q := pts[(k+1)%4]
		if math.Abs(p.x-q.x) > eps && math.Abs(p.y-q.y) > eps {
			return 0, 0, 0, 0, false // a slanted side
		}
		atX := math.Abs(p.x-minX) < eps || math.Abs(p.x-maxX) < eps
		atY := math.Abs(p.y-minY) < eps || math.Abs(p.y-maxY) < eps
		if !atX || !atY {
			return 0, 0, 0, 0, false
		}
	}
	return minX, minY, maxX, maxY, true
}

// Compose builds the working copy of the document: the source with the
// background instruction applied and an identifier on every block that has
// none. The source slice is not modified. The same analysis and the same
// instruction always give the same bytes.
//
// An instruction that names an element other than the detected background
// (the file changed since the project was saved) is not applied; the second
// result tells it, so that the caller can say so.
func (a *Analysis) Compose(bg model.Background) (work []byte, applied bool, err error) {
	edits := append([]edit(nil), a.idEdits...)
	b := a.Background
	if b != nil && bg.Document != model.KeepBackground && bg.Document != "" && (bg.Element == "" || bg.Element == b.Ref) {
		switch bg.Document {
		case model.RemoveBackground:
			// Drop the edits inside the element, which is going away.
			kept := edits[:0]
			for _, e := range edits {
				if e.start < b.start || e.start >= b.end {
					kept = append(kept, e)
				}
			}
			edits = append(kept, edit{b.start, b.end, ""})
			applied = true
		case model.WhitenBackground:
			edits = append(edits, whiten(a.src, b))
			applied = true
		}
	}
	work, err = apply(a.src, edits)
	return work, applied, err
}

const whiteFill = "fill:#ffffff;fill-opacity:1"

// whiten gives the background element an opaque white fill, by the style
// attribute, which wins over presentation attributes and style sheets.
func whiten(src []byte, b *Background) edit {
	if b.styleVal[0] >= 0 {
		old := string(src[b.styleVal[0]:b.styleVal[1]])
		sep := ";"
		if strings.TrimSpace(old) == "" || strings.HasSuffix(strings.TrimSpace(old), ";") {
			sep = ""
		}
		return edit{b.styleVal[1], b.styleVal[1], sep + whiteFill}
	}
	return edit{b.nameEnd, b.nameEnd, ` style="` + whiteFill + `"`}
}

// BackgroundIs tells whether a project's instruction is about the background
// the analysis detected.
func (a *Analysis) BackgroundIs(element string) bool {
	return a.Background != nil && (element == "" || element == a.Background.Ref)
}

func parseFloat(s string) (float64, bool) {
	nums, ok := parseNumbers(s)
	if !ok || len(nums) != 1 {
		return 0, false
	}
	return nums[0], true
}

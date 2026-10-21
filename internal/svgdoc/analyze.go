// SPDX-License-Identifier: GPL-3.0-only

// Package svgdoc reads SVG documents without drawing them: what the document
// declares, what the rendering engine will not do faithfully (filters,
// animations, SVG fonts, text frames...), the fonts it asks for, its
// top-level blocks, and the rectangle that serves as its background. It also
// builds the working copy, in which the background is whitened or removed and
// the blocks carry an identifier to be measured by.
//
// The source is never written: the working copy is a new slice of bytes. The
// text of the document is edited in place (a few bytes spliced in or out), not
// parsed and written again, so that everything the user did not ask to change
// stays as it was written.
//
// Pure Go, no cgo.
package svgdoc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/units"
)

// maxPerCode is how many diagnostics of one code the report lists; the others
// are counted in one line.
const maxPerCode = 25

// Declared is what the root element says about the size of the document.
type Declared struct {
	// WidthPx and HeightPx are the declared size in CSS pixels, 0 when it
	// cannot be told.
	WidthPx, HeightPx float64
	// ViewBox is minX, minY, width, height in user units.
	ViewBox    [4]float64
	HasViewBox bool
}

// Block is a top-level block of the drawing: the unit the cuts try not to
// cross. Its extent is measured by the engine, by ID.
type Block struct {
	// ID is the identifier of the element in the working copy.
	ID string
	// Ref designates the element in the source: its own id, or its path.
	Ref string
	// Label is the name the author gave it (Inkscape's label), or its id.
	Label string
}

// Background is the rectangle that serves as the background of the document.
type Background struct {
	// Ref is the id of the element, or its path of indices when it has none.
	Ref string
	// Element is the name of the element ("rect" or "path").
	Element string
	// Fill is its fill as the document writes it ("#f4f5f8").
	Fill string

	// Where it sits in the source.
	start, nameEnd, tagEnd, end int
	styleVal                    [2]int // the style attribute value, or {-1, -1}
}

// Analysis is what is known of a document without drawing it.
type Analysis struct {
	Declared Declared
	// Items are the findings, in document order.
	Items []diag.Item
	// Fonts are the font-family values the document uses, each once, in the
	// order they appear.
	Fonts []string
	// Blocks are the top-level blocks, in document order.
	Blocks []Block
	// Background is the detected background rectangle, or nil.
	Background *Background

	src     []byte
	idEdits []edit
}

// Analyze reads a document. baseDir is where linked images are looked for
// (the folder of the document); it may be empty. It never fails: a document
// that cannot be read as XML gives a finding, and the rendering engine, which
// is the judge of what can be drawn, reports its own refusal.
func Analyze(src []byte, baseDir string) *Analysis {
	a := &Analysis{src: src}
	r := &report{a: a, baseDir: baseDir, sheet: newStylesheet(), perCode: map[string]int{}, ids: map[string]int{}}
	err := walk(src, r.visit)
	if err != nil {
		a.Items = append(a.Items, diag.New("svg.unreadable", 0, "reason", err.Error()))
	}
	r.finish()
	if err == nil && a.Declared.WidthPx == 0 && !a.Declared.HasViewBox && !r.sawRoot {
		a.Items = append(a.Items, diag.New("svg.no_size", 0))
	}
	if err == nil {
		a.findBlocks(r.sheet, r.ids, r.bgAncestors, r.bgStart)
	}
	return a
}

// report is the state of the first pass.
type report struct {
	a       *Analysis
	baseDir string
	sheet   *stylesheet
	perCode map[string]int
	more    map[string]int
	ids     map[string]int // how many elements carry each id
	fonts   map[string]bool

	sawRoot     bool
	vb          [4]float64
	images      int
	foundBG     bool
	bgAncestors map[int]bool // start offsets of the ancestors of the background
	bgStart     int
}

// add records a finding, within the limit per code.
func (r *report) add(code string, e *element, args ...any) {
	r.perCode[code]++
	if r.perCode[code] > maxPerCode {
		if r.more == nil {
			r.more = map[string]int{}
		}
		r.more[code]++
		return
	}
	item := diag.New(code, 0, args...)
	if e != nil {
		item.Ref = refOf(e)
	}
	r.a.Items = append(r.a.Items, item)
}

// refOf designates an element: its id, or its path of indices.
func refOf(e *element) string {
	if id, ok := e.attr("id"); ok && id != "" {
		return id
	}
	return e.path()
}

var nonRendered = map[string]bool{
	"defs": true, "clipPath": true, "mask": true, "pattern": true, "symbol": true, "marker": true,
	"filter": true, "linearGradient": true, "radialGradient": true, "metadata": true, "style": true,
	"script": true, "title": true, "desc": true, "namedview": true, "font": true, "font-face": true,
}

func (r *report) visit(kind eventKind, e *element, text string) {
	switch kind {
	case evText:
		if e.Local == "style" {
			e.text.WriteString(text)
		}
	case evEnd:
		if b := r.a.Background; b != nil && e.Start == b.start {
			b.end = e.End
		}
		if e.Local == "style" {
			r.sheet.add(e.text.String())
			if r.sheet.webFonts {
				r.sheet.webFonts = false
				r.add("svg.webfont", e)
			}
		}
	case evStart:
		r.start(e)
	}
}

func (r *report) start(e *element) {
	if id, ok := e.attr("id"); ok && id != "" {
		r.ids[id]++
	}
	p := r.sheet.propsOf(e)

	// Inheritance down the tree.
	e.m, e.mKnown, e.fill = identity, true, "black"
	if par := e.Parent; par != nil {
		e.m, e.mKnown, e.fill = par.m, par.mKnown, par.fill
		e.hidden = par.hidden
		e.inactive = par.inactive || nonRendered[par.Local]
	}
	if t, ok := e.attr("transform"); ok {
		if m, valid := parseTransform(t); valid {
			e.m = e.m.mul(m)
		} else {
			e.mKnown = false
		}
	}
	if p.fill != "" {
		e.fill = p.fill
	}
	if p.display == "none" || p.opacity == "0" {
		e.hidden = true
	}
	if nonRendered[e.Local] {
		e.inactive = true
	}

	if e.Parent == nil {
		r.root(e)
		return
	}

	if p.fontFamily != "" {
		r.font(p.fontFamily)
	}
	r.rules(e, p)
	if !r.foundBG && !e.hidden && !e.inactive && e.mKnown {
		r.background(e, p)
	}
}

func (r *report) font(family string) {
	family = strings.TrimSpace(family)
	if family == "" || strings.EqualFold(family, "inherit") {
		return
	}
	if r.fonts == nil {
		r.fonts = map[string]bool{}
	}
	if !r.fonts[family] {
		r.fonts[family] = true
		r.a.Fonts = append(r.a.Fonts, family)
	}
}

// finish adds what is known only at the end: the fonts of the style sheets,
// the images embedded, the findings that were left out.
func (r *report) finish() {
	for _, f := range r.sheet.fontFamilies {
		r.font(f)
	}
	if r.images > 0 {
		r.a.Items = append(r.a.Items, diag.New("svg.images_embedded", 0, "count", r.images))
	}
	codes := make([]string, 0, len(r.more))
	for c := range r.more {
		codes = append(codes, c)
	}
	sortStrings(codes)
	for _, c := range codes {
		r.a.Items = append(r.a.Items, diag.New("svg.more", 0, "count", r.more[c]))
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// root reads the declared size.
func (r *report) root(e *element) {
	r.sawRoot = true
	d := &r.a.Declared
	if vb, ok := e.attr("viewBox"); ok {
		if nums, ok := parseNumbers(vb); ok && len(nums) == 4 && nums[2] > 0 && nums[3] > 0 {
			copy(d.ViewBox[:], nums)
			d.HasViewBox = true
		}
	}
	px := func(name string) float64 {
		s, ok := e.attr(name)
		if !ok {
			return 0
		}
		l, err := units.ParseLength(s)
		if err != nil {
			return 0
		}
		v, err := l.ToPx()
		if err != nil || v <= 0 {
			return 0
		}
		return v
	}
	d.WidthPx, d.HeightPx = px("width"), px("height")
	if d.HasViewBox {
		switch {
		case d.WidthPx == 0 && d.HeightPx == 0:
			d.WidthPx, d.HeightPx = d.ViewBox[2], d.ViewBox[3]
		case d.WidthPx == 0:
			d.WidthPx = d.HeightPx * d.ViewBox[2] / d.ViewBox[3]
		case d.HeightPx == 0:
			d.HeightPx = d.WidthPx * d.ViewBox[3] / d.ViewBox[2]
		}
		r.vb = d.ViewBox
	} else if d.WidthPx > 0 && d.HeightPx > 0 {
		r.vb = [4]float64{0, 0, d.WidthPx, d.HeightPx}
	}
	if d.WidthPx == 0 || d.HeightPx == 0 {
		r.add("svg.no_size", e)
	}
}

// rules applies the catalogue of findings (ARCHITECTURE §12) to an element.
func (r *report) rules(e *element, p props) {
	if p.filter != "" && p.filter != "none" {
		r.add("svg.filter", e)
	}
	switch e.Local {
	case "script":
		r.add("svg.script", e)
	case "animate", "animateTransform", "animateMotion", "animateColor", "set":
		r.add("svg.animation", e)
	case "font", "glyph", "missing-glyph":
		if e.Local == "font" {
			r.add("svg.svgfont", e)
		}
	case "foreignObject":
		r.add("svg.foreign_object", e)
	case "flowRoot":
		r.add("svg.flow_root", e)
	case "image":
		r.image(e)
	case "use":
		if href := hrefOf(e); href != "" && !strings.HasPrefix(href, "#") {
			r.add("svg.use_external", e, "href", href)
		}
	}
}

func hrefOf(e *element) string {
	for _, a := range e.Attrs {
		if a.Local == "href" {
			return strings.TrimSpace(a.Value)
		}
	}
	return ""
}

// image classifies the link of an image element.
func (r *report) image(e *element) {
	href := hrefOf(e)
	switch {
	case href == "":
	case strings.HasPrefix(href, "data:"):
		r.images++
	case isRemote(href):
		r.add("svg.image_remote", e, "url", href)
	default:
		path := strings.TrimPrefix(href, "file://")
		path = filepath.FromSlash(path)
		if !filepath.IsAbs(path) {
			if r.baseDir == "" {
				return // nothing to check against
			}
			path = filepath.Join(r.baseDir, path)
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			r.add("svg.image_missing", e, "path", href)
		}
	}
}

func isRemote(href string) bool {
	i := strings.Index(href, ":")
	if i < 2 { // a drive letter ("C:") is not a scheme
		return false
	}
	scheme := strings.ToLower(href[:i])
	return scheme == "http" || scheme == "https" || scheme == "ftp" || strings.Contains(href, "://") && scheme != "file"
}

// String describes the analysis for a test failure.
func (a *Analysis) String() string {
	return fmt.Sprintf("analysis{%d items, %d fonts, %d blocks, background %v}", len(a.Items), len(a.Fonts), len(a.Blocks), a.Background != nil)
}

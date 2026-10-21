// SPDX-License-Identifier: GPL-3.0-only

package svgdoc

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/model"
	"github.com/gillesmagneville/image-print-layout/internal/testsvg"
)

func codes(items []diag.Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Code)
	}
	return out
}

func has(items []diag.Item, code string) bool {
	for _, it := range items {
		if it.Code == code {
			return true
		}
	}
	return false
}

func wellFormed(t *testing.T, data []byte) {
	t.Helper()
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		if _, err := d.Token(); err == io.EOF {
			return
		} else if err != nil {
			t.Fatalf("not well-formed: %v\n%s", err, data)
		}
	}
}

func TestMiniCADEquivalent(t *testing.T) {
	src := testsvg.Generate(testsvg.Options{})
	a := Analyze(src, "")

	if a.Declared.WidthPx != 1280 || a.Declared.HeightPx != 5479 || !a.Declared.HasViewBox {
		t.Errorf("declared = %+v", a.Declared)
	}
	if len(a.Items) != 0 {
		t.Errorf("findings on a clean document: %v", a.Items)
	}
	if a.Background == nil || a.Background.Ref != testsvg.BackgroundID || a.Background.Fill != testsvg.BackgroundFill || a.Background.Element != "rect" {
		t.Fatalf("background = %+v", a.Background)
	}
	if len(a.Fonts) == 0 || a.Fonts[0] != "IBM Plex Sans, Arial, Helvetica, sans-serif" {
		t.Errorf("fonts = %q", a.Fonts)
	}
	// The background rectangle is not a block, nor its layer.
	for _, b := range a.Blocks {
		if b.Ref == testsvg.BackgroundID || b.ID == "fond" {
			t.Errorf("the background is a block: %+v", b)
		}
	}
	// The blocks of the sections are there, with their own ids.
	ids := map[string]bool{}
	for _, b := range a.Blocks {
		ids[b.ID] = true
	}
	for _, want := range []string{testsvg.BannerID, testsvg.BlockID(1, 1), testsvg.BlockID(6, 6), testsvg.BlockID(3, 4), "transforme"} {
		if !ids[want] {
			t.Errorf("block %s is missing from %d blocks", want, len(a.Blocks))
		}
	}
	// Every block has an id of its own.
	if len(ids) != len(a.Blocks) {
		t.Errorf("%d blocks share %d ids", len(a.Blocks), len(ids))
	}
	// Titles and the footer had no id: they got one in the working copy.
	work, applied, err := a.Compose(model.Background{Document: model.KeepBackground, Page: model.WhitePage})
	if err != nil || applied {
		t.Fatalf("Compose: applied %v, %v", applied, err)
	}
	wellFormed(t, work)
	if !bytes.Contains(work, []byte(`id="svgpl-b`)) {
		t.Error("no identifier was added to the blocks that had none")
	}
	// The source is untouched.
	if !bytes.Equal(src, testsvg.Generate(testsvg.Options{})) {
		t.Error("the source was modified")
	}
}

func TestWhitenAndRemoveTheBackground(t *testing.T) {
	src := testsvg.Generate(testsvg.Options{})
	a := Analyze(src, "")

	white, applied, err := a.Compose(model.Background{Document: model.WhitenBackground, Element: a.Background.Ref, Page: model.WhitePage})
	if err != nil || !applied {
		t.Fatalf("whiten: applied %v, %v", applied, err)
	}
	wellFormed(t, white)
	if !bytes.Contains(white, []byte(`style="fill:#f4f5f8;stroke:none;fill:#ffffff;fill-opacity:1"`)) {
		t.Errorf("the style of the background does not end with the white fill:\n%s", grepLine(white, "fond-rect"))
	}
	// Everything else is byte for byte what it was.
	keep, _, _ := a.Compose(model.Background{Document: model.KeepBackground})
	if got := strings.Replace(string(white), ";fill:#ffffff;fill-opacity:1", "", 1); got != string(keep) {
		t.Error("whitening changed more than the background rectangle")
	}

	gone, applied, err := a.Compose(model.Background{Document: model.RemoveBackground, Element: a.Background.Ref, Page: model.WhitePage})
	if err != nil || !applied {
		t.Fatalf("remove: applied %v, %v", applied, err)
	}
	wellFormed(t, gone)
	if bytes.Contains(gone, []byte(testsvg.BackgroundID)) {
		t.Error("the background rectangle is still in the working copy")
	}
	if !bytes.Contains(gone, []byte(testsvg.BannerID)) {
		t.Error("the removal took more than the rectangle")
	}

	// Same instruction, same bytes.
	again, _, _ := a.Compose(model.Background{Document: model.WhitenBackground, Element: a.Background.Ref})
	if !bytes.Equal(white, again) {
		t.Error("two compositions of the same instruction differ")
	}
}

func grepLine(data []byte, needle string) string {
	for _, l := range strings.Split(string(data), "\n") {
		if strings.Contains(l, needle) {
			return l
		}
	}
	return ""
}

// An instruction about another element than the detected background is not
// applied: the document is not the one the project was made for.
func TestAnInstructionForAnotherElementIsNotApplied(t *testing.T) {
	a := Analyze(testsvg.Generate(testsvg.Options{}), "")
	_, applied, err := a.Compose(model.Background{Document: model.RemoveBackground, Element: "rect9999"})
	if err != nil || applied {
		t.Errorf("applied %v, %v", applied, err)
	}
	if a.BackgroundIs("rect9999") || !a.BackgroundIs(testsvg.BackgroundID) || !a.BackgroundIs("") {
		t.Error("BackgroundIs")
	}
}

func TestBackgroundVariants(t *testing.T) {
	for name, tt := range map[string]struct {
		svg  string
		ref  string
		fill string
	}{
		"rect without id": {
			`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50" viewBox="0 0 100 50"><g><rect width="100" height="50" fill="#ddd"/></g><rect x="5" y="5" width="10" height="10"/></svg>`,
			"/svg/g[1]/rect[1]", "#ddd"},
		"path with four segments": {
			`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50" viewBox="0 0 100 50"><path id="p" d="M0,0 H100 V50 H0 Z" style="fill:#eeeeee"/></svg>`,
			"p", "#eeeeee"},
		"relative path": {
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"><path id="p" d="m 0,0 h 100 v 50 h -100 z" fill="red"/></svg>`,
			"p", "red"},
		"percent size": {
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"><rect id="r" width="100%" height="100%" fill="#abc"/></svg>`,
			"r", "#abc"},
		"inside a transformed layer": {
			`<svg xmlns="http://www.w3.org/2000/svg" xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" width="200" height="100" viewBox="0 0 200 100"><g inkscape:groupmode="layer" transform="translate(0,-900)"><rect id="r" x="0" y="900" width="200" height="100" fill="#123456"/></g></svg>`,
			"r", "#123456"},
		"scaled": {
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><g transform="scale(2)"><rect id="r" width="100" height="50" fill="#123456"/></g></svg>`,
			"r", "#123456"},
		"slightly short of the edge": {
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><rect id="r" x="0.4" y="0.4" width="199.4" height="99.4" fill="#123456"/></svg>`,
			"r", "#123456"},
		"fill inherited from the group": {
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><g fill="#abcdef"><rect id="r" width="200" height="100"/></g></svg>`,
			"r", "#abcdef"},
		"fill from the style sheet": {
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><style>.cls-1{fill:#fafafa}</style><rect id="r" class="cls-1" width="200" height="100"/></svg>`,
			"r", "#fafafa"},
		"the first of two candidates": {
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><rect id="first" width="200" height="100" fill="#111"/><rect id="second" width="200" height="100" fill="#222"/></svg>`,
			"first", "#111"},
	} {
		a := Analyze([]byte(tt.svg), "")
		if a.Background == nil {
			t.Errorf("%s: no background found", name)
			continue
		}
		if a.Background.Ref != tt.ref || a.Background.Fill != tt.fill {
			t.Errorf("%s: background %+v, want %s %s", name, a.Background, tt.ref, tt.fill)
		}
	}
}

func TestNoBackground(t *testing.T) {
	for name, svg := range map[string]string{
		"smaller than the page":   `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><rect width="150" height="100" fill="#111"/></svg>`,
		"no fill":                 `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><rect width="200" height="100" fill="none" stroke="red"/></svg>`,
		"fill none by style":      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><rect width="200" height="100" style="fill:none"/></svg>`,
		"hidden":                  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><rect width="200" height="100" style="display:none"/></svg>`,
		"in the defs":             `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><defs><rect width="200" height="100"/></defs></svg>`,
		"in a clip path":          `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><clipPath id="c"><rect width="200" height="100"/></clipPath></svg>`,
		"a circle":                `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><circle cx="100" cy="50" r="500"/></svg>`,
		"a slanted path":          `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><path d="M0,0 L200,10 L200,100 L0,100 Z"/></svg>`,
		"a path with a curve":     `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><path d="M0,0 C 10,0 190,0 200,0 V100 H0 Z"/></svg>`,
		"nothing":                 `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"/>`,
		"invisible fill":          `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><rect width="200" height="100" fill-opacity="0"/></svg>`,
		"an unreadable transform": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><g transform="spin(3)"><rect width="200" height="100"/></g></svg>`,
	} {
		if a := Analyze([]byte(svg), ""); a.Background != nil {
			t.Errorf("%s: found %+v", name, a.Background)
		}
	}
}

func TestComposeOnADocumentWithoutBackgroundChangesNothingButIDs(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><rect x="1" y="1" width="20" height="20"/></svg>`
	a := Analyze([]byte(src), "")
	work, applied, err := a.Compose(model.Background{Document: model.RemoveBackground})
	if err != nil || applied {
		t.Fatalf("applied %v, %v", applied, err)
	}
	if want := strings.Replace(src, `<rect `, `<rect id="svgpl-b1" `, 1); string(work) != want {
		t.Errorf("work = %s", work)
	}
}

func TestWhitenWithoutAStyleAttribute(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><rect id="bg" width="200" height="100" fill="#aaa"/><rect id="x" width="5" height="5"/></svg>`
	a := Analyze([]byte(src), "")
	work, _, err := a.Compose(model.Background{Document: model.WhitenBackground, Element: "bg"})
	if err != nil {
		t.Fatal(err)
	}
	wellFormed(t, work)
	if !strings.Contains(string(work), `<rect style="fill:#ffffff;fill-opacity:1" id="bg"`) {
		t.Errorf("work = %s", work)
	}
}

func TestBlocksOfASimpleDocument(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" viewBox="0 0 200 400">
  <defs><linearGradient id="g"/></defs>
  <metadata/>
  <rect id="bg" width="200" height="400" fill="#eee"/>
  <g id="a"><rect width="10" height="10"/></g>
  <g inkscape:groupmode="layer" inkscape:label="Calque 1" id="layer1">
    <g id="b"/><g id="c" style="display:none"/><text>x</text>
    <g inkscape:groupmode="layer" id="nested"><g id="d"/></g>
  </g>
  <g id="hidden-layer" inkscape:groupmode="layer" style="display:none"><g id="e"/></g>
  <path d="M0 0 L5 5" id="a"/>
</svg>`
	a := Analyze([]byte(src), "")
	var ids []string
	for _, b := range a.Blocks {
		ids = append(ids, b.ID)
	}
	want := []string{"a", "b", "svgpl-b1", "d", "svgpl-b2"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Errorf("blocks = %v, want %v", ids, want)
	}
	// The duplicate id was replaced in the working copy: the first keeps it.
	work, _, err := a.Compose(model.Background{})
	if err != nil {
		t.Fatal(err)
	}
	wellFormed(t, work)
	if strings.Count(string(work), `id="a"`) != 1 {
		t.Errorf("the duplicate id is still there:\n%s", work)
	}
	if a.Blocks[2].Label != "svgpl-b1" {
		t.Errorf("a block without a label takes its id: %+v", a.Blocks[2])
	}
}

func TestLayerLabelIsTheLabelOfABlock(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" viewBox="0 0 10 10">
<g id="grp" inkscape:label="Serveurs"><rect width="1" height="1"/></g></svg>`
	a := Analyze([]byte(src), "")
	if len(a.Blocks) != 1 || a.Blocks[0].Label != "Serveurs" || a.Blocks[0].Ref != "grp" {
		t.Errorf("blocks = %+v", a.Blocks)
	}
}

// A group that holds the background is gone down into; the rest of its content
// is made of blocks.
func TestGroupHoldingTheBackground(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100">
<g id="all"><rect id="bg" width="200" height="100" fill="#ddd"/><g id="one"/><g id="two"/></g></svg>`
	a := Analyze([]byte(src), "")
	var ids []string
	for _, b := range a.Blocks {
		ids = append(ids, b.ID)
	}
	if strings.Join(ids, ",") != "one,two" {
		t.Errorf("blocks = %v", ids)
	}
}

func TestFindings(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 200 100">
<style>@font-face { font-family: "Web"; src: url(w.woff) } .t { font-family: Fira Sans }</style>
<defs><filter id="f"><feGaussianBlur stdDeviation="2"/></filter></defs>
<rect id="blur" width="10" height="10" filter="url(#f)"/>
<rect id="blur2" width="10" height="10" style="filter:url(#f)"/>
<rect id="nofilter" width="10" height="10" filter="none"/>
<script>alert(1)</script>
<animate attributeName="x" from="0" to="1"/>
<set attributeName="y" to="2"/>
<font id="svgfont"><glyph/></font>
<foreignObject id="fo" width="10" height="10"/>
<flowRoot id="flow"/>
<image id="i1" xlink:href="data:image/png;base64,AAAA"/>
<image id="i2" xlink:href="data:image/png;base64,BBBB"/>
<image id="i3" xlink:href="nowhere/missing.png"/>
<image id="i4" href="https://example.org/a.png"/>
<use id="u1" xlink:href="other.svg#x"/>
<use id="u2" xlink:href="#blur"/>
<text id="t1" font-family="Arial, sans-serif">x</text>
<text id="t2" style="font-family:'IBM Plex Sans'">y</text>
<text id="t3" font-family="Arial, sans-serif">z</text>
</svg>`
	a := Analyze([]byte(src), t.TempDir())
	count := map[string]int{}
	for _, it := range a.Items {
		count[it.Code]++
	}
	want := map[string]int{
		"svg.filter": 2, "svg.script": 1, "svg.animation": 2, "svg.svgfont": 1, "svg.foreign_object": 1,
		"svg.flow_root": 1, "svg.images_embedded": 1, "svg.image_missing": 1, "svg.image_remote": 1,
		"svg.use_external": 1, "svg.webfont": 1,
	}
	for code, n := range want {
		if count[code] != n {
			t.Errorf("%s: %d findings, want %d (all: %v)", code, count[code], n, codes(a.Items))
		}
	}
	for code := range count {
		if _, ok := want[code]; !ok {
			t.Errorf("unexpected finding %s", code)
		}
	}
	// A finding names its element, and the embedded images are counted.
	for _, it := range a.Items {
		switch it.Code {
		case "svg.filter":
			if it.Ref != "blur" && it.Ref != "blur2" {
				t.Errorf("filter finding about %q", it.Ref)
			}
		case "svg.images_embedded":
			if it.Args[1] != 2 {
				t.Errorf("embedded images: %v", it.Args)
			}
		case "svg.image_missing":
			if it.Ref != "i3" || it.Args[1] != "nowhere/missing.png" {
				t.Errorf("missing image: %+v", it)
			}
		case "svg.foreign_object":
			if it.Ref != "fo" {
				t.Errorf("foreignObject finding about %q", it.Ref)
			}
		}
	}
	// A web font declared by @font-face is a finding, not a font to look for.
	if len(a.Fonts) != 3 || a.Fonts[2] != "Fira Sans" || a.Fonts[0] != "Arial, sans-serif" || a.Fonts[1] != "'IBM Plex Sans'" {
		t.Errorf("fonts = %q", a.Fonts)
	}
}

func TestLinkedImagesAreLookedForNextToTheDocument(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/pics/a.png")
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><image href="pics/a.png"/><image href="pics/b.png"/></svg>`
	a := Analyze([]byte(src), dir)
	if n := countCode(a.Items, "svg.image_missing"); n != 1 {
		t.Errorf("%d missing images, want 1 (%v)", n, codes(a.Items))
	}
	// Without a folder to look in, nothing is claimed.
	if n := countCode(Analyze([]byte(src), "").Items, "svg.image_missing"); n != 0 {
		t.Errorf("%d missing images without a base folder", n)
	}
}

func TestFindingsAreLimited(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">`)
	for i := 0; i < 100; i++ {
		b.WriteString(`<rect width="1" height="1" filter="url(#f)"/>`)
	}
	b.WriteString(`</svg>`)
	a := Analyze([]byte(b.String()), "")
	if n := countCode(a.Items, "svg.filter"); n != maxPerCode {
		t.Errorf("%d filter findings, want %d", n, maxPerCode)
	}
	var more *diag.Item
	for i := range a.Items {
		if a.Items[i].Code == "svg.more" {
			more = &a.Items[i]
		}
	}
	if more == nil || more.Args[1] != 75 {
		t.Errorf("the rest is not counted: %+v", more)
	}
}

func TestDeclaredSize(t *testing.T) {
	for name, tt := range map[string]struct {
		svg  string
		w, h float64
		vb   bool
	}{
		"pixels":        {`<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="5479" viewBox="0 0 1280 5479"/>`, 1280, 5479, true},
		"millimetres":   {`<svg xmlns="http://www.w3.org/2000/svg" width="210mm" height="297mm" viewBox="0 0 210 297"/>`, 210 * 96 / 25.4, 297 * 96 / 25.4, true},
		"viewBox only":  {`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"/>`, 640, 480, true},
		"width only":    {`<svg xmlns="http://www.w3.org/2000/svg" width="200" viewBox="0 0 100 50"/>`, 200, 100, true},
		"no viewBox":    {`<svg xmlns="http://www.w3.org/2000/svg" width="300" height="100"/>`, 300, 100, false},
		"percent sizes": {`<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%" viewBox="0 0 640 480"/>`, 640, 480, true},
	} {
		a := Analyze([]byte(tt.svg), "")
		d := a.Declared
		if !close(d.WidthPx, tt.w) || !close(d.HeightPx, tt.h) || d.HasViewBox != tt.vb {
			t.Errorf("%s: %+v, want %v x %v", name, d, tt.w, tt.h)
		}
		if has(a.Items, "svg.no_size") {
			t.Errorf("%s: reported without a size", name)
		}
	}
	if a := Analyze([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), ""); !has(a.Items, "svg.no_size") {
		t.Errorf("a document without a size: %v", codes(a.Items))
	}
}

func TestUnreadableDocument(t *testing.T) {
	a := Analyze([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><g></svg>`), "")
	if !has(a.Items, "svg.unreadable") {
		t.Errorf("findings = %v", codes(a.Items))
	}
	if work, _, err := a.Compose(model.Background{}); err != nil || !strings.Contains(string(work), "<g>") {
		t.Errorf("Compose: %v", err)
	}
	if a := Analyze(nil, ""); a == nil {
		t.Error("nil analysis for an empty document")
	}
}

func TestEntitiesAndOddMarkupDoNotStopTheAnalysis(t *testing.T) {
	src := `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd" [<!ENTITY ns_svg "http://www.w3.org/2000/svg">]>
<svg xmlns="&ns_svg;" viewBox="0 0 100 50"><!-- un commentaire --><![CDATA[x]]><rect id="bg" width="100" height="50" fill="#eee"/><g id="g1"><text>é à &amp; &lt;</text></g></svg>`
	a := Analyze([]byte(src), "")
	if a.Background == nil || a.Background.Ref != "bg" || len(a.Blocks) != 1 || a.Blocks[0].ID != "g1" {
		t.Errorf("analysis = %v, background %+v, blocks %+v, findings %v", a, a.Background, a.Blocks, codes(a.Items))
	}
}

func countCode(items []diag.Item, code string) int {
	n := 0
	for _, it := range items {
		if it.Code == code {
			n++
		}
	}
	return n
}

func close(a, b float64) bool {
	d := a - b
	return d < 1e-6 && d > -1e-6
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := mkdirAll(path); err != nil {
		t.Fatal(err)
	}
}

func TestFamilies(t *testing.T) {
	for in, want := range map[string]string{
		`IBM Plex Sans, Arial, Helvetica, sans-serif`: "IBM Plex Sans|Arial|Helvetica|sans-serif",
		`'IBM Plex Sans', "Fira, Code", monospace`:    "IBM Plex Sans|Fira, Code|monospace",
		`  Arial  `: "Arial",
		``:          "",
		`,,`:        "",
	} {
		if got := strings.Join(Families(in), "|"); got != want {
			t.Errorf("Families(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheckFonts(t *testing.T) {
	installed := map[string]string{
		"arial": "Liberation Sans", "dejavu sans": "DejaVu Sans", "sans-serif": "DejaVu Sans", "monospace": "DejaVu Sans Mono",
	}
	match := func(f string) string {
		if got, ok := installed[strings.ToLower(f)]; ok {
			return got
		}
		return "DejaVu Sans"
	}
	stacks := []string{
		"DejaVu Sans, sans-serif",                // installed: nothing to say
		"sans-serif",                             // generic: nothing to say
		"IBM Plex Sans, DejaVu Sans, sans-serif", // the first is absent, the second is there
		"IBM Plex Mono, monospace",               // only the generic family is left
		"Arial, Helvetica",                       // substituted by the system
		"IBM Plex Sans, Arial",                   // already reported
	}
	items := CheckFonts(stacks, match)
	var got []string
	for _, it := range items {
		got = append(got, it.Args[1].(string)+">"+it.Args[3].(string))
	}
	want := "IBM Plex Sans>DejaVu Sans,IBM Plex Mono>DejaVu Sans Mono,Arial>Liberation Sans"
	if strings.Join(got, ",") != want {
		t.Errorf("reports = %v, want %s", got, want)
	}
}

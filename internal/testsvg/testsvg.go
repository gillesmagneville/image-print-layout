// SPDX-License-Identifier: GPL-3.0-only

// Package testsvg generates deterministic SVG documents for tests: a grey
// background layer, a full-width banner, sections made of text blocks
// separated by vertical gaps, and a transformed group with a known
// position. The default size is that of the Mini-CAD schema of the
// acceptance tests (1 280 x 5 479 units).
//
// The same Options always give the same bytes. Pure Go, no cgo.
package testsvg

import (
	"bytes"
	"fmt"
)

// Options select the shape of the generated document. Zero values take the
// defaults noted on each field.
type Options struct {
	Width, Height    float64 // document size in user units; default 1280 x 5479
	Sections         int     // number of section layers; default 6
	BlocksPerSection int     // blocks per section; default 6
	// Filler adds that many small rectangles, for load tests.
	Filler int
}

// Known geometry of the generated document, for tests.
const (
	// BackgroundID is the full-document grey rectangle.
	BackgroundID = "fond-rect"
	// BackgroundFill is its colour.
	BackgroundFill = "#f4f5f8"
	// BannerID is the full-width banner.
	BannerID = "banniere"
	// TransformedID is a rectangle inside a transformed group: its box in
	// document units is TransformedBox.
	TransformedID = "rect-transforme"
)

// TransformedBox is the box of TransformedID in document units: the group
// is translate(100, 200) scale(2) and the rectangle is x=10 y=20 w=30 h=40.
var TransformedBox = [4]float64{120, 240, 60, 80}

// BlockHeight is the height of every generated block, and BlockGap the empty
// space between two consecutive blocks of a section.
const (
	BlockHeight = 60.0
	BlockGap    = 40.0
)

func (o *Options) defaults() {
	if o.Width == 0 {
		o.Width = 1280
	}
	if o.Height == 0 {
		o.Height = 5479
	}
	if o.Sections == 0 {
		o.Sections = 6
	}
	if o.BlocksPerSection == 0 {
		o.BlocksPerSection = 6
	}
}

// BlockID returns the id of block b (1-based) of section s (1-based).
func BlockID(s, b int) string { return fmt.Sprintf("bloc-%d-%d", s, b) }

// BlockTop returns the top, in user units, of block b of section s for the
// given options: blocks are laid out from a fixed top margin, in sections
// of equal height.
func BlockTop(o Options, s, b int) float64 {
	o.defaults()
	sectionH := (o.Height - 200) / float64(o.Sections)
	return 120 + float64(s-1)*sectionH + 30 + float64(b-1)*(BlockHeight+BlockGap)
}

// Generate returns the SVG document.
func Generate(o Options) []byte {
	o.defaults()
	var b bytes.Buffer
	w, h := o.Width, o.Height
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<svg version="1.1" width="%g" height="%g" viewBox="0 0 %g %g"
   xmlns="http://www.w3.org/2000/svg"
   xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape">
  <title>Document de test</title>
  <g id="fond" inkscape:groupmode="layer" inkscape:label="Fond">
    <rect id="%s" x="0" y="0" width="%g" height="%g" style="fill:%s;stroke:none"/>
  </g>
  <g id="entete" inkscape:groupmode="layer" inkscape:label="En-tête">
    <rect id="%s" x="0" y="0" width="%g" height="84" style="fill:#2b3a55;stroke:none"/>
    <text x="24" y="52" font-family="IBM Plex Sans, Arial, Helvetica, sans-serif" font-size="28" fill="#ffffff">Flux réseau — document de test</text>
  </g>
`, w, h, w, h, BackgroundID, w, h, BackgroundFill, BannerID, w)

	sectionH := (h - 200) / float64(o.Sections)
	for s := 1; s <= o.Sections; s++ {
		top := 120 + float64(s-1)*sectionH
		fmt.Fprintf(&b, `  <g id="section-%d" inkscape:groupmode="layer" inkscape:label="Section %d">
    <text x="40" y="%g" font-family="IBM Plex Sans, Arial, Helvetica, sans-serif" font-size="20" font-weight="bold" fill="#222222">%d · Section de test</text>
`, s, s, top+22, s)
		for k := 1; k <= o.BlocksPerSection; k++ {
			y := BlockTop(o, s, k)
			if y+BlockHeight > top+sectionH {
				break
			}
			fmt.Fprintf(&b, `    <g id="%s">
      <rect x="40" y="%g" width="%g" height="%g" style="fill:#ffffff;stroke:#8790a3;stroke-width:1.2"/>
      <text x="56" y="%g" font-family="IBM Plex Sans, Arial, Helvetica, sans-serif" font-size="16" fill="#111111">Bloc %d.%d — équipement é à ç ü</text>
      <text x="56" y="%g" font-family="IBM Plex Mono, DejaVu Sans Mono, monospace" font-size="13" fill="#444444">10.%d.%d.0/24 · VLAN %d</text>
    </g>
`, BlockID(s, k), y, w-80, BlockHeight, y+26, s, k, y+46, s, k, 2800+s*10+k)
		}
		fmt.Fprint(&b, "  </g>\n")
	}

	fmt.Fprintf(&b, `  <g id="transforme" transform="translate(100,200) scale(2)">
    <rect id="%s" x="10" y="20" width="30" height="40" style="fill:#bf5f14;stroke:none"/>
  </g>
`, TransformedID)

	if o.Filler > 0 {
		fmt.Fprint(&b, `  <g id="remplissage" inkscape:groupmode="layer" inkscape:label="Remplissage">
`)
		cols := 100
		for i := 0; i < o.Filler; i++ {
			x := float64(i%cols) * (w / float64(cols))
			y := 90 + float64(i/cols)*4
			if y > h-4 {
				y = h - 4
			}
			fmt.Fprintf(&b, `    <rect x="%.1f" y="%.1f" width="3" height="3" style="fill:#5a6b8c;stroke:none"/>
`, x, y)
		}
		fmt.Fprint(&b, "  </g>\n")
	}

	fmt.Fprintf(&b, `  <g id="pied" inkscape:groupmode="layer" inkscape:label="Pied de page">
    <text x="40" y="%g" font-family="IBM Plex Sans, Arial, Helvetica, sans-serif" font-size="14" fill="#666666">Pied de page du document de test</text>
  </g>
</svg>
`, h-30)
	return b.Bytes()
}

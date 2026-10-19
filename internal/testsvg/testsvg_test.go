// SPDX-License-Identifier: GPL-3.0-only

package testsvg

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestDeterministic(t *testing.T) {
	a, b := Generate(Options{}), Generate(Options{})
	if !bytes.Equal(a, b) {
		t.Fatal("two generations differ")
	}
}

func TestWellFormedWithDefaultSize(t *testing.T) {
	doc := Generate(Options{})
	dec := xml.NewDecoder(bytes.NewReader(doc))
	var ids = map[string]bool{}
	var root xml.StartElement
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("not well-formed XML: %v", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			if root.Name.Local == "" {
				root = se
			}
			for _, a := range se.Attr {
				if a.Name.Local == "id" {
					if ids[a.Value] {
						t.Errorf("duplicate id %q", a.Value)
					}
					ids[a.Value] = true
				}
			}
		}
	}
	attr := map[string]string{}
	for _, a := range root.Attr {
		attr[a.Name.Local] = a.Value
	}
	if attr["width"] != "1280" || attr["height"] != "5479" || attr["viewBox"] != "0 0 1280 5479" {
		t.Errorf("unexpected root size: %v", attr)
	}
	for _, id := range []string{BackgroundID, BannerID, TransformedID, "section-1", "section-6", BlockID(1, 1), "pied"} {
		if !ids[id] {
			t.Errorf("missing id %q", id)
		}
	}
}

func TestBlocksStayInsideTheirSection(t *testing.T) {
	o := Options{}
	o.defaults()
	sectionH := (o.Height - 200) / float64(o.Sections)
	for s := 1; s <= o.Sections; s++ {
		top := 120 + float64(s-1)*sectionH
		for k := 1; k <= o.BlocksPerSection; k++ {
			y := BlockTop(o, s, k)
			if y+BlockHeight > top+sectionH {
				continue // not generated
			}
			if y < top {
				t.Errorf("block %d of section %d starts above its section", k, s)
			}
		}
	}
}

func TestFillerAddsElements(t *testing.T) {
	small := strings.Count(string(Generate(Options{})), "<rect")
	big := strings.Count(string(Generate(Options{Filler: 1000})), "<rect")
	if big-small != 1000 {
		t.Errorf("filler added %d rectangles, want 1000", big-small)
	}
}

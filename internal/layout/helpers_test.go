// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"math"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/model"
	"github.com/gillesmagneville/image-print-layout/internal/testsvg"
)

// miniCAD is the size of the Mini-CAD schema of the acceptance tests.
var miniCAD = Doc{W: 1280, H: 5479}

// acceptance is the project of acceptance tests 2 to 4: A4 portrait, a content
// width of 200 mm, the default margins (10 mm above and below, 5 mm at the
// sides), proportions kept.
func acceptance() model.Project {
	p := model.Default()
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 200}
	return p
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func mustPlan(t *testing.T, p model.Project, doc Doc, blocks []Block) *Plan {
	t.Helper()
	pl, err := Compute(p, doc, blocks)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	return pl
}

func mustAdd(t *testing.T, p model.Project, doc Doc, y float64) (model.Project, int) {
	t.Helper()
	q, id, err := AddCut(p, doc, y)
	if err != nil {
		t.Fatalf("AddCut(%v): %v", y, err)
	}
	return q, id
}

func hasCode(items []diag.Item, code string) bool {
	for _, it := range items {
		if it.Code == code {
			return true
		}
	}
	return false
}

func errCode(err error) string { return diag.CodeOf(err) }

// miniCADBlocks are the boxes of the blocks of the generated Mini-CAD
// equivalent, from the geometry the generator documents: the banner, the
// title of each section, the blocks of each section, the transformed
// rectangle and the footer. The grey background is not a block.
func miniCADBlocks() []Block {
	o := testsvg.Options{}
	blocks := []Block{{ID: testsvg.BannerID, Top: 0, Bottom: 84}}
	const sections, perSection = 6, 6
	sectionH := (miniCAD.H - 200) / sections
	for s := 1; s <= sections; s++ {
		top := 120 + float64(s-1)*sectionH
		blocks = append(blocks, Block{ID: "titre", Top: top + 22 - 20, Bottom: top + 22 + 5})
		for b := 1; b <= perSection; b++ {
			y := testsvg.BlockTop(o, s, b)
			if y+testsvg.BlockHeight > top+sectionH {
				break
			}
			blocks = append(blocks, Block{ID: testsvg.BlockID(s, b), Top: y - 0.6, Bottom: y + testsvg.BlockHeight + 0.6})
		}
	}
	tb := testsvg.TransformedBox
	blocks = append(blocks, Block{ID: testsvg.TransformedID, Top: tb[1], Bottom: tb[1] + tb[3]})
	blocks = append(blocks, Block{ID: "pied", Top: miniCAD.H - 30 - 14, Bottom: miniCAD.H - 30 + 4})
	return blocks
}

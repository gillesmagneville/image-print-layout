// SPDX-License-Identifier: GPL-3.0-only

package svgdoc

import (
	"fmt"
	"strings"
)

// notABlock lists the elements that carry no visible content of their own.
var notABlock = map[string]bool{
	"defs": true, "metadata": true, "title": true, "desc": true, "style": true, "script": true,
	"namedview": true, "linearGradient": true, "radialGradient": true, "pattern": true, "clipPath": true,
	"mask": true, "marker": true, "symbol": true, "filter": true, "font": true, "cursor": true, "view": true,
	"font-face": true, "perspective": true, "RDF": true,
}

// findBlocks finds the top-level blocks of the drawing (ARCHITECTURE §7): the
// groups of the highest level that carry visible content, going down into the
// layers of Inkscape. A group that holds the background rectangle is gone down
// into too, and the rectangle is left out: otherwise every position would
// cross it. Every other graphic element at the top level is a block by
// itself. Each gets an identifier the engine can measure it by.
func (a *Analysis) findBlocks(sheet *stylesheet, ids map[string]int, bgAncestors map[int]bool, bgStart int) {
	taken := ids
	counter := 0
	newID := func() string {
		for {
			counter++
			id := fmt.Sprintf("svgpl-b%d", counter)
			if taken[id] == 0 {
				taken[id]++
				return id
			}
		}
	}
	seen := map[string]bool{}

	_ = walk(a.src, func(kind eventKind, e *element, _ string) {
		if kind == evText {
			return
		}
		if kind == evEnd {
			return
		}
		if e.Parent == nil {
			e.container = true
			return
		}
		if !e.Parent.container {
			return
		}
		if notABlock[e.Local] {
			return
		}
		p := sheet.propsOf(e)
		if p.display == "none" || p.opacity == "0" {
			return
		}
		if mode, ok := e.attr("groupmode"); e.Local == "g" && ((ok && mode == "layer") || bgAncestors[e.Start]) {
			e.container = true
			return
		}
		if e.Start == bgStart && a.Background != nil {
			return
		}

		id, hasID := e.attr("id")
		unique := hasID && id != "" && !seen[id] // of duplicates, the first keeps its id
		if unique {
			seen[id] = true
		} else {
			gen := newID()
			switch {
			case !hasID:
				a.idEdits = append(a.idEdits, edit{e.NameEnd, e.NameEnd, ` id="` + gen + `"`})
			default:
				for _, at := range e.Attrs {
					if at.Name == "id" {
						a.idEdits = append(a.idEdits, edit{e.Start + at.ValStart, e.Start + at.ValEnd, gen})
					}
				}
			}
			id = gen
		}

		label := ""
		for _, at := range e.Attrs {
			if at.Local == "label" && strings.Contains(at.Name, ":") {
				label = at.Value
			}
		}
		if label == "" {
			label = id
		}
		a.Blocks = append(a.Blocks, Block{ID: id, Ref: refOf(e), Label: label})
	})
}

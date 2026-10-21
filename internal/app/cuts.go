// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"fmt"
	"math"
	"strings"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/layout"
	"github.com/gillesmagneville/pageslice/internal/model"
)

// The commands on the cuts that the window gives to the mouse and the
// keyboard. They only change the cuts: none of them reads or writes the scale
// (acceptance tests 6 and 7), and each is one entry of the history, a drag
// included.

// CutView is a cut as the preview and the list of cuts show it.
type CutView struct {
	// ID is the identifier of the cut, stable while the project lives.
	ID int
	// Number is its rank from the top, from 1.
	Number int
	// Position is where it sits on the document, in document units.
	Position float64
	// Crossed is how many blocks of the drawing the cut crosses, and Names the
	// first few of them, by the name their author gave (a block of a document
	// read from an image has none).
	Crossed int
	Names   []string
}

// errNoDocument is the refusal of a command on the cuts while no document is
// open.
var errNoDocument = diag.Fail("app.no_document")

// maxNamed is how many crossed blocks a CutView names.
const maxNamed = 3

// Cuts lists the cuts of the project, from the top.
func (s *Session) Cuts() []CutView {
	out := make([]CutView, 0, len(s.project.Cuts))
	for i, c := range s.project.Cuts {
		v := CutView{ID: c.ID, Number: i + 1, Position: c.Position}
		for _, id := range layout.Crossed(c.Position, s.Blocks()) {
			v.Crossed++
			if name := s.blockName(id); name != "" && len(v.Names) < maxNamed {
				v.Names = append(v.Names, name)
			}
		}
		out = append(out, v)
	}
	return out
}

// blockName is the name of a measured block: its label, when it has one.
func (s *Session) blockName(id string) string {
	if s.doc == nil || s.doc.Analysis == nil {
		return ""
	}
	for _, b := range s.doc.Analysis.Blocks {
		if b.ID == id {
			if b.Label == b.ID && strings.HasPrefix(b.ID, "svgpl-b") {
				return "" // an identifier made for the working copy means nothing to the user
			}
			return b.Label
		}
	}
	return ""
}

// Voids are the stretches of the document where no block is: where a cut
// crosses nothing. They are the same for the life of the document.
func (s *Session) Voids() []layout.Interval {
	if s.doc == nil {
		return nil
	}
	if s.voids == nil {
		s.voids = layout.Voids(s.doc.Blocks, s.doc.Size.H)
		if s.voids == nil {
			s.voids = []layout.Interval{}
		}
	}
	return s.voids
}

// snap moves y to the nearest void within radius units, when snapping is on
// (radius above zero) and a void is that close.
func (s *Session) snap(y, radius float64) float64 {
	if radius <= 0 {
		return y
	}
	if t, ok := layout.SnapToVoid(y, s.Voids(), radius); ok {
		return t
	}
	return y
}

// AddCut adds a cut at y, moved to a void within snapRadius units (0 for no
// snapping). It returns the identifier of the new cut.
func (s *Session) AddCut(y, snapRadius float64) (int, error) {
	if s.doc == nil {
		return 0, errNoDocument
	}
	y = s.snap(y, snapRadius)
	id := 0
	err := s.Apply("undo.add_cut", "", func(p model.Project, d layout.Doc) (model.Project, error) {
		q, n, err := layout.AddCut(p, d, y)
		id = n
		return q, err
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

// DragCut moves a cut as part of a gesture. y is where the pointer is; the cut
// takes the nearest position it may have: moved to a void within snapRadius
// units (0 for no snapping), then kept between its neighbours, so that a drag
// that goes too far stops the line against the next one instead of failing. It
// returns the position taken. The history is not touched until EndDrag.
func (s *Session) DragCut(id int, y, snapRadius float64) (float64, error) {
	if s.doc == nil {
		return 0, errNoDocument
	}
	y = s.snap(y, snapRadius)
	taken := y
	err := s.Drag("undo.move_cut", func(p model.Project, d layout.Doc) (model.Project, error) {
		lo, hi, err := layout.CutBounds(p, d, id)
		if err != nil {
			return p, err
		}
		taken = math.Min(math.Max(y, lo), hi)
		return layout.MoveCut(p, d, id, taken)
	})
	return taken, err
}

// MoveCut puts a cut at a position typed by the user, in document units. A
// position that is not allowed is refused, not corrected: the user typed it.
func (s *Session) MoveCut(id int, y float64) error {
	return s.Apply("undo.move_cut", fmt.Sprintf("cut-%d", id), func(p model.Project, d layout.Doc) (model.Project, error) {
		return layout.MoveCut(p, d, id, y)
	})
}

// NudgeCut moves a cut by delta units (a key of the keyboard), stopping against
// its neighbours. Successive nudges of the same cut are one entry of the
// history.
func (s *Session) NudgeCut(id int, delta float64) error {
	return s.Apply("undo.move_cut", fmt.Sprintf("cut-%d", id), func(p model.Project, d layout.Doc) (model.Project, error) {
		lo, hi, err := layout.CutBounds(p, d, id)
		if err != nil {
			return p, err
		}
		var at float64
		for _, c := range p.Cuts {
			if c.ID == id {
				at = c.Position
			}
		}
		return layout.MoveCut(p, d, id, math.Min(math.Max(at+delta, lo), hi))
	})
}

// DeleteCut removes a cut.
func (s *Session) DeleteCut(id int) error {
	return s.Apply("undo.delete_cut", "", func(p model.Project, _ layout.Doc) (model.Project, error) {
		return layout.DeleteCut(p, id)
	})
}

// AutoCuts replaces the cuts by the proposal of the layout, as one change that
// can be undone, and returns the proposal for what it has to say. The cuts the
// user placed are replaced: the proposal is a starting point, and each cut can
// be moved afterwards.
func (s *Session) AutoCuts() (layout.Proposal, error) {
	var prop layout.Proposal
	if s.doc == nil {
		return prop, errNoDocument
	}
	err := s.Apply("undo.auto_cuts", "", func(p model.Project, d layout.Doc) (model.Project, error) {
		var err error
		if prop, err = layout.AutoCuts(p, d, s.doc.Blocks); err != nil {
			return p, err
		}
		return layout.SetCuts(p, d, prop.Positions)
	})
	return prop, err
}

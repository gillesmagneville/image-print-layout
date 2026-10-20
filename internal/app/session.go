// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"bytes"
	"time"

	"github.com/gillesmagneville/svg-print-layout/internal/diag"
	"github.com/gillesmagneville/svg-print-layout/internal/layout"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
	"github.com/gillesmagneville/svg-print-layout/internal/project"
)

// Command changes a project. It receives the current project and the document
// and returns the new project; it must not modify its argument. An error
// refuses the change and leaves the session as it was.
type Command func(p model.Project, doc layout.Doc) (model.Project, error)

// Edit makes a Command of a function that changes a copy of the project, for
// the settings that need no check of their own: the session checks the result.
func Edit(change func(p *model.Project)) Command {
	return func(p model.Project, _ layout.Doc) (model.Project, error) {
		q := p.Clone()
		change(&q)
		return q, nil
	}
}

// coalesceWindow is how long successive changes of the same setting count as
// one entry of the history: typing 1, 12, 125 in a field is one change.
const coalesceWindow = 2 * time.Second

type entry struct {
	project model.Project
	label   string
	key     string
	at      time.Time
}

// Session is the state of the work in progress: the open document, the
// project, the page plan it gives, and the history. It knows nothing of the
// toolkit and holds no engine document. It is used from one goroutine.
type Session struct {
	// Now is the clock; tests replace it.
	Now func() time.Time

	doc     *Document
	project model.Project
	plan    *layout.Plan

	undo, redo []entry
	drag       *entry
	saved      []byte
	path       string // of the project file, empty while it has none
	generation int
}

// NewSession starts with no document and the default project.
func NewSession() *Session {
	s := &Session{Now: time.Now, project: model.Default()}
	s.saved = s.signature()
	return s
}

// Document is the open document, or nil.
func (s *Session) Document() *Document { return s.doc }

// Project is the current project. It must not be modified: change it with
// Apply.
func (s *Session) Project() model.Project { return s.project }

// Plan is the page plan of the project for the document, or nil without a
// document. A project that cannot make a plan is never accepted, so a plan
// always goes with the project.
func (s *Session) Plan() *layout.Plan { return s.plan }

// Generation changes each time the project or the document does: a view that
// remembers it knows when to draw again.
func (s *Session) Generation() int { return s.generation }

// ProjectPath is the file the project was opened from or saved to, or "".
func (s *Session) ProjectPath() string { return s.path }

// SetProjectPath records where the project was saved. It is a save: the
// project is clean afterwards.
func (s *Session) SetProjectPath(path string) {
	s.path = path
	s.MarkSaved()
}

// Blocks returns the measured blocks of the document, or nil.
func (s *Session) Blocks() []layout.Block {
	if s.doc == nil {
		return nil
	}
	return s.doc.Blocks
}

// docSize is the size the layout works with.
func (s *Session) docSize() layout.Doc {
	if s.doc == nil {
		return layout.Doc{}
	}
	return s.doc.Size
}

// Open makes a document the current one. The settings of the paper, the scale
// and the placement stay: they are the user's way of printing. What belongs to
// the previous drawing does not: its cuts, the pages' own positions and the
// choice about its background. The history starts again, and the project is
// clean.
func (s *Session) Open(d *Document) error {
	p := s.project.Clone()
	p.Cuts, p.PageOverrides, p.NextCutID = nil, nil, 1
	p.Background.Document, p.Background.Element = model.KeepBackground, ""
	p.Source = model.Source{}
	if p.Scale.PortionUnits > d.Size.H {
		p.Scale.PortionUnits = 0
	}
	plan, err := layout.Compute(p, d.Size, d.Blocks)
	if err != nil {
		// The settings of the paper do not suit this document (a width that
		// the sheet cannot take is not an error; an unusable size is).
		p = model.Default()
		plan, err = layout.Compute(p, d.Size, d.Blocks)
		if err != nil {
			return err
		}
	}
	s.doc, s.project, s.plan = d, p, plan
	s.undo, s.redo, s.drag, s.path = nil, nil, nil, ""
	s.saved = s.signature()
	s.generation++
	return nil
}

// Replace puts a project read from a file in place of the current one. The
// history starts again, and the project is clean.
func (s *Session) Replace(p model.Project, path string) error {
	plan, err := s.compute(p)
	if err != nil {
		return err
	}
	s.project, s.plan = p, plan
	s.undo, s.redo, s.drag, s.path = nil, nil, nil, path
	s.saved = s.signature()
	s.generation++
	return nil
}

func (s *Session) compute(p model.Project) (*layout.Plan, error) {
	if s.doc == nil {
		// Nothing to cut yet: only check that the sheet exists.
		_, err := layout.NewSheet(p.Paper, p.Margins)
		return nil, err
	}
	return layout.Compute(p, s.doc.Size, s.doc.Blocks)
}

// Apply carries out a command. key names the setting for the history: changes
// of the same setting in quick succession are one entry. An empty key never
// merges. label is the key of the text the Undo menu shows.
func (s *Session) Apply(label, key string, c Command) error {
	if s.drag != nil {
		s.EndDrag()
	}
	next, err := c(s.project, s.docSize())
	if err != nil {
		return err
	}
	return s.commit(entry{project: s.project, label: label, key: key, at: s.Now()}, next, true)
}

// Drag carries out a command as part of a gesture (moving a cut): the history
// is not touched until EndDrag, which makes the whole gesture one entry.
func (s *Session) Drag(label string, c Command) error {
	next, err := c(s.project, s.docSize())
	if err != nil {
		return err
	}
	if s.drag == nil {
		s.drag = &entry{project: s.project, label: label, at: s.Now()}
	}
	return s.commit(entry{}, next, false)
}

// EndDrag ends the gesture. A gesture that changed nothing leaves no entry.
func (s *Session) EndDrag() {
	d := s.drag
	if d == nil {
		return
	}
	s.drag = nil
	if !sameProject(d.project, s.project) {
		s.undo = append(s.undo, *d)
		s.redo = nil
	}
}

func (s *Session) commit(before entry, next model.Project, record bool) error {
	plan, err := s.compute(next)
	if err != nil {
		return err
	}
	if sameProject(s.project, next) {
		return nil
	}
	if record {
		if n := len(s.undo); n > 0 && before.key != "" && s.undo[n-1].key == before.key && before.at.Sub(s.undo[n-1].at) < coalesceWindow && len(s.redo) == 0 {
			s.undo[n-1].at = before.at
		} else {
			s.undo = append(s.undo, before)
		}
		s.redo = nil
	}
	s.project, s.plan = next, plan
	s.generation++
	return nil
}

// ApplyCorrection carries out a proposal of the plan the user chose.
func (s *Session) ApplyCorrection(c layout.Correction) error {
	return s.Apply("undo.correction", "", func(p model.Project, d layout.Doc) (model.Project, error) {
		return layout.Apply(p, d, c)
	})
}

// CanUndo and CanRedo tell whether there is something to go back to or forward
// to; the labels are the keys of the texts of the menu.
func (s *Session) CanUndo() bool { return len(s.undo) > 0 }
func (s *Session) CanRedo() bool { return len(s.redo) > 0 }

// UndoLabel is the label of the change Undo would take back, or "".
func (s *Session) UndoLabel() string {
	if n := len(s.undo); n > 0 {
		return s.undo[n-1].label
	}
	return ""
}

// RedoLabel is the label of the change Redo would carry out again, or "".
func (s *Session) RedoLabel() string {
	if n := len(s.redo); n > 0 {
		return s.redo[n-1].label
	}
	return ""
}

// Undo takes back the last change. It reports whether there was one.
func (s *Session) Undo() bool {
	s.EndDrag()
	n := len(s.undo)
	if n == 0 {
		return false
	}
	e := s.undo[n-1]
	current := s.project
	if !s.restore(e.project) {
		return false
	}
	s.undo = s.undo[:n-1]
	s.redo = append(s.redo, entry{project: current, label: e.label})
	return true
}

// Redo carries out again the change that Undo took back.
func (s *Session) Redo() bool {
	n := len(s.redo)
	if n == 0 {
		return false
	}
	e := s.redo[n-1]
	current := s.project
	if !s.restore(e.project) {
		return false
	}
	s.redo = s.redo[:n-1]
	s.undo = append(s.undo, entry{project: current, label: e.label})
	return true
}

// restore sets a project of the history. The document does not change while
// there is history, so its plan can always be computed again.
func (s *Session) restore(p model.Project) bool {
	plan, err := s.compute(p)
	if err != nil {
		return false
	}
	s.project, s.plan = p, plan
	s.generation++
	return true
}

// signature is the encoding of the settings that a save writes: two projects
// with the same signature save the same file. The source is left out: it is
// filled when saving.
func (s *Session) signature() []byte {
	p := s.project
	p.Source = model.Source{}
	b, err := project.Encode(p)
	if err != nil {
		return nil
	}
	return b
}

// Dirty tells whether the project has unsaved changes.
func (s *Session) Dirty() bool { return !bytes.Equal(s.saved, s.signature()) }

// MarkSaved records the current project as the saved one.
func (s *Session) MarkSaved() { s.saved = s.signature() }

func sameProject(a, b model.Project) bool {
	a.Source, b.Source = model.Source{}, model.Source{}
	x, errA := project.Encode(a)
	y, errB := project.Encode(b)
	return errA == nil && errB == nil && bytes.Equal(x, y)
}

// Findings returns what the analysis and the fonts have to say about the
// document, by level.
func (s *Session) Findings() []diag.Item {
	if s.doc == nil {
		return nil
	}
	return s.doc.Findings
}

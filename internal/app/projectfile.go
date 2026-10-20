// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"path/filepath"

	"github.com/gillesmagneville/svg-print-layout/internal/diag"
	"github.com/gillesmagneville/svg-print-layout/internal/layout"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
	"github.com/gillesmagneville/svg-print-layout/internal/project"
)

// The project file in the session (ARCHITECTURE §10): saving writes the
// settings with a description of the document, opening reads them back over a
// document that is found, or located by the user when it has moved. The SVG is
// only ever read.

// ProjectFile is a project file that has been read, and the place where its
// document was looked for.
type ProjectFile struct {
	// Path is the absolute path of the project file.
	Path string
	// Project is what the file holds.
	Project model.Project
	// SourcePath is where the SVG document was found: next to the project
	// first, then where it was when the project was saved. It is empty when
	// it is in neither place; the caller then asks the user to locate it and
	// sets it. The settings are kept in the meantime.
	SourcePath string
}

// ReadProject reads a project file and looks for its document.
func ReadProject(path string) (*ProjectFile, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	p, err := project.Load(abs)
	if err != nil {
		return nil, err
	}
	found, _ := project.ResolveSource(abs, p.Source)
	return &ProjectFile{Path: abs, Project: p, SourcePath: found}, nil
}

// SaveProject writes the project of the session to path. The file records
// where the document is (relative to the project file when it can be, absolute
// too), its fingerprint as it was when it was opened, and its size. The write
// is atomic. The session keeps the path, and the project is clean afterwards.
func (s *Session) SaveProject(path string) error {
	if s.doc == nil {
		return errNoDocument
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	p := s.project.Clone()
	p.Source = project.DescribeSource(abs, s.doc.Path, s.doc.SHA256, s.doc.Size.W, s.doc.Size.H)
	if err := project.Save(abs, p); err != nil {
		return err
	}
	s.project.Source = p.Source
	s.SetProjectPath(abs)
	return nil
}

// OutsideCuts lists the cuts of a project that no longer fall inside the
// document it is opened on (a document that has become shorter). They cannot
// be kept: opening the project removes them, once the user has agreed.
func OutsideCuts(p model.Project, d *Document) []int {
	return project.CutsOutside(p, d.Size.H)
}

// OpenProject makes a document and the project read from a file the current
// ones. The document was opened with the background instruction of the project.
// Nothing changes when it fails. It returns what the project no longer matches:
// a document that changed, a size that changed, cuts that were removed, a
// background that is no longer there. None of it blocks: the user decides.
// Cuts outside the document are removed; the caller has asked first (see
// OutsideCuts).
func (s *Session) OpenProject(d *Document, pf *ProjectFile) ([]diag.Item, error) {
	p := pf.Project.Clone()
	items := project.Verify(p, d.SHA256, d.Size.W, d.Size.H)
	for _, id := range project.CutsOutside(p, d.Size.H) {
		var err error
		if p, err = layout.DeleteCut(p, id); err != nil {
			return nil, err
		}
	}
	// The instruction on the background only counts if it finds its element:
	// otherwise the working copy was loaded as it is, and the project must
	// say what the preview shows.
	if bg := p.Background; bg.Document != model.KeepBackground && bg.Document != "" {
		if _, applied, err := d.Analysis.Compose(bg); err != nil || !applied {
			p.Background.Document, p.Background.Element = model.KeepBackground, ""
			items = append(items, diag.New("project.background_lost", 0))
		}
	}
	if p.Scale.PortionUnits > d.Size.H {
		p.Scale.PortionUnits = 0
	}
	plan, err := layout.Compute(p, d.Size, d.Blocks)
	if err != nil {
		return nil, err
	}
	// The saved state is the file as it was read: what was removed or reset
	// above is a change the user can save.
	file := pf.Project
	file.Source = model.Source{}
	saved, _ := project.Encode(file)
	s.doc, s.project, s.plan, s.voids = d, p, plan, nil
	s.undo, s.redo, s.drag, s.path = nil, nil, nil, pf.Path
	s.saved = saved
	s.generation++
	return items, nil
}

// SPDX-License-Identifier: GPL-3.0-only

package export

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/engine"
	"github.com/gillesmagneville/pageslice/internal/layout"
	"github.com/gillesmagneville/pageslice/internal/model"
)

// Request is an export to run.
type Request struct {
	// Doc is the engine document the preview shows: the working copy, with the
	// background the project asks for.
	Doc  engine.Document
	Plan *layout.Plan
	// Project gives the background of the sheets.
	Project model.Project
	// Path is the file to write.
	Path string
	// Progress is called after each page, with the pages done and their
	// number; returning false stops the export.
	Progress func(done, total int) bool
}

// Result is what a successful export says.
type Result struct {
	Pages int
	Bytes int64
}

// VerifyError is the failure of the checks made on the file once written:
// nothing was put at the destination.
type VerifyError struct {
	Items []diag.Item
}

func (e *VerifyError) Error() string {
	if len(e.Items) == 0 {
		return "export: the PDF written does not match the plan"
	}
	return fmt.Sprintf("export: the PDF written does not match the plan (%s)", e.Items[0])
}

// Run writes the PDF. The file is written beside its destination under a
// temporary name, read back and compared with the plan; only then does it take
// the place of the destination, so that a failed or stopped export never
// leaves a half-written file, nor replaces a good one with a bad one.
func Run(req Request) (*Result, error) {
	if req.Doc == nil || req.Plan == nil || len(req.Plan.Pages) == 0 {
		return nil, diag.Fail("app.no_document")
	}
	specs := Specs(req.Plan)
	dir, base := filepath.Split(req.Path)
	part := filepath.Join(dir, "."+base+".part")

	f, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("cannot create the file in %s: %w", filepath.Dir(part), err)
	}
	done := false
	defer func() {
		if !done {
			f.Close()
			os.Remove(part)
		}
	}()

	w := bufio.NewWriterSize(f, 64<<10)
	opts := engine.PDFOptions{WhitePage: WhitePage(req.Project), Progress: req.Progress}
	if err := req.Doc.ExportPDF(w, specs, opts); err != nil {
		return nil, err
	}
	if err := w.Flush(); err != nil {
		return nil, fmt.Errorf("cannot write %s: %w", base, err)
	}
	if err := f.Sync(); err != nil {
		return nil, fmt.Errorf("cannot write %s: %w", base, err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("cannot write %s: %w", base, err)
	}

	// The file is read back from the disk, as the user's PDF reader will.
	data, err := os.ReadFile(part)
	if err != nil {
		return nil, fmt.Errorf("cannot read back %s: %w", base, err)
	}
	pdf, err := ReadPDF(data)
	if err != nil {
		return nil, &VerifyError{Items: []diag.Item{diag.New("export.pdf_unreadable", 0, "reason", err.Error())}}
	}
	if items := Verify(pdf, specs, req.Plan.Doc.W, req.Plan.Doc.H, opts.WhitePage); len(items) > 0 {
		return nil, &VerifyError{Items: items}
	}

	if err := os.Rename(part, req.Path); err != nil {
		return nil, fmt.Errorf("cannot replace %s (is it open in another program?): %w", base, err)
	}
	done = true
	return &Result{Pages: len(pdf.Pages), Bytes: int64(len(data))}, nil
}

// IsCancelled tells whether an error is the user stopping the export.
func IsCancelled(err error) bool { return errors.Is(err, engine.ErrCancelled) }

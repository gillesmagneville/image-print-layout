// SPDX-License-Identifier: GPL-3.0-only

package export

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/layout"
	"github.com/gillesmagneville/image-print-layout/internal/model"
)

// fakeDoc is an engine document whose PDF is whatever the test says.
type fakeDoc struct {
	write func(out io.Writer, pages []engine.PageSpec, opt engine.PDFOptions) error
}

func (d *fakeDoc) Size() (float64, float64)           { return miniCAD.W, miniCAD.H }
func (d *fakeDoc) Measure(string) (engine.Box, error) { return engine.Box{}, errors.New("fake") }
func (d *fakeDoc) MeasureBlocks([]string) map[string]engine.Box {
	return nil
}
func (d *fakeDoc) Render(engine.Box, float64, color.Color) (*image.RGBA, error) {
	return nil, errors.New("fake")
}
func (d *fakeDoc) RenderPage(engine.PageSpec, float64, bool) (*image.RGBA, error) {
	return nil, errors.New("fake")
}
func (d *fakeDoc) ExportPDF(out io.Writer, pages []engine.PageSpec, opt engine.PDFOptions) error {
	return d.write(out, pages, opt)
}
func (d *fakeDoc) Close() {}

// goodPDF writes a PDF that says what the plan says, one page at a time, and
// reports its progress.
func goodPDF(out io.Writer, pages []engine.PageSpec, opt engine.PDFOptions) error {
	var content []synthPage
	for _, p := range pdfOf(pages, miniCAD.W, miniCAD.H).Pages {
		content = append(content, synthPage{w: p.WidthPt, h: p.HeightPt, content: p.Content})
	}
	if opt.Progress != nil {
		for i := range pages {
			if !opt.Progress(i+1, len(pages)) {
				return engine.ErrCancelled
			}
		}
	}
	_, err := out.Write(synthetic(content, true))
	return err
}

func request(t *testing.T, doc engine.Document, path string) Request {
	t.Helper()
	p := acceptance(1380.5, 2760, 4120.25)
	return Request{Doc: doc, Plan: planOf(t, p), Project: p, Path: path}
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range list {
		names = append(names, e.Name())
	}
	return names
}

func TestRunWritesChecksAndPlacesTheFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "plan.pdf")
	var done []int
	req := request(t, &fakeDoc{write: goodPDF}, out)
	req.Progress = func(d, total int) bool {
		done = append(done, d)
		return true
	}
	res, err := Run(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Pages != 4 || res.Bytes <= 0 {
		t.Errorf("result = %+v", res)
	}
	if got := entries(t, dir); len(got) != 1 || got[0] != "plan.pdf" {
		t.Errorf("the folder holds %v, want only plan.pdf", got)
	}
	if fmt.Sprint(done) != "[1 2 3 4]" {
		t.Errorf("progress = %v", done)
	}
}

func TestRunPassesTheBackgroundOfThePageToTheEngine(t *testing.T) {
	for _, page := range []model.PageBackground{model.WhitePage, model.TransparentPage} {
		var got engine.PDFOptions
		doc := &fakeDoc{write: func(out io.Writer, pages []engine.PageSpec, opt engine.PDFOptions) error {
			got = opt
			return goodPDF(out, pages, opt)
		}}
		req := request(t, doc, filepath.Join(t.TempDir(), "a.pdf"))
		req.Project.Background.Page = page
		if _, err := Run(req); err != nil {
			t.Fatal(err)
		}
		if want := page == model.WhitePage; got.WhitePage != want {
			t.Errorf("page %s: WhitePage = %v", page, got.WhitePage)
		}
	}
}

func TestRunNeverReplacesAGoodFileWithABadOne(t *testing.T) {
	cases := map[string]struct {
		write func(io.Writer, []engine.PageSpec, engine.PDFOptions) error
		code  string
	}{
		"a page is missing": {func(out io.Writer, pages []engine.PageSpec, opt engine.PDFOptions) error {
			return goodPDF(out, pages[:3], opt)
		}, "export.pdf_pages"},
		"not a PDF": {func(out io.Writer, _ []engine.PageSpec, _ engine.PDFOptions) error {
			_, err := out.Write([]byte("this is not a PDF"))
			return err
		}, "export.pdf_unreadable"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "plan.pdf")
			if err := os.WriteFile(out, []byte("the previous export"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Run(request(t, &fakeDoc{write: c.write}, out))
			var ve *VerifyError
			if !errors.As(err, &ve) {
				t.Fatalf("error = %v, want a VerifyError", err)
			}
			if !has(ve.Items, c.code) {
				t.Errorf("items = %v, want %s", codes(ve.Items), c.code)
			}
			if data, _ := os.ReadFile(out); string(data) != "the previous export" {
				t.Errorf("the previous file was replaced by %q", data)
			}
			if got := entries(t, dir); len(got) != 1 {
				t.Errorf("the folder holds %v: the temporary file was not removed", got)
			}
		})
	}
}

func TestRunStoppedByTheUserLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	req := request(t, &fakeDoc{write: goodPDF}, filepath.Join(dir, "plan.pdf"))
	req.Progress = func(done, total int) bool { return done < 2 }
	_, err := Run(req)
	if !IsCancelled(err) {
		t.Fatalf("error = %v, want a cancellation", err)
	}
	if got := entries(t, dir); len(got) != 0 {
		t.Errorf("a stopped export left %v", got)
	}
}

func TestRunExplainsADestinationThatCannotBeWritten(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-folder", "plan.pdf")
	_, err := Run(request(t, &fakeDoc{write: goodPDF}, missing))
	if err == nil || !os.IsNotExist(errors.Unwrap(err)) && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v", err)
	}
	if got := err.Error(); !strings.Contains(got, "no-such-folder") {
		t.Errorf("the message %q does not say where", got)
	}
}

func TestRunNeedsADocumentAndAPlan(t *testing.T) {
	_, err := Run(Request{Path: filepath.Join(t.TempDir(), "a.pdf")})
	if diag.CodeOf(err) != "app.no_document" {
		t.Errorf("error = %v", err)
	}
	empty := Request{Doc: &fakeDoc{write: goodPDF}, Plan: &layout.Plan{}, Path: filepath.Join(t.TempDir(), "a.pdf")}
	if _, err := Run(empty); err == nil {
		t.Error("a plan without pages was exported")
	}
}

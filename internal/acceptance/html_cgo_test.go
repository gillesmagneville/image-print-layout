// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package acceptance

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/export"
	"github.com/gillesmagneville/image-print-layout/internal/model"
	"github.com/gillesmagneville/image-print-layout/internal/testimg"
)

// The page HTML on the whole chain: a real document, opened, laid out with the
// cuts of the scenario, written, and read back from the disk by the export
// (which compares it with the plan before it keeps the file).

var embeddedRe = regexp.MustCompile(`url\("data:([^;"]+);base64,([A-Za-z0-9+/=]*)"\)`)

// embedded is the picture a page carries, and its type.
func embedded(t *testing.T, path string) (mime string, data []byte, page string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := embeddedRe.FindStringSubmatch(string(b))
	if m == nil {
		t.Fatalf("%s carries no picture", path)
	}
	data, err = base64.StdEncoding.DecodeString(m[2])
	if err != nil {
		t.Fatal(err)
	}
	return m[1], data, string(b)
}

// writeHTMLPage writes the page the way the window does.
func (sc *scenario) writeHTMLPage(name string, cut bool) (path string, res *export.Result) {
	sc.t.Helper()
	doc := sc.sess.Document()
	project := sc.sess.Project()
	src, err := doc.HTMLSource(sc.eng, project.Background)
	sc.must(err)
	path = filepath.Join(sc.dir, name)
	res, err = export.RunHTML(export.HTMLRequest{
		Plan: sc.sess.Plan(), Source: src, Path: path,
		Options: export.HTMLOptions{Title: "mini-cad", Lang: "en", Cut: cut, WhitePage: export.WhitePage(project)},
	})
	if err != nil {
		sc.t.Fatalf("html export: %v", err)
	}
	return path, res
}

func TestHTMLPageOfADrawing(t *testing.T) {
	for _, fx := range fixtures(t) {
		t.Run(fx.name, func(t *testing.T) {
			sc := newScenario(t, fx.svg)
			before := fileHash(t, sc.path)
			sc.layOut(scenarioCuts)

			// Cut at the three cuts: four pieces. Whole: one.
			cutPath, res := sc.writeHTMLPage("cut.html", true)
			if res.Pages != 4 {
				t.Errorf("the cut page has %d pieces, want 4", res.Pages)
			}
			wholePath, res := sc.writeHTMLPage("whole.html", false)
			if res.Pages != 1 {
				t.Errorf("the whole page has %d pieces, want 1", res.Pages)
			}
			mime, svg, page := embedded(t, cutPath)
			if mime != "image/svg+xml" || !bytes.Contains(svg, []byte("<svg")) {
				t.Errorf("the page carries a %s of %d bytes, want the SVG", mime, len(svg))
			}
			if strings.Count(page, "base64,") != 1 {
				t.Error("the drawing is not written once")
			}
			// Both pages carry the same drawing, and nothing in them depends on
			// the pieces.
			if _, other, _ := embedded(t, wholePath); !bytes.Equal(other, svg) {
				t.Error("the cut and the whole page carry different drawings")
			}

			// Deterministic: the same export gives the same bytes.
			again, _ := sc.writeHTMLPage("again.html", true)
			a, _ := os.ReadFile(cutPath)
			b, _ := os.ReadFile(again)
			if !bytes.Equal(a, b) {
				t.Error("two exports of the same plan differ")
			}

			// The background of the document is the one the project says.
			if info, found := sc.sess.DocumentBackground(); found {
				sc.must(sc.sess.SetDocBackground(model.RemoveBackground))
				p, _ := sc.writeHTMLPage("nobg.html", true)
				_, noBG, _ := embedded(t, p)
				if bytes.Equal(noBG, svg) || len(noBG) >= len(svg) {
					t.Errorf("without the background (%s %s) the drawing is not smaller: %d bytes against %d", info.Element, info.Fill, len(noBG), len(svg))
				}
			}

			if after := fileHash(t, sc.path); after != before {
				t.Error("the source file was modified")
			}
		})
	}
}

func TestHTMLPageOfAPicture(t *testing.T) {
	photo := testimg.Quadrants(120, 80)
	jpeg := testimg.JPEG(photo)
	for _, c := range []struct {
		name, file string
		data       []byte
		mime       string
		kept       bool // the page carries the file as it was
	}{
		{"jpeg", "photo.jpg", jpeg, "image/jpeg", true},
		{"png", "photo.png", testimg.PNG(photo, 300, 300), "image/png", true},
		{"tiff", "scan.tif", testimg.TIFF(photo, 300, 300, 1), "image/png", false},
		{"tiff turned", "scan6.tif", testimg.TIFF(photo, 300, 300, 6), "image/png", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc := newScenarioFile(t, c.file, c.data)
			path, res := sc.writeHTMLPage("picture.html", false)
			if res.Pages != 1 {
				t.Errorf("%d pieces, want 1", res.Pages)
			}
			mime, data, _ := embedded(t, path)
			if mime != c.mime {
				t.Errorf("the page carries a %s, want a %s", mime, c.mime)
			}
			if got := bytes.Equal(data, c.data); got != c.kept {
				t.Errorf("the page carries the file as it was: %v, want %v", got, c.kept)
			}
			if c.kept {
				return
			}
			// A picture drawn by the engine is upright, whatever its tag said: the
			// red quadrant, at the top left of the pixels of the file, is at the top
			// left of the picture, and at its top right when the tag turns the
			// picture a quarter turn clockwise (6).
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			w, h := img.Bounds().Dx(), img.Bounds().Dy()
			if (c.name == "tiff") && (w != 120 || h != 80) {
				t.Errorf("the PNG is %dx%d, want 120x80", w, h)
			}
			if c.name == "tiff turned" && (w != 80 || h != 120) {
				t.Errorf("the turned PNG is %dx%d, want 80x120", w, h)
			}
			x := w / 8
			if c.name == "tiff turned" {
				x = w - 1 - w/8
			}
			if r, g, b, _ := img.At(x, h/8).RGBA(); r>>8 < 200 || g>>8 > 80 || b>>8 > 80 {
				t.Errorf("the red quadrant is not where the picture says: %v %v %v", r>>8, g>>8, b>>8)
			}
		})
	}
}

// A page can be cut at the cuts of a picture as at those of a drawing.
func TestHTMLPageOfACutPicture(t *testing.T) {
	sc := newScenarioFile(t, "mini-cad.png", miniCADPicture(t))
	sc.layOut(scenarioCuts)
	_, res := sc.writeHTMLPage("cut.html", true)
	if res.Pages != 4 {
		t.Errorf("%d pieces, want 4", res.Pages)
	}
}

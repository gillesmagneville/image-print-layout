// SPDX-License-Identifier: GPL-3.0-only

package svgdoc

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/model"
)

// A PNG of one pixel, and a JPEG header: what is told by the content.
var (
	onePixelPNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/q842iQAAAABJRU5ErkJggg==")
	jpegStart      = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0}
)

func linkedDocument(t *testing.T) (dir string, svg []byte) {
	t.Helper()
	dir = t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(dir, "img"), 0o755))
	must(os.WriteFile(filepath.Join(dir, "img", "photo.png"), onePixelPNG, 0o644))
	must(os.WriteFile(filepath.Join(dir, "a b.jpg"), jpegStart, 0o644))
	must(os.WriteFile(filepath.Join(dir, "scan.tif"), []byte("II*\x00\x08\x00\x00\x00"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "logo.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`), 0o644))
	svg = []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="400" height="300" viewBox="0 0 400 300">
<rect id="bg" width="400" height="300" fill="#cccccc"/>
<g id="a"><image id="i1" width="10" height="10" xlink:href="img/photo.png"/>
<image id="i2" width="10" height="10" href='a%20b.jpg'/>
<image id="i3" width="10" height="10" xlink:href="scan.tif"/>
<image id="i4" width="10" height="10" xlink:href="logo.svg"/>
<image id="i5" width="10" height="10" xlink:href="missing.png"/>
<image id="i6" width="10" height="10" xlink:href="https://example.org/x.png"/>
<image id="i7" width="10" height="10" xlink:href="data:image/png;base64,AAAA"/></g>
</svg>`)
	return dir, svg
}

func hrefs(work []byte) map[string]string {
	out := map[string]string{}
	re := regexp.MustCompile(`<image id="(i\d)"[^>]*?href=["']([^"']*)["']`)
	for _, m := range re.FindAllStringSubmatch(string(work), -1) {
		out[m[1]] = m[2]
	}
	return out
}

func TestComposeStandaloneWritesTheLinkedImagesIn(t *testing.T) {
	dir, src := linkedDocument(t)
	a := Analyze(src, dir)
	work, n, err := a.ComposeStandalone(model.Background{})
	if err != nil {
		t.Fatal(err)
	}
	got := hrefs(work)
	// The images a browser shows are written in, found by their content, and the
	// name with %20 is found; the others keep their link.
	for id, prefix := range map[string]string{"i1": "data:image/png;base64,", "i2": "data:image/jpeg;base64,", "i4": "data:image/svg+xml;base64,"} {
		if !strings.HasPrefix(got[id], prefix) {
			t.Errorf("%s is %.40q, want a %s address", id, got[id], prefix)
		}
	}
	if enc := strings.TrimPrefix(got["i1"], "data:image/png;base64,"); enc != base64.StdEncoding.EncodeToString(onePixelPNG) {
		t.Errorf("the PNG written in is not the file")
	}
	for id, want := range map[string]string{"i3": "scan.tif", "i5": "missing.png", "i6": "https://example.org/x.png", "i7": "data:image/png;base64,AAAA"} {
		if got[id] != want {
			t.Errorf("%s is %q, want it left as %q", id, got[id], want)
		}
	}
	if n != 3 {
		t.Errorf("%d images written in, want 3", n)
	}

	// The ordinary copy is as it was: nothing is read from the disk.
	plain, _, err := a.Compose(model.Background{})
	if err != nil {
		t.Fatal(err)
	}
	if h := hrefs(plain); h["i1"] != "img/photo.png" || h["i2"] != "a%20b.jpg" {
		t.Errorf("Compose changed the links: %v", h)
	}
	// And the same analysis gives the same bytes twice.
	again, _, _ := a.ComposeStandalone(model.Background{})
	if string(again) != string(work) {
		t.Error("two standalone copies differ")
	}
}

func TestComposeStandaloneFollowsTheBackgroundInstruction(t *testing.T) {
	dir, src := linkedDocument(t)
	a := Analyze(src, dir)
	if a.Background == nil {
		t.Fatal("no background found in the test document")
	}
	white, _, err := a.ComposeStandalone(model.Background{Document: model.WhitenBackground, Element: a.Background.Ref})
	if err != nil || !strings.Contains(string(white), "fill:#ffffff") || !strings.Contains(string(white), "data:image/png;base64,iVBOR") {
		t.Errorf("whitened copy: %v\n%s", err, white)
	}
	gone, n, err := a.ComposeStandalone(model.Background{Document: model.RemoveBackground, Element: a.Background.Ref})
	if err != nil || strings.Contains(string(gone), `id="bg"`) || n != 3 {
		t.Errorf("copy with no background: %v, %d images written in", err, n)
	}
}

func TestAnImageInsideTheBackgroundThatGoesIsNotWrittenIn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.png"), onePixelPNG, 0o644); err != nil {
		t.Fatal(err)
	}
	src := []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="400" height="300">
<g id="back"><rect id="bg" width="400" height="300" fill="#cccccc"/><image id="i1" width="1" height="1" xlink:href="p.png"/></g>
<rect id="r" width="5" height="5"/></svg>`)
	a := Analyze(src, dir)
	if a.Background == nil {
		t.Skip("the document has no background to remove")
	}
	work, _, err := a.ComposeStandalone(model.Background{Document: model.RemoveBackground, Element: a.Background.Ref})
	if err != nil {
		t.Fatalf("a link inside the element that goes made the copy fail: %v", err)
	}
	if strings.Contains(string(work), `id="bg"`) {
		t.Error("the background is still there")
	}
}

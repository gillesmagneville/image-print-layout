// SPDX-License-Identifier: GPL-3.0-only

package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gillesmagneville/svg-print-layout/internal/diag"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
)

func writeSVG(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHashFile(t *testing.T) {
	path := writeSVG(t, t.TempDir(), "a.svg")
	got, err := HashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	sum := sha256.Sum256(data)
	if got != hex.EncodeToString(sum[:]) {
		t.Errorf("hash = %s", got)
	}
	if _, err := HashFile(filepath.Join(t.TempDir(), "missing.svg")); err == nil {
		t.Error("no error for a missing file")
	}
}

// Acceptance test 14, with files: save, reopen, find the settings and the
// document, which was not touched.
func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	svg := writeSVG(t, dir, "schemas/mini-cad.svg")
	before, _ := HashFile(svg)
	projectPath := filepath.Join(dir, "projets", "mini-cad"+Extension)
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}

	p := referenceProject()
	src, err := SourceFor(projectPath, svg, 1280, 5479)
	if err != nil {
		t.Fatal(err)
	}
	p.Source = src
	if src.Path != "../schemas/mini-cad.svg" {
		t.Errorf("relative path = %q", src.Path)
	}
	if err := Save(projectPath, p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, p) {
		t.Errorf("loaded %+v\nsaved  %+v", got, p)
	}
	// The relative path is the one of the place the file was saved to.
	data, _ := os.ReadFile(projectPath)
	if !bytes.Contains(data, []byte(`"path": "../schemas/mini-cad.svg"`)) {
		t.Errorf("the file does not hold the relative path:\n%s", data)
	}
	// The source SVG is intact.
	if after, _ := HashFile(svg); after != before {
		t.Error("the source document was modified")
	}
	// No temporary file is left next to the project.
	entries, _ := os.ReadDir(filepath.Dir(projectPath))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("a temporary file was left: %s", e.Name())
		}
	}
	// Saving twice gives the same bytes.
	if err := Save(projectPath, p); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(projectPath)
	if !bytes.Equal(data, again) {
		t.Error("two saves of the same project differ")
	}
}

func TestSaveReplacesAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.svgpl")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, referenceProject()); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Errorf("the saved file cannot be read: %v", err)
	}
}

func TestSaveInAMissingFolderFails(t *testing.T) {
	err := Save(filepath.Join(t.TempDir(), "nope", "a.svgpl"), referenceProject())
	if err == nil {
		t.Fatal("no error")
	}
	if !strings.Contains(err.Error(), "cannot write") {
		t.Errorf("error = %v", err)
	}
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.svgpl")); err == nil || !strings.Contains(err.Error(), "cannot read") {
		t.Errorf("missing file: %v", err)
	}
	path := filepath.Join(t.TempDir(), "a.svgpl")
	_ = os.WriteFile(path, []byte("hello"), 0o644)
	if _, err := Load(path); diag.CodeOf(err) != "project.not_a_project" {
		t.Errorf("not a project: %v", err)
	}
}

func TestResolveSource(t *testing.T) {
	dir := t.TempDir()
	svg := writeSVG(t, dir, "schemas/mini-cad.svg")
	projectPath := filepath.Join(dir, "projets", "a.svgpl")

	// By the relative path, when the absolute one is gone (the folder moved).
	src := model.Source{Path: "../schemas/mini-cad.svg", AbsolutePath: "/no/such/place/mini-cad.svg"}
	if got, ok := ResolveSource(projectPath, src); !ok || filepath.Clean(got) != filepath.Clean(svg) {
		t.Errorf("relative: %q, %v", got, ok)
	}
	// By the absolute path, when the relative one does not lead anywhere.
	src = model.Source{Path: "../elsewhere/x.svg", AbsolutePath: svg}
	if got, ok := ResolveSource(projectPath, src); !ok || got != svg {
		t.Errorf("absolute: %q, %v", got, ok)
	}
	// Neither: the settings are kept and the caller offers to locate it.
	src = model.Source{Path: "../elsewhere/x.svg", AbsolutePath: "/no/such/place/x.svg"}
	if got, ok := ResolveSource(projectPath, src); ok {
		t.Errorf("found %q", got)
	}
	// A folder is not a document.
	src = model.Source{AbsolutePath: dir}
	if _, ok := ResolveSource(projectPath, src); ok {
		t.Error("a folder was taken for a document")
	}
	if _, ok := ResolveSource(projectPath, model.Source{}); ok {
		t.Error("an empty source was found")
	}
}

func TestVerify(t *testing.T) {
	p := referenceProject()
	sha := p.Source.SHA256
	if items := Verify(p, sha, 1280, 5479); len(items) != 0 {
		t.Errorf("nothing changed, yet %v", items)
	}
	items := Verify(p, strings.Repeat("cd", 32), 1280, 5479)
	if len(items) != 1 || items[0].Code != "project.source_changed" || items[0].Level != diag.Warning {
		t.Errorf("a changed file: %v", items)
	}
	// A shorter document: the size changed and one cut falls outside.
	items = Verify(p, sha, 1280, 3000)
	var codes []string
	for _, it := range items {
		codes = append(codes, it.Code)
	}
	if !reflect.DeepEqual(codes, []string{"project.size_changed", "project.cuts_outside"}) {
		t.Errorf("codes = %v", codes)
	}
	if got := items[1].Args; len(got) != 2 || got[1] != 1 {
		t.Errorf("cuts outside = %v", got)
	}
	if ids := CutsOutside(p, 3000); !reflect.DeepEqual(ids, []int{3}) {
		t.Errorf("CutsOutside = %v", ids)
	}
	// A project that recorded no fingerprint cannot say the file changed.
	p.Source.SHA256 = ""
	if items := Verify(p, "ffff", 1280, 5479); len(items) != 0 {
		t.Errorf("no fingerprint, yet %v", items)
	}
}

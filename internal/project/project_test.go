// SPDX-License-Identifier: GPL-3.0-only

package project

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/model"
)

// referenceProject is the project of the acceptance scenario: A4 portrait,
// 200 mm wide, three cuts, every page 10 mm from the top, the grey background
// of the document made white.
func referenceProject() model.Project {
	p := model.Default()
	p.Source = model.Source{Path: "schemas/mini-cad.svg", AbsolutePath: "/home/lea/schemas/mini-cad.svg",
		SHA256: strings.Repeat("ab", 32), Width: 1280, Height: 5479}
	p.Scale = model.Scale{Mode: model.Width, WidthMM: 200}
	p.Background = model.Background{Document: model.WhitenBackground, Element: "rect4521", Page: model.WhitePage}
	p.Cuts = []model.Cut{
		{ID: 1, Axis: model.AxisY, Position: 1380.5},
		{ID: 2, Axis: model.AxisY, Position: 2760},
		{ID: 3, Axis: model.AxisY, Position: 4120.25},
	}
	p.NextCutID = 4
	center := model.CenterV
	p.PageOverrides = []model.PageOverride{{Page: 2, Vertical: center}}
	p.Export = model.Export{OutputPath: "mini-cad.pdf"}
	return p
}

// The reference file: the format is a contract with files on disk, so a
// change of the writing shows up as a diff of this file.
func TestEncodeMatchesTheReferenceFile(t *testing.T) {
	got, err := Encode(referenceProject())
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "reference.pageslice"))
	if err != nil {
		t.Fatal(err)
	}
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(got, want) {
		t.Errorf("the bytes differ from testdata/reference.pageslice:\n%s", got)
	}
}

// A file written by version 1.0 of the program, under its first name (format
// "svgprintlayout-project", extension .svgpl), still opens and gives the same
// project as its current form.
func TestLegacyFileStillOpens(t *testing.T) {
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	legacy, err := Decode(read("legacy-1.0.svgpl"))
	if err != nil {
		t.Fatalf("the legacy file does not open: %v", err)
	}
	current, err := Decode(read("reference.pageslice"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacy, current) {
		t.Errorf("the legacy file gives another project:\n%+v\n%+v", legacy, current)
	}
	again, err := Encode(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(again, []byte(`"format": "`+Format+`"`)) {
		t.Errorf("a legacy project is not written back in the current format:\n%s", again)
	}
}

func TestIsProjectPath(t *testing.T) {
	for path, want := range map[string]bool{
		"plan.pageslice": true, "PLAN.PAGESLICE": true, "plan.svgpl": true, "dir.d/plan.SVGPL": true,
		"plan.svg": false, "plan": false, "plan.pageslice.bak": false,
	} {
		if got := IsProjectPath(path); got != want {
			t.Errorf("IsProjectPath(%q) = %v, want %v", path, got, want)
		}
	}
}

// Acceptance test 14, at the level of the logic: save, reload, deep equality of
// the settings.
func TestRoundTrip(t *testing.T) {
	want := referenceProject()
	data, err := Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("after a round trip:\n got  %+v\n want %+v", got, want)
	}
}

func TestRoundTripKeepsEveryChoice(t *testing.T) {
	offset := 4.5
	p := model.Default()
	p.Paper = model.Paper{Format: model.CustomFormat, Orientation: model.Landscape, CustomWidthMM: 123.4, CustomHeightMM: 345.6}
	p.Margins = model.Margins{Top: 12.5, Bottom: 7, Left: 3.25, Right: 0}
	p.Scale = model.Scale{Mode: model.FitHeight, WidthMM: 150, Percent: 87.5, PortionUnits: 1380.5}
	p.Placement = model.Placement{Horizontal: model.CustomH, Vertical: model.CenterV, OffsetXMM: -2.5, OffsetYMM: 1.125}
	p.Background = model.Background{Document: model.RemoveBackground, Element: "/svg/g[1]/rect[1]", Page: model.TransparentPage}
	p.Cuts = []model.Cut{{ID: 1, Axis: model.AxisY, Position: 100.1234567891}}
	p.NextCutID = 2
	p.PageOverrides = []model.PageOverride{{Page: 2, Horizontal: model.Right, OffsetXMM: &offset}}
	got, err := Decode(mustEncode(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, p) {
		t.Errorf("got  %+v\nwant %+v", got, p)
	}
}

func mustEncode(t *testing.T, p model.Project) []byte {
	t.Helper()
	b, err := Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Cuts are not stored with their ids: they are numbered again at each opening.
func TestCutsAreNumberedAgainAtOpening(t *testing.T) {
	p := referenceProject()
	p.Cuts[0].ID, p.Cuts[1].ID, p.Cuts[2].ID = 7, 9, 12
	p.NextCutID = 13
	got, err := Decode(mustEncode(t, p))
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range got.Cuts {
		if c.ID != i+1 || c.Position != p.Cuts[i].Position {
			t.Errorf("cut %d: %+v", i, c)
		}
	}
	if got.NextCutID != 4 {
		t.Errorf("NextCutID = %d", got.NextCutID)
	}
	if bytes.Contains(mustEncode(t, p), []byte(`"id"`)) {
		t.Error("an id was written")
	}
}

func TestEncodingIsDeterministic(t *testing.T) {
	a, b := mustEncode(t, referenceProject()), mustEncode(t, referenceProject())
	if !bytes.Equal(a, b) {
		t.Error("two encodings of the same project differ")
	}
	if !bytes.HasSuffix(a, []byte("}\n")) || bytes.Contains(a, []byte("\t")) || bytes.Contains(a, []byte("\r")) {
		t.Errorf("the file must use two-space indentation and end with one newline:\n%s", a)
	}
	// Text with characters HTML would escape is written as it is.
	p := referenceProject()
	p.Export.OutputPath = "<a&b>.pdf"
	if !bytes.Contains(mustEncode(t, p), []byte("<a&b>.pdf")) {
		t.Error("characters were escaped")
	}
}

// Nothing derived is stored: not the number of pages, nor their sizes or
// states, nor the scale.
func TestOnlySettingsAreStored(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(mustEncode(t, referenceProject()), &doc); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	want := map[string]bool{"format": true, "version": true, "source": true, "paper": true, "marginsMm": true, "scale": true,
		"placement": true, "background": true, "cuts": true, "pageOverrides": true, "export": true}
	for _, k := range keys {
		if !want[k] {
			t.Errorf("unexpected key %q", k)
		}
	}
	if len(keys) != len(want) {
		t.Errorf("keys = %v", keys)
	}
}

func decodeError(t *testing.T, text string) string {
	t.Helper()
	_, err := Decode([]byte(text))
	if err == nil {
		t.Fatalf("no error for %s", text)
	}
	e, ok := diag.As(err)
	if !ok {
		t.Fatalf("error without a code: %v", err)
	}
	if f := ""; len(e.Args) >= 2 {
		f, _ = e.Args[1].(string)
		return e.Code + ":" + f
	}
	return e.Code
}

func TestDecodeRefusesWhatIsNotAProject(t *testing.T) {
	for name, text := range map[string]string{
		"not JSON":       `<svg/>`,
		"another JSON":   `{"hello": 1}`,
		"another format": `{"format": "other", "version": 1}`,
		"empty":          ``,
	} {
		if got := decodeError(t, text); got != "project.not_a_project" {
			t.Errorf("%s: %s", name, got)
		}
	}
}

func TestDecodeVersions(t *testing.T) {
	if got := decodeError(t, `{"format":"pageslice-project","version":2}`); got != "project.too_new:2" && !strings.HasPrefix(got, "project.too_new") {
		t.Errorf("a newer version: %s", got)
	}
	_, err := Decode([]byte(`{"format":"pageslice-project","version":99}`))
	e, _ := diag.As(err)
	if e == nil || e.Code != "project.too_new" || e.Args[1] != 99 || e.Args[3] != Version {
		t.Errorf("a newer version: %v", err)
	}
	for _, text := range []string{
		`{"format":"pageslice-project"}`,
		`{"format":"pageslice-project","version":0}`,
		`{"format":"pageslice-project","version":-3}`,
	} {
		if got := decodeError(t, text); !strings.HasPrefix(got, "project.bad_version") {
			t.Errorf("%s: %s", text, got)
		}
	}
}

func TestDecodeChecksTheValues(t *testing.T) {
	base := string(mustEncode(t, referenceProject()))
	for name, tt := range map[string]struct{ old, new, want string }{
		"unknown paper":       {`"format": "A4"`, `"format": "B9"`, "project.invalid:paper.format"},
		"bad orientation":     {`"orientation": "portrait"`, `"orientation": "oblique"`, "project.invalid:paper.orientation"},
		"negative margin":     {`"top": 10`, `"top": -1`, "project.invalid:marginsMm"},
		"unknown scale mode":  {`"mode": "width"`, `"mode": "stretch"`, "project.invalid:scale.mode"},
		"zero width":          {`"widthMm": 200`, `"widthMm": 0`, "project.invalid:scale"},
		"bad alignment":       {`"horizontal": "center"`, `"horizontal": "middle"`, "project.invalid:placement"},
		"bad document choice": {`"document": "whiten"`, `"document": "hide"`, "project.invalid:background"},
		"bad page background": {`"page": "white"`, `"page": "grey"`, "project.invalid:background"},
		"unknown field":       {`"export": {`, `"colour": 1, "export": {`, "project.invalid:colour"},
		"cut on another axis": {`"axis": "y",
      "position": 1380.5`, `"axis": "x",
      "position": 1380.5`, "project.invalid:cuts[0].axis"},
		"cut out of the order": {`"position": 2760`, `"position": 1000`, "project.invalid:cuts[1].position"},
		"cut below the end":    {`"position": 4120.25`, `"position": 6000`, "project.invalid:cuts[2].position"},
		"override of page 9":   {`"page": 2`, `"page": 9`, "project.invalid:pageOverrides[0]"},
		"wrong type":           {`"version": 1`, `"version": 1, "cuts2": "x"`, "project.invalid:cuts2"},
	} {
		if !strings.Contains(base, tt.old) {
			t.Fatalf("%s: %q is not in the reference file", name, tt.old)
		}
		text := strings.Replace(base, tt.old, tt.new, 1)
		if got := decodeError(t, text); got != tt.want {
			t.Errorf("%s: got %s, want %s", name, got, tt.want)
		}
	}
	// A custom format needs its size.
	custom := strings.Replace(base, `"format": "A4"`, `"format": "custom"`, 1)
	if got := decodeError(t, custom); got != "project.invalid:paper.customMm" {
		t.Errorf("custom without a size: %s", got)
	}
	// A project written by hand may leave out what has a default.
	minimal := `{"format":"pageslice-project","version":1,"paper":{"format":"A4"},"marginsMm":{"top":1,"bottom":1,"left":1,"right":1},"scale":{"mode":"fit-width"}}`
	p, err := Decode([]byte(minimal))
	if err != nil {
		t.Fatal(err)
	}
	if p.Paper.Orientation != model.Portrait || p.Placement.Horizontal != model.CenterH || p.Placement.Vertical != model.Top ||
		p.Background.Document != model.KeepBackground || p.Background.Page != model.WhitePage {
		t.Errorf("defaults: %+v", p)
	}
}

func TestEncodeRefusesNonFiniteNumbers(t *testing.T) {
	p := referenceProject()
	p.Margins.Top = nan()
	if _, err := Encode(p); diag.CodeOf(err) != "project.invalid" {
		t.Errorf("error = %v", err)
	}
}

func nan() float64 { z := 0.0; return z / z }

// The machinery of migrations: a file of an older version is converted step
// by step, then read as a current one.
func TestMigrations(t *testing.T) {
	table := map[int]migration{
		// version 1 -> 2: "marginsMm" had been called "margins".
		1: func(doc map[string]any) error {
			doc["marginsMm"] = doc["margins"]
			delete(doc, "margins")
			return nil
		},
		// version 2 -> 3: the scale mode "fit" became "fit-width".
		2: func(doc map[string]any) error {
			sc := doc["scale"].(map[string]any)
			if sc["mode"] == "fit" {
				sc["mode"] = "fit-width"
			}
			return nil
		},
	}
	old := `{"format":"pageslice-project","version":1,"paper":{"format":"A3","orientation":"landscape"},
	  "margins":{"top":8,"bottom":9,"left":6,"right":7},"scale":{"mode":"fit"}}`
	p, err := decode([]byte(old), 3, table)
	if err != nil {
		t.Fatal(err)
	}
	if p.Margins != (model.Margins{Top: 8, Bottom: 9, Left: 6, Right: 7}) || p.Scale.Mode != model.FitWidth || p.Paper.Format != "A3" {
		t.Errorf("after the migrations: %+v", p)
	}
	// A missing step is refused, not skipped.
	if _, err := decode([]byte(old), 4, table); diag.CodeOf(err) != "project.bad_version" {
		t.Errorf("a missing migration: %v", err)
	}
	// The current version needs none.
	if _, err := decode(mustEncode(t, referenceProject()), Version, nil); err != nil {
		t.Errorf("the current version: %v", err)
	}
}

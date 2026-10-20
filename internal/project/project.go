// SPDX-License-Identifier: GPL-3.0-only

// Package project reads and writes the layout project file (.svgpl): a
// readable, versioned JSON document that stores the choices of the user and
// never the drawing. The source SVG stays in its own file, untouched; of the
// results (number of pages, states, sizes) nothing is stored, all is computed
// again at opening.
//
// The writing is deterministic: the same project always gives the same bytes
// (UTF-8, keys in a fixed order, two-space indentation, the shortest decimal
// form of each number) and is atomic (a temporary file, then a rename).
//
// Pure Go, no cgo.
package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/gillesmagneville/svg-print-layout/internal/diag"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
	"github.com/gillesmagneville/svg-print-layout/internal/units"
)

// Format is the value of the "format" key of every project file.
const Format = "svgprintlayout-project"

// Version is the version of the format this program writes and reads without
// conversion. Each change of the format adds a conversion from the previous
// version (migrations), tested on a sample file.
const Version = 1

// Extension is the extension of a project file.
const Extension = ".svgpl"

// The file is described by its own types, apart from model: the format is a
// contract with files on disk and must not change when an internal type does.
type file struct {
	Format        string         `json:"format"`
	Version       int            `json:"version"`
	Source        fileSource     `json:"source"`
	Paper         filePaper      `json:"paper"`
	MarginsMm     fileMargins    `json:"marginsMm"`
	Scale         fileScale      `json:"scale"`
	Placement     filePlacement  `json:"placement"`
	Background    fileBackground `json:"background"`
	Cuts          []fileCut      `json:"cuts"`
	PageOverrides []fileOverride `json:"pageOverrides"`
	Export        fileExport     `json:"export"`
}

type fileSource struct {
	Path         string  `json:"path"`
	AbsolutePath string  `json:"absolutePath"`
	SHA256       string  `json:"sha256"`
	Width        float64 `json:"width"`
	Height       float64 `json:"height"`
}

type filePaper struct {
	Format      string        `json:"format"`
	Orientation string        `json:"orientation"`
	CustomMm    *fileCustomMm `json:"customMm"`
}

type fileCustomMm struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type fileMargins struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
}

type fileScale struct {
	Mode         string  `json:"mode"`
	WidthMm      float64 `json:"widthMm"`
	Percent      float64 `json:"percent"`
	PortionUnits float64 `json:"portionUnits"`
}

type filePlacement struct {
	Horizontal string  `json:"horizontal"`
	Vertical   string  `json:"vertical"`
	OffsetXMm  float64 `json:"offsetXMm"`
	OffsetYMm  float64 `json:"offsetYMm"`
}

type fileBackground struct {
	Document string `json:"document"`
	Element  string `json:"element"`
	Page     string `json:"page"`
}

type fileCut struct {
	Axis     string  `json:"axis"`
	Position float64 `json:"position"`
}

type fileOverride struct {
	Page       int      `json:"page"`
	Horizontal string   `json:"horizontal,omitempty"`
	Vertical   string   `json:"vertical,omitempty"`
	OffsetXMm  *float64 `json:"offsetXMm,omitempty"`
	OffsetYMm  *float64 `json:"offsetYMm,omitempty"`
}

type fileExport struct {
	OutputPath string `json:"outputPath"`
}

// Encode writes a project as the bytes of a project file.
func Encode(p model.Project) ([]byte, error) {
	f := file{
		Format:  Format,
		Version: Version,
		Source: fileSource{
			Path: p.Source.Path, AbsolutePath: p.Source.AbsolutePath, SHA256: p.Source.SHA256,
			Width: p.Source.Width, Height: p.Source.Height,
		},
		Paper:     filePaper{Format: p.Paper.Format, Orientation: string(p.Paper.Orientation)},
		MarginsMm: fileMargins{p.Margins.Top, p.Margins.Bottom, p.Margins.Left, p.Margins.Right},
		Scale: fileScale{
			Mode: string(p.Scale.Mode), WidthMm: p.Scale.WidthMM, Percent: p.Scale.Percent, PortionUnits: p.Scale.PortionUnits,
		},
		Placement: filePlacement{
			Horizontal: string(p.Placement.Horizontal), Vertical: string(p.Placement.Vertical),
			OffsetXMm: p.Placement.OffsetXMM, OffsetYMm: p.Placement.OffsetYMM,
		},
		Background: fileBackground{Document: string(p.Background.Document), Element: p.Background.Element, Page: string(p.Background.Page)},
		Cuts:       []fileCut{},
		Export:     fileExport{OutputPath: p.Export.OutputPath},
	}
	if p.Paper.CustomWidthMM != 0 || p.Paper.CustomHeightMM != 0 {
		f.Paper.CustomMm = &fileCustomMm{p.Paper.CustomWidthMM, p.Paper.CustomHeightMM}
	}
	for _, c := range p.Cuts {
		axis := string(c.Axis)
		if axis == "" {
			axis = string(model.AxisY)
		}
		f.Cuts = append(f.Cuts, fileCut{Axis: axis, Position: c.Position})
	}
	f.PageOverrides = []fileOverride{}
	for _, o := range p.PageOverrides {
		f.PageOverrides = append(f.PageOverrides, fileOverride{
			Page: o.Page, Horizontal: string(o.Horizontal), Vertical: string(o.Vertical),
			OffsetXMm: o.OffsetXMM, OffsetYMm: o.OffsetYMM,
		})
	}

	for _, v := range []float64{
		f.Source.Width, f.Source.Height, f.MarginsMm.Top, f.MarginsMm.Bottom, f.MarginsMm.Left, f.MarginsMm.Right,
		f.Scale.WidthMm, f.Scale.Percent, f.Scale.PortionUnits, f.Placement.OffsetXMm, f.Placement.OffsetYMm,
	} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, diag.Fail("project.invalid", "field", "number")
		}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		return nil, fmt.Errorf("project: cannot encode: %w", err)
	}
	return buf.Bytes(), nil
}

// migration turns the content of a file of one version into that of the next
// one. It works on the generic form of the JSON document.
type migration func(doc map[string]any) error

// migrations holds, for each version, the conversion to the following one.
// None exists yet: version 1 is the first.
var migrations = map[int]migration{}

// Decode reads the bytes of a project file. A file of an older version is
// converted; a file of a newer version is refused with a clear message and
// never opened partly. The project that comes back is valid: its values
// have been checked.
func Decode(data []byte) (model.Project, error) {
	return decode(data, Version, migrations)
}

func decode(data []byte, current int, table map[int]migration) (model.Project, error) {
	var head struct {
		Format  string `json:"format"`
		Version *int   `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil || head.Format != Format {
		return model.Project{}, diag.Fail("project.not_a_project")
	}
	if head.Version == nil || *head.Version < 1 {
		v := 0
		if head.Version != nil {
			v = *head.Version
		}
		return model.Project{}, diag.Fail("project.bad_version", "version", v)
	}
	if *head.Version > current {
		return model.Project{}, diag.Fail("project.too_new", "version", *head.Version, "supported", current)
	}

	if *head.Version < current {
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			return model.Project{}, diag.Fail("project.not_a_project")
		}
		for v := *head.Version; v < current; v++ {
			step, ok := table[v]
			if !ok {
				return model.Project{}, diag.Fail("project.bad_version", "version", *head.Version)
			}
			if err := step(doc); err != nil {
				return model.Project{}, fmt.Errorf("project: converting from version %d: %w", v, err)
			}
			doc["version"] = float64(v + 1)
		}
		converted, err := json.Marshal(doc)
		if err != nil {
			return model.Project{}, fmt.Errorf("project: converting: %w", err)
		}
		data = converted
	}

	var f file
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return model.Project{}, diag.Fail("project.invalid", "field", fieldOf(err))
	}
	return fromFile(f)
}

// fieldOf names the field a decoding error is about, as well as the error
// allows.
func fieldOf(err error) string {
	if ute, ok := err.(*json.UnmarshalTypeError); ok && ute.Field != "" {
		return ute.Field
	}
	msg := err.Error()
	if i := strings.Index(msg, "unknown field "); i >= 0 {
		return strings.Trim(msg[i+len("unknown field "):], `"`)
	}
	return "file"
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// fromFile checks the values and builds the project.
func fromFile(f file) (model.Project, error) {
	bad := func(field string) (model.Project, error) {
		return model.Project{}, diag.Fail("project.invalid", "field", field)
	}
	p := model.Default()
	p.Source = model.Source{Path: f.Source.Path, AbsolutePath: f.Source.AbsolutePath, SHA256: f.Source.SHA256,
		Width: f.Source.Width, Height: f.Source.Height}
	if !finite(f.Source.Width) || !finite(f.Source.Height) || f.Source.Width < 0 || f.Source.Height < 0 {
		return bad("source")
	}

	// Paper.
	p.Paper = model.Paper{Format: f.Paper.Format, Orientation: model.Orientation(f.Paper.Orientation)}
	if p.Paper.Orientation == "" {
		p.Paper.Orientation = model.Portrait
	}
	if !p.Paper.Orientation.Valid() {
		return bad("paper.orientation")
	}
	if f.Paper.CustomMm != nil {
		p.Paper.CustomWidthMM, p.Paper.CustomHeightMM = f.Paper.CustomMm.Width, f.Paper.CustomMm.Height
		if !finite(p.Paper.CustomWidthMM) || !finite(p.Paper.CustomHeightMM) || p.Paper.CustomWidthMM < 0 || p.Paper.CustomHeightMM < 0 {
			return bad("paper.customMm")
		}
	}
	if p.Paper.Format == model.CustomFormat {
		if _, err := units.Custom(p.Paper.CustomWidthMM, p.Paper.CustomHeightMM); err != nil {
			return bad("paper.customMm")
		}
	} else if _, ok := units.Lookup(p.Paper.Format); !ok {
		return bad("paper.format")
	}

	// Margins.
	p.Margins = model.Margins{Top: f.MarginsMm.Top, Bottom: f.MarginsMm.Bottom, Left: f.MarginsMm.Left, Right: f.MarginsMm.Right}
	for _, v := range []float64{p.Margins.Top, p.Margins.Bottom, p.Margins.Left, p.Margins.Right} {
		if !finite(v) || v < 0 {
			return bad("marginsMm")
		}
	}

	// Scale.
	p.Scale = model.Scale{Mode: model.ScaleMode(f.Scale.Mode), WidthMM: f.Scale.WidthMm, Percent: f.Scale.Percent, PortionUnits: f.Scale.PortionUnits}
	if !p.Scale.Mode.Valid() {
		return bad("scale.mode")
	}
	for _, v := range []float64{p.Scale.WidthMM, p.Scale.Percent, p.Scale.PortionUnits} {
		if !finite(v) || v < 0 {
			return bad("scale")
		}
	}
	if (p.Scale.Mode == model.Width && p.Scale.WidthMM <= 0) || (p.Scale.Mode == model.Factor && p.Scale.Percent <= 0) {
		return bad("scale")
	}

	// Placement.
	p.Placement = model.Placement{
		Horizontal: model.HAlign(f.Placement.Horizontal), Vertical: model.VAlign(f.Placement.Vertical),
		OffsetXMM: f.Placement.OffsetXMm, OffsetYMM: f.Placement.OffsetYMm,
	}
	if p.Placement.Horizontal == model.InheritH {
		p.Placement.Horizontal = model.CenterH
	}
	if p.Placement.Vertical == model.InheritV {
		p.Placement.Vertical = model.Top
	}
	if !p.Placement.Horizontal.Valid() || !p.Placement.Vertical.Valid() || !finite(p.Placement.OffsetXMM) || !finite(p.Placement.OffsetYMM) {
		return bad("placement")
	}

	// Background.
	p.Background = model.Background{
		Document: model.DocBackground(f.Background.Document), Element: f.Background.Element, Page: model.PageBackground(f.Background.Page),
	}
	if p.Background.Document == "" {
		p.Background.Document = model.KeepBackground
	}
	if p.Background.Page == "" {
		p.Background.Page = model.WhitePage
	}
	if !p.Background.Document.Valid() || !p.Background.Page.Valid() {
		return bad("background")
	}

	// Cuts: numbered again at each opening.
	prev := 0.0
	p.Cuts = nil
	for i, c := range f.Cuts {
		if c.Axis != "" && c.Axis != string(model.AxisY) {
			return bad(fmt.Sprintf("cuts[%d].axis", i))
		}
		if !finite(c.Position) || c.Position <= prev || (p.Source.Height > 0 && c.Position >= p.Source.Height) {
			return bad(fmt.Sprintf("cuts[%d].position", i))
		}
		prev = c.Position
		p.Cuts = append(p.Cuts, model.Cut{ID: i + 1, Axis: model.AxisY, Position: c.Position})
	}
	p.NextCutID = len(p.Cuts) + 1

	// Overrides of single pages.
	p.PageOverrides = nil
	seen := map[int]bool{}
	for i, o := range f.PageOverrides {
		h, v := model.HAlign(o.Horizontal), model.VAlign(o.Vertical)
		if o.Page < 1 || o.Page > len(p.Cuts)+1 || seen[o.Page] ||
			(h != model.InheritH && !h.Valid()) || (v != model.InheritV && !v.Valid()) ||
			(o.OffsetXMm != nil && !finite(*o.OffsetXMm)) || (o.OffsetYMm != nil && !finite(*o.OffsetYMm)) {
			return bad(fmt.Sprintf("pageOverrides[%d]", i))
		}
		seen[o.Page] = true
		p.PageOverrides = append(p.PageOverrides, model.PageOverride{
			Page: o.Page, Horizontal: h, Vertical: v, OffsetXMM: o.OffsetXMm, OffsetYMM: o.OffsetYMm,
		})
	}

	p.Export = model.Export{OutputPath: f.Export.OutputPath}
	return p, nil
}

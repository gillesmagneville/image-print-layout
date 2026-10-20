// SPDX-License-Identifier: GPL-3.0-only

package model

import (
	"reflect"
	"testing"
)

func TestDefaultIsTheProjectOfADocumentJustOpened(t *testing.T) {
	p := Default()
	if p.Paper.Format != "A4" || p.Paper.Orientation != Portrait {
		t.Errorf("paper = %+v", p.Paper)
	}
	if p.Margins != (Margins{Top: 10, Bottom: 10, Left: 5, Right: 5}) {
		t.Errorf("margins = %+v", p.Margins)
	}
	if p.Scale.Mode != FitWidth {
		t.Errorf("scale = %+v", p.Scale)
	}
	if p.Placement.Horizontal != CenterH || p.Placement.Vertical != Top {
		t.Errorf("placement = %+v", p.Placement)
	}
	if p.Background.Document != KeepBackground || p.Background.Page != WhitePage {
		t.Errorf("background = %+v", p.Background)
	}
	if len(p.Cuts) != 0 || p.NextCutID != 1 {
		t.Errorf("cuts = %+v, next id %d", p.Cuts, p.NextCutID)
	}
}

// A copy shares nothing with its original: a command that changes the copy
// leaves the project it came from as it was.
func TestCloneSharesNothing(t *testing.T) {
	offset := 2.5
	p := Default()
	p.Cuts = []Cut{{ID: 1, Axis: AxisY, Position: 100}}
	p.PageOverrides = []PageOverride{{Page: 1, Vertical: CenterV, OffsetXMM: &offset}}

	q := p.Clone()
	if !reflect.DeepEqual(p, q) {
		t.Fatal("a clone differs from its original")
	}
	q.Cuts[0].Position = 5
	*q.PageOverrides[0].OffsetXMM = 99
	q.PageOverrides[0].Vertical = Top
	if p.Cuts[0].Position != 100 || *p.PageOverrides[0].OffsetXMM != 2.5 || p.PageOverrides[0].Vertical != CenterV {
		t.Errorf("the original changed: %+v", p)
	}
	if empty := Default().Clone(); empty.Cuts != nil || empty.PageOverrides != nil {
		t.Errorf("an empty project clones to empty slices: %+v", empty)
	}
}

func TestOverride(t *testing.T) {
	p := Default()
	p.PageOverrides = []PageOverride{{Page: 3, Horizontal: Left}}
	if o, ok := p.Override(3); !ok || o.Horizontal != Left {
		t.Errorf("Override(3) = %+v, %v", o, ok)
	}
	if _, ok := p.Override(2); ok {
		t.Error("page 2 has no override")
	}
}

func TestValidity(t *testing.T) {
	for _, tt := range []struct {
		name string
		ok   bool
		bad  bool
	}{
		{"portrait", Portrait.Valid(), Orientation("x").Valid()},
		{"scale mode", FitHeight.Valid(), ScaleMode("").Valid()},
		{"horizontal", CustomH.Valid(), InheritH.Valid()},
		{"vertical", CenterV.Valid(), InheritV.Valid()},
		{"document background", RemoveBackground.Valid(), DocBackground("x").Valid()},
		{"page background", TransparentPage.Valid(), PageBackground("").Valid()},
	} {
		if !tt.ok || tt.bad {
			t.Errorf("%s: valid value %v, invalid value %v", tt.name, tt.ok, tt.bad)
		}
	}
}

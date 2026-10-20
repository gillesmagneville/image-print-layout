// SPDX-License-Identifier: GPL-3.0-only

package layout

import (
	"reflect"
	"testing"
)

func TestCrossed(t *testing.T) {
	blocks := []Block{{"a", 100, 200}, {"b", 150, 300}, {"c", 400, 500}}
	for _, tt := range []struct {
		y    float64
		want []string
	}{
		{50, nil},
		{100, nil},    // on the edge: not crossed
		{100.04, nil}, // inside the tolerance
		{100.2, []string{"a"}},
		{175, []string{"a", "b"}},
		{200, []string{"b"}},
		{350, nil},
		{499.96, nil},
		{450, []string{"c"}},
	} {
		if got := Crossed(tt.y, blocks); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Crossed(%v) = %v, want %v", tt.y, got, tt.want)
		}
	}
}

func TestVoids(t *testing.T) {
	blocks := []Block{{"b", 300, 400}, {"a", 100, 200}, {"c", 150, 250}, {"d", 400, 450}}
	got := Voids(blocks, 600)
	want := []Interval{{0, 100}, {250, 300}, {450, 600}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Voids = %v, want %v", got, want)
	}
	if got := Voids(nil, 600); !reflect.DeepEqual(got, []Interval{{0, 600}}) {
		t.Errorf("no block: %v", got)
	}
	// A block covering everything leaves no void.
	if got := Voids([]Block{{"all", -10, 700}}, 600); len(got) != 0 {
		t.Errorf("a block covering the document: %v", got)
	}
	// A gap too short to hold a cut is not a void.
	if got := Voids([]Block{{"a", 0, 100}, {"b", 100.05, 200}}, 200); len(got) != 0 {
		t.Errorf("a gap of 0.05: %v", got)
	}
	if c := (Interval{250, 300}).Center(); c != 275 {
		t.Errorf("Center = %v", c)
	}
}

// A position a cut can take in a void crosses nothing.
func TestVoidsCrossNothing(t *testing.T) {
	blocks := miniCADBlocks()
	for _, v := range Voids(blocks, miniCAD.H) {
		for _, y := range []float64{v.Start + 1, v.Center(), v.End - 1} {
			if got := Crossed(y, blocks); len(got) != 0 {
				t.Fatalf("y = %v in the void %v crosses %v", y, v, got)
			}
		}
	}
}

func TestSnapToVoid(t *testing.T) {
	voids := []Interval{{250, 300}, {450, 460}, {700, 701}}
	for _, tt := range []struct {
		y       float64
		want    float64
		snapped bool
	}{
		{275, 275, true},  // already in a void: kept
		{244, 251, true},  // 7 from the void: one unit inside its edge
		{240, 240, false}, // 11 from the point it would reach: out of the 8 radius
		{307, 299, true},  // below the void
		{305, 299, true},
		{444, 451, true},   // a void of 10 units: the inset is one unit
		{703, 700.5, true}, // a void of one unit: its centre
		{400, 400, false},  // far from every void
	} {
		got, ok := SnapToVoid(tt.y, voids, 8)
		if got != tt.want || ok != tt.snapped {
			t.Errorf("SnapToVoid(%v, 8) = %v, %v; want %v, %v", tt.y, got, ok, tt.want, tt.snapped)
		}
	}
	if _, ok := SnapToVoid(10, nil, 8); ok {
		t.Error("no void, yet a snap")
	}
}

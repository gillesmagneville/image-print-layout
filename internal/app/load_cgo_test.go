// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package app

import (
	"testing"
	"time"

	"github.com/gillesmagneville/svg-print-layout/internal/engine"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
	"github.com/gillesmagneville/svg-print-layout/internal/testsvg"
)

// TestHeavyDocument opens a document of 100 000 elements with the real engine
// (ARCHITECTURE §15, "Documents très lourds"): it must open, be measured and
// be drawn, in times a person waits for.
func TestHeavyDocument(t *testing.T) {
	if testing.Short() {
		t.Skip("load test")
	}
	src := testsvg.Generate(testsvg.Options{Filler: 100000})
	svc := Services{Load: engine.LoadDocument, Match: engine.MatchFont}

	start := time.Now()
	op, err := Prepare(svc, src, "", model.Default().Background)
	if err != nil {
		t.Fatal(err)
	}
	defer op.Engine.Close()
	open := time.Since(start)
	if !op.Document.Coarse || len(op.Document.Blocks) == 0 {
		t.Fatalf("a document of 100 000 blocks must be read from an image: coarse %v, %d blocks", op.Document.Coarse, len(op.Document.Blocks))
	}

	start = time.Now()
	w, h := op.Engine.Size()
	if _, err := op.Engine.Render(engine.Box{W: w, H: h}, 0.2, nil); err != nil {
		t.Fatal(err)
	}
	whole := time.Since(start)

	start = time.Now()
	if _, err := op.Engine.Render(engine.Box{X: 100, Y: 1000, W: 400, H: 300}, 2, nil); err != nil {
		t.Fatal(err)
	}
	region := time.Since(start)

	t.Logf("open (analysis, load, ink of %d stretches): %v; whole at 0.2: %v; region at 2: %v",
		len(op.Document.Blocks), open, whole, region)
	if open > 60*time.Second || whole > 30*time.Second || region > 30*time.Second {
		t.Errorf("too slow: open %v, whole %v, region %v", open, whole, region)
	}
}

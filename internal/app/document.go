// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/layout"
	"github.com/gillesmagneville/image-print-layout/internal/model"
	"github.com/gillesmagneville/image-print-layout/internal/svgdoc"
)

// Services are what the application needs from the rendering engine. They are
// given by the program, so that the use cases are tested with a fake engine,
// without cgo.
type Services struct {
	// Load reads a document into the engine.
	Load engine.Loader
	// Match asks the system for the family it uses for a requested font; nil
	// when the system cannot be asked.
	Match engine.FontMatcher
}

// Document is what is known of an opened SVG: its analysis, its size as the
// engine reads it, and the blocks of the drawing as the engine measured them.
// It describes the file as it was when it was opened.
type Document struct {
	// Path is the absolute path of the file.
	Path string
	// SHA256 is the fingerprint of the file as it was read, in hexadecimal: a
	// project records it, to say later whether the document has changed.
	SHA256 string
	// Analysis is the reading of the source, with the findings and the
	// background.
	Analysis *svgdoc.Analysis
	// Size is the size of the document in document units, as the engine
	// resolves it: the unit of every position of the project.
	Size layout.Doc
	// Blocks are the measured blocks, in document order.
	Blocks []layout.Block
	// Findings are the analysis findings and the fonts that are missing.
	Findings []diag.Item
	// Format is the kind of file: an SVG, or the raster format of an image.
	Format engine.Format
	// Image says what the file of an image was; nil for an SVG, and for an
	// engine that does not tell.
	Image *engine.ImageInfo
	// Coarse tells that the document has too many blocks to measure each: the
	// blocks are the stretches of ink read from an image (MaxExactBlocks).
	Coarse bool

	// source is the file as it was read, kept for an image: it has no
	// analysis to load the working copy from again.
	source []byte
}

// IsImage tells that the document is a raster image, not a vector drawing.
func (d *Document) IsImage() bool { return d.Format.IsImage() }

// Opened is a document and the engine's copy of it. The engine document
// belongs to whoever drives the engine; the Document is plain data.
type Opened struct {
	Document *Document
	Engine   engine.Document
}

// Open reads an SVG file: it analyses the source, builds the working copy with
// the background instruction, loads it into the engine and measures the
// blocks. The file is only read. It is slow on a large document: callers run
// it off the thread of the window.
func Open(svc Services, path string, bg model.Background) (*Opened, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return Prepare(svc, src, abs, bg)
}

// Prepare is Open for a source already in memory.
//
// The kind of the file is told by its content, not by its name: an image is
// opened as a picture, anything else as an SVG.
func Prepare(svc Services, src []byte, path string, bg model.Background) (*Opened, error) {
	format := engine.Detect(src)
	if format.IsImage() {
		return prepareImage(svc, src, path, format)
	}
	a := svgdoc.Analyze(src, filepath.Dir(path))
	eng, err := load(svc, a, path, bg)
	if err != nil {
		if format == engine.FormatUnknown {
			// Neither an image nor, by its first bytes, an SVG: say so,
			// instead of only that it is not a well-formed SVG.
			return nil, diag.Fail("file.unrecognised", "reason", err.Error())
		}
		return nil, err
	}
	w, h := eng.Size()
	sum := sha256.Sum256(src)
	d := &Document{Path: path, SHA256: hex.EncodeToString(sum[:]), Format: engine.FormatSVG,
		Analysis: a, Size: layout.Doc{W: w, H: h}}
	d.Findings = append(d.Findings, a.Items...)
	if len(a.Blocks) <= MaxExactBlocks {
		d.Blocks = Measure(eng, a)
	} else {
		blocks, err := coarseBlocks(svc, a, eng, path, bg)
		if err != nil {
			eng.Close()
			return nil, err
		}
		d.Blocks, d.Coarse = blocks, true
		d.Findings = append(d.Findings, diag.New("svg.coarse_blocks", 0, "count", len(a.Blocks)))
	}
	if svc.Match != nil {
		d.Findings = append(d.Findings, svgdoc.CheckFonts(a.Fonts, svc.Match)...)
	}
	return &Opened{Document: d, Engine: eng}, nil
}

// Rebuild loads the working copy again with another background instruction, for
// the same analysis. The blocks do not change: the background is not one.
func Rebuild(svc Services, d *Document, bg model.Background) (engine.Document, error) {
	if d.Analysis == nil {
		// An image has no background to change: it is read again as it was.
		if d.source == nil {
			return nil, errors.New("app: the document has nothing to load again")
		}
		eng, err := svc.Load(d.source, d.Path)
		return eng, imageError(err)
	}
	return load(svc, d.Analysis, d.Path, bg)
}

func load(svc Services, a *svgdoc.Analysis, path string, bg model.Background) (engine.Document, error) {
	work, _, err := a.Compose(bg)
	if err != nil {
		return nil, fmt.Errorf("app: cannot prepare the working copy: %w", err)
	}
	return svc.Load(work, path)
}

// Measure asks the engine for the extent of every block of the analysis.
func Measure(eng engine.Document, a *svgdoc.Analysis) []layout.Block {
	ids := make([]string, len(a.Blocks))
	for i, b := range a.Blocks {
		ids[i] = b.ID
	}
	return BlocksFrom(a.Blocks, eng.MeasureBlocks(ids))
}

// BlocksFrom keeps the blocks that were measured and have a height.
func BlocksFrom(blocks []svgdoc.Block, measured map[string]engine.Box) []layout.Block {
	out := make([]layout.Block, 0, len(blocks))
	for _, b := range blocks {
		box, ok := measured[b.ID]
		if !ok || !(box.H > 0) {
			continue
		}
		out = append(out, layout.Block{ID: b.ID, Top: box.Y, Bottom: box.Y + box.H})
	}
	return out
}

// coarseBlocks finds the blocks of a document too large to measure by element.
// The engine document that is open is used when it has no background
// rectangle; otherwise the working copy is loaded again without it.
func coarseBlocks(svc Services, a *svgdoc.Analysis, eng engine.Document, path string, bg model.Background) ([]layout.Block, error) {
	if a.Background == nil || bg.Document == model.RemoveBackground {
		return inkBlocks(eng)
	}
	bare, err := load(svc, a, path, model.Background{Document: model.RemoveBackground})
	if err != nil {
		return nil, err
	}
	defer bare.Close()
	return inkBlocks(bare)
}

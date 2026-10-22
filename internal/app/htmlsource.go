// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"errors"

	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/export"
	"github.com/gillesmagneville/image-print-layout/internal/model"
)

// HTMLSource is the picture a page HTML carries of the document (ARCHITECTURE
// §6, "Page HTML"): the same the preview and the PDF are drawn from.
//
// For an SVG it is the working copy for the background the project asks for,
// with the images it links to written into it, as a page read away from the
// folder of the document could not find them. For a picture it is the file, when
// a browser shows it as the engine does, and what the engine draws otherwise
// (export.ImageSource); eng is that engine's copy of the document.
func (d *Document) HTMLSource(eng engine.Document, bg model.Background) (export.HTMLSource, error) {
	if d.IsImage() {
		if d.source == nil || eng == nil {
			return export.HTMLSource{}, errors.New("app: the document has nothing to write")
		}
		info := engine.ImageInfo{Format: d.Format}
		if d.Image != nil {
			info = *d.Image
		}
		return export.ImageSource(eng, info, d.source)
	}
	if d.Analysis == nil {
		return export.HTMLSource{}, errors.New("app: the document has nothing to write")
	}
	work, _, err := d.Analysis.ComposeStandalone(bg)
	if err != nil {
		return export.HTMLSource{}, err
	}
	return export.HTMLSource{MIME: "image/svg+xml", Data: work}, nil
}

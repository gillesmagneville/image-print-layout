// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"strings"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/i18n"
)

// documentFilters are the filters of the dialogs that ask for a document: all
// that can be opened (the one shown first), the SVG documents, and the images.
// The images are the formats this installation of gdk-pixbuf can read: a
// format it cannot read is not offered.
func documentFilters(tr *i18n.Translator) (filters *gio.ListStore, first *gtk.FileFilter) {
	add := func(f *gtk.FileFilter, format engine.Format) {
		f.AddMIMEType(format.MIME())
		for _, ext := range format.Extensions() {
			f.AddPattern("*" + ext)
			f.AddPattern("*" + strings.ToUpper(ext))
		}
	}
	all, svg, images := gtk.NewFileFilter(), gtk.NewFileFilter(), gtk.NewFileFilter()
	all.SetName(tr.T("dialog.filter.all"))
	svg.SetName(tr.T("dialog.filter.svg"))
	images.SetName(tr.T("dialog.filter.images"))
	add(all, engine.FormatSVG)
	add(svg, engine.FormatSVG)
	supported := engine.SupportedImageFormats()
	for _, format := range supported {
		add(all, format)
		add(images, format)
	}
	filters = gio.NewListStore(gtk.GTypeFileFilter)
	filters.Append(all.Object)
	filters.Append(svg.Object)
	if len(supported) > 0 {
		filters.Append(images.Object)
	}
	return filters, all
}

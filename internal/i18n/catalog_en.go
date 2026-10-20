// SPDX-License-Identifier: GPL-3.0-only

package i18n

var en = map[string]string{
	// Command line.
	"cli.flag.version":    "print the version and exit",
	"cli.flag.selftest":   "check that the rendering engine works (loads a built-in SVG, draws it and exports it as a PDF), then exit",
	"cli.flag.ready_file": "create this file once the window is shown and the document drawn, then carry on (tests, documentation)",
	"cli.error.no_gui":    "this program was built without a graphical interface (cgo disabled)",
	"cli.flag.lang":       "language of the interface: fr, en, es or de (default: the language of the system)",
	"cli.warning.runtime": "Warning: setting up the rendering engine: {error}",
	"cli.error.selftest":  "Failed: {error}",
	"cli.error.language":  "unknown language “{lang}”: choose fr, en, es or de",

	// Application.
	"app.tagline": "Layout, cutting and printing of vector documents",

	// Title bar and menu.
	"action.open_svg":         "Open an SVG…",
	"action.open_svg.tooltip": "Open an SVG document (Ctrl+O)",
	"menu.title":              "Main menu",
	"menu.language":           "Language",
	"menu.language.auto":      "System language",
	"menu.about":              "About",
	"menu.quit":               "Quit",

	// Left panel: settings.
	"panel.source.title":     "Source file",
	"panel.paper.title":      "Paper",
	"panel.margins.title":    "Margins",
	"panel.scale.title":      "Scale",
	"panel.placement.title":  "Placement",
	"panel.background.title": "Background",
	"panel.export.title":     "Export",
	"panel.soon":             "Available in a later version.",
	"source.none":            "No file open.",
	"source.file":            "File: {name}",
	"source.size":            "Size: {w} × {h} units",
	"source.size.mm":         "At 100%: {w} × {h} mm",

	// Centre: preview.
	"preview.empty.title": "No document",
	"preview.empty.hint":  "Open an SVG document to prepare its layout.",

	// Right panel: pages.
	"pages.title": "Pages",
	"pages.none":  "No page yet.",

	// Status bar.
	"status.ready":   "Ready.",
	"status.loading": "Loading {name}…",
	"status.loaded":  "{name}: {w} × {h} units.",

	// Dialogs.
	"dialog.open_svg.title":    "Open an SVG document",
	"dialog.filter.svg":        "SVG documents",
	"dialog.close":             "Close",
	"dialog.error.open":        "The document could not be opened",
	"dialog.error.open.detail": "“{path}” could not be opened: {reason}",

	// Diagnostics: the texts of the codes of internal/diag.
	"diag.paper.unknown":          "The paper format \"{format}\" does not exist.",
	"diag.paper.custom_invalid":   "A custom format needs a positive width and height (got {w} × {h} mm).",
	"diag.paper.orientation":      "The orientation \"{orientation}\" does not exist: choose portrait or landscape.",
	"diag.margins.invalid":        "Margins must be positive numbers or zero.",
	"diag.margins.no_usable":      "The margins leave no room on the sheet (usable area: {w} × {h} mm). Reduce them.",
	"diag.scale.invalid_width":    "Enter a width greater than zero.",
	"diag.scale.invalid_percent":  "Enter a percentage greater than zero.",
	"diag.scale.invalid":          "The scale is not valid: check the mode and the value entered.",
	"diag.doc.invalid_size":       "The document has no usable size ({w} × {h} units).",
	"diag.page.too_tall":          "The content is {h} mm tall for a usable area of {usable} mm: add a cut, or change the scale or the paper.",
	"diag.page.too_wide":          "The content is {w} mm wide for a usable area of {usable} mm: change the scale or the paper.",
	"diag.page.shifted":           "The content leaves the usable area because of its position: check the offset or the centring.",
	"diag.page.off_sheet":         "Part of the content is off the sheet: the export is not possible.",
	"diag.page.empty":             "No block of the drawing is on this page: it will be almost empty.",
	"diag.cut.out_of_range":       "A cut must lie inside the document.",
	"diag.cut.too_close":          "A cut must stay at least {min} mm from another cut and from the edge of the document.",
	"diag.cut.unknown":            "This cut no longer exists.",
	"diag.cut.invalid":            "The cuts are not valid: they must be in order and inside the document.",
	"diag.auto.no_cut_needed":     "The drawing fits on one page: no cut is needed.",
	"diag.auto.cut_crosses":       "Cut {n} crosses blocks of the drawing ({count}): no gap allowed avoiding them.",
	"diag.auto.cut_near":          "Cut {n} passes only {mm} mm from a block: move it if that is too tight.",
	"diag.auto.more_pages":        "{count} pages instead of {min}: that is what avoids crossing blocks.",
	"diag.correction.none":        "No proposal to apply.",
	"diag.project.not_a_project":  "This file is not a layout project.",
	"diag.project.too_new":        "This project was saved by a newer version of the program (format {version}; this one reads up to format {supported}). Update the program.",
	"diag.project.bad_version":    "The version number of the project is not valid ({version}).",
	"diag.project.invalid":        "The project contains a value that is not valid: \"{field}\".",
	"diag.project.source_changed": "The SVG document has changed since the project was saved. Cuts are in document units: they may no longer fall where you placed them.",
	"diag.project.size_changed":   "The size of the document has changed: {w} × {h} units, against {was_w} × {was_h} when saved.",
	"diag.project.cuts_outside":   "Cuts now outside the document: {count}. They will be removed if you confirm.",
}

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
}

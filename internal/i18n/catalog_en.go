// SPDX-License-Identifier: GPL-3.0-only

package i18n

var en = map[string]string{
	// Command line.
	"cli.flag.version":    "print the version and exit",
	"cli.flag.selftest":   "check that the rendering engine works (loads a built-in SVG, draws it and exports it as a PDF), then exit",
	"cli.flag.lang":       "language of the interface: fr, en, es or de (default: the language of the system)",
	"cli.warning.runtime": "Warning: setting up the rendering engine: {error}",
	"cli.error.selftest":  "Failed: {error}",
	"cli.error.language":  "unknown language “{lang}”: choose fr, en, es or de",
	"cli.gui.missing":     "svg-print-layout: the graphical interface is not there yet.",
}

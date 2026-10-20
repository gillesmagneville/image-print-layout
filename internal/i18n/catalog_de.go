// SPDX-License-Identifier: GPL-3.0-only

package i18n

var de = map[string]string{
	// Command line.
	"cli.flag.version":    "gibt die Version aus und beendet das Programm",
	"cli.flag.selftest":   "prüft, ob die Darstellungsengine funktioniert (lädt ein eingebautes SVG, zeichnet es und exportiert es als PDF), und beendet dann das Programm",
	"cli.flag.lang":       "Sprache der Oberfläche: fr, en, es oder de (Standard: die Sprache des Systems)",
	"cli.warning.runtime": "Warnung: Einrichtung der Darstellungsengine: {error}",
	"cli.error.selftest":  "Fehlgeschlagen: {error}",
	"cli.error.language":  "unbekannte Sprache „{lang}“: wählen Sie fr, en, es oder de",
	"cli.gui.missing":     "svg-print-layout: Die grafische Oberfläche ist noch nicht verfügbar.",
}

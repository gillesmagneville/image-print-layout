// SPDX-License-Identifier: GPL-3.0-only

package i18n

var de = map[string]string{
	// Command line.
	"cli.flag.version":    "gibt die Version aus und beendet das Programm",
	"cli.flag.selftest":   "prüft, ob die Darstellungsengine funktioniert (lädt ein eingebautes SVG, zeichnet es und exportiert es als PDF), und beendet dann das Programm",
	"cli.flag.ready_file": "legt diese Datei an, sobald das Fenster angezeigt und das Dokument gezeichnet ist, und läuft dann weiter (Tests, Dokumentation)",
	"cli.error.no_gui":    "dieses Programm wurde ohne grafische Oberfläche gebaut (cgo deaktiviert)",
	"cli.flag.lang":       "Sprache der Oberfläche: fr, en, es oder de (Standard: die Sprache des Systems)",
	"cli.warning.runtime": "Warnung: Einrichtung der Darstellungsengine: {error}",
	"cli.error.selftest":  "Fehlgeschlagen: {error}",
	"cli.error.language":  "unbekannte Sprache „{lang}“: wählen Sie fr, en, es oder de",

	// Anwendung.
	"app.tagline": "Seitenaufteilung, Zuschnitt und Druck von Vektordokumenten",

	// Titelleiste und Menü.
	"action.open_svg":         "SVG öffnen…",
	"action.open_svg.tooltip": "Ein SVG-Dokument öffnen (Strg+O)",
	"menu.title":              "Hauptmenü",
	"menu.language":           "Sprache",
	"menu.language.auto":      "Systemsprache",
	"menu.about":              "Info",
	"menu.quit":               "Beenden",

	// Linkes Feld: Einstellungen.
	"panel.source.title":     "Quelldatei",
	"panel.paper.title":      "Papier",
	"panel.margins.title":    "Ränder",
	"panel.scale.title":      "Maßstab",
	"panel.placement.title":  "Platzierung",
	"panel.background.title": "Hintergrund",
	"panel.export.title":     "Export",
	"panel.soon":             "In einer späteren Version verfügbar.",
	"source.none":            "Keine Datei geöffnet.",
	"source.file":            "Datei: {name}",
	"source.size":            "Größe: {w} × {h} Einheiten",
	"source.size.mm":         "Bei 100 %: {w} × {h} mm",

	// Mitte: Vorschau.
	"preview.empty.title": "Kein Dokument",
	"preview.empty.hint":  "Öffnen Sie ein SVG-Dokument, um die Seitenaufteilung vorzubereiten.",

	// Rechtes Feld: Seiten.
	"pages.title": "Seiten",
	"pages.none":  "Noch keine Seiten.",

	// Statusleiste.
	"status.ready":   "Bereit.",
	"status.loading": "{name} wird geladen…",
	"status.loaded":  "{name}: {w} × {h} Einheiten.",

	// Dialoge.
	"dialog.open_svg.title":    "SVG-Dokument öffnen",
	"dialog.filter.svg":        "SVG-Dokumente",
	"dialog.close":             "Schließen",
	"dialog.error.open":        "Das Dokument konnte nicht geöffnet werden",
	"dialog.error.open.detail": "„{path}“ konnte nicht geöffnet werden: {reason}",
}

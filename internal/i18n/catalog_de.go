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

	// Diagnostics: the texts of the codes of internal/diag.
	"diag.paper.unknown":          "Das Papierformat „{format}“ gibt es nicht.",
	"diag.paper.custom_invalid":   "Ein benutzerdefiniertes Format braucht eine positive Breite und Höhe (erhalten: {w} × {h} mm).",
	"diag.paper.orientation":      "Die Ausrichtung „{orientation}“ gibt es nicht: Wählen Sie Hoch- oder Querformat.",
	"diag.margins.invalid":        "Ränder müssen positive Zahlen oder null sein.",
	"diag.margins.no_usable":      "Die Ränder lassen keinen Platz auf dem Blatt (Nutzfläche: {w} × {h} mm). Verkleinern Sie sie.",
	"diag.scale.invalid_width":    "Geben Sie eine Breite größer als null ein.",
	"diag.scale.invalid_percent":  "Geben Sie einen Prozentwert größer als null ein.",
	"diag.scale.invalid":          "Der Maßstab ist ungültig: Prüfen Sie den Modus und den eingegebenen Wert.",
	"diag.doc.invalid_size":       "Das Dokument hat keine verwendbare Größe ({w} × {h} Einheiten).",
	"diag.page.too_tall":          "Der Inhalt ist {h} mm hoch bei {usable} mm Nutzfläche: Fügen Sie einen Schnitt hinzu oder ändern Sie Maßstab oder Format.",
	"diag.page.too_wide":          "Der Inhalt ist {w} mm breit bei {usable} mm Nutzfläche: Ändern Sie Maßstab oder Format.",
	"diag.page.shifted":           "Der Inhalt ragt wegen seiner Position aus der Nutzfläche: Prüfen Sie Versatz oder Zentrierung.",
	"diag.page.off_sheet":         "Ein Teil des Inhalts liegt außerhalb des Blatts: Der Export ist nicht möglich.",
	"diag.page.empty":             "Kein Block der Zeichnung liegt auf dieser Seite: Sie bleibt fast leer.",
	"diag.cut.out_of_range":       "Ein Schnitt muss innerhalb des Dokuments liegen.",
	"diag.cut.too_close":          "Ein Schnitt muss mindestens {min} mm von einem anderen Schnitt und vom Dokumentrand entfernt bleiben.",
	"diag.cut.unknown":            "Dieser Schnitt existiert nicht mehr.",
	"diag.cut.invalid":            "Die Schnitte sind ungültig: Sie müssen geordnet und innerhalb des Dokuments liegen.",
	"diag.auto.no_cut_needed":     "Die Zeichnung passt auf eine Seite: Kein Schnitt nötig.",
	"diag.auto.cut_crosses":       "Schnitt {n} durchquert Blöcke der Zeichnung ({count}): Keine Lücke erlaubte es, sie zu vermeiden.",
	"diag.auto.cut_near":          "Schnitt {n} verläuft nur {mm} mm von einem Block entfernt: Verschieben Sie ihn, wenn es zu knapp ist.",
	"diag.auto.more_pages":        "{count} Seiten statt {min}: So werden keine Blöcke durchquert.",
	"diag.correction.none":        "Kein Vorschlag zum Anwenden.",
	"diag.project.not_a_project":  "Diese Datei ist kein Layoutprojekt.",
	"diag.project.too_new":        "Dieses Projekt wurde von einer neueren Version des Programms gespeichert (Format {version}; diese Version liest bis Format {supported}). Aktualisieren Sie das Programm.",
	"diag.project.bad_version":    "Die Versionsnummer des Projekts ist ungültig ({version}).",
	"diag.project.invalid":        "Das Projekt enthält einen ungültigen Wert: „{field}“.",
	"diag.project.source_changed": "Das SVG-Dokument hat sich seit dem Speichern des Projekts geändert. Die Schnitte sind in Dokumenteinheiten angegeben: Sie liegen eventuell nicht mehr dort, wo Sie sie gesetzt haben.",
	"diag.project.size_changed":   "Die Größe des Dokuments hat sich geändert: {w} × {h} Einheiten statt {was_w} × {was_h} beim Speichern.",
	"diag.project.cuts_outside":   "Schnitte, die jetzt außerhalb des Dokuments liegen: {count}. Sie werden entfernt, wenn Sie bestätigen.",
}

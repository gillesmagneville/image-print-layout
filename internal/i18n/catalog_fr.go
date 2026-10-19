// SPDX-License-Identifier: GPL-3.0-only

package i18n

// fr is the reference catalog. The keys are grouped by the part of the
// program that shows them.
var fr = map[string]string{
	// Command line.
	"cli.flag.version":    "affiche la version et quitte",
	"cli.flag.selftest":   "vérifie que le moteur de rendu fonctionne (charge un SVG intégré, le dessine et l'exporte en PDF) puis quitte",
	"cli.flag.ready_file": "écrit ce fichier quand la fenêtre est affichée et le document dessiné, puis continue (tests, documentation)",
	"cli.error.no_gui":    "ce programme a été compilé sans interface graphique (cgo désactivé)",
	"cli.flag.lang":       "langue de l'interface : fr, en, es ou de (par défaut, celle du système)",
	"cli.warning.runtime": "Avertissement : configuration du moteur de rendu : {error}",
	"cli.error.selftest":  "Échec : {error}",
	"cli.error.language":  "langue inconnue « {lang} » : choisissez fr, en, es ou de",

	// Application.
	"app.tagline": "Mise en page, découpage et impression de documents vectoriels",

	// Barre de titre et menu.
	"action.open_svg":         "Ouvrir un SVG…",
	"action.open_svg.tooltip": "Ouvrir un document SVG (Ctrl+O)",
	"menu.title":              "Menu principal",
	"menu.language":           "Langue",
	"menu.language.auto":      "Langue du système",
	"menu.about":              "À propos",
	"menu.quit":               "Quitter",

	// Panneau de gauche : paramètres.
	"panel.source.title":     "Fichier source",
	"panel.paper.title":      "Papier",
	"panel.margins.title":    "Marges",
	"panel.scale.title":      "Échelle",
	"panel.placement.title":  "Positionnement",
	"panel.background.title": "Fond",
	"panel.export.title":     "Export",
	"panel.soon":             "Disponible dans une prochaine version.",
	"source.none":            "Aucun fichier ouvert.",
	"source.file":            "Fichier : {name}",
	"source.size":            "Taille : {w} × {h} unités",
	"source.size.mm":         "À 100 % : {w} × {h} mm",

	// Zone centrale : aperçu.
	"preview.empty.title": "Aucun document",
	"preview.empty.hint":  "Ouvrez un document SVG pour préparer sa mise en page.",

	// Panneau de droite : pages.
	"pages.title": "Pages",
	"pages.none":  "Aucune page pour l'instant.",

	// Barre d'état.
	"status.ready":   "Prêt.",
	"status.loading": "Chargement de {name}…",
	"status.loaded":  "{name} : {w} × {h} unités.",

	// Boîtes de dialogue.
	"dialog.open_svg.title":    "Ouvrir un document SVG",
	"dialog.filter.svg":        "Documents SVG",
	"dialog.close":             "Fermer",
	"dialog.error.open":        "Impossible d'ouvrir le document",
	"dialog.error.open.detail": "« {path} » n'a pas pu être ouvert : {reason}",
}

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

	// Diagnostics: the texts of the codes of internal/diag.
	"diag.paper.unknown":          "Le format de papier « {format} » n'existe pas.",
	"diag.paper.custom_invalid":   "Le format personnalisé doit avoir une largeur et une hauteur positives (reçu : {w} × {h} mm).",
	"diag.paper.orientation":      "L'orientation « {orientation} » n'existe pas : choisissez portrait ou paysage.",
	"diag.margins.invalid":        "Les marges doivent être des nombres positifs ou nuls.",
	"diag.margins.no_usable":      "Les marges ne laissent aucune place sur la feuille (zone utile : {w} × {h} mm). Réduisez-les.",
	"diag.scale.invalid_width":    "Saisissez une largeur supérieure à zéro.",
	"diag.scale.invalid_percent":  "Saisissez un pourcentage supérieur à zéro.",
	"diag.scale.invalid":          "L'échelle n'est pas valable : vérifiez le mode et la valeur saisie.",
	"diag.doc.invalid_size":       "Le document n'a pas de taille exploitable ({w} × {h} unités).",
	"diag.page.too_tall":          "Le contenu mesure {h} mm de haut pour {usable} mm de zone utile : ajoutez une coupure, ou changez l'échelle ou le format.",
	"diag.page.too_wide":          "Le contenu mesure {w} mm de large pour {usable} mm de zone utile : changez l'échelle ou le format.",
	"diag.page.shifted":           "Le contenu sort de la zone utile à cause de sa position : vérifiez le décalage ou le centrage.",
	"diag.page.off_sheet":         "Une partie du contenu est hors de la feuille : l'export est impossible.",
	"diag.page.empty":             "Aucun bloc du dessin ne se trouve sur cette page : elle sera presque vide.",
	"diag.cut.out_of_range":       "Une coupure doit se trouver à l'intérieur du document.",
	"diag.cut.too_close":          "Une coupure doit rester à {min} mm au moins d'une autre coupure et du bord du document.",
	"diag.cut.unknown":            "Cette coupure n'existe plus.",
	"diag.cut.invalid":            "Les coupures ne sont pas valables : elles doivent être dans l'ordre et à l'intérieur du document.",
	"diag.auto.no_cut_needed":     "Le dessin tient sur une seule page : aucune coupure n'est nécessaire.",
	"diag.auto.cut_crosses":       "La coupure {n} traverse des blocs du dessin ({count}) : aucun vide ne permettait de l'éviter.",
	"diag.auto.cut_near":          "La coupure {n} passe à {mm} mm seulement d'un bloc : déplacez-la si c'est trop serré.",
	"diag.auto.more_pages":        "{count} pages au lieu de {min} : c'est ce qui évite de traverser des blocs.",
	"diag.correction.none":        "Aucune proposition à appliquer.",
	"diag.project.not_a_project":  "Ce fichier n'est pas un projet de mise en page.",
	"diag.project.too_new":        "Ce projet a été enregistré par une version plus récente du programme (format {version} ; celui-ci lit jusqu'au format {supported}). Mettez le programme à jour.",
	"diag.project.bad_version":    "Le numéro de version du projet n'est pas valable ({version}).",
	"diag.project.invalid":        "Le projet contient une valeur qui n'est pas valable : « {field} ».",
	"diag.project.source_changed": "Le document SVG a changé depuis l'enregistrement du projet. Les coupures sont en unités du document : elles peuvent ne plus tomber où vous les aviez placées.",
	"diag.project.size_changed":   "La taille du document a changé : {w} × {h} unités, contre {was_w} × {was_h} à l'enregistrement.",
	"diag.project.cuts_outside":   "Coupures désormais hors du document : {count}. Elles seront retirées si vous confirmez.",
}

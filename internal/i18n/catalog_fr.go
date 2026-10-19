// SPDX-License-Identifier: GPL-3.0-only

package i18n

// fr is the reference catalog. The keys are grouped by the part of the
// program that shows them.
var fr = map[string]string{
	// Command line.
	"cli.flag.version":    "affiche la version et quitte",
	"cli.flag.selftest":   "vérifie que le moteur de rendu fonctionne (charge un SVG intégré, le dessine et l'exporte en PDF) puis quitte",
	"cli.flag.lang":       "langue de l'interface : fr, en, es ou de (par défaut, celle du système)",
	"cli.warning.runtime": "Avertissement : configuration du moteur de rendu : {error}",
	"cli.error.selftest":  "Échec : {error}",
	"cli.error.language":  "langue inconnue « {lang} » : choisissez fr, en, es ou de",
	"cli.gui.missing":     "svg-print-layout : l'interface graphique n'est pas encore là.",
}

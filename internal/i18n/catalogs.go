// SPDX-License-Identifier: GPL-3.0-only

package i18n

import "strings"

// catalogs holds the texts of every language. French is the reference: the
// others are translated from it, and a missing text falls back to English,
// then to French.
var catalogs = map[Lang]map[string]string{
	French:  fr,
	English: en,
	Spanish: es,
	German:  de,
}

// The French catalog is written with plain spaces, and typography is applied
// once at start: a no-break space before « : » and inside the guillemets, a
// narrow no-break space before « ; », « ? » and « ! ». Without it, a label
// can break a line before its colon.
func init() {
	typography := strings.NewReplacer(
		" :", " :",
		" ;", " ;",
		" ?", " ?",
		" !", " !",
		"« ", "« ",
		" »", " »",
	)
	for key, text := range fr {
		fr[key] = typography.Replace(text)
	}
}

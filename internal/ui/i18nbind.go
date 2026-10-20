// SPDX-License-Identifier: GPL-3.0-only

package ui

import "github.com/gillesmagneville/svg-print-layout/internal/i18n"

// translations is the list of the texts of the window. Each widget that shows
// a text registers a function that sets it; changing the language runs them
// all again, so the window follows the language without being built twice.
type translations struct {
	tr    *i18n.Translator
	funcs []func(*i18n.Translator)
}

// bind runs set now and again at every change of language.
func (t *translations) bind(set func(*i18n.Translator)) {
	t.funcs = append(t.funcs, set)
	set(t.tr)
}

// switchTo changes the language and runs every registered function.
func (t *translations) switchTo(l i18n.Lang) {
	t.tr = i18n.New(l)
	for _, set := range t.funcs {
		set(t.tr)
	}
}

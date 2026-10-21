// SPDX-License-Identifier: GPL-3.0-only

package ui

import (
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/i18n"
)

// Every function that sets a text runs once when it is registered, and again
// at every change of language, with the translator of the new language.
func TestTranslationsRunEverySetterAtEveryChange(t *testing.T) {
	texts := translations{tr: i18n.New(i18n.French)}
	var a, b string
	texts.bind(func(tr *i18n.Translator) { a = tr.T("pages.title") })
	texts.bind(func(tr *i18n.Translator) { b = tr.T("menu.quit") })
	if a != "Pages" || b != "Quitter" {
		t.Fatalf("after bind: %q, %q", a, b)
	}
	for lang, want := range map[i18n.Lang][2]string{
		i18n.German:  {"Seiten", "Beenden"},
		i18n.Spanish: {"Páginas", "Salir"},
		i18n.English: {"Pages", "Quit"},
	} {
		texts.switchTo(lang)
		if a != want[0] || b != want[1] {
			t.Errorf("%s: %q, %q; want %q, %q", lang, a, b, want[0], want[1])
		}
		if texts.tr.Lang() != lang {
			t.Errorf("the translator is in %s, want %s", texts.tr.Lang(), lang)
		}
	}
}

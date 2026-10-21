// SPDX-License-Identifier: GPL-3.0-only

package repocheck

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The program speaks four languages (ARCHITECTURE.md, "Langues"), and so do
// the files the system reads to present it: the menu entry, the software
// centre page and the manual. English is the fallback of each; the others must
// not be forgotten when a text changes.
var otherLanguages = []string{"fr", "es", "de"}

// The menu entry (.desktop): each translatable key has a line per language.
func TestDesktopEntryIsTranslated(t *testing.T) {
	root := repoRoot(t)
	desktop := read(t, filepath.Join(root, "metainfo", "io.github.gillesmagneville.imageprintlayout.desktop"))
	for _, key := range []string{"GenericName", "Comment", "Keywords"} {
		if !strings.Contains(desktop, "\n"+key+"=") {
			t.Errorf("the .desktop file has no %s in English (the fallback)", key)
		}
		for _, lang := range otherLanguages {
			if !strings.Contains(desktop, "\n"+key+"["+lang+"]=") {
				t.Errorf("the .desktop file has no %s[%s]", key, lang)
			}
		}
	}
}

// The software centre page (AppStream): summary, description paragraphs and
// the description of each release, in every language, as many paragraphs in
// each language as in English.
func TestAppStreamIsTranslated(t *testing.T) {
	root := repoRoot(t)
	xml := read(t, filepath.Join(root, "metainfo", "io.github.gillesmagneville.imageprintlayout.metainfo.xml"))

	count := func(element, lang string) int {
		var re *regexp.Regexp
		if lang == "" {
			re = regexp.MustCompile(`<` + element + `>`)
		} else {
			re = regexp.MustCompile(`<` + element + ` xml:lang="` + lang + `">`)
		}
		return len(re.FindAllString(xml, -1))
	}
	for _, element := range []string{"summary", "p"} {
		english := count(element, "")
		if english == 0 {
			t.Errorf("AppStream: no <%s> in English (the fallback)", element)
		}
		for _, lang := range otherLanguages {
			if got := count(element, lang); got != english {
				t.Errorf("AppStream: %d <%s> in English but %d in %s", english, element, got, lang)
			}
		}
	}
}

// Every language has its manual page, and it is not the English one copied.
func TestManualPagesExistInEveryLanguage(t *testing.T) {
	root := repoRoot(t)
	english := read(t, filepath.Join(root, "docs", "imageprintlayout.1"))
	for _, lang := range otherLanguages {
		page := read(t, filepath.Join(root, "docs", lang, "imageprintlayout.1"))
		if page == english {
			t.Errorf("docs/%s/imageprintlayout.1 is the English page", lang)
		}
	}
}

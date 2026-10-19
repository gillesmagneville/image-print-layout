// SPDX-License-Identifier: GPL-3.0-only

package i18n

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var placeholder = regexp.MustCompile(`\{([^{}]*)\}`)

func placeholders(text string) []string {
	var names []string
	for _, m := range placeholder.FindAllStringSubmatch(text, -1) {
		names = append(names, m[1])
	}
	sort.Strings(names)
	return names
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Every language has the keys of the reference, no more and no fewer: a
// message missing from one language would show in French or English inside a
// German window.
func TestEveryLanguageHasTheSameKeys(t *testing.T) {
	reference := catalogs[French]
	if len(reference) == 0 {
		t.Fatal("the French catalog is empty")
	}
	for _, lang := range Languages {
		catalog, ok := catalogs[lang]
		if !ok {
			t.Errorf("no catalog for %s", lang)
			continue
		}
		for _, key := range sortedKeys(reference) {
			if _, ok := catalog[key]; !ok {
				t.Errorf("%s: missing %q", lang, key)
			}
		}
		for _, key := range sortedKeys(catalog) {
			if _, ok := reference[key]; !ok {
				t.Errorf("%s: %q is not in the French catalog", lang, key)
			}
		}
	}
}

// A text is not empty, has no space at its ends and no unmatched brace.
func TestTextsAreWellFormed(t *testing.T) {
	for _, lang := range Languages {
		for key, text := range catalogs[lang] {
			switch {
			case strings.TrimSpace(text) == "":
				t.Errorf("%s: %q is empty", lang, key)
			case text != strings.TrimSpace(text):
				t.Errorf("%s: %q starts or ends with a space", lang, key)
			case strings.Count(text, "{") != strings.Count(text, "}"):
				t.Errorf("%s: %q has an unmatched brace: %q", lang, key, text)
			}
		}
	}
}

// A translation uses the same placeholders as the reference: one that drops
// {path} would hide the value, and one that invents {file} would show the
// braces.
func TestTranslationsKeepThePlaceholders(t *testing.T) {
	for key, text := range catalogs[French] {
		want := strings.Join(placeholders(text), ",")
		for _, lang := range Languages[1:] {
			got, ok := catalogs[lang][key]
			if !ok {
				continue
			}
			if have := strings.Join(placeholders(got), ","); have != want {
				t.Errorf("%s: %q has the placeholders {%s}, the French text has {%s}", lang, key, have, want)
			}
		}
	}
}

// A count has both forms, ".one" and ".other", and {n} in each.
func TestPluralKeysComeInPairs(t *testing.T) {
	for _, lang := range Languages {
		for key, text := range catalogs[lang] {
			var other string
			switch {
			case strings.HasSuffix(key, ".one"):
				other = strings.TrimSuffix(key, ".one") + ".other"
			case strings.HasSuffix(key, ".other"):
				other = strings.TrimSuffix(key, ".other") + ".one"
			default:
				continue
			}
			if _, ok := catalogs[lang][other]; !ok {
				t.Errorf("%s: %q has no %q", lang, key, other)
			}
			if !strings.Contains(text, "{n}") && strings.HasSuffix(key, ".other") {
				t.Errorf("%s: %q does not show {n}", lang, key)
			}
		}
	}
}

// French typography: the colon and the guillemets are not separated from
// their word by a space that can break the line.
func TestFrenchTypography(t *testing.T) {
	for key, text := range catalogs[French] {
		for _, bad := range []string{" :", " ;", " ?", " !", "« ", " »"} {
			if strings.Contains(text, bad) {
				t.Errorf("fr: %q still has %q with a plain space: %q", key, bad, text)
			}
		}
	}
	if got := New(French).T("cli.warning.runtime", "error", "x"); !strings.Contains(got, "Avertissement :") {
		t.Errorf("no no-break space before the colon: %q", got)
	}
}

var (
	// Calls in the code: tr.T("key", ...) and tr.N("key", n, ...).
	keyInCode = regexp.MustCompile(`\.(T|N)\(\s*"([a-z][a-z0-9_]*(?:\.[a-z0-9_]+)+)"`)
	// Lines of comments quote examples that are not keys.
	commentLine = regexp.MustCompile(`^\s*//`)
)

// Every key the code asks for exists. A typo in a key would show the key in
// the window; this finds it without running the program.
func TestKeysUsedInTheCodeExist(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(text), "\n") {
			if commentLine.MatchString(line) {
				continue
			}
			for _, m := range keyInCode.FindAllStringSubmatch(line, -1) {
				found++
				keys := []string{m[2]}
				if m[1] == "N" {
					keys = []string{m[2] + ".one", m[2] + ".other"}
				}
				for _, key := range keys {
					if _, ok := catalogs[French][key]; !ok {
						rel, _ := filepath.Rel(root, path)
						t.Errorf("%s asks for %q, which no catalog has", filepath.ToSlash(rel), key)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Error("no call of T or N found in the code: the test looks in the wrong place")
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Lang
		ok   bool
	}{
		{"fr", French, true},
		{"fr_FR.UTF-8", French, true},
		{"fr-CA", French, true},
		{"de_DE@euro", German, true},
		{"EN-us", English, true},
		{"es_419", Spanish, true},
		{"  es  ", Spanish, true},
		{"C", "", false},
		{"C.UTF-8", "", false},
		{"POSIX", "", false},
		{"it_IT.UTF-8", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := Parse(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("Parse(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func env(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestFromEnvironment(t *testing.T) {
	cases := []struct {
		name string
		vars map[string]string
		want Lang
		ok   bool
	}{
		{"LANG", map[string]string{"LANG": "de_DE.UTF-8"}, German, true},
		{"LC_ALL wins over LANG", map[string]string{"LC_ALL": "es_ES.UTF-8", "LANG": "de_DE.UTF-8"}, Spanish, true},
		{"LANGUAGE wins over all", map[string]string{"LANGUAGE": "fr", "LC_ALL": "es_ES.UTF-8", "LANG": "de_DE.UTF-8"}, French, true},
		{"LANGUAGE is a list", map[string]string{"LANGUAGE": "it:pt:es:fr"}, Spanish, true},
		{"C locale, then LANG", map[string]string{"LC_ALL": "C", "LANG": "fr_FR"}, French, true},
		{"nothing known", map[string]string{"LANG": "C.UTF-8"}, "", false},
		{"nothing", map[string]string{}, "", false},
	}
	for _, c := range cases {
		got, ok := FromEnvironment(env(c.vars))
		if got != c.want || ok != c.ok {
			t.Errorf("%s: FromEnvironment = %q, %v; want %q, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestResolve(t *testing.T) {
	// The setting of the user wins over the system.
	for _, lang := range Languages {
		if got := Resolve(string(lang), env(map[string]string{"LANG": "de_DE"})); got != lang {
			t.Errorf("Resolve(%q) = %q", lang, got)
		}
	}
	// "auto", empty and unknown follow the system.
	for _, setting := range []string{Auto, "", "klingon"} {
		if got := Resolve(setting, env(map[string]string{"LANG": "es_ES.UTF-8"})); got != Spanish {
			t.Errorf("Resolve(%q) with LANG=es_ES = %q, want es", setting, got)
		}
	}
	// With nothing in the environment the answer comes from the platform (the
	// language of Windows) or is Default; either way it is one of the four.
	if got := Resolve(Auto, env(nil)); !got.Valid() {
		t.Errorf("Resolve(auto) with no environment = %q, not a language", got)
	}
}

func TestTranslate(t *testing.T) {
	fr, en := New(French), New(English)
	if got, want := fr.T("cli.error.language", "lang", "xx"), "langue inconnue « xx » : choisissez fr, en, es ou de"; strings.ReplaceAll(got, " ", " ") != strings.ReplaceAll(want, " ", " ") {
		t.Errorf("French: %q", got)
	}
	if got, want := en.T("cli.error.language", "lang", "xx"), "unknown language “xx”: choose fr, en, es or de"; got != want {
		t.Errorf("English: %q, want %q", got, want)
	}
	// A key that does not exist is shown as it is; a placeholder without a
	// value stays visible.
	if got := en.T("no.such.key"); got != "no.such.key" {
		t.Errorf("missing key: %q", got)
	}
	if got := en.T("cli.warning.runtime"); !strings.Contains(got, "{error}") {
		t.Errorf("a placeholder without a value should stay visible: %q", got)
	}
	// An invalid language is English.
	if got := New("xx").Lang(); got != English {
		t.Errorf("New(xx).Lang() = %q", got)
	}
}

// A language that misses a text is completed from English, then French.
func TestFallback(t *testing.T) {
	const key = "test.only.in.french"
	fr[key] = "seulement en français"
	defer delete(fr, key)
	if got := New(German).T(key); got != "seulement en français" {
		t.Errorf("German fallback = %q", got)
	}
	en[key] = "only in English"
	defer delete(en, key)
	if got := New(German).T(key); got != "only in English" {
		t.Errorf("German fallback = %q, want the English text", got)
	}
}

func TestPlural(t *testing.T) {
	const key = "test.pages"
	for lang, texts := range map[Lang][2]string{
		French:  {"{n} page", "{n} pages"},
		English: {"{n} page", "{n} pages"},
		Spanish: {"{n} página", "{n} páginas"},
		German:  {"{n} Seite", "{n} Seiten"},
	} {
		catalogs[lang][key+".one"], catalogs[lang][key+".other"] = texts[0], texts[1]
		defer delete(catalogs[lang], key+".one")
		defer delete(catalogs[lang], key+".other")
	}
	cases := []struct {
		lang Lang
		n    int
		want string
	}{
		{French, 0, "0 page"}, // French counts zero as singular.
		{French, 1, "1 page"},
		{French, 2, "2 pages"},
		{English, 0, "0 pages"},
		{English, 1, "1 page"},
		{English, 4, "4 pages"},
		{Spanish, 1, "1 página"},
		{Spanish, 0, "0 páginas"},
		{German, 1, "1 Seite"},
		{German, 12, "12 Seiten"},
	}
	for _, c := range cases {
		if got := New(c.lang).N(key, c.n); got != c.want {
			t.Errorf("%s N(%d) = %q, want %q", c.lang, c.n, got, c.want)
		}
	}
}

func TestNumber(t *testing.T) {
	cases := []struct {
		lang     Lang
		v        float64
		decimals int
		want     string
	}{
		{English, 200, -1, "200"},
		{English, 0.15625, -1, "0.15625"},
		{French, 0.15625, -1, "0,15625"},
		{German, 856.0625, 1, "856,1"},
		{Spanish, 277, 2, "277,00"},
		{English, 856.0625, 1, "856.1"},
		{French, -0.0000001, -1, "0"},
		{French, 12.3456789, -1, "12,345679"},
		{French, 1280, -1, "1280"},
	}
	for _, c := range cases {
		if got := New(c.lang).Number(c.v, c.decimals); got != c.want {
			t.Errorf("%s Number(%v, %d) = %q, want %q", c.lang, c.v, c.decimals, got, c.want)
		}
	}
	// A float64 given to T is written the same way.
	const key = "test.mm"
	fr[key], en[key] = "{w} mm", "{w} mm"
	defer delete(fr, key)
	defer delete(en, key)
	if got := New(French).T(key, "w", 0.5); got != "0,5 mm" {
		t.Errorf("French float in T: %q", got)
	}
	if got := New(English).T(key, "w", 0.5); got != "0.5 mm" {
		t.Errorf("English float in T: %q", got)
	}
}

func TestParseNumber(t *testing.T) {
	good := map[string]float64{
		"200":     200,
		"200.5":   200.5,
		"200,5":   200.5,
		" 12 ":    12,
		"1 280":   1280,
		"1 280,5": 1280.5,
		"-3,25":   -3.25,
		"0,15625": 0.15625,
		"1e2":     100,
		".5":      0.5,
		",5":      0.5,
		"5,":      5,
	}
	for in, want := range good {
		got, err := ParseNumber(in)
		if err != nil || got != want {
			t.Errorf("ParseNumber(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "1,234.5", "1,2,3", "12 mm", "--1"} {
		if got, err := ParseNumber(in); err == nil {
			t.Errorf("ParseNumber(%q) = %v, want an error", in, got)
		}
	}
}

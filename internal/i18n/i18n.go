// SPDX-License-Identifier: GPL-3.0-only

// Package i18n translates the interface into French, English, Spanish and
// German, and formats and reads numbers the way each of them writes them.
//
// A message is found by its key ("panel.paper.title"); the four catalogs
// (catalog_fr.go ... catalog_de.go) hold the same keys, and a test fails when
// one language misses a key, an empty text, a placeholder or a plural form.
// A text names its values with placeholders — "{count}", "{path}" — so that a
// translation can put them in the order its language needs.
//
// Pure Go, no cgo and no files to install: the catalogs are compiled in.
package i18n

import (
	"fmt"
	"strconv"
	"strings"
)

// Lang is a language of the interface, by its ISO 639-1 code.
type Lang string

const (
	French  Lang = "fr"
	English Lang = "en"
	Spanish Lang = "es"
	German  Lang = "de"
)

// Languages lists the languages of the interface, in the order of the
// language menu.
var Languages = []Lang{French, English, Spanish, German}

// Default is the language used when the language of the system is none of the
// four, and the first one tried when a language misses a message.
const Default = English

// Name is the name of the language in the language itself, as the language
// menu shows it: a person who cannot read the current language still finds
// their own.
func (l Lang) Name() string {
	switch l {
	case French:
		return "Français"
	case English:
		return "English"
	case Spanish:
		return "Español"
	case German:
		return "Deutsch"
	}
	return string(l)
}

// Valid reports whether l is one of the four languages.
func (l Lang) Valid() bool {
	for _, known := range Languages {
		if l == known {
			return true
		}
	}
	return false
}

// Parse reads a language from a locale name as systems write it — "fr",
// "fr_FR", "fr_FR.UTF-8", "de_DE@euro", "es-419", "EN-us" — and reports
// whether it is one of the four. "C", "POSIX" and unknown languages are not.
func Parse(name string) (Lang, bool) {
	name = strings.TrimSpace(name)
	if i := strings.IndexAny(name, "_-.@"); i >= 0 {
		name = name[:i]
	}
	l := Lang(strings.ToLower(name))
	if !l.Valid() {
		return "", false
	}
	return l, true
}

// FromEnvironment finds the language of the system the way gettext does: the
// first of LANGUAGE (a colon-separated list), LC_ALL, LC_MESSAGES and LANG
// that names one of the four languages. getenv is os.Getenv in the program.
func FromEnvironment(getenv func(string) string) (Lang, bool) {
	for _, list := range []string{getenv("LANGUAGE"), getenv("LC_ALL"), getenv("LC_MESSAGES"), getenv("LANG")} {
		for _, name := range strings.Split(list, ":") {
			if l, ok := Parse(name); ok {
				return l, true
			}
		}
	}
	return "", false
}

// Auto is the setting that follows the language of the system.
const Auto = "auto"

// Resolve turns the language setting of the user into a language: a language
// code is taken as it is; Auto, an empty or an unknown setting follows the
// system, and the system in a language that is not one of the four gives
// Default.
func Resolve(setting string, getenv func(string) string) Lang {
	if l := Lang(setting); l.Valid() {
		return l
	}
	if l, ok := FromEnvironment(getenv); ok {
		return l
	}
	if l, ok := systemLanguage(); ok {
		return l
	}
	return Default
}

// Translator gives the texts of one language.
type Translator struct {
	lang Lang
}

// New returns the translator of a language; a language that is not one of the
// four gives Default.
func New(l Lang) *Translator {
	if !l.Valid() {
		l = Default
	}
	return &Translator{lang: l}
}

// Lang is the language of the translator.
func (t *Translator) Lang() Lang { return t.lang }

// lookup finds a text in the language, then in Default, then in French, whose
// catalog is the reference.
func (t *Translator) lookup(key string) (string, bool) {
	for _, l := range []Lang{t.lang, Default, French} {
		if text, ok := catalogs[l][key]; ok {
			return text, true
		}
	}
	return "", false
}

// T is the text of a key. args are pairs of a placeholder name and its value:
//
//	t.T("export.done", "path", "/home/a/plan.pdf")
//
// The values of float64 are written with the decimal separator of the
// language, with as many decimals as the number needs (Number gives the
// number of decimals). A key that no catalog has gives the key itself, which
// shows in the interface as a mistake to fix; a test looks for those.
func (t *Translator) T(key string, args ...any) string {
	text, ok := t.lookup(key)
	if !ok {
		return key
	}
	return t.expand(text, args)
}

// N is the text of a key whose wording depends on a count: it reads the key
// followed by ".one" or ".other", whichever the language uses for n, and
// offers n to the text as {n}.
//
//	t.N("pages.count", 3) // "pages.count.other": "{n} pages"
func (t *Translator) N(key string, n int, args ...any) string {
	form := ".other"
	if t.isSingular(n) {
		form = ".one"
	}
	return t.T(key+form, append([]any{"n", n}, args...)...)
}

// isSingular tells whether the language writes n with the singular. French
// counts 0 as singular (« 0 page »); English, Spanish and German do not.
func (t *Translator) isSingular(n int) bool {
	if t.lang == French {
		return n == 0 || n == 1
	}
	return n == 1 || n == -1
}

// expand replaces {name} by the value that args gives for it. A placeholder
// without a value is left as it is, so that the omission shows.
func (t *Translator) expand(text string, args []any) string {
	if !strings.Contains(text, "{") {
		return text
	}
	values := make(map[string]string, len(args)/2)
	for i := 0; i+1 < len(args); i += 2 {
		name, _ := args[i].(string)
		values[name] = t.format(args[i+1])
	}
	var out strings.Builder
	for {
		open := strings.IndexByte(text, '{')
		if open < 0 {
			out.WriteString(text)
			return out.String()
		}
		end := strings.IndexByte(text[open:], '}')
		if end < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:open])
		name := text[open+1 : open+end]
		if v, ok := values[name]; ok {
			out.WriteString(v)
		} else {
			out.WriteString(text[open : open+end+1])
		}
		text = text[open+end+1:]
	}
}

func (t *Translator) format(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case float64:
		return t.Number(v, -1)
	case float32:
		return t.Number(float64(v), -1)
	case int:
		return strconv.Itoa(v)
	case fmt.Stringer:
		return v.String()
	}
	return fmt.Sprint(v)
}

// Number writes a number with the decimal separator of the language: a comma
// in French, Spanish and German, a point in English. decimals is the number
// of decimals; a negative value writes as many as the number needs (at most
// 6, and none for a whole number).
func (t *Translator) Number(v float64, decimals int) string {
	var s string
	if decimals < 0 {
		s = strconv.FormatFloat(v, 'f', 6, 64)
		if strings.Contains(s, ".") {
			s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		}
	} else {
		s = strconv.FormatFloat(v, 'f', decimals, 64)
	}
	if s == "-0" {
		s = "0"
	}
	if t.lang != English {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}

// ParseNumber reads a number typed by a person, whatever their language: the
// decimal separator may be a point or a comma, and spaces — also the
// no-break ones that French writes between thousands — are ignored. A number
// with a comma and a point ("1,234.5") is refused: it is ambiguous.
func ParseNumber(s string) (float64, error) {
	s = strings.NewReplacer(" ", "", " ", "", " ", "", " ", "").Replace(strings.TrimSpace(s))
	if strings.Contains(s, ",") {
		if strings.Contains(s, ".") || strings.Count(s, ",") > 1 {
			return 0, fmt.Errorf("i18n: %q is not a number", s)
		}
		s = strings.Replace(s, ",", ".", 1)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("i18n: %q is not a number", s)
	}
	return v, nil
}

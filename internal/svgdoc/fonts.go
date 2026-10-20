// SPDX-License-Identifier: GPL-3.0-only

package svgdoc

import (
	"strings"

	"github.com/gillesmagneville/svg-print-layout/internal/diag"
)

// generic lists the CSS generic families: the system always has one for them.
var generic = map[string]bool{
	"serif": true, "sans-serif": true, "monospace": true, "cursive": true, "fantasy": true,
	"system-ui": true, "ui-serif": true, "ui-sans-serif": true, "ui-monospace": true, "ui-rounded": true,
	"emoji": true, "math": true, "fangsong": true,
}

// Families splits a font-family value into its family names, in order, without
// quotes. "IBM Plex Sans, 'Arial', sans-serif" gives three names.
func Families(stack string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	flush := func() {
		name := strings.TrimSpace(cur.String())
		cur.Reset()
		if name != "" {
			out = append(out, name)
		}
	}
	for _, r := range stack {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ',':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// CheckFonts compares the fonts a document asks for with the ones the system
// has. match gives the family the system uses for a requested one (the same
// name when it is installed). For each stack whose first family is not
// installed, it reports the family that will be drawn instead: the export
// does not warn about it, and the PDF embeds the replacement.
//
// A stack that starts with a generic family (sans-serif) is never reported:
// the document did not ask for anything particular.
func CheckFonts(stacks []string, match func(family string) string) []diag.Item {
	var out []diag.Item
	reported := map[string]bool{}
	for _, stack := range stacks {
		families := Families(stack)
		if len(families) == 0 || generic[strings.ToLower(families[0])] {
			continue
		}
		installed := func(f string) (bool, string) {
			got := match(f)
			return got != "" && strings.EqualFold(got, f), got
		}
		if ok, _ := installed(families[0]); ok {
			continue
		}
		used := ""
		for _, f := range families[1:] {
			if generic[strings.ToLower(f)] {
				if got := match(f); got != "" {
					used = got
				}
				break
			}
			if ok, got := installed(f); ok {
				used = got
				break
			}
		}
		if used == "" {
			used = match(families[0])
		}
		if used == "" || reported[families[0]] {
			continue
		}
		reported[families[0]] = true
		out = append(out, diag.New("svg.font_missing", 0, "requested", families[0], "used", used))
	}
	return out
}

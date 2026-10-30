// SPDX-License-Identifier: GPL-3.0-only

package repocheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every document of the project exists in two languages: English, the default
// file that GitHub shows (X.md), and French (X.fr.md). Each starts, under its
// title, with a link to the other, so that a reader who lands on the wrong one
// finds the right one at once. A document added in one language only, or whose
// link was lost, fails here.
func TestEveryDocumentExistsInBothLanguages(t *testing.T) {
	root := repoRoot(t)
	walk(t, root, func(path, rel string) {
		if filepath.Ext(rel) != ".md" {
			return
		}
		var other, link string
		if strings.HasSuffix(rel, ".fr.md") {
			other = strings.TrimSuffix(rel, ".fr.md") + ".md"
			link = "🇬🇧 [English version](" + filepath.Base(other) + ")"
		} else {
			other = strings.TrimSuffix(rel, ".md") + ".fr.md"
			link = "🇫🇷 [Version française](" + filepath.Base(other) + ")"
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(other))); err != nil {
			t.Errorf("%s has no counterpart %s", rel, other)
			return
		}
		lines := strings.SplitN(read(t, path), "\n", 9)
		if len(lines) > 8 {
			lines = lines[:8]
		}
		if !strings.Contains(strings.Join(lines, "\n"), link) {
			t.Errorf("%s: no %q in its first lines", rel, link)
		}
	})
}

// headingLevels lists the level of every heading of a Markdown text, leaving
// out the lines of code blocks (a shell comment is not a heading).
func headingLevels(text string) []int {
	var levels []int
	inCode := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if inCode || !strings.HasPrefix(line, "#") {
			continue
		}
		level := len(line) - len(strings.TrimLeft(line, "#"))
		if level <= 6 && strings.HasPrefix(line[level:], " ") {
			levels = append(levels, level)
		}
	}
	return levels
}

// The two versions of a document have the same sections, in the same order:
// the same headings, translated. It does not prove that the texts say the same
// thing, but it catches a section added, removed or moved in only one of them.
func TestTheTwoLanguagesHaveTheSameSections(t *testing.T) {
	root := repoRoot(t)
	walk(t, root, func(path, rel string) {
		if !strings.HasSuffix(rel, ".md") || strings.HasSuffix(rel, ".fr.md") {
			return
		}
		frPath := strings.TrimSuffix(path, ".md") + ".fr.md"
		if _, err := os.Stat(frPath); err != nil {
			return // TestEveryDocumentExistsInBothLanguages says so
		}
		en, fr := headingLevels(read(t, path)), headingLevels(read(t, frPath))
		if len(en) != len(fr) {
			t.Errorf("%s has %d headings but its French version has %d", rel, len(en), len(fr))
			return
		}
		for i := range en {
			if en[i] != fr[i] {
				t.Errorf("%s: heading %d is of level %d but of level %d in its French version", rel, i+1, en[i], fr[i])
				return
			}
		}
	})
}

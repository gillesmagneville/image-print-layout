// SPDX-License-Identifier: GPL-3.0-only

package repocheck

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/i18n"
)

// A code is produced by diag.New and diag.Fail, or by the add method of the
// report of internal/svgdoc, which builds the same items.
var diagCall = regexp.MustCompile(`(?:diag\.(?:New|Fail)|\br\.add)\(\s*"([a-z_.]+)"`)
var placeholderName = regexp.MustCompile(`\{([^{}]*)\}`)

// Every diagnostic the program can produce has a text in each language, which
// uses only the values the code carries; every code a source file produces is
// in the registry, and every code of the registry is produced somewhere. A
// diagnostic without a text would show its key in the window.
func TestEveryDiagnosticCodeHasATextInEveryLanguage(t *testing.T) {
	for code, spec := range diag.Registry {
		key := "diag." + code
		allowed := map[string]bool{}
		for _, a := range spec.Args {
			allowed[a] = true
		}
		for _, lang := range i18n.Languages {
			text := i18n.New(lang).T(key)
			if text == key {
				t.Errorf("%s: no text for %s", lang, key)
				continue
			}
			for _, m := range placeholderName.FindAllStringSubmatch(text, -1) {
				if !allowed[m[1]] {
					t.Errorf("%s: %s uses {%s}, which the code does not carry (it carries %v)", lang, key, m[1], spec.Args)
				}
			}
		}
	}
}

func TestDiagnosticCodesInTheSourcesAreRegistered(t *testing.T) {
	root := repoRoot(t)
	used := map[string]string{}
	walk(t, root, func(path, rel string) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/diag/") {
			return
		}
		for _, m := range diagCall.FindAllStringSubmatch(read(t, path), -1) {
			used[m[1]] = rel
		}
	})
	for code, file := range used {
		if _, ok := diag.Registry[code]; !ok {
			t.Errorf("%s produces the code %q, which is not in diag.Registry", file, code)
		}
	}
	var unused []string
	for code := range diag.Registry {
		if _, ok := used[code]; !ok {
			unused = append(unused, code)
		}
	}
	sort.Strings(unused)
	if len(unused) > 0 {
		t.Errorf("codes of diag.Registry that no source produces (remove them or use them): %v", unused)
	}
}

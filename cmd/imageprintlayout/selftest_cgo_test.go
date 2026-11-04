// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/i18n"
)

// The self-test passes, and says so in each of the four languages: what it
// writes is in the language of the interface, not in one fixed language.
func TestSelfTestPasses(t *testing.T) {
	outputs := map[string]i18n.Lang{}
	for _, lang := range i18n.Languages {
		tr := i18n.New(lang)
		var out bytes.Buffer
		if err := runSelfTest(&out, tr); err != nil {
			t.Fatalf("%s: self-test failed: %v\n%s", lang, err, out.String())
		}
		if !strings.Contains(out.String(), tr.T("selftest.success")) {
			t.Errorf("%s: unexpected output:\n%s", lang, out.String())
		}
		if other, dup := outputs[out.String()]; dup {
			t.Errorf("%s and %s give the same output: the self-test is not translated", lang, other)
		}
		outputs[out.String()] = lang
	}
}

func TestVersionLineNamesTheLibraries(t *testing.T) {
	line := versionLine()
	for _, want := range []string{"imageprintlayout", "librsvg", "cairo"} {
		if !strings.Contains(line, want) {
			t.Errorf("%q lacks %q", line, want)
		}
	}
}

// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSelfTestPasses(t *testing.T) {
	var out bytes.Buffer
	if err := runSelfTest(&out); err != nil {
		t.Fatalf("self-test failed: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Tout fonctionne.") {
		t.Errorf("unexpected output:\n%s", out.String())
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

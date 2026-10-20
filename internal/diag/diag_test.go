// SPDX-License-Identifier: GPL-3.0-only

package diag

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNewTakesTheLevelOfTheRegistry(t *testing.T) {
	it := New("page.too_tall", 3, "h", 300.5, "usable", 277.0)
	if it.Level != Warning || it.Page != 3 || it.Key() != "diag.page.too_tall" {
		t.Errorf("got %+v", it)
	}
	if got := it.String(); !strings.Contains(got, "page.too_tall") || !strings.Contains(got, "page 3") || !strings.Contains(got, "h=300.5") {
		t.Errorf("String() = %q", got)
	}
	if New("no.such.code", 0).Level != Failure {
		t.Error("an unknown code must be a failure, so that a test catches it")
	}
}

func TestErrorIsFoundInAChain(t *testing.T) {
	err := fmt.Errorf("while adding: %w", Fail("cut.too_close", "min", 0.5))
	e, ok := As(err)
	if !ok || e.Code != "cut.too_close" || e.Key() != "diag.cut.too_close" {
		t.Fatalf("As = %+v, %v", e, ok)
	}
	if CodeOf(err) != "cut.too_close" {
		t.Errorf("CodeOf = %q", CodeOf(err))
	}
	if CodeOf(errors.New("plain")) != "" {
		t.Error("a plain error has no code")
	}
	if got := e.Error(); got != "cut.too_close: min=0.5" {
		t.Errorf("Error() = %q", got)
	}
}

func TestEveryCodeIsWellFormed(t *testing.T) {
	for code, spec := range Registry {
		if code == "" || strings.ContainsAny(code, " {}") || strings.HasPrefix(code, ".") || strings.HasSuffix(code, ".") {
			t.Errorf("code %q is malformed", code)
		}
		seen := map[string]bool{}
		for _, a := range spec.Args {
			if a == "" || seen[a] {
				t.Errorf("code %q: empty or repeated value name %q", code, a)
			}
			seen[a] = true
		}
	}
}

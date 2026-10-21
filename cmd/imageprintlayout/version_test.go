// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestVersionLineStartsWithVersionAndPlatform(t *testing.T) {
	old := version
	version = "1.2.3"
	defer func() { version = old }()
	want := "imageprintlayout 1.2.3 (" + runtime.GOOS + "/" + runtime.GOARCH + ")"
	if got := versionLine(); !strings.HasPrefix(got, want) {
		t.Errorf("versionLine() = %q, want a line starting with %q", got, want)
	}
}

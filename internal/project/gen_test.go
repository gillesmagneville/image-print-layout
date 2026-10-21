// SPDX-License-Identifier: GPL-3.0-only

package project

import (
	"os"
	"testing"
)

// TestWriteReference regenerates testdata/reference.pageslice when UPDATE_REFERENCE
// is set. It is how the file is made; the test above compares against it.
func TestWriteReference(t *testing.T) {
	if os.Getenv("UPDATE_REFERENCE") == "" {
		t.Skip("set UPDATE_REFERENCE=1 to rewrite testdata/reference.pageslice")
	}
	if err := os.WriteFile("testdata/reference.pageslice", mustEncode(t, referenceProject()), 0o644); err != nil {
		t.Fatal(err)
	}
}

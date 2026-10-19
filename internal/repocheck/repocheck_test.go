// SPDX-License-Identifier: GPL-3.0-only

package repocheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot is the directory holding go.mod. Tests of a package run in the
// package directory, so it is two levels up from here.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected go.mod at %s: %v", root, err)
	}
	return root
}

// read returns the text of a file with Unix line endings: on Windows, git
// checks text files out with CRLF, which would break every pattern anchored on
// the end of a line. The byte order mark, if any, is kept.
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// walk calls fn for every tracked-looking file under root, skipping .git.
func walk(t *testing.T, root string, fn func(path, rel string)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fn(path, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

const spdx = "SPDX-License-Identifier: GPL-3.0-only"

// Every source, script, document, packaging and CI file says under which
// licence it is distributed, in its first three lines. (A byte order mark at
// the start of a PowerShell or NSIS file does not matter: the test looks for
// the text.)
func TestEveryFileHasTheLicenceHeader(t *testing.T) {
	root := repoRoot(t)
	walk(t, root, func(path, rel string) {
		switch filepath.Ext(rel) {
		case ".go", ".md", ".yml", ".yaml", ".sh", ".ps1", ".nsi", ".nsh", ".desktop", ".xml", ".1":
		default:
			return
		}
		lines := strings.SplitN(read(t, path), "\n", 4)
		if len(lines) > 3 {
			lines = lines[:3]
		}
		if !strings.Contains(strings.Join(lines, "\n"), spdx) {
			t.Errorf("%s: missing %q in its first lines", rel, spdx)
		}
	})
}

var semver = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?$`)

// VERSION is a Semantic Version, and a version without a pre-release suffix
// is one the changelog has a section for.
func TestVersionAndChangelogAgree(t *testing.T) {
	root := repoRoot(t)
	version := strings.TrimSpace(read(t, filepath.Join(root, "VERSION")))
	if !semver.MatchString(version) {
		t.Fatalf("VERSION %q is not a Semantic Version", version)
	}
	changelog := read(t, filepath.Join(root, "CHANGELOG.md"))
	if !strings.Contains(changelog, "\n## [Non publié]") {
		t.Error("CHANGELOG.md has no \"## [Non publié]\" section")
	}
	if !strings.Contains(version, "-") && !strings.Contains(changelog, "\n## ["+version+"]") {
		t.Errorf("VERSION is %s but CHANGELOG.md has no \"## [%s]\" section", version, version)
	}
}

var importC = regexp.MustCompile(`(?m)^import "C"`)

// Only the interface and the rendering engine may use cgo: everything else
// must build and test without a C toolchain or graphics libraries.
func TestCgoIsLimitedToUIAndEngine(t *testing.T) {
	root := repoRoot(t)
	walk(t, root, func(path, rel string) {
		if filepath.Ext(rel) != ".go" {
			return
		}
		if strings.HasPrefix(rel, "internal/ui/") || strings.HasPrefix(rel, "internal/engine/") {
			return
		}
		if importC.MatchString(read(t, path)) {
			t.Errorf("%s uses cgo; only internal/ui and internal/engine may", rel)
		}
	})
}

// ARCHITECTURE.md is the canonical contract: its repository layout names
// every package of internal/.
func TestArchitectureNamesEveryPackage(t *testing.T) {
	root := repoRoot(t)
	architecture := read(t, filepath.Join(root, "ARCHITECTURE.md"))
	entries, err := os.ReadDir(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !strings.Contains(architecture, "── "+e.Name()+"/") {
			t.Errorf("ARCHITECTURE.md, \"Disposition du dépôt\", does not list internal/%s", e.Name())
		}
	}
}

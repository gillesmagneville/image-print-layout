// SPDX-License-Identifier: GPL-3.0-only

package repocheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// The documents of the project exist in two languages: English, the default
// file that GitHub shows (X.md), and French (X.fr.md). Each language has its
// own changelog and its own heading for the notes not yet released.
var changelogs = []struct{ file, unreleased string }{
	{"CHANGELOG.md", "## [Unreleased]"},
	{"CHANGELOG.fr.md", "## [Non publié]"},
}

// architectures are the two languages of ARCHITECTURE.md.
var architectures = []string{"ARCHITECTURE.md", "ARCHITECTURE.fr.md"}

// VERSION is a Semantic Version, and a version without a pre-release suffix
// is one both changelogs have a section for.
func TestVersionAndChangelogAgree(t *testing.T) {
	root := repoRoot(t)
	version := strings.TrimSpace(read(t, filepath.Join(root, "VERSION")))
	if !semver.MatchString(version) {
		t.Fatalf("VERSION %q is not a Semantic Version", version)
	}
	for _, c := range changelogs {
		changelog := read(t, filepath.Join(root, c.file))
		if !strings.Contains(changelog, "\n"+c.unreleased) {
			t.Errorf("%s has no %q section", c.file, c.unreleased)
		}
		if !strings.Contains(version, "-") && !strings.Contains(changelog, "\n## ["+version+"]") {
			t.Errorf("VERSION is %s but %s has no \"## [%s]\" section", version, c.file, version)
		}
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
// every package of internal/. So does its French version.
func TestArchitectureNamesEveryPackage(t *testing.T) {
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range architectures {
		architecture := read(t, filepath.Join(root, file))
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if !strings.Contains(architecture, "── "+e.Name()+"/") {
				t.Errorf("%s, \"Repository layout\", does not list internal/%s", file, e.Name())
			}
		}
	}
}

var architectureSection = regexp.MustCompile(`(?m)^## (\d+)\. `)

// lastArchitectureSection is the number of the last numbered section of
// ARCHITECTURE.md ("Risks and open questions").
const lastArchitectureSection = 15

// ARCHITECTURE.md numbers its sections from 1 to the last with no gap, in
// order: the other documents and the comments of the code refer to them
// (§6, §11...), and a section lost when the file is rewritten would otherwise
// go unnoticed until someone looks for it.
func TestArchitectureSectionsAreAllThere(t *testing.T) {
	root := repoRoot(t)
	var want []string
	for n := 1; n <= lastArchitectureSection; n++ {
		want = append(want, strconv.Itoa(n))
	}
	for _, file := range architectures {
		var got []string
		for _, m := range architectureSection.FindAllStringSubmatch(read(t, filepath.Join(root, file)), -1) {
			got = append(got, m[1])
		}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s has the sections %s, want %s", file, strings.Join(got, " "), strings.Join(want, " "))
		}
	}
}

// SPDX-License-Identifier: GPL-3.0-only

package project

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/model"
)

// HashFile is the SHA-256 of a file, in hexadecimal. The file is only read.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SourceFor describes an SVG file for a project to be saved at projectPath:
// its absolute path, the path relative to the project file (with "/" as the
// separator on every platform) when there is one, its fingerprint and its
// size in units.
func SourceFor(projectPath, svgPath string, width, height float64) (model.Source, error) {
	abs, err := filepath.Abs(svgPath)
	if err != nil {
		return model.Source{}, err
	}
	sum, err := HashFile(abs)
	if err != nil {
		return model.Source{}, err
	}
	return DescribeSource(projectPath, abs, sum, width, height), nil
}

// DescribeSource is SourceFor for a fingerprint that is already known, such as
// the one of the file as it was read when it was opened: it reads nothing, so
// a document that has changed or gone since is still described as it was
// worked on.
func DescribeSource(projectPath, svgPath, sha string, width, height float64) model.Source {
	abs, err := filepath.Abs(svgPath)
	if err != nil {
		abs = svgPath
	}
	return model.Source{AbsolutePath: abs, Path: relativeTo(projectPath, abs), SHA256: sha, Width: width, Height: height}
}

// relativeTo writes target relative to the directory of projectPath, or ""
// when there is no such path (another drive on Windows).
func relativeTo(projectPath, target string) string {
	base, err := filepath.Abs(filepath.Dir(projectPath))
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

// Save writes the project file. The relative path of the source is computed
// for the place the file is written to, so that a project can be moved with
// its document. The write is atomic: a temporary file in the same folder, then
// a rename, so that a crash never leaves half a file.
func Save(path string, p model.Project) error {
	if p.Source.AbsolutePath != "" {
		p.Source.Path = relativeTo(path, p.Source.AbsolutePath)
	}
	data, err := Encode(p)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".pageslice-*.tmp")
	if err != nil {
		return fmt.Errorf("project: cannot write %s: %w", path, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("project: cannot write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("project: cannot write %s: %w", path, err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("project: cannot write %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("project: cannot write %s: %w", path, err)
	}
	return nil
}

// Load reads a project file.
func Load(path string) (model.Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Project{}, fmt.Errorf("project: cannot read %s: %w", path, err)
	}
	return Decode(data)
}

// ResolveSource finds the SVG of a project that was loaded from projectPath:
// the relative path first, then the absolute one. It returns false when the
// document is in neither place; the caller then offers to locate it and the
// settings are kept.
func ResolveSource(projectPath string, src model.Source) (string, bool) {
	var tries []string
	if src.Path != "" {
		tries = append(tries, filepath.Join(filepath.Dir(projectPath), filepath.FromSlash(src.Path)))
	}
	if src.AbsolutePath != "" {
		tries = append(tries, src.AbsolutePath)
	}
	for _, candidate := range tries {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			continue
		}
	}
	return "", false
}

// Verify compares what the project recorded about its document with the
// document that was found, and says what no longer matches. It blocks nothing:
// the cuts are in document units and the user decides what to do.
func Verify(p model.Project, sha string, width, height float64) []diag.Item {
	var items []diag.Item
	if p.Source.SHA256 != "" && sha != "" && p.Source.SHA256 != sha {
		items = append(items, diag.New("project.source_changed", 0))
	}
	if p.Source.Width > 0 && p.Source.Height > 0 &&
		(math.Abs(p.Source.Width-width) > 1e-6 || math.Abs(p.Source.Height-height) > 1e-6) {
		items = append(items, diag.New("project.size_changed", 0,
			"w", width, "h", height, "was_w", p.Source.Width, "was_h", p.Source.Height))
	}
	if n := len(CutsOutside(p, height)); n > 0 {
		items = append(items, diag.New("project.cuts_outside", 0, "count", n))
	}
	return items
}

// CutsOutside lists the ids of the cuts that no longer fall inside a document
// of the given height. They are removed, after a confirmation, by the caller.
func CutsOutside(p model.Project, height float64) []int {
	var ids []int
	for _, c := range p.Cuts {
		if c.Position >= height {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

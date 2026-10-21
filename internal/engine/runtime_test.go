// SPDX-License-Identifier: GPL-3.0-only

package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func TestBundledEnvironmentOfACompleteLayout(t *testing.T) {
	prefix := t.TempDir()
	touch(t, filepath.Join(prefix, "etc", "fonts", "fonts.conf"))
	touch(t, filepath.Join(prefix, "lib", "gdk-pixbuf-2.0", "2.10.0", "loaders.cache"))
	exe := filepath.Join(prefix, "bin", "imageprintlayout.exe")

	got := BundledEnvironment(exe, fileExists)
	want := map[string]string{
		"FONTCONFIG_PATH":        filepath.Join(prefix, "etc", "fonts"),
		"FONTCONFIG_FILE":        filepath.Join(prefix, "etc", "fonts", "fonts.conf"),
		"PANGO_CAIRO_BACKEND":    "fc",
		"GDK_PIXBUF_MODULE_FILE": filepath.Join(prefix, "lib", "gdk-pixbuf-2.0", "2.10.0", "loaders.cache"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A program installed some other way (a Linux package, say) must not have its
// libraries redirected: only files that exist are named.
func TestBundledEnvironmentOnlyNamesWhatExists(t *testing.T) {
	prefix := t.TempDir()
	exe := filepath.Join(prefix, "bin", "imageprintlayout")
	if got := BundledEnvironment(exe, fileExists); len(got) != 0 {
		t.Errorf("an empty layout gave %v, want nothing", got)
	}

	touch(t, filepath.Join(prefix, "lib", "gdk-pixbuf-2.0", "2.10.0", "loaders.cache"))
	got := BundledEnvironment(exe, fileExists)
	if len(got) != 1 || got["GDK_PIXBUF_MODULE_FILE"] == "" {
		t.Errorf("a layout with image loaders only gave %v", got)
	}
}

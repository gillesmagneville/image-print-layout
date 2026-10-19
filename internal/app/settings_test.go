// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFirstStartGivesTheDefaultSettings(t *testing.T) {
	got, err := LoadSettings(filepath.Join(t.TempDir(), "nothing", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultSettings() || got.Language != "auto" {
		t.Errorf("got %+v, want the default settings", got)
	}
}

func TestSettingsAreSavedAndReadBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "svgprintlayout", "settings.json")
	if err := (Settings{Language: "de"}).Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(path)
	if err != nil || got.Language != "de" {
		t.Errorf("LoadSettings = %+v, %v; want de", got, err)
	}
	// No temporary file is left next to the settings.
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("the directory has %d entries, want only settings.json", len(entries))
	}
}

// The same settings are the same bytes.
func TestSettingsAreWrittenDeterministically(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.json"), filepath.Join(dir, "b.json")
	for _, p := range []string{a, b} {
		if err := (Settings{Language: "es"}).Save(p); err != nil {
			t.Fatal(err)
		}
	}
	da, _ := os.ReadFile(a)
	db, _ := os.ReadFile(b)
	if string(da) != string(db) || string(da) != "{\n  \"language\": \"es\"\n}\n" {
		t.Errorf("unexpected bytes %q / %q", da, db)
	}
}

// A damaged file does not stop the program: the defaults and the reason come
// back.
func TestUnreadableSettingsGiveTheDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(path)
	if err == nil {
		t.Error("expected an error for a damaged file")
	}
	if got != DefaultSettings() {
		t.Errorf("got %+v, want the defaults", got)
	}
}

// A language the program does not know (a file from a later version) is
// ignored.
func TestUnknownLanguageFallsBackToAuto(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"language":"it"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(path)
	if err != nil || got.Language != "auto" {
		t.Errorf("got %+v, %v; want auto", got, err)
	}
}

func TestSettingsPathIsInTheConfigDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	got, err := SettingsPath()
	if err != nil {
		t.Skipf("no configuration directory here: %v", err)
	}
	if filepath.Base(got) != "settings.json" || filepath.Base(filepath.Dir(got)) != "svgprintlayout" {
		t.Errorf("SettingsPath() = %s", got)
	}
}

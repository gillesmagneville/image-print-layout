// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gillesmagneville/image-print-layout/internal/i18n"
)

// Settings are the preferences of the user. They belong to the person, not to
// a project: a project opened by someone else keeps its layout whatever the
// language of the interface is.
type Settings struct {
	// Language is a language code ("fr", "en", "es", "de") or "auto" for the
	// language of the system.
	Language string `json:"language"`
}

// DefaultSettings are the settings of a first start.
func DefaultSettings() Settings { return Settings{Language: i18n.Auto} }

// SettingsPath is the file that holds the settings of the user: in the
// configuration directory of the system (~/.config on Linux, %AppData% on
// Windows).
func SettingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("app: no configuration directory: %w", err)
	}
	return filepath.Join(dir, "imageprintlayout", "settings.json"), nil
}

// legacySettingsPath is where the program, under its first name (SVG Print
// Layout, up to version 1.0.x), kept the settings.
func legacySettingsPath(path string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(path)), "svgprintlayout", "settings.json")
}

// ImportLegacySettings gives the settings of the first name of the program to
// the current one: when path does not exist yet and the old file does, the old
// settings are read and written at path. The old file stays where it is (a
// downgrade finds its settings). It returns whether something was imported;
// any failure leaves things as they were, and the program starts with its
// defaults.
func ImportLegacySettings(path string) bool {
	if _, err := os.Stat(path); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return false
	}
	old := legacySettingsPath(path)
	if _, err := os.Stat(old); err != nil {
		return false
	}
	s, err := LoadSettings(old)
	if err != nil {
		return false
	}
	return s.Save(path) == nil
}

// LoadSettings reads the settings. A file that does not exist is not an error
// (first start): it gives the default settings. A file that cannot be read
// gives the default settings and the reason, so that the program starts and
// can tell the user.
func LoadSettings(path string) (Settings, error) {
	s := DefaultSettings()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("app: cannot read the settings: %w", err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return DefaultSettings(), fmt.Errorf("app: the settings file %s is not valid: %w", path, err)
	}
	if s.Language != i18n.Auto && !i18n.Lang(s.Language).Valid() {
		s.Language = i18n.Auto
	}
	return s, nil
}

// Save writes the settings: to a temporary file first, then renamed over the
// real one, so that a crash never leaves half a file. The bytes depend only on
// the settings.
func (s Settings) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("app: cannot create the settings directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.tmp")
	if err != nil {
		return fmt.Errorf("app: cannot write the settings: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("app: cannot write the settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("app: cannot write the settings: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("app: cannot write the settings: %w", err)
	}
	return nil
}

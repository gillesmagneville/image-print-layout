// SPDX-License-Identifier: GPL-3.0-only

package svgdoc

import (
	"os"
	"path/filepath"
)

func mkdirAll(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, []byte("x"), 0o644)
}

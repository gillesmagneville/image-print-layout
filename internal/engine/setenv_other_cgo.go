// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo && !windows

package engine

import "os"

// setEnv sets an environment variable. With cgo, Go also updates the C
// runtime's copy on these systems.
func setEnv(key, value string) error {
	return os.Setenv(key, value)
}

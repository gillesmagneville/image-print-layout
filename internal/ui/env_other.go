// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo && !windows

package ui

import "os"

// setenv sets an environment variable for the program and for the C libraries
// it runs (GTK reads LANGUAGE through the C library). With cgo, Go passes the
// change on to the C library by itself on this platform.
func setenv(key, value string) { os.Setenv(key, value) }

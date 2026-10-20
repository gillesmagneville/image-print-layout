// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo && windows

package ui

/*
#include <stdlib.h>
*/
import "C"

import (
	"os"
	"unsafe"
)

// setenv sets an environment variable for the program and for the C libraries
// it runs. On Windows, Go changes the environment of the process but not the
// copy the C library keeps, which is where GTK and gettext read LANGUAGE: it
// is updated here too.
func setenv(key, value string) {
	os.Setenv(key, value)
	k, v := C.CString(key), C.CString(value)
	defer C.free(unsafe.Pointer(k))
	defer C.free(unsafe.Pointer(v))
	C._putenv_s(k, v)
}

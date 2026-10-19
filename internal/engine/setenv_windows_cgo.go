// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo && windows

package engine

/*
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// setEnv sets a variable in the C runtime's environment, which is the one
// fontconfig reads with getenv. os.Setenv only changes the Windows process
// environment, and the C runtime keeps its own copy of it; _putenv_s updates
// both.
func setEnv(key, value string) error {
	k, v := C.CString(key), C.CString(value)
	defer C.free(unsafe.Pointer(k))
	defer C.free(unsafe.Pointer(v))
	if rc := C._putenv_s(k, v); rc != 0 {
		return fmt.Errorf("impossible de définir la variable d'environnement %s (code %d)", key, int(rc))
	}
	return nil
}

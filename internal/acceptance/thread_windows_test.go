// SPDX-License-Identifier: GPL-3.0-only

package acceptance

import "syscall"

var getCurrentThreadID = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")

// osThreadID is the identifier of the OS thread that runs the caller.
func osThreadID() uint32 {
	id, _, _ := getCurrentThreadID.Call()
	return uint32(id)
}

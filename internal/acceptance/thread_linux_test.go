// SPDX-License-Identifier: GPL-3.0-only

package acceptance

import "syscall"

// osThreadID is the identifier of the OS thread that runs the caller.
func osThreadID() uint32 { return uint32(syscall.Gettid()) }

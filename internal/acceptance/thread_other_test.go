// SPDX-License-Identifier: GPL-3.0-only

//go:build !windows

package acceptance

// osThreadID is 0 where the tests do not read the thread (see
// thread_windows_test.go).
func osThreadID() uint32 { return 0 }

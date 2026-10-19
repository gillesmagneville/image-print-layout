// SPDX-License-Identifier: GPL-3.0-only

//go:build windows

package i18n

import (
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// localeNameMaxLength is LOCALE_NAME_MAX_LENGTH of the Windows API.
const localeNameMaxLength = 85

// systemLanguage asks Windows for the language of the user (« fr-FR »):
// Windows sets no LANG for a program started from the Start menu.
func systemLanguage() (Lang, bool) {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	var buf [localeNameMaxLength]uint16
	// The call returns the number of characters written, with the final zero.
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n < 2 || n > uintptr(len(buf)) {
		return "", false
	}
	return Parse(string(utf16.Decode(buf[:n-1])))
}

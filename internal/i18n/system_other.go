// SPDX-License-Identifier: GPL-3.0-only

//go:build !windows

package i18n

// systemLanguage has nothing to add to the environment variables on Linux:
// that is where the language of the session is.
func systemLanguage() (Lang, bool) { return "", false }

// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"strings"

	"github.com/gillesmagneville/pageslice/internal/i18n"
)

// requestedLanguage finds the value of -lang in the arguments before the flag
// package reads them: the descriptions the flag package prints are in the
// language of the interface, so the language must be known to declare them.
// It returns "" when -lang is absent.
func requestedLanguage(args []string) string {
	for i, arg := range args {
		if arg == "--" {
			break
		}
		name := strings.TrimLeft(arg, "-")
		if len(name) == len(arg) {
			continue
		}
		switch {
		case name == "lang" && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(name, "lang="):
			return strings.TrimPrefix(name, "lang=")
		}
	}
	return ""
}

// knownLanguageSetting tells whether a -lang value is a language of the
// interface, or "auto" or empty (the language of the system).
func knownLanguageSetting(setting string) bool {
	return setting == "" || setting == i18n.Auto || i18n.Lang(setting).Valid()
}

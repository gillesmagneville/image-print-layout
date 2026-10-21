// SPDX-License-Identifier: GPL-3.0-only

package main

import "testing"

func TestRequestedLanguage(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"-version"}, ""},
		{[]string{"-lang", "de"}, "de"},
		{[]string{"--lang", "es"}, "es"},
		{[]string{"-lang=fr"}, "fr"},
		{[]string{"-version", "--lang=en", "x"}, "en"},
		{[]string{"-lang"}, ""},
		{[]string{"--", "-lang", "de"}, ""},
		{[]string{"lang", "de"}, ""},
	}
	for _, c := range cases {
		if got := requestedLanguage(c.args); got != c.want {
			t.Errorf("requestedLanguage(%q) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestKnownLanguageSetting(t *testing.T) {
	for _, ok := range []string{"", "auto", "fr", "en", "es", "de"} {
		if !knownLanguageSetting(ok) {
			t.Errorf("%q should be accepted", ok)
		}
	}
	for _, bad := range []string{"it", "FR", "fr_FR", "klingon"} {
		if knownLanguageSetting(bad) {
			t.Errorf("%q should be refused", bad)
		}
	}
}

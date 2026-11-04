// SPDX-License-Identifier: GPL-3.0-only

//go:build !cgo

package main

import (
	"errors"
	"io"

	"github.com/gillesmagneville/image-print-layout/internal/i18n"
)

// prepareRuntime has nothing to prepare without the engine.
func prepareRuntime() error { return nil }

// libraries is empty: a build without cgo has no rendering engine.
func libraries() string { return "" }

// runSelfTest has nothing to check: it says so, in the language of tr.
func runSelfTest(_ io.Writer, tr *i18n.Translator) error {
	return errors.New(tr.T("selftest.error.no_engine"))
}

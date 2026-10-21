// SPDX-License-Identifier: GPL-3.0-only

//go:build !cgo

package main

import (
	"errors"
	"io"
)

// prepareRuntime has nothing to prepare without the engine.
func prepareRuntime() error { return nil }

// libraries is empty: a build without cgo has no rendering engine.
func libraries() string { return "" }

func runSelfTest(io.Writer) error {
	return errors.New("cette version a été compilée sans le moteur de rendu (cgo désactivé) : il n'y a rien à vérifier")
}

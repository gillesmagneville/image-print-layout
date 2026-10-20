// SPDX-License-Identifier: GPL-3.0-only

//go:build !cgo

package ui

// Available tells whether this build has a graphical interface. GTK is a C
// library: a program built without cgo has none (the pure Go packages and the
// tests that build everywhere do not need it).
const Available = false

// Run does nothing in a program built without cgo.
func Run(Options) int { return 1 }

// SPDX-License-Identifier: GPL-3.0-only

// Package ui is the graphical interface: parameters panel on the left,
// interactive preview with draggable cut lines in the centre, page list on
// the right. It is the only package that depends on the GUI toolkit, GTK4
// through gotk4.
//
// The window is a thin layer: it shows what package app computes and sends
// back what the user does. Every text it shows comes from package i18n, and
// the whole window follows a change of language without a restart.
//
// Built with cgo only (GTK is a C library); without it the package has a
// stub, so that everything else builds and tests without GTK installed.
package ui

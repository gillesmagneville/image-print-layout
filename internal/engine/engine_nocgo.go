// SPDX-License-Identifier: GPL-3.0-only

//go:build !cgo

package engine

// Load fails: a build without cgo has no rendering engine. It exists so that
// the packages that use a Loader build everywhere.
func Load(svg []byte, path string) (Document, error) { return nil, ErrNoEngine }

// MatchFont returns the family unchanged: without the engine there is no font
// system to ask.
func MatchFont(family string) string { return family }

// LoadDocument fails like Load.
func LoadDocument(svg []byte, path string) (Document, error) { return nil, ErrNoEngine }

// SupportedImageFormats is empty: without the engine no image can be read.
func SupportedImageFormats() []Format { return nil }

// SPDX-License-Identifier: GPL-3.0-only

// Package diag collects the diagnostics shown to the user: warnings about a
// page that overflows, a cut that crosses a block, an unsupported SVG
// feature, a failed verification — and the errors a command can answer with.
//
// A diagnostic is a code and its values, never a sentence: the packages that
// compute it do not know which language the window speaks. The window turns
// the code "page.too_tall" into the text of the key "diag.page.too_tall" of
// the catalogs, with the values as placeholders. Registry lists every code
// with its level and the names of its values, and a test checks that the four
// catalogs have a text for each, using exactly those placeholders.
//
// Pure Go, no cgo.
package diag

import (
	"errors"
	"fmt"
	"strings"
)

// Level is how serious a diagnostic is.
type Level int

const (
	// Info tells something without asking for anything.
	Info Level = iota
	// Warning asks for attention and, at export time, for a confirmation.
	Warning
	// Failure forbids what was asked.
	Failure
)

// String names the level for logs and tests.
func (l Level) String() string {
	switch l {
	case Info:
		return "info"
	case Warning:
		return "warning"
	case Failure:
		return "failure"
	}
	return fmt.Sprintf("level(%d)", int(l))
}

// Item is one diagnostic.
type Item struct {
	Level Level
	// Code identifies the diagnostic: "page.too_tall".
	Code string
	// Page is the number of the page it is about (from 1), or 0.
	Page int
	// Args are pairs of a placeholder name and its value, in the form the
	// translator takes them: string, int or float64.
	Args []any
}

// Key is the key of the text of the diagnostic in the catalogs.
func (i Item) Key() string { return "diag." + i.Code }

// String writes the diagnostic for a log or a test failure, never for the
// user.
func (i Item) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s", i.Level, i.Code)
	if i.Page > 0 {
		fmt.Fprintf(&b, " (page %d)", i.Page)
	}
	writeArgs(&b, i.Args)
	return b.String()
}

func writeArgs(b *strings.Builder, args []any) {
	for k := 0; k+1 < len(args); k += 2 {
		if k == 0 {
			b.WriteString(":")
		}
		fmt.Fprintf(b, " %v=%v", args[k], args[k+1])
	}
}

// New builds the diagnostic of a code: its level is the one the registry
// gives. A code the registry does not know gives a Failure, which a test
// catches.
func New(code string, page int, args ...any) Item {
	spec, ok := Registry[code]
	level := spec.Level
	if !ok {
		level = Failure
	}
	return Item{Level: level, Code: code, Page: page, Args: args}
}

// Error is what a command answers when it refuses to do what was asked: a
// code and its values, like an Item, so that the window can explain the
// refusal in its own language.
type Error struct {
	Code string
	Args []any
}

// Fail builds an Error.
func Fail(code string, args ...any) *Error { return &Error{Code: code, Args: args} }

// Error writes the error for a log or a test failure, never for the user.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Code)
	writeArgs(&b, e.Args)
	return b.String()
}

// Key is the key of the text of the error in the catalogs.
func (e *Error) Key() string { return "diag." + e.Code }

// As finds the Error in a chain of errors.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// CodeOf is the code of the Error in the chain, or "".
func CodeOf(err error) string {
	if e, ok := As(err); ok {
		return e.Code
	}
	return ""
}

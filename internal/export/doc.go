// SPDX-License-Identifier: GPL-3.0-only

// Package export drives the multipage PDF export (ARCHITECTURE §6 and §11):
// it turns the page plan into the pages the engine draws, runs the checks
// before the export, writes the file, reads it back and compares what it says
// with the plan, and only then puts it where the user asked.
//
// The package is plain Go: the drawing is done by the engine behind the
// engine.Document interface, and the PDF written by cairo is read back with
// the standard library alone.
package export

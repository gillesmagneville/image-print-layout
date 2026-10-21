// SPDX-License-Identifier: GPL-3.0-only

// Package project reads and writes the layout project file (.imgpl): a
// versioned, deterministically serialised JSON document that stores the
// source path (and its hash), paper, margins, scale, background options, cut
// positions and per-page positioning. The source SVG is never modified.
// Pure Go, no cgo.
package project

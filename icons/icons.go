// SPDX-License-Identifier: GPL-3.0-only

// Package icons holds the icon of the program, embedded in the executable. The
// packages install the same pictures in the icon theme (hicolor), where the
// desktop finds the icon of the launcher and of the window; the About box
// draws this copy, which is there whatever the way the program was installed
// (a build in a directory, a portable archive).
package icons

import _ "embed"

// PNG is the icon at 256 × 256 pixels.
//
//go:embed svgprintlayout-256.png
var PNG []byte

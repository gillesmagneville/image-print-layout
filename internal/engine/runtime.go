// SPDX-License-Identifier: GPL-3.0-only

package engine

import (
	"path/filepath"
)

// BundledEnvironment returns the environment variables the engine's libraries
// need to find the data shipped with the program, for the relocatable layout
// of the Windows packages:
//
//	<prefix>/bin/imageprintlayout.exe        the program and its DLLs
//	<prefix>/etc/fonts/fonts.conf          fontconfig configuration
//	<prefix>/lib/gdk-pixbuf-2.0/2.10.0/loaders.cache   image loaders
//
// A Windows program is not installed under a fixed prefix, so these libraries
// cannot know where their data is. Only what the layout actually contains is
// returned: a program installed some other way gets no override, and the
// libraries keep their own defaults. exists reports whether a file is there.
func BundledEnvironment(executable string, exists func(path string) bool) map[string]string {
	prefix := filepath.Dir(filepath.Dir(executable))
	env := map[string]string{}

	fontsDir := filepath.Join(prefix, "etc", "fonts")
	fontsConf := filepath.Join(fontsDir, "fonts.conf")
	if exists(fontsConf) {
		env["FONTCONFIG_PATH"] = fontsDir
		env["FONTCONFIG_FILE"] = fontsConf
		// With a fontconfig of our own, text must be laid out with fontconfig
		// as on Linux, not with the Windows font API: the fonts a document
		// asks for are then resolved, and substituted, the same way everywhere.
		env["PANGO_CAIRO_BACKEND"] = "fc"
	}

	cache := filepath.Join(prefix, "lib", "gdk-pixbuf-2.0", "2.10.0", "loaders.cache")
	if exists(cache) {
		env["GDK_PIXBUF_MODULE_FILE"] = cache
	}
	return env
}

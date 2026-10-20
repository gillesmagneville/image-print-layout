// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"fmt"
	"os"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

// announceReady writes Options.ReadyFile once the window is shown and nothing
// is left to load or draw. A test or a script that wants a picture of the
// window waits for the file, takes the picture from outside (the toolkit cannot
// save its own window in this binding), then stops the program.
func (w *window) announceReady() {
	const tick = 250 // ms
	ticks := 0
	glib.TimeoutAdd(tick, func() bool {
		ticks++
		// The window needs a moment to be mapped and laid out; a document on
		// its way is waited for, but not forever.
		if (ticks < 4 || w.busy > 0 || w.pv.render.pending()) && ticks < 120 {
			return true
		}
		if err := os.WriteFile(w.opts.ReadyFile, []byte("ready\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			w.exitCode = 1
			w.app.Quit()
		}
		return false
	})
}

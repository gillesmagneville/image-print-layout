// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package acceptance

import "sync"

// The OS thread that runs the engine matters: Pango, which librsvg draws text
// with, keeps its default font map for each thread, and a PDF whose pages were
// drawn by two threads can embed the same font twice. The tests note the thread
// that was running each export after each of its pages, so that a PDF that
// differs from another one says whether the export changed threads.
var (
	traceMu sync.Mutex
	traces  = map[string][]uint32{}
)

// traceThreads is a Progress function for the export of the PDF at path.
func traceThreads(path string) func(done, total int) bool {
	return func(done, total int) bool {
		id := osThreadID()
		traceMu.Lock()
		traces[path] = append(traces[path], id)
		traceMu.Unlock()
		return true
	}
}

// threadsOf are the threads noted for the export of the PDF at path.
func threadsOf(path string) []uint32 {
	traceMu.Lock()
	defer traceMu.Unlock()
	return append([]uint32(nil), traces[path]...)
}

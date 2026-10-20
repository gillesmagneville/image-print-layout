// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

// worker runs, one after the other and off the thread of the window, the
// jobs that touch the rendering engine: loading a document and drawing it.
// A loaded document is used by this goroutine only, so it is never used by
// two threads at once, and the window stays responsive while a large document
// loads. Results come back to the thread of the window with glib.IdleAdd.
type worker struct {
	jobs chan func()
}

func newWorker() *worker {
	w := &worker{jobs: make(chan func(), 64)}
	go func() {
		for job := range w.jobs {
			job()
		}
	}()
	return w
}

// do queues a job.
func (w *worker) do(job func()) { w.jobs <- job }

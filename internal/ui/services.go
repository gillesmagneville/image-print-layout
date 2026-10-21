// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"github.com/gillesmagneville/pageslice/internal/app"
	"github.com/gillesmagneville/pageslice/internal/engine"
)

// defaultServices are the rendering engine of the program and the font system
// it draws text with.
func defaultServices() app.Services {
	return app.Services{Load: engine.LoadDocument, Match: engine.MatchFont}
}

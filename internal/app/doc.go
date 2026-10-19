// Package app holds the application state and use cases (open an SVG, change
// the paper, add or move a cut, export...) independently of the toolkit, so
// that the behaviour can be tested without a display. The ui package calls
// into it and never the other way round.
package app

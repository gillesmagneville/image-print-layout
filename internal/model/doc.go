// Package model holds the plain data types shared by the other packages
// (document description, layout settings, cut lines, background options).
//
// It contains data only, no behaviour and no dependency on any other
// internal package, so that every layer can import it without cycles.
// Pure Go, no cgo.
package model

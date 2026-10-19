// Command svgprintlayout is the entry point of SVG Print Layout.
//
// At this stage (M0 skeleton) it only reports its version: the graphical
// interface will be wired in internal/ui once the M0 prototype has validated
// the rendering engine and the GTK4 binding.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
)

// version is overridden at build time:
//
//	go build -ldflags "-X main.version=v0.1.0" ./cmd/svgprintlayout
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("svg-print-layout %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}

	fmt.Fprintln(os.Stderr, "svg-print-layout : l'interface graphique n'est pas encore implémentée (squelette M0).")
	os.Exit(2)
}

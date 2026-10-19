// SPDX-License-Identifier: GPL-3.0-only

// Command svgprintlayout is the entry point of SVG Print Layout.
//
// At this stage (M0) it reports its version and can check its rendering
// engine: the graphical interface will be wired in internal/ui once the M0
// prototype has validated the GTK4 binding.
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
	selfTest := flag.Bool("selftest", false, "check that the rendering engine works (loads a built-in SVG, renders it and exports a PDF) and exit")
	flag.Parse()

	// A failure here is not fatal: the libraries then use their own defaults,
	// and the self-test says what is missing.
	if err := prepareRuntime(); err != nil {
		fmt.Fprintf(os.Stderr, "Avertissement : configuration du moteur de rendu : %v\n", err)
	}

	if *showVersion {
		fmt.Println(versionLine())
		return
	}

	if *selfTest {
		if err := runSelfTest(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Échec : %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Fprintln(os.Stderr, "svg-print-layout : l'interface graphique n'est pas encore implémentée (squelette M0).")
	os.Exit(2)
}

// versionLine is what -version prints, and what a bug report should quote:
// the version, the platform and, when the engine is built in, the versions of
// the libraries it runs with.
func versionLine() string {
	line := fmt.Sprintf("svg-print-layout %s (%s/%s)", version, runtime.GOOS, runtime.GOARCH)
	if libs := libraries(); libs != "" {
		line += ", " + libs
	}
	return line
}

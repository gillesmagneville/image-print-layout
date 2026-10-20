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

	"github.com/gillesmagneville/svg-print-layout/internal/i18n"
)

// version is overridden at build time:
//
//	go build -ldflags "-X main.version=v0.1.0" ./cmd/svgprintlayout
var version = "dev"

func main() {
	// The language comes first: the descriptions of the flags are in it.
	setting := requestedLanguage(os.Args[1:])
	tr := i18n.New(i18n.Resolve(setting, os.Getenv))
	showVersion := flag.Bool("version", false, tr.T("cli.flag.version"))
	selfTest := flag.Bool("selftest", false, tr.T("cli.flag.selftest"))
	lang := flag.String("lang", i18n.Auto, tr.T("cli.flag.lang"))
	flag.Parse()
	if !knownLanguageSetting(*lang) {
		fmt.Fprintln(os.Stderr, tr.T("cli.error.language", "lang", *lang))
		os.Exit(2)
	}

	// A failure here is not fatal: the libraries then use their own defaults,
	// and the self-test says what is missing.
	if err := prepareRuntime(); err != nil {
		fmt.Fprintln(os.Stderr, tr.T("cli.warning.runtime", "error", err.Error()))
	}

	if *showVersion {
		fmt.Println(versionLine())
		return
	}

	if *selfTest {
		if err := runSelfTest(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, tr.T("cli.error.selftest", "error", err.Error()))
			os.Exit(1)
		}
		return
	}

	fmt.Fprintln(os.Stderr, tr.T("cli.gui.missing"))
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

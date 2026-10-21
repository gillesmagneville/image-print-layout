// SPDX-License-Identifier: GPL-3.0-only

// Command pageslice is the entry point of Pageslice.
//
// Without a mode flag it opens the window (internal/ui). -version reports the
// version and -selftest checks the rendering engine; both write to the
// terminal and exit without starting the toolkit.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/gillesmagneville/pageslice/internal/i18n"
	"github.com/gillesmagneville/pageslice/internal/ui"
)

// version is overridden at build time:
//
//	go build -ldflags "-X main.version=v0.1.0" ./cmd/pageslice
var version = "dev"

func main() {
	// The language comes first: the descriptions of the flags are in it.
	setting := requestedLanguage(os.Args[1:])
	tr := i18n.New(i18n.Resolve(setting, os.Getenv))
	showVersion := flag.Bool("version", false, tr.T("cli.flag.version"))
	selfTest := flag.Bool("selftest", false, tr.T("cli.flag.selftest"))
	lang := flag.String("lang", i18n.Auto, tr.T("cli.flag.lang"))
	readyFile := flag.String("ready-file", "", tr.T("cli.flag.ready_file"))
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

	if !ui.Available {
		fmt.Fprintln(os.Stderr, tr.T("cli.error.no_gui"))
		os.Exit(1)
	}
	// The language given on the command line wins; without it, the preference
	// the user saved (or the language of the system) applies.
	os.Exit(ui.Run(ui.Options{
		Language:  setting,
		Open:      flag.Arg(0),
		ReadyFile: *readyFile,
		Version:   version,
	}))
}

// versionLine is what -version prints, and what a bug report should quote:
// the version, the platform and, when the engine is built in, the versions of
// the libraries it runs with.
func versionLine() string {
	line := fmt.Sprintf("pageslice %s (%s/%s)", version, runtime.GOOS, runtime.GOARCH)
	if libs := libraries(); libs != "" {
		line += ", " + libs
	}
	return line
}

// SPDX-License-Identifier: GPL-3.0-only

package ui

// Options start the window. The zero value is what a user gets by running the
// program with no argument.
type Options struct {
	// Language is the -lang setting of the command line: a language code or
	// "auto". Empty follows the saved preference of the user.
	Language string
	// Open is the SVG document to open as soon as the window is shown.
	Open string
	// ReadyFile, when not empty, is a file the program creates once the window
	// is shown and the document drawn. A test or a script that takes a picture
	// of the window from outside waits for it.
	ReadyFile string
	// Version is shown in the About box.
	Version string
}

// AppID is the identifier of the application: the name of its desktop file
// and its AppStream component, in reverse domain order.
const AppID = "io.github.gillesmagneville.imageprintlayout"

// AppName is the name of the program as people read it.
const AppName = "Image Print Layout"

// IconName is the name of the icon of the program in the icon theme: the one
// the packages install (hicolor/scalable/apps/imageprintlayout.svg) and the one
// the desktop file names. It is not the AppID.
const IconName = "imageprintlayout"

// Author and Website are shown in the About box and in the documents.
const Author = "Gilles MAGNEVILLE"

// Website is the home of the project, shown in the About box.
const Website = "https://github.com/gillesmagneville/image-print-layout"

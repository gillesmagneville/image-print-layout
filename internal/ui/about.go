// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/image-print-layout/icons"
)

// aboutLogoSize is the size of the icon in the About box, in pixels.
const aboutLogoSize = 128

// showAbout opens the About box: the icon, the name and the version, who made
// the program, the licence. It is a window of our own rather than a
// GtkAboutDialog: that one selects its title when it opens, shows its texts in
// the language of the system and not in the one chosen in the program, and
// draws its logo from the icon theme, where the icon is not when the program
// runs from a directory.
func (w *window) showAbout() {
	tr := w.texts.tr
	about := gtk.NewWindow()
	about.SetApplication(w.app)
	about.SetTitle(tr.T("menu.about"))
	about.SetTransientFor(&w.win.Window)
	about.SetModal(true)
	about.SetResizable(false)
	about.SetDestroyWithParent(true)

	box := gtk.NewBox(gtk.OrientationVertical, 6)
	box.SetMarginTop(24)
	box.SetMarginBottom(18)
	box.SetMarginStart(32)
	box.SetMarginEnd(32)
	box.SetHAlign(gtk.AlignCenter)

	if logo := aboutLogo(); logo != nil {
		logo.SetMarginBottom(6)
		box.Append(logo)
	}

	name := gtk.NewLabel(AppName)
	name.AddCSSClass("title-1")
	name.SetSelectable(false)
	box.Append(name)

	if w.opts.Version != "" {
		version := gtk.NewLabel(tr.T("about.version", "version", w.opts.Version))
		version.AddCSSClass("dim-label")
		box.Append(version)
	}

	tagline := centered(tr.T("app.tagline"))
	tagline.SetMarginTop(6)
	box.Append(tagline)

	credits := gtk.NewBox(gtk.OrientationVertical, 2)
	credits.SetMarginTop(14)
	credits.Append(centered(tr.T("about.developed", "name", Author)))
	credits.Append(centered(tr.T("about.designed")))
	box.Append(credits)

	site := gtk.NewLinkButtonWithLabel(Website, tr.T("about.website"))
	site.SetMarginTop(8)
	site.SetHAlign(gtk.AlignCenter)
	box.Append(site)

	licence := centered(tr.T("about.license"))
	licence.AddCSSClass("caption")
	licence.AddCSSClass("dim-label")
	licence.SetMarginTop(8)
	licence.SetMaxWidthChars(48)
	box.Append(licence)

	closeButton := gtk.NewButtonWithLabel(tr.T("dialog.close"))
	closeButton.SetMarginTop(14)
	closeButton.SetHAlign(gtk.AlignCenter)
	closeButton.ConnectClicked(func() { about.Close() })
	box.Append(closeButton)

	keys := gtk.NewEventControllerKey()
	keys.ConnectKeyPressed(func(keyval, _ uint, _ gdk.ModifierType) bool {
		if keyval == gdk.KEY_Escape {
			about.Close()
			return true
		}
		return false
	})
	about.AddController(keys)

	about.SetChild(box)
	about.SetDefaultWidget(closeButton)
	about.SetFocus(closeButton)
	about.Present()
}

// centered is a label for a paragraph of the About box: wrapped, centred, and
// not selectable (a selectable label takes the focus and shows up selected).
func centered(text string) *gtk.Label {
	l := gtk.NewLabel(text)
	l.SetWrap(true)
	l.SetJustify(gtk.JustifyCenter)
	l.SetSelectable(false)
	return l
}

// aboutLogo is the picture of the icon, or nil if it cannot be read: the box
// is then shown without it, which beats not showing it.
func aboutLogo() *gtk.Image {
	texture, err := gdk.NewTextureFromBytes(glib.NewBytes(icons.PNG))
	if err != nil {
		return nil
	}
	image := gtk.NewImageFromPaintable(texture)
	image.SetPixelSize(aboutLogoSize)
	return image
}

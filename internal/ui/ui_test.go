// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/svg-print-layout/internal/app"
	"github.com/gillesmagneville/svg-print-layout/internal/i18n"
	"github.com/gillesmagneville/svg-print-layout/internal/testsvg"
)

// GTK may be driven from one thread only: the one that starts it. The test
// binary's main goroutine runs TestMain, so it is locked to the main thread
// here and the whole scenario runs in TestMain, before the tests; the test
// below reports what the scenario found. Without a display (a plain terminal,
// a CI job without a virtual screen) there is no scenario and the test skips.
func init() { runtime.LockOSThread() }

var scenario struct {
	ran      bool
	failures []string
}

func TestMain(m *testing.M) {
	if os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" {
		runScenario()
	}
	os.Exit(m.Run())
}

func TestWindowScenario(t *testing.T) {
	if !scenario.ran {
		t.Skip("no display: run it under xvfb-run")
	}
	for _, f := range scenario.failures {
		t.Error(f)
	}
}

// rawKey is the shape of a message key shown by mistake: panel.paper.title.
var rawKey = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$`)

// texts returns every text the widget and its descendants show.
func texts(root gtk.Widgetter) []string {
	var out []string
	var walk func(gtk.Widgetter)
	walk = func(widget gtk.Widgetter) {
		switch w := widget.(type) {
		case *gtk.Label:
			out = append(out, w.Label())
		case *gtk.Button:
			out = append(out, w.Label())
		case *gtk.Expander:
			out = append(out, w.Label())
		}
		for child := gtk.BaseWidget(widget).FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
			walk(child)
		}
	}
	walk(root)
	return out
}

// plain turns the no-break spaces of French typography into ordinary ones, so
// that a text can be written in a test the way it reads.
func plain(s string) string {
	return strings.NewReplacer("\u00a0", " ", "\u202f", " ").Replace(s)
}

// contains reports whether one of the texts reads as want, typography apart.
func contains(list []string, want string) bool {
	for _, s := range list {
		if plain(s) == plain(want) {
			return true
		}
	}
	return false
}

// runScenario opens a window and drives it from the main loop: each step runs
// on the thread of the window, and a step that has to wait for the engine
// (a document loading) returns false until it is done.
func runScenario() {
	scenario.ran = true
	fail := func(format string, args ...any) {
		scenario.failures = append(scenario.failures, fmt.Sprintf(format, args...))
	}

	dir, err := os.MkdirTemp("", "svgpl-ui-test")
	if err != nil {
		fail("%v", err)
		return
	}
	defer os.RemoveAll(dir)
	svg := filepath.Join(dir, "plan.svg")
	if err := os.WriteFile(svg, testsvg.Generate(testsvg.Options{}), 0o644); err != nil {
		fail("%v", err)
		return
	}
	notSVG := filepath.Join(dir, "notes.svg")
	if err := os.WriteFile(notSVG, []byte("this is not an SVG"), 0o644); err != nil {
		fail("%v", err)
		return
	}
	settingsPath := filepath.Join(dir, "config", "settings.json")

	application := gtk.NewApplication(AppID+".test", gio.ApplicationFlagsNone)
	application.ConnectActivate(func() {
		w := newWindow(application, Options{Version: "test"}, app.DefaultSettings(), settingsPath, i18n.French)
		w.show()

		// checkTexts verifies that the window speaks the language: the texts of
		// the catalog are there, and no key or placeholder shows through.
		checkTexts := func(context string, lang i18n.Lang, keys ...string) {
			tr := i18n.New(lang)
			shown := texts(w.win)
			for _, key := range keys {
				if want := tr.T(key); !contains(shown, want) {
					fail("%s: %q (%s) is not shown; the window shows %q", context, want, key, shown)
				}
			}
			for _, s := range shown {
				if rawKey.MatchString(s) || strings.Contains(s, "{") {
					fail("%s: the window shows %q, a key or a placeholder", context, s)
				}
			}
		}
		panelKeys := []string{"panel.source.title", "panel.paper.title", "panel.margins.title", "panel.scale.title",
			"panel.placement.title", "panel.background.title", "panel.export.title", "pages.title", "pages.none",
			"preview.empty.title", "preview.empty.hint", "action.open_svg", "status.ready", "source.none"}

		type step func() bool
		var steps []step
		add := func(f step) { steps = append(steps, f) }

		// 1. The window opens in the language it was given, and follows each of
		// the four when the menu changes it; the choice is saved.
		add(func() bool {
			checkTexts("start", i18n.French, panelKeys...)
			return true
		})
		for _, lang := range i18n.Languages {
			add(func() bool {
				w.setLanguageSetting(string(lang))
				checkTexts("language "+string(lang), lang, panelKeys...)
				if saved, err := app.LoadSettings(settingsPath); err != nil || saved.Language != string(lang) {
					fail("language %s: the saved setting is %+v, %v", lang, saved, err)
				}
				return true
			})
		}

		// 2. A document opens off the thread of the window and is shown.
		add(func() bool { w.setLanguageSetting("fr"); w.openPath(svg); return true })
		add(func() bool {
			if !w.hasDoc || w.busy > 0 || w.pv.surface == nil {
				return false
			}
			if name := w.stack.VisibleChildName(); name != "preview" {
				fail("after opening: the visible page is %q, want the preview", name)
			}
			if w.docW != 1280 || w.docH != 5479 {
				fail("the document is %v x %v, want 1280 x 5479", w.docW, w.docH)
			}
			if got := w.win.Title(); got != "plan.svg — SVG Print Layout" {
				fail("the title is %q", got)
			}
			checkTexts("document (fr)", i18n.French)
			shown := texts(w.win)
			for _, want := range []string{"Fichier : plan.svg", "Taille : 1280 × 5479 unités", "À 100 % : 338,7 × 1449,7 mm", "plan.svg : 1280 × 5479 unités."} {
				if !contains(shown, want) {
					fail("document (fr): %q is not shown; the window shows %q", want, shown)
				}
			}
			if size := w.pv.size; size.X <= 0 || size.Y <= 0 || size.X > w.pv.area.Width() || size.Y > w.pv.area.Height() {
				fail("the drawing is %v in a %d x %d area", size, w.pv.area.Width(), w.pv.area.Height())
			}
			return true
		})

		// 3. The document's texts follow a change of language, with the decimal
		// notation of the language.
		add(func() bool {
			w.setLanguageSetting("en")
			shown := texts(w.win)
			for _, want := range []string{"Size: 1280 × 5479 units", "At 100%: 338.7 × 1449.7 mm", "plan.svg: 1280 × 5479 units."} {
				if !contains(shown, want) {
					fail("document (en): %q is not shown; the window shows %q", want, shown)
				}
			}
			w.setLanguageSetting("de")
			if shown := texts(w.win); !contains(shown, "Bei 100 %: 338,7 × 1449,7 mm") {
				fail("document (de): the size at 100%% is not shown with a comma; the window shows %q", shown)
			}
			return true
		})

		// 4. A file that is not an SVG is refused with a message, and the
		// document that was open stays.
		add(func() bool { w.openPath(notSVG); return true })
		add(func() bool {
			if w.busy > 0 {
				return false
			}
			if !w.hasDoc || filepath.Base(w.path) != "plan.svg" {
				fail("after a refused file the open document is %q (hasDoc %v), want plan.svg", w.path, w.hasDoc)
			}
			windows := application.Windows()
			if len(windows) != 2 {
				fail("a message window should be open: %d windows", len(windows))
				return true
			}
			var message []string
			for _, win := range windows {
				if win.Object.Native() != w.win.Object.Native() {
					message = texts(win)
					win.Close()
				}
			}
			tr := i18n.New(i18n.German)
			if !contains(message, tr.T("dialog.error.open")) || !contains(message, tr.T("dialog.close")) {
				fail("the message is not in German: %q", message)
			}
			return true
		})

		// The steps run one at a time from the main loop; a step that is not
		// done yet is asked again, for at most ten seconds.
		i, started := 0, time.Now()
		glib.TimeoutAdd(40, func() bool {
			if i >= len(steps) {
				application.Quit()
				return false
			}
			if steps[i]() {
				i++
				started = time.Now()
			} else if time.Since(started) > 10*time.Second {
				fail("step %d did not finish in ten seconds", i+1)
				application.Quit()
				return false
			}
			return true
		})
	})
	application.Run([]string{"test"})
}

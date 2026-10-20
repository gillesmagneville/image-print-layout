// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/svg-print-layout/internal/app"
	"github.com/gillesmagneville/svg-print-layout/internal/engine"
	"github.com/gillesmagneville/svg-print-layout/internal/export"
	"github.com/gillesmagneville/svg-print-layout/internal/i18n"
	"github.com/gillesmagneville/svg-print-layout/internal/layout"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
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

// expandAll opens every expander under root.
func expandAll(root gtk.Widgetter) {
	if e, ok := root.(*gtk.Expander); ok {
		e.SetExpanded(true)
	}
	for child := gtk.BaseWidget(root).FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
		expandAll(child)
	}
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
			if w.session.Document() == nil || w.busy > 0 || !w.pv.hasImage {
				return false
			}
			doc := w.session.Document()
			if name := w.stack.VisibleChildName(); name != "preview" {
				fail("after opening: the visible page is %q, want the preview", name)
			}
			if doc.Size.W != 1280 || doc.Size.H != 5479 {
				fail("the document is %v x %v, want 1280 x 5479", doc.Size.W, doc.Size.H)
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
			// The whole document is in view, inside the widget.
			if vis := w.pv.view.Visible(); vis.W < 1279 || vis.H < 5478 {
				fail("the view shows %+v, want the whole document", vis)
			}
			if w.pv.view.Zoom <= 0 || w.pv.view.Zoom*5479 > float64(w.pv.marks.Height()) {
				fail("zoom %v does not fit a document of 5479 units in %d pixels", w.pv.view.Zoom, w.pv.marks.Height())
			}
			return true
		})

		// 2b. The plan of the default project is in the page list: 200 mm
		// wide on A4 portrait, one page that is far too tall, and proposals.
		add(func() bool {
			plan := w.session.Plan()
			if plan == nil || plan.ContentWidthMM != 200 || len(plan.Pages) != 1 {
				fail("the first plan is %+v", plan)
				return true
			}
			shown := texts(w.win)
			for _, want := range []string{"1 page", "Largeur du contenu : 200 mm (20 cm)", "Largeur du dessin : 200 mm (20 cm)",
				"Page 1", "Dépasse la feuille", "Propositions", "Zone utile : 200 × 277 mm", "Feuille : 210 × 297 mm"} {
				if !contains(shown, want) {
					fail("plan (fr): %q is not shown; the window shows %q", want, shown)
				}
			}
			if !w.pages.proposals.IsVisible() {
				fail("no proposal is offered for a page that is too tall")
			}
			return true
		})

		// 2c. A value typed in a field is applied, in the notation of the
		// language, and Undo takes it back.
		add(func() bool {
			f := w.panel.manualWidth
			w.panel.modeC.SetActive(true)
			if p := w.session.Project(); p.Scale.Mode != model.Width || p.Scale.WidthMM != 200 {
				fail("going to the manual mode gave %+v, want a width of 200 mm (the width the drawing had)", p.Scale)
			}
			f.entry.SetText("190,5")
			f.typing.cancel()
			f.commit()
			if p := w.session.Project(); p.Scale.WidthMM != 190.5 {
				fail("a typed width of 190,5 gave %v", p.Scale.WidthMM)
			}
			if got := texts(w.win); !contains(got, "Largeur du dessin : 190,5 mm (19,05 cm)") {
				fail("the result is not shown after typing; the window shows %q", got)
			}
			// A value the session refuses is put back.
			f.entry.SetText("0,5")
			f.typing.cancel()
			f.commit()
			if w.session.Project().Scale.WidthMM != 190.5 {
				fail("a width under the minimum of the field was accepted")
			}
			w.undo()
			w.undo()
			if p := w.session.Project(); p.Scale.Mode != model.FitWidth {
				fail("after two undos the scale is %+v, want the fit to the width", p.Scale)
			}
			if !contains(texts(w.win), "Largeur du dessin : 200 mm (20 cm)") {
				fail("the result is not 200 mm after undo")
			}
			return true
		})

		// 2d. A margin changes the usable area; an impossible one is refused
		// and the field shows the old value again.
		add(func() bool {
			w.panel.margin["left"].entry.SetText("20")
			w.panel.margin["left"].typing.cancel()
			w.panel.margin["left"].commit()
			if m := w.session.Project().Margins; m.Left != 20 || m.Right != 5 || m.Top != 10 {
				fail("the margins are %+v, want left 20 only", m)
			}
			if !contains(texts(w.win), "Zone utile : 185 × 277 mm") {
				fail("the usable area did not follow the margin; the window shows %q", texts(w.win))
			}
			w.panel.margin["left"].entry.SetText("400")
			w.panel.margin["left"].typing.cancel()
			w.panel.margin["left"].commit()
			if w.session.Project().Margins.Left != 20 {
				fail("margins that leave no room were accepted")
			}
			if got := w.panel.margin["left"].entry.Text(); got != "20" {
				fail("the field shows %q after a refusal, want 20", got)
			}
			if !strings.Contains(plain(w.status.Label()), "Réglage refusé") {
				fail("the status bar says %q after a refusal", w.status.Label())
			}
			w.undo()
			return true
		})

		// 2e. A proposal is applied only when the user chooses it.
		add(func() bool {
			plan := w.session.Plan()
			if len(plan.Pages) != 1 {
				fail("a proposal was applied by itself: %d pages", len(plan.Pages))
			}
			for _, c := range plan.Corrections {
				if c.Kind == layout.CorrectionAddCut {
					w.applyCorrection(c)
					break
				}
			}
			if n := len(w.session.Plan().Pages); n < 2 {
				fail("after the proposal to add a cut there are %d pages", n)
			}
			if w.session.Plan().ContentWidthMM != 200 {
				fail("a cut changed the width of the drawing")
			}
			if !contains(texts(w.win), "Page 2") || len(w.pages.rows) != len(w.session.Plan().Pages) {
				fail("the page list does not show the pages; %d rows", len(w.pages.rows))
			}
			return true
		})

		// 2f. Zoom and move.
		add(func() bool {
			z := w.pv.view.Zoom
			w.pv.zoomIn()
			if w.pv.view.Zoom <= z {
				fail("zoom in did not zoom")
			}
			w.pv.showWidth()
			if want := (float64(w.pv.marks.Width()) - 48) / 1280; math.Abs(w.pv.view.Zoom-want) > 1e-6 {
				fail("fit to width gives a zoom of %v, want %v", w.pv.view.Zoom, want)
			}
			w.pv.showAll()
			return true
		})
		add(func() bool { return w.busy == 0 && !w.pv.render.pending() })

		// 2g. The cuts with the mouse and the keyboard (acceptance tests 5, 6 and
		// 7): the gestures of the preview are driven through the functions the
		// gesture controllers call.
		add(func() bool {
			pv := w.pv
			cuts := w.session.Cuts()
			plan := w.session.Plan()
			if len(cuts) != 1 || len(plan.Pages) != 2 {
				fail("cuts: %d cuts and %d pages before the gestures, want 1 and 2", len(cuts), len(plan.Pages))
				return true
			}
			scale, width := plan.Scale, plan.ContentWidthMM
			x0, x1 := pv.sheetX()
			x := (x0 + x1) / 2
			screenY := func(y float64) float64 { _, sy := pv.view.ToScreen(0, y); return sy }
			unchanged := func(what string) {
				if p := w.session.Plan(); p.Scale != scale || p.ContentWidthMM != width {
					fail("cuts: %s changed the scale (%v to %v) or the width (%v to %v)", what, scale, p.Scale, width, p.ContentWidthMM)
				}
			}
			checkTexts("cuts (fr)", i18n.French, "cuts.title", "cuts.add", "cuts.auto")
			if !contains(texts(w.win), "Coupure 1") {
				fail("cuts: the list does not show the first cut; the window shows %q", texts(w.win))
			}

			// The pointer over the line selects it for the cursor and the handle.
			c := cuts[0]
			pv.hover(x, screenY(c.Position)+2)
			if pv.hoverCut != c.ID {
				fail("cuts: the pointer on the line of cut %d hovers %d", c.ID, pv.hoverCut)
			}
			pv.hover(x, screenY(c.Position)+40)
			if pv.hoverCut != 0 {
				fail("cuts: the pointer 40 pixels from the line hovers cut %d", pv.hoverCut)
			}

			// A drag moves the cut, by the offset of the pointer, and is one
			// entry of the history.
			before := len(w.session.Cuts())
			history := w.session.UndoLabel()
			pv.beginDrag(x, screenY(c.Position)+1, 1)
			if pv.mode != dragCut {
				fail("cuts: a press on the line does not start a drag of the cut")
			}
			for _, dy := range []float64{-10, -20, -30, -40} {
				pv.updateDrag(0, dy, true)
			}
			pv.endDrag(0, -40)
			want := c.Position - 40/pv.view.Zoom
			if got := w.session.Cuts()[0].Position; math.Abs(got-want) > 1e-6 {
				fail("cuts: dragged 40 pixels up, the cut is at %v, want %v", got, want)
			}
			unchanged("a drag")
			if w.session.UndoLabel() != "undo.move_cut" || history == "undo.move_cut" {
				fail("cuts: the history says %q after a drag (before: %q)", w.session.UndoLabel(), history)
			}
			w.undo()
			if got := w.session.Cuts()[0].Position; got != c.Position {
				fail("cuts: undo of a drag gives %v, want %v", got, c.Position)
			}

			// A press away from every line moves the view, not a cut.
			oy := pv.view.OY
			pv.beginDrag(x, screenY(c.Position)+80, 1)
			if pv.mode != dragPan {
				fail("cuts: a press away from the lines does not move the view")
			}
			pv.endDrag(0, 0)
			if pv.view.OY != oy {
				fail("cuts: a click moved the view")
			}

			// A double click adds a cut where it is; a double click on a line does not.
			y := 0.0
			for _, cand := range []float64{900, 1800, 2600, 3300, 4200} {
				if math.Abs(cand-c.Position) > 300 {
					y = cand
					break
				}
			}
			pv.doubleClickAt(x, screenY(y), true)
			cuts = w.session.Cuts()
			if len(cuts) != before+1 {
				fail("cuts: a double click gives %d cuts, want %d", len(cuts), before+1)
			}
			unchanged("adding a cut")
			pv.doubleClickAt(x, screenY(cuts[0].Position), true)
			if len(w.session.Cuts()) != before+1 {
				fail("cuts: a double click on a line added a cut")
			}
			if len(w.pages.cutRows) != before+1 || !contains(texts(w.win), "Coupure 2") {
				fail("cuts: the list shows %d rows, want %d", len(w.pages.cutRows), before+1)
			}

			// The keyboard moves and removes the selected cut.
			var added app.CutView
			for _, cv := range cuts {
				if cv.Position != c.Position {
					added = cv
				}
			}
			w.selectCut(added.ID)
			pv.marks.QueueDraw()
			if !pv.keyPressed(gdk.KEY_Down, 0) {
				fail("cuts: the arrow keys do nothing on a selected cut")
			}
			for _, cv := range w.session.Cuts() {
				if cv.ID == added.ID && math.Abs(cv.Position-(added.Position+1)) > 1e-9 {
					fail("cuts: an arrow moved the cut from %v to %v, want one unit", added.Position, cv.Position)
				}
			}
			unchanged("a key")
			if !pv.keyPressed(gdk.KEY_Delete, 0) || len(w.session.Cuts()) != before {
				fail("cuts: Delete did not remove the selected cut")
			}
			if pv.selCut != 0 {
				fail("cuts: the removed cut is still selected")
			}
			w.undo()
			if len(w.session.Cuts()) != before+1 {
				fail("cuts: undo of a removal does not bring the cut back")
			}
			w.undo() // the move by key
			w.undo() // the added cut
			if len(w.session.Cuts()) != before {
				fail("cuts: %d cuts after undoing the double click, want %d", len(w.session.Cuts()), before)
			}

			// A click on the drawing selects the page; so does a row of the list.
			pv.clickAt(x, screenY(c.Position-200))
			if w.selPage != 1 || w.pages.list.SelectedRow() == nil || w.pages.list.SelectedRow().Index() != 0 {
				fail("cuts: a click on page 1 selects page %d", w.selPage)
			}
			w.pages.list.SelectRow(w.pages.rows[1].row)
			if w.selPage != 2 || pv.selPage != 2 {
				fail("cuts: the second row of the list selects page %d (preview: %d)", w.selPage, pv.selPage)
			}
			pv.clickAt(x, screenY(c.Position)-3) // on the line: the cut, not the page
			if w.selCut != c.ID || w.selPage != 0 {
				fail("cuts: a click on the line selects cut %d and page %d", w.selCut, w.selPage)
			}
			w.selectCut(0)
			return true
		})

		// 2h. The automatic cuts replace the cuts by a proposal that undoing brings back.
		add(func() bool {
			before := w.session.Cuts()
			w.pv.showAll()
			w.autoCuts()
			plan := w.session.Plan()
			if !plan.AllConforming() {
				fail("automatic cuts leave a page that does not fit")
			}
			if w.session.Plan().ContentWidthMM != 200 {
				fail("automatic cuts changed the width of the drawing")
			}
			for _, cv := range w.session.Cuts() {
				if cv.Crossed != 0 {
					fail("automatic cuts: cut %d crosses %d blocks although the document has voids", cv.Number, cv.Crossed)
				}
			}
			if status := plain(w.status.Label()); !strings.Contains(status, "coupures proposées") {
				fail("automatic cuts: the status says %q", status)
			}
			w.undo()
			if got := w.session.Cuts(); len(got) != len(before) {
				fail("undo of the automatic cuts gives %d cuts, want %d", len(got), len(before))
			}
			return true
		})

		// 3. The document's texts follow a change of language, with the decimal
		// notation of the language.
		add(func() bool {
			w.setLanguageSetting("en")
			shown := texts(w.win)
			// The status bar still says what the automatic cuts did, now in English,
			// with the plural of the language.
			for _, want := range []string{"Size: 1280 × 5479 units", "At 100%: 338.7 × 1449.7 mm", "3 cuts proposed, for 4 pages (Ctrl+Z to go back)."} {
				if !contains(shown, want) {
					fail("document (en): %q is not shown; the window shows %q", want, shown)
				}
			}
			w.setLanguageSetting("de")
			if shown := texts(w.win); !contains(shown, "Bei 100 %: 338,7 × 1449,7 mm") {
				fail("document (de): the size at 100%% is not shown with a comma; the window shows %q", shown)
			}
			if shown := texts(w.win); !contains(shown, "3 Schnitte vorgeschlagen, für 4 Seiten (Strg+Z zum Zurückgehen).") {
				fail("document (de): the status of the automatic cuts is not in German; the window shows %q", shown)
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
			if doc := w.session.Document(); doc == nil || filepath.Base(doc.Path) != "plan.svg" {
				fail("after a refused file the open document is %+v, want plan.svg", doc)
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

		// 5. The background of the document: the choice is applied to the working
		// copy off the thread of the window, and what the engine draws follows it,
		// whatever way the choice was made (a radio button, undo, redo).
		add(func() bool { w.setLanguageSetting("fr"); return true })
		pixelAt := func() [4]uint8 {
			// A spot of the document that only the background paints.
			img, err := w.engineDoc.Render(engine.Box{X: 1270, Y: 2000, W: 2, H: 2}, 1, nil)
			if err != nil {
				fail("background: the engine cannot draw: %v", err)
				return [4]uint8{}
			}
			c := img.RGBAAt(0, 0)
			return [4]uint8{c.R, c.G, c.B, c.A}
		}
		ready := func() bool {
			return !w.rebuilding && w.busy == 0 && w.loadedBG == w.session.Project().Background.Document
		}
		add(func() bool {
			if got := pixelAt(); got != [4]uint8{0xf4, 0xf5, 0xf8, 255} {
				fail("background: the kept background is %v, want #f4f5f8", got)
			}
			if _, ok := w.session.DocumentBackground(); !ok || !w.panel.bgWhiten.IsSensitive() || !w.panel.bgRemove.IsSensitive() {
				fail("background: the detected background is not offered for replacement")
			}
			info, _ := w.session.DocumentBackground()
			tr := i18n.New(i18n.French)
			if want := tr.T("bg.doc.found", "element", tr.T("bg.element.rect"), "fill", info.Fill); plain(w.panel.bgFound.Label()) != plain(want) || !w.panel.bgFound.IsVisible() {
				fail("background: the detected background is not described: %q, want %q", w.panel.bgFound.Label(), want)
			}
			docRev := w.docRev
			w.panel.bgWhiten.SetActive(true)
			if p := w.session.Project(); p.Background.Document != model.WhitenBackground || p.Background.Element == "" {
				fail("background: the project says %+v after the choice of white", p.Background)
			}
			if w.session.Plan().ContentWidthMM != 200 {
				fail("background: the choice changed the width of the drawing")
			}
			if !w.rebuilding || w.docRev != docRev {
				fail("background: the working copy is not being prepared (rebuilding=%v)", w.rebuilding)
			}
			return true
		})
		add(func() bool {
			if !ready() {
				return false
			}
			if got := pixelAt(); got != [4]uint8{255, 255, 255, 255} {
				fail("background: the whitened background is %v, want opaque white", got)
			}
			w.panel.bgRemove.SetActive(true)
			return true
		})
		add(func() bool {
			if !ready() {
				return false
			}
			if got := pixelAt(); got[3] != 0 {
				fail("background: the removed background still paints %v", got)
			}
			w.undo()
			w.undo()
			return true
		})
		add(func() bool {
			if !ready() {
				return false
			}
			if got := pixelAt(); got != [4]uint8{0xf4, 0xf5, 0xf8, 255} {
				fail("background: after two undos the background is %v, want #f4f5f8", got)
			}
			if !w.panel.bgKeep.Active() {
				fail("background: the radio buttons do not follow the undo")
			}
			w.redo()
			return true
		})
		add(func() bool {
			if !ready() {
				return false
			}
			if got := pixelAt(); got != [4]uint8{255, 255, 255, 255} {
				fail("background: after a redo the background is %v, want white", got)
			}
			// The page background is another thing: no engine work for it.
			rev := w.docRev
			w.panel.pageTransparent.SetActive(true)
			if w.session.Project().Background.Page != model.TransparentPage || !w.pv.transparentPage {
				fail("background: the transparent page is not applied")
			}
			if w.rebuilding || w.docRev != rev {
				fail("background: the background of the page reloaded the document")
			}
			w.panel.pageWhite.SetActive(true)
			if w.pv.transparentPage {
				fail("background: the page is still shown transparent")
			}
			w.panel.bgKeep.SetActive(true)
			return true
		})

		// 6. The sheets: every page drawn by the function that writes the PDF.
		add(func() bool {
			if !ready() {
				return false
			}
			w.autoCuts()
			w.showSheets(2)
			return true
		})
		add(func() bool {
			if w.busy > 0 {
				return false
			}
			plan := w.session.Plan()
			if name := w.stack.VisibleChildName(); name != "sheets" || !w.viewSheets.Active() {
				fail("sheets: the visible page is %q", name)
			}
			if len(w.sheets.cards) != len(plan.Pages) || len(plan.Pages) != 4 {
				fail("sheets: %d cards for %d pages", len(w.sheets.cards), len(plan.Pages))
				return true
			}
			for i, c := range w.sheets.cards {
				if !c.drawn.ok || !c.picture.IsVisible() {
					fail("sheets: sheet %d is not drawn", i+1)
				}
				if c.selected != (i == 1) {
					fail("sheets: sheet %d selected = %v", i+1, c.selected)
				}
			}
			if w.selPage != 2 {
				fail("sheets: page %d is selected, want 2", w.selPage)
			}
			// The preview button of a row shows that sheet.
			w.pages.rows[2].preview.Emit("clicked")
			if w.selPage != 3 {
				fail("sheets: the preview button of page 3 selects page %d", w.selPage)
			}
			// The page background reaches the sheets; the document is not drawn again.
			w.panel.pageTransparent.SetActive(true)
			return true
		})
		add(func() bool {
			if w.busy > 0 {
				return false
			}
			for i, c := range w.sheets.cards {
				if c.drawn.white || !c.drawn.ok {
					fail("sheets: sheet %d was not drawn again without its white page", i+1)
				}
			}
			w.panel.pageWhite.SetActive(true)
			w.showView("preview")
			if w.stack.VisibleChildName() != "preview" || !w.viewDoc.Active() {
				fail("sheets: the switch does not go back to the document")
			}
			return true
		})

		// 7. The export: stopped by the checks, then written, then read back.
		exportPath := filepath.Join(dir, "plan.pdf")
		stoppedPath := filepath.Join(dir, "stopped.pdf")
		add(func() bool {
			if w.busy > 0 {
				return false
			}
			// One page too tall: the export is refused, with the reason.
			for _, c := range w.session.Cuts() {
				_ = w.session.DeleteCut(c.ID)
			}
			w.refreshAll()
			if !export.Blocked(w.session.CheckExport()) {
				fail("export: a page that is too tall is not blocked")
			}
			w.exportPDF()
			windows := application.Windows()
			if len(windows) != 2 {
				fail("export: a report should be open: %d windows", len(windows))
				return true
			}
			var report []string
			for _, win := range windows {
				if win.Object.Native() != w.win.Object.Native() {
					report = texts(win)
					win.Close()
				}
			}
			tr := i18n.New(i18n.French)
			for _, key := range []string{"dialog.export_blocked.title", "dialog.export_blocked.detail", "dialog.close"} {
				if !contains(report, tr.T(key)) {
					fail("export: the report does not show %q; it shows %q", tr.T(key), report)
				}
			}
			if !contains(report, "Page 1 : "+tr.T("diag.export.outside_sheet")) {
				fail("export: the report does not name the page and the reason; it shows %q", report)
			}
			if !contains(texts(w.win), tr.T("export.state.blocked")) {
				fail("export: the export section does not say it is blocked")
			}
			w.autoCuts() // the automatic cuts again
			return true
		})
		add(func() bool {
			items := w.session.CheckExport()
			if export.Blocked(items) {
				fail("export: with the automatic cuts the export is still blocked: %v", items)
				return true
			}
			tr := i18n.New(i18n.French)
			if export.NeedsConfirmation(items) {
				// A warning asks for a confirmation, and says what it is about.
				w.exportPDF()
				var report []string
				for _, win := range application.Windows() {
					if win.Object.Native() != w.win.Object.Native() {
						report = texts(win)
						win.Close()
					}
				}
				if !contains(report, tr.T("dialog.export_confirm.title")) || !contains(report, tr.T("dialog.export_confirm.go")) || !contains(report, tr.T("dialog.cancel")) {
					fail("export: the confirmation is not shown; the dialog shows %q", report)
				}
			}
			w.runExport(exportPath)
			if !w.exporting || !w.panel.exportBar.IsVisible() {
				fail("export: the progress is not shown")
			}
			return true
		})
		add(func() bool {
			if w.exporting || w.busy > 0 {
				return false
			}
			data, err := os.ReadFile(exportPath)
			if err != nil {
				fail("export: no file: %v", err)
				return true
			}
			pdf, err := export.ReadPDF(data)
			if err != nil || len(pdf.Pages) != 4 {
				fail("export: the file has %d pages (%v), want 4", len(pdf.Pages), err)
			}
			if status := plain(w.status.Label()); !strings.HasPrefix(status, "plan.pdf : 4 pages, ") || !strings.Contains(status, "relu") {
				fail("export: the status says %q", status)
			}
			if w.panel.exportBar.IsVisible() {
				fail("export: the progress is still shown")
			}
			if entries, _ := filepath.Glob(filepath.Join(dir, ".*.part")); len(entries) != 0 {
				fail("export: a temporary file is left: %v", entries)
			}
			if w.lastExportDir != dir {
				fail("export: the folder of the next export is %q", w.lastExportDir)
			}
			// An export that is stopped writes nothing.
			w.runExport(stoppedPath)
			w.cancelExportRun()
			return true
		})
		add(func() bool {
			if w.exporting || w.busy > 0 {
				return false
			}
			if _, err := os.Stat(stoppedPath); err == nil {
				fail("export: a stopped export left a file")
			}
			if entries, _ := filepath.Glob(filepath.Join(dir, ".*.part")); len(entries) != 0 {
				fail("export: a stopped export left %v", entries)
			}
			if status := plain(w.status.Label()); !strings.HasPrefix(status, "Export annulé") {
				fail("export: the status after stopping says %q", status)
			}
			return true
		})

		// 8. The new texts are in every language.
		outputKeys := []string{"panel.background.title", "panel.export.title", "bg.doc.title", "bg.doc.keep", "bg.doc.whiten",
			"bg.doc.remove", "bg.page.title", "bg.page.white", "bg.page.transparent", "bg.page.hint", "bg.workspace.hint",
			"export.preview", "export.notes", "action.export", "view.document", "view.sheets"}
		for _, lang := range i18n.Languages {
			add(func() bool {
				w.setLanguageSetting(string(lang))
				// The texts of a section that is folded are not shown.
				expandAll(w.win)
				checkTexts("output panels "+string(lang), lang, outputKeys...)
				return true
			})
		}

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

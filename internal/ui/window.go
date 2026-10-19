// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/svg-print-layout/internal/app"
	"github.com/gillesmagneville/svg-print-layout/internal/engine"
	"github.com/gillesmagneville/svg-print-layout/internal/i18n"
	"github.com/gillesmagneville/svg-print-layout/internal/units"
)

// Available tells whether this build has a graphical interface.
const Available = true

// Run starts the application and returns its exit code. It returns when the
// last window is closed.
func Run(opts Options) int {
	settingsPath, _ := app.SettingsPath()
	settings := app.DefaultSettings()
	if settingsPath != "" {
		// A damaged file gives the defaults: the program starts either way.
		settings, _ = app.LoadSettings(settingsPath)
	}
	setting := opts.Language
	if setting == "" {
		setting = settings.Language
	}
	lang := i18n.Resolve(setting, os.Getenv)
	if setting != i18n.Auto {
		// GTK translates its own texts (the buttons of the file chooser...)
		// from this variable, read before the toolkit starts.
		os.Setenv("LANGUAGE", string(lang))
	}

	application := gtk.NewApplication(AppID, gio.ApplicationFlagsNone)
	var win *window
	application.ConnectActivate(func() {
		win = newWindow(application, opts, settings, settingsPath, lang)
		win.show()
	})
	// The arguments of the command line were read by the program; GTK is given
	// only its name.
	code := application.Run([]string{os.Args[0]})
	if code == 0 && win != nil {
		code = win.exitCode
	}
	return code
}

// window is the main window and what it shows.
type window struct {
	app          *gtk.Application
	win          *gtk.ApplicationWindow
	opts         Options
	settings     app.Settings
	settingsPath string
	texts        translations

	engine *worker

	// What the document is. These fields are written on the thread of the
	// window only; the loaded document itself belongs to the worker.
	path       string
	docW, docH float64
	hasDoc     bool

	// A drawing is asked for after every change of size; an older answer is
	// dropped (renderGen), and the program does not quit for a screenshot
	// while one is on its way (busy).
	renderGen   int
	busy        int
	resizeTimer glib.SourceHandle

	// Widgets.
	pv         *preview
	stack      *gtk.Stack
	status     *gtk.Label
	menuButton *gtk.MenuButton
	langAction *gio.SimpleAction
	statusKey  string
	statusArgs []any
	exitCode   int
	sourceNone *gtk.Label
	sourceBox  *gtk.Box
	sourceFile *gtk.Label
	sourceSize *gtk.Label
	sourceMM   *gtk.Label
	pagesNone  *gtk.Label

	// The loaded engine document: only the worker reads or writes it.
	engineDoc *engine.Doc
}

func newWindow(application *gtk.Application, opts Options, settings app.Settings, settingsPath string, lang i18n.Lang) *window {
	w := &window{
		app:          application,
		opts:         opts,
		settings:     settings,
		settingsPath: settingsPath,
		texts:        translations{tr: i18n.New(lang)},
		engine:       newWorker(),
	}
	w.win = gtk.NewApplicationWindow(application)
	w.win.SetDefaultSize(1280, 800)
	gtk.WindowSetDefaultIconName(AppID)

	w.buildActions()
	w.buildHeaderBar()

	body := gtk.NewBox(gtk.OrientationVertical, 0)
	right := gtk.NewPaned(gtk.OrientationHorizontal)
	right.SetStartChild(w.buildCentre())
	right.SetEndChild(w.buildPagesPanel())
	right.SetResizeStartChild(true)
	right.SetResizeEndChild(false)
	right.SetShrinkStartChild(false)
	right.SetShrinkEndChild(false)

	left := gtk.NewPaned(gtk.OrientationHorizontal)
	left.SetStartChild(w.buildSettingsPanel())
	left.SetEndChild(right)
	left.SetResizeStartChild(false)
	left.SetResizeEndChild(true)
	left.SetShrinkStartChild(false)
	left.SetShrinkEndChild(false)
	left.SetPosition(300)
	// The right panel is 320 pixels wide: the paned divides what is left.
	right.SetPosition(1280 - 300 - 320)
	left.SetVExpand(true)

	body.Append(left)
	body.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
	body.Append(w.buildStatusBar())
	w.win.SetChild(body)

	w.texts.bind(func(tr *i18n.Translator) { w.updateTitle() })
	return w
}

func (w *window) show() {
	w.win.Present()
	if w.opts.Open != "" {
		w.openPath(w.opts.Open)
	}
	if w.opts.ReadyFile != "" {
		w.announceReady()
	}
}

// buildActions creates the actions of the menu and the keyboard shortcuts.
func (w *window) buildActions() {
	add := func(name string, run func()) {
		a := gio.NewSimpleAction(name, nil)
		a.ConnectActivate(func(*glib.Variant) { run() })
		w.win.AddAction(a)
	}
	add("open", w.openDialog)
	add("about", w.showAbout)
	add("quit", w.app.Quit)

	w.langAction = gio.NewSimpleActionStateful("language", glib.NewVariantType("s"), glib.NewVariantString(w.settings.Language))
	w.langAction.ConnectActivate(func(param *glib.Variant) {
		if param != nil {
			w.setLanguageSetting(param.String())
		}
	})
	w.win.AddAction(w.langAction)

	w.app.SetAccelsForAction("win.open", []string{"<Control>o"})
	w.app.SetAccelsForAction("win.quit", []string{"<Control>q"})
}

// buildHeaderBar fills the title bar: open on the left, the menu on the right.
func (w *window) buildHeaderBar() {
	bar := gtk.NewHeaderBar()

	open := gtk.NewButton()
	open.SetActionName("win.open")
	w.texts.bind(func(tr *i18n.Translator) {
		open.SetLabel(tr.T("action.open_svg"))
		open.SetTooltipText(tr.T("action.open_svg.tooltip"))
	})
	bar.PackStart(open)

	w.menuButton = gtk.NewMenuButton()
	w.menuButton.SetIconName("open-menu-symbolic")
	w.texts.bind(func(tr *i18n.Translator) {
		w.menuButton.SetTooltipText(tr.T("menu.title"))
		w.menuButton.SetMenuModel(w.buildMenu(tr))
	})
	bar.PackEnd(w.menuButton)

	w.win.SetTitlebar(bar)
}

// buildMenu is the main menu in the language of the translator.
func (w *window) buildMenu(tr *i18n.Translator) *gio.Menu {
	languages := gio.NewMenu()
	languages.Append(tr.T("menu.language.auto"), "win.language::"+i18n.Auto)
	for _, l := range i18n.Languages {
		// A language is named in itself, whatever the language of the window.
		languages.Append(l.Name(), "win.language::"+string(l))
	}
	other := gio.NewMenu()
	other.Append(tr.T("menu.about"), "win.about")
	other.Append(tr.T("menu.quit"), "win.quit")

	menu := gio.NewMenu()
	menu.AppendSubmenu(tr.T("menu.language"), languages)
	menu.AppendSection("", other)
	return menu
}

// buildSettingsPanel is the left panel: the settings, one folding section
// after another (ARCHITECTURE §9). Only the source file is filled for now.
func (w *window) buildSettingsPanel() gtk.Widgetter {
	column := gtk.NewBox(gtk.OrientationVertical, 6)
	column.SetMarginTop(12)
	column.SetMarginBottom(12)
	column.SetMarginStart(12)
	column.SetMarginEnd(12)

	// Source file.
	w.sourceBox = gtk.NewBox(gtk.OrientationVertical, 4)
	w.sourceNone = newLabel()
	w.sourceFile = newLabel()
	w.sourceSize = newLabel()
	w.sourceMM = newLabel()
	for _, l := range []*gtk.Label{w.sourceFile, w.sourceSize, w.sourceMM} {
		w.sourceBox.Append(l)
	}
	w.sourceBox.SetVisible(false)
	source := gtk.NewBox(gtk.OrientationVertical, 4)
	source.Append(w.sourceNone)
	source.Append(w.sourceBox)
	w.texts.bind(func(tr *i18n.Translator) {
		w.sourceNone.SetLabel(tr.T("source.none"))
		w.refreshSource()
	})
	column.Append(w.section("panel.source.title", source, true))

	// The other sections wait for the commands that drive them.
	for _, key := range []string{"panel.paper.title", "panel.margins.title", "panel.scale.title", "panel.placement.title", "panel.background.title", "panel.export.title"} {
		soon := newLabel()
		soon.AddCSSClass("dim-label")
		w.texts.bind(func(tr *i18n.Translator) { soon.SetLabel(tr.T("panel.soon")) })
		column.Append(w.section(key, soon, false))
	}

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(column)
	scroll.SetSizeRequest(240, -1)
	return scroll
}

// section is a folding block of the settings panel with a translated title.
func (w *window) section(titleKey string, content gtk.Widgetter, expanded bool) *gtk.Expander {
	e := gtk.NewExpander("")
	e.SetExpanded(expanded)
	e.SetChild(content)
	w.texts.bind(func(tr *i18n.Translator) { e.SetLabel(tr.T(titleKey)) })
	return e
}

// buildCentre is the preview, or an invitation to open a document.
func (w *window) buildCentre() gtk.Widgetter {
	w.pv = newPreview()
	w.pv.area.ConnectResize(func(int, int) { w.scheduleRender() })

	empty := gtk.NewBox(gtk.OrientationVertical, 12)
	empty.SetVAlign(gtk.AlignCenter)
	empty.SetHAlign(gtk.AlignCenter)
	title := gtk.NewLabel("")
	title.AddCSSClass("title-2")
	hint := gtk.NewLabel("")
	hint.AddCSSClass("dim-label")
	open := gtk.NewButton()
	open.SetActionName("win.open")
	open.SetHAlign(gtk.AlignCenter)
	open.AddCSSClass("suggested-action")
	empty.Append(title)
	empty.Append(hint)
	empty.Append(open)
	w.texts.bind(func(tr *i18n.Translator) {
		title.SetLabel(tr.T("preview.empty.title"))
		hint.SetLabel(tr.T("preview.empty.hint"))
		open.SetLabel(tr.T("action.open_svg"))
	})

	w.stack = gtk.NewStack()
	w.stack.AddNamed(empty, "empty")
	w.stack.AddNamed(w.pv.area, "preview")
	w.stack.SetVisibleChildName("empty")
	w.stack.SetHExpand(true)
	w.stack.SetVExpand(true)
	return w.stack
}

// buildPagesPanel is the right panel: the list of pages (empty until there is
// a document to cut).
func (w *window) buildPagesPanel() gtk.Widgetter {
	column := gtk.NewBox(gtk.OrientationVertical, 6)
	column.SetMarginTop(12)
	column.SetMarginBottom(12)
	column.SetMarginStart(12)
	column.SetMarginEnd(12)
	title := gtk.NewLabel("")
	title.AddCSSClass("heading")
	title.SetXAlign(0)
	w.pagesNone = newLabel()
	w.pagesNone.AddCSSClass("dim-label")
	column.Append(title)
	column.Append(w.pagesNone)
	w.texts.bind(func(tr *i18n.Translator) {
		title.SetLabel(tr.T("pages.title"))
		w.pagesNone.SetLabel(tr.T("pages.none"))
	})
	column.SetSizeRequest(240, -1)
	return column
}

func (w *window) buildStatusBar() gtk.Widgetter {
	w.status = newLabel()
	w.status.SetMarginTop(4)
	w.status.SetMarginBottom(4)
	w.status.SetMarginStart(12)
	w.status.SetMarginEnd(12)
	w.setStatus("status.ready")
	return w.status
}

// shown writes a length the way the interface shows it: rounded to a tenth,
// and without decimals when it is whole (1280, not 1280,0), in the decimal
// notation of the language.
func shown(tr *i18n.Translator, v float64) string {
	return tr.Number(math.Round(v*10)/10, -1)
}

// newLabel is a label that wraps and is aligned to the start.
func newLabel() *gtk.Label {
	l := gtk.NewLabel("")
	l.SetXAlign(0)
	l.SetWrap(true)
	return l
}

// setStatus says something in the status bar, and remembers it so that it can
// be said again in another language.
func (w *window) setStatus(key string, args ...any) {
	w.statusKey, w.statusArgs = key, args
	w.status.SetLabel(w.texts.tr.T(key, args...))
}

func (w *window) updateTitle() {
	title := "SVG Print Layout"
	if w.path != "" {
		title = filepath.Base(w.path) + " — " + title
	}
	w.win.SetTitle(title)
	if w.status != nil && w.statusKey != "" {
		w.status.SetLabel(w.texts.tr.T(w.statusKey, w.statusArgs...))
	}
}

// refreshSource shows what is known of the document in the source section.
func (w *window) refreshSource() {
	tr := w.texts.tr
	w.sourceNone.SetVisible(!w.hasDoc)
	w.sourceBox.SetVisible(w.hasDoc)
	if !w.hasDoc {
		return
	}
	w.sourceFile.SetLabel(tr.T("source.file", "name", filepath.Base(w.path)))
	w.sourceSize.SetLabel(tr.T("source.size", "w", shown(tr, w.docW), "h", shown(tr, w.docH)))
	w.sourceMM.SetLabel(tr.T("source.size.mm", "w", shown(tr, units.PxToMM(w.docW)), "h", shown(tr, units.PxToMM(w.docH))))
}

// setLanguageSetting applies and saves the language chosen in the menu.
func (w *window) setLanguageSetting(setting string) {
	lang := i18n.Resolve(setting, os.Getenv)
	w.settings.Language = setting
	w.langAction.SetState(glib.NewVariantString(setting))
	w.texts.switchTo(lang)
	if w.settingsPath != "" {
		// A preference that cannot be saved is not worth interrupting the
		// user: the language still changes for this session.
		_ = w.settings.Save(w.settingsPath)
	}
}

// showAbout opens the About box.
func (w *window) showAbout() {
	tr := w.texts.tr
	about := gtk.NewAboutDialog()
	about.SetProgramName("SVG Print Layout")
	about.SetVersion(w.opts.Version)
	about.SetComments(tr.T("app.tagline"))
	about.SetWebsite(Website)
	about.SetLicenseType(gtk.LicenseGPL30_Only)
	about.SetLogoIconName(AppID)
	about.SetTransientFor(&w.win.Window)
	about.SetModal(true)
	about.Present()
}

// openDialog asks for an SVG document.
func (w *window) openDialog() {
	tr := w.texts.tr
	filter := gtk.NewFileFilter()
	filter.SetName(tr.T("dialog.filter.svg"))
	filter.AddMIMEType("image/svg+xml")
	filter.AddPattern("*.svg")
	filter.AddPattern("*.SVG")
	filters := gio.NewListStore(gtk.GTypeFileFilter)
	filters.Append(filter.Object)

	dialog := gtk.NewFileDialog()
	dialog.SetTitle(tr.T("dialog.open_svg.title"))
	dialog.SetFilters(filters)
	dialog.SetDefaultFilter(filter)
	dialog.Open(context.Background(), &w.win.Window, func(res gio.AsyncResulter) {
		file, err := dialog.OpenFinish(res)
		if err != nil || file == nil {
			return // cancelled
		}
		w.openPath(file.Path())
	})
}

// openPath loads a document off the thread of the window and shows it.
func (w *window) openPath(path string) {
	w.setStatus("status.loading", "name", filepath.Base(path))
	w.busy++
	w.engine.do(func() {
		doc, err := loadDocument(path)
		glib.IdleAdd(func() {
			w.busy--
			if err != nil {
				w.setStatus("status.ready")
				w.showOpenError(path, err)
				return
			}
			w.adopt(path, doc)
		})
	})
}

// loadDocument reads and parses a file. It runs on the worker.
func loadDocument(path string) (*engine.Doc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return engine.Load(data)
}

// adopt makes a loaded document the current one. It runs on the thread of the
// window; the previous document is closed by the worker, which owns it.
func (w *window) adopt(path string, doc *engine.Doc) {
	previous := w.engineDoc
	w.engineDoc = doc
	if previous != nil {
		w.engine.do(previous.Close)
	}
	w.path = path
	w.docW, w.docH = doc.Size()
	w.hasDoc = true
	w.updateTitle()
	w.refreshSource()
	w.stack.SetVisibleChildName("preview")
	w.setStatus("status.loaded", "name", filepath.Base(path), "w", shown(w.texts.tr, w.docW), "h", shown(w.texts.tr, w.docH))
	w.renderNow()
}

// showOpenError tells the user that a document could not be opened.
func (w *window) showOpenError(path string, err error) {
	tr := w.texts.tr
	w.showMessage(tr.T("dialog.error.open"), tr.T("dialog.error.open.detail", "path", path, "reason", err.Error()))
}

// showMessage opens a small modal window with a heading, an explanation and a
// button to close it. It is made here rather than with GtkAlertDialog or
// GtkMessageDialog so that its button is in the language of the interface:
// the stock buttons are translated by GTK, from catalogs that may not be
// installed.
func (w *window) showMessage(heading, detail string) {
	dialog := gtk.NewWindow()
	dialog.SetApplication(w.app)
	dialog.SetTransientFor(&w.win.Window)
	dialog.SetModal(true)
	dialog.SetResizable(false)
	dialog.SetTitle("SVG Print Layout")

	column := gtk.NewBox(gtk.OrientationVertical, 12)
	column.SetMarginTop(18)
	column.SetMarginBottom(18)
	column.SetMarginStart(18)
	column.SetMarginEnd(18)

	title := gtk.NewLabel(heading)
	title.AddCSSClass("title-3")
	title.SetXAlign(0)
	title.SetWrap(true)
	body := gtk.NewLabel(detail)
	body.SetXAlign(0)
	body.SetWrap(true)
	body.SetSelectable(true)
	body.SetMaxWidthChars(60)
	closeButton := gtk.NewButtonWithLabel(w.texts.tr.T("dialog.close"))
	closeButton.SetHAlign(gtk.AlignEnd)
	closeButton.ConnectClicked(dialog.Close)

	column.Append(title)
	column.Append(body)
	column.Append(closeButton)
	dialog.SetChild(column)
	dialog.SetDefaultWidget(closeButton)
	dialog.Present()
}

// scheduleRender draws the document again once the window has stopped
// changing size: a drag of the window edge makes dozens of resizes.
func (w *window) scheduleRender() {
	if !w.hasDoc {
		return
	}
	if w.resizeTimer != 0 {
		glib.SourceRemove(w.resizeTimer)
	}
	w.resizeTimer = glib.TimeoutAdd(120, func() bool {
		w.resizeTimer = 0
		w.renderNow()
		return false
	})
}

// renderNow draws the whole document so that it fits the preview.
func (w *window) renderNow() {
	if !w.hasDoc {
		return
	}
	width, height := w.pv.area.Width(), w.pv.area.Height()
	const margin = 24.0
	scale := math.Min((float64(width)-2*margin)/w.docW, (float64(height)-2*margin)/w.docH)
	if width <= 0 || height <= 0 || scale <= 0 {
		return
	}
	w.renderGen++
	gen := w.renderGen
	docW, docH := w.docW, w.docH
	w.busy++
	w.engine.do(func() {
		img, err := w.engineDoc.Render(engine.Box{W: docW, H: docH}, scale, nil)
		glib.IdleAdd(func() {
			w.busy--
			if gen != w.renderGen {
				return // a newer drawing is on its way
			}
			if err != nil {
				w.showOpenError(w.path, err)
				return
			}
			w.pv.setImage(img)
		})
	})
}

// String describes the window for a test failure.
func (w *window) String() string {
	return fmt.Sprintf("window{path=%q language=%s}", w.path, w.texts.tr.Lang())
}

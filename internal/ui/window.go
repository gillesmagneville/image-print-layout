// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/pageslice/internal/app"
	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/engine"
	"github.com/gillesmagneville/pageslice/internal/i18n"
	"github.com/gillesmagneville/pageslice/internal/layout"
	"github.com/gillesmagneville/pageslice/internal/model"
)

// Available tells whether this build has a graphical interface.
const Available = true

// Run starts the application and returns its exit code. It returns when the
// last window is closed.
func Run(opts Options) int {
	settingsPath, _ := app.SettingsPath()
	settings := app.DefaultSettings()
	if settingsPath != "" {
		app.ImportLegacySettings(settingsPath)
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
		setenv("LANGUAGE", string(lang))
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

	// engine runs the jobs that touch the rendering engine, one at a time.
	engine   *worker
	services app.Services
	// session is the work in progress: the document, the project, the plan and
	// the history. It is used on the thread of the window only.
	session *app.Session
	// engineDoc is the loaded document; only the worker uses it. It holds the
	// working copy with the background of the document that loadedBG names:
	// when the project asks for another one, it is loaded again (background.go).
	engineDoc  engine.Document
	loadedBG   model.DocBackground
	rebuilding bool
	// afterRebuild is what waits for the working copy to be ready.
	afterRebuild []func()
	// docRev changes with the engine document, so that the drawings of the
	// sheets made from the old one are drawn again.
	docRev int

	// The export in progress, and where the last one went.
	exporting     bool
	cancelExport  atomic.Bool
	lastExportDir string
	lastReport    *gtk.Window

	// A drawing is asked for after every movement of the view; an older answer
	// is dropped (renderGen) when a newer one is on its way, and the program
	// does not quit for a screenshot while one is (busy).
	renderGen int
	busy      int

	// The cuts as the lists and the preview show them, and what is selected: a
	// cut or a page (0 for none).
	cuts    []app.CutView
	selCut  int
	selPage int

	// Widgets.
	pv         *preview
	sheets     *sheetsView
	panel      *settingsPanel
	pages      *pagesPanel
	viewBar    *gtk.Box
	viewDoc    *gtk.ToggleButton
	viewSheets *gtk.ToggleButton
	stack      *gtk.Stack
	status     *gtk.Label
	menuButton *gtk.MenuButton
	langAction *gio.SimpleAction
	undoAction *gio.SimpleAction
	redoAction *gio.SimpleAction
	exportAct  *gio.SimpleAction
	saveAct    *gio.SimpleAction
	saveAsAct  *gio.SimpleAction
	projectBtn *gtk.MenuButton
	// forceClose lets the window close without asking about unsaved changes:
	// the user has answered.
	forceClose bool
	undoButton *gtk.Button
	redoButton *gtk.Button
	statusKey  string
	statusArgs []any
	// What the status bar said before a refusal, to say again once it is over.
	beforeRefusal     string
	beforeRefusalArgs []any
	exitCode          int
}

func newWindow(application *gtk.Application, opts Options, settings app.Settings, settingsPath string, lang i18n.Lang) *window {
	w := &window{
		app:          application,
		opts:         opts,
		settings:     settings,
		settingsPath: settingsPath,
		texts:        translations{tr: i18n.New(lang)},
		engine:       newWorker(),
		services:     defaultServices(),
		session:      app.NewSession(),
	}
	w.win = gtk.NewApplicationWindow(application)
	w.win.SetDefaultSize(1280, 800)
	gtk.WindowSetDefaultIconName(IconName)

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
	w.win.ConnectCloseRequest(w.closeRequested)
	w.refreshAll()
	return w
}

func (w *window) show() {
	w.win.Present()
	if w.opts.Open != "" {
		if app.IsProjectPath(w.opts.Open) {
			w.openProject(w.opts.Open)
		} else {
			w.openPath(w.opts.Open)
		}
	}
	if w.opts.ReadyFile != "" {
		w.announceReady()
	}
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

// section is a folding block of the settings panel with a translated title.
func (w *window) section(titleKey string, content gtk.Widgetter, expanded bool) *gtk.Expander {
	e := gtk.NewExpander("")
	e.SetExpanded(expanded)
	e.SetChild(content)
	w.texts.bind(func(tr *i18n.Translator) { e.SetLabel(tr.T(titleKey)) })
	return e
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
	if w.statusKey != "status.refused" {
		w.beforeRefusal, w.beforeRefusalArgs = w.statusKey, w.statusArgs
	}
	w.statusKey, w.statusArgs = key, args
	w.status.SetLabel(w.statusText())
}

// lazyText is a value of a status that is written when it is shown, in the
// language of the moment: the reason of a refusal follows a change of language.
type lazyText func(tr *i18n.Translator) string

// plural is the first value of a status text that has a singular and a plural
// form (key.one, key.other): the count.
type plural struct{ n int }

// statusText is the status in the current language.
func (w *window) statusText() string {
	if len(w.statusArgs) > 0 {
		if pl, ok := w.statusArgs[0].(plural); ok {
			return w.texts.tr.N(w.statusKey, pl.n, w.statusArgs[1:]...)
		}
	}
	args := make([]any, len(w.statusArgs))
	for i, a := range w.statusArgs {
		if f, ok := a.(lazyText); ok {
			a = f(w.texts.tr)
		}
		args[i] = a
	}
	return w.texts.tr.T(w.statusKey, args...)
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
	dialog.SetTitle("Pageslice")

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
	// The first widget that can take the focus gets it when the window opens, and
	// a selectable label then shows its whole text selected.
	dialog.SetFocus(closeButton)
	dialog.Present()
}

// String describes the window for a test failure.
func (w *window) String() string {
	path := ""
	if doc := w.session.Document(); doc != nil {
		path = doc.Path
	}
	return fmt.Sprintf("window{path=%q language=%s}", path, w.texts.tr.Lang())
}

// ---- construction ------------------------------------------------------------

// buildActions creates the actions of the menu and the keyboard shortcuts.
func (w *window) buildActions() {
	add := func(name string, run func()) *gio.SimpleAction {
		a := gio.NewSimpleAction(name, nil)
		a.ConnectActivate(func(*glib.Variant) { run() })
		w.win.AddAction(a)
		return a
	}
	add("open", func() { w.guardUnsaved(w.openDialog) })
	add("open-project", func() { w.guardUnsaved(w.openProjectDialog) })
	w.saveAct = add("save-project", w.saveProject)
	w.saveAct.SetEnabled(false)
	w.saveAsAct = add("save-project-as", func() { w.saveProjectAs(nil) })
	w.saveAsAct.SetEnabled(false)
	w.exportAct = add("export", w.exportPDF)
	w.exportAct.SetEnabled(false)
	add("about", w.showAbout)
	add("quit", w.win.Close)
	w.undoAction = add("undo", w.undo)
	w.redoAction = add("redo", w.redo)
	add("zoom-in", func() { w.pv.zoomIn() })
	add("zoom-out", func() { w.pv.zoomOut() })
	add("fit-all", func() { w.pv.showAll() })
	add("fit-width", func() { w.pv.showWidth() })

	w.langAction = gio.NewSimpleActionStateful("language", glib.NewVariantType("s"), glib.NewVariantString(w.settings.Language))
	w.langAction.ConnectActivate(func(param *glib.Variant) {
		if param != nil {
			w.setLanguageSetting(param.String())
		}
	})
	w.win.AddAction(w.langAction)

	w.app.SetAccelsForAction("win.open", []string{"<Control>o"})
	w.app.SetAccelsForAction("win.open-project", []string{"<Control><Shift>o"})
	w.app.SetAccelsForAction("win.save-project", []string{"<Control>s"})
	w.app.SetAccelsForAction("win.save-project-as", []string{"<Control><Shift>s"})
	w.app.SetAccelsForAction("win.export", []string{"<Control>e"})
	w.app.SetAccelsForAction("win.quit", []string{"<Control>q"})
	w.app.SetAccelsForAction("win.undo", []string{"<Control>z"})
	w.app.SetAccelsForAction("win.redo", []string{"<Control><Shift>z", "<Control>y"})
	w.app.SetAccelsForAction("win.zoom-in", []string{"<Control>plus", "<Control>equal", "<Control>KP_Add"})
	w.app.SetAccelsForAction("win.zoom-out", []string{"<Control>minus", "<Control>KP_Subtract"})
	w.app.SetAccelsForAction("win.fit-all", []string{"<Control>0", "<Control>KP_0"})
	w.app.SetAccelsForAction("win.fit-width", []string{"<Control>1", "<Control>KP_1"})
}

// buildHeaderBar fills the title bar: open, project and history on the left,
// the menu on the right. The export is in the left panel (and Ctrl+E): a second
// button here would only say the same thing twice.
func (w *window) buildHeaderBar() {
	bar := gtk.NewHeaderBar()

	open := gtk.NewButton()
	open.SetActionName("win.open")
	w.texts.bind(func(tr *i18n.Translator) {
		open.SetLabel(tr.T("action.open_svg"))
		open.SetTooltipText(tr.T("action.open_svg.tooltip"))
	})
	bar.PackStart(open)

	w.projectBtn = gtk.NewMenuButton()
	w.texts.bind(func(tr *i18n.Translator) {
		w.projectBtn.SetLabel(tr.T("action.project"))
		w.projectBtn.SetTooltipText(tr.T("action.project.tooltip"))
		w.projectBtn.SetMenuModel(w.buildProjectMenu(tr))
	})
	bar.PackStart(w.projectBtn)

	history := gtk.NewBox(gtk.OrientationHorizontal, 0)
	history.AddCSSClass("linked")
	w.undoButton = gtk.NewButtonFromIconName("edit-undo-symbolic")
	w.undoButton.SetActionName("win.undo")
	w.redoButton = gtk.NewButtonFromIconName("edit-redo-symbolic")
	w.redoButton.SetActionName("win.redo")
	history.Append(w.undoButton)
	history.Append(w.redoButton)
	bar.PackStart(history)

	w.menuButton = gtk.NewMenuButton()
	w.menuButton.SetIconName("open-menu-symbolic")
	w.texts.bind(func(tr *i18n.Translator) {
		w.menuButton.SetTooltipText(tr.T("menu.title"))
		w.menuButton.SetMenuModel(w.buildMenu(tr))
	})
	bar.PackEnd(w.menuButton)

	w.win.SetTitlebar(bar)
}

// buildCentre is the preview and the sheets, with the switch between them, or
// an invitation to open a document.
func (w *window) buildCentre() gtk.Widgetter {
	w.pv = newPreview(&w.texts)
	w.pv.pageLabel = func(n int) string { return w.texts.tr.T("pages.row.title", "n", n) }
	w.pv.onNeedRender = func() { w.renderView(false) }
	w.wireCuts()
	w.sheets = newSheetsView(w)

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
	w.stack.AddNamed(w.pv.widget(), "preview")
	w.stack.AddNamed(w.sheets.widget(), "sheets")
	w.stack.SetVisibleChildName("empty")
	w.stack.SetHExpand(true)
	w.stack.SetVExpand(true)

	// The switch between the drawing and the sheets.
	w.viewDoc = gtk.NewToggleButton()
	w.viewSheets = gtk.NewToggleButton()
	w.viewSheets.SetGroup(w.viewDoc)
	w.viewDoc.SetActive(true)
	w.viewDoc.ConnectToggled(func() {
		if w.viewDoc.Active() {
			w.showView("preview")
		}
	})
	w.viewSheets.ConnectToggled(func() {
		if w.viewSheets.Active() {
			w.showView("sheets")
		}
	})
	w.texts.bind(func(tr *i18n.Translator) {
		w.viewDoc.SetLabel(tr.T("view.document"))
		w.viewDoc.SetTooltipText(tr.T("view.document.tooltip"))
		w.viewSheets.SetLabel(tr.T("view.sheets"))
		w.viewSheets.SetTooltipText(tr.T("view.sheets.tooltip"))
	})
	w.viewBar = gtk.NewBox(gtk.OrientationHorizontal, 0)
	w.viewBar.AddCSSClass("linked")
	w.viewBar.SetHAlign(gtk.AlignCenter)
	w.viewBar.SetMarginTop(6)
	w.viewBar.SetMarginBottom(6)
	w.viewBar.Append(w.viewDoc)
	w.viewBar.Append(w.viewSheets)
	w.viewBar.SetVisible(false)

	centre := gtk.NewBox(gtk.OrientationVertical, 0)
	centre.Append(w.viewBar)
	centre.Append(w.stack)
	return centre
}

// showView shows the drawing ("preview") or the sheets ("sheets"). It keeps the
// switch in step, whoever asked.
func (w *window) showView(name string) {
	if w.session.Document() == nil {
		return
	}
	w.stack.SetVisibleChildName(name)
	w.sheets.setShown(name == "sheets")
	if name == "sheets" {
		w.viewSheets.SetActive(true)
	} else {
		w.viewDoc.SetActive(true)
	}
}

// showSheets shows the sheets, with page n selected (0 for none).
func (w *window) showSheets(n int) {
	if w.session.Document() == nil {
		return
	}
	if n != 0 {
		w.selPage, w.selCut = n, 0
	}
	w.showView("sheets")
	w.showSelection()
	w.sheets.scrollTo(n)
}

// ---- state -------------------------------------------------------------------------

func (w *window) updateTitle() {
	title := "Pageslice"
	if doc := w.session.Document(); doc != nil {
		name := baseName(doc.Path)
		if path := w.session.ProjectPath(); path != "" {
			name = baseName(path)
		}
		if w.session.Dirty() {
			name = "*" + name
		}
		title = name + " — " + title
	}
	w.win.SetTitle(title)
	if w.status != nil && w.statusKey != "" {
		w.status.SetLabel(w.statusText())
	}
}

// refreshAll shows the session everywhere: the panels, the list of pages, the
// marks on the preview, the buttons of the history.
func (w *window) refreshAll() {
	if w.panel == nil || w.pages == nil || w.pv == nil {
		return
	}
	w.cuts = w.session.Cuts()
	w.pruneSelection()
	w.panel.sync()
	w.pages.update()
	w.pv.setPlan(w.session.Plan())
	w.pv.setCuts(w.cuts)
	w.pv.setSelection(w.selCut, w.selPage)
	w.pv.setTransparentPage(w.session.Project().Background.Page == model.TransparentPage)
	w.updateHistory()
	w.syncBackground()
	w.sheets.update()
	w.refreshExportState()
	w.refreshProjectState()
	w.updateTitle()
}

// refreshExportState enables the export and says in the export section what
// the checks make of the plan.
func (w *window) refreshExportState() {
	if w.exportAct == nil {
		return
	}
	w.exportAct.SetEnabled(w.session.Document() != nil && !w.exporting)
	w.panel.syncExport()
}

// updateHistory enables the undo and redo buttons and says what they do.
func (w *window) updateHistory() {
	tr := w.texts.tr
	if w.undoAction == nil {
		return
	}
	w.undoAction.SetEnabled(w.session.CanUndo())
	w.redoAction.SetEnabled(w.session.CanRedo())
	tip := func(action, what string) string {
		if what == "" {
			return tr.T(action)
		}
		return tr.T(action+".tooltip", "what", tr.T(what))
	}
	w.undoButton.SetTooltipText(tip("action.undo", w.session.UndoLabel()))
	w.redoButton.SetTooltipText(tip("action.redo", w.session.RedoLabel()))
}

// apply carries out a command of the user. A command the session refuses leaves
// everything as it was; the widgets are set again from the session, which puts
// back the value they had, and the status bar says why.
func (w *window) apply(label, key string, c app.Command) {
	if err := w.session.Apply(label, key, c); err != nil {
		w.refuse(err)
	} else {
		w.clearRefusal()
	}
	w.refreshAll()
}

// clearRefusal removes the message of a refused setting once something else
// has been done.
func (w *window) clearRefusal() {
	if w.statusKey != "status.refused" {
		return
	}
	key, args := w.beforeRefusal, w.beforeRefusalArgs
	if key == "" {
		key = "status.ready"
	}
	w.statusKey, w.statusArgs = "", nil
	w.setStatus(key, args...)
}

// applyCorrection carries out a proposal of the page list.
func (w *window) applyCorrection(c layout.Correction) {
	if err := w.session.ApplyCorrection(c); err != nil {
		w.refuse(err)
	} else {
		w.clearRefusal()
	}
	w.refreshAll()
}

func (w *window) undo() {
	if w.session.Undo() {
		w.clearRefusal()
		w.refreshAll()
	}
}

func (w *window) redo() {
	if w.session.Redo() {
		w.clearRefusal()
		w.refreshAll()
	}
}

// refuse tells in the status bar that a setting was not accepted.
func (w *window) refuse(err error) {
	w.setStatus("status.refused", "reason", lazyText(func(tr *i18n.Translator) string { return errorText(tr, err) }))
}

// errorText writes an error for the user: the text of its code when it has
// one.
func errorText(tr *i18n.Translator, err error) string {
	if e, ok := diag.As(err); ok {
		return diagText(tr, e.Code, e.Args)
	}
	return err.Error()
}

// diagText is the text of a diagnostic code. Lengths are shown to a tenth: the
// layout keeps them exact, the sentence does not need six decimals.
func diagText(tr *i18n.Translator, code string, args []any) string {
	shown := make([]any, len(args))
	for i, a := range args {
		if v, ok := a.(float64); ok {
			a = round(v, 1)
		}
		shown[i] = a
	}
	return tr.T("diag."+code, shown...)
}

func baseName(path string) string { return filepath.Base(path) }

// setLanguageSetting applies and saves the language chosen in the menu.
func (w *window) setLanguageSetting(setting string) {
	lang := i18n.Resolve(setting, os.Getenv)
	w.settings.Language = setting
	w.langAction.SetState(glib.NewVariantString(setting))
	w.texts.switchTo(lang)
	w.refreshAll()
	if w.settingsPath != "" {
		// A preference that cannot be saved is not worth interrupting the
		// user: the language still changes for this session.
		_ = w.settings.Save(w.settingsPath)
	}
}

// ---- opening a document ----------------------------------------------------------------

// openPath loads a document off the thread of the window and shows it.
func (w *window) openPath(path string) {
	w.setStatus("status.loading", "name", baseName(path))
	w.busy++
	keep := model.Background{Document: model.KeepBackground, Page: model.WhitePage}
	w.engine.do(func() {
		opened, err := app.Open(w.services, path, keep)
		glib.IdleAdd(func() {
			w.busy--
			if err != nil {
				w.setStatus("status.ready")
				w.showOpenError(path, err)
				return
			}
			w.adopt(opened)
		})
	})
}

// adopt makes a loaded document the current one. It runs on the thread of the
// window; the previous engine document is closed by the worker, which owns it.
func (w *window) adopt(o *app.Opened) {
	if err := w.session.Open(o.Document); err != nil {
		w.engine.do(o.Engine.Close)
		w.setStatus("status.ready")
		w.showOpenError(o.Document.Path, err)
		return
	}
	w.install(o)
	d := o.Document
	w.setStatus("status.loaded", "name", baseName(d.Path), "w", shown(w.texts.tr, d.Size.W), "h", shown(w.texts.tr, d.Size.H))
}

// install shows the document the session has just accepted: the engine copy
// replaces the previous one, and the views start from the whole drawing.
func (w *window) install(o *app.Opened) {
	previous := w.engineDoc
	w.engineDoc = o.Engine
	if previous != nil {
		w.engine.do(previous.Close)
	}
	d := o.Document
	// The working copy was loaded with the background the project asks for.
	w.loadedBG = w.session.Project().Background.Document
	w.rebuilding = false
	w.docRev++
	w.selCut, w.selPage = 0, 0
	w.viewBar.SetVisible(true)
	w.showView("preview")
	w.pv.setDocument(d.Size.W, d.Size.H)
	w.updateTitle()
	w.refreshAll()
}

// ---- drawing ----------------------------------------------------------------------------------

// renderView asks the worker to draw the part of the document in view, when the
// drawing on show no longer serves. force draws it in any case.
func (w *window) renderView(force bool) {
	doc := w.engineDoc
	if doc == nil {
		return
	}
	win, scale, ok := w.pv.wantedWindow(force)
	if !ok {
		return
	}
	w.renderGen++
	gen := w.renderGen
	w.busy++
	w.engine.do(func() {
		img, err := doc.Render(engine.Box{X: win.X, Y: win.Y, W: win.W, H: win.H}, scale, nil)
		glib.IdleAdd(func() {
			w.busy--
			if doc != w.engineDoc {
				return // another document has been opened since
			}
			// An answer to an older request is still good if it serves the
			// view as it is now: otherwise a pan that never stops would never
			// show a drawing.
			if gen != w.renderGen && w.pv.view.Needs(win, scale, w.pv.view.Zoom*w.pv.scaleFactor()) {
				return
			}
			if err != nil {
				w.setStatus("status.rendering_failed", "reason", err.Error())
				return
			}
			w.pv.setImage(img, win, scale)
		})
	})
}

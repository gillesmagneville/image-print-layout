// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/svg-print-layout/internal/app"
	"github.com/gillesmagneville/svg-print-layout/internal/i18n"
	"github.com/gillesmagneville/svg-print-layout/internal/layout"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
	"github.com/gillesmagneville/svg-print-layout/internal/units"
)

// settingsPanel is the left panel: one folding section after another, in the
// order of the specification (ARCHITECTURE §9). Every widget reads the project
// of the session and changes it through a command; a value that the session
// refuses is shown again as it was.
type settingsPanel struct {
	w        *window
	updating bool // the widgets are being set from the project: ignore their signals

	// Source file.
	sourceNone     *gtk.Label
	sourceBox      *gtk.Box
	sourceFile     *gtk.Label
	sourceSize     *gtk.Label
	sourceMM       *gtk.Label
	findingsExpand *gtk.Expander
	findingsList   *gtk.Box

	// Paper.
	formatDrop *gtk.DropDown
	formats    []string // the names of the dropdown; the last one is custom
	portrait   *gtk.ToggleButton
	landscape  *gtk.ToggleButton
	customBox  *gtk.Box
	customW    *numField
	customH    *numField
	paperInfo  *gtk.Label
	usableInfo *gtk.Label

	// Margins.
	margin map[string]*numField // top, bottom, left, right
	link   *gtk.ToggleButton

	// Scale.
	modeA, modeB, modeC *gtk.CheckButton
	portion             *numField
	portionBox          *gtk.Box
	manualBox           *gtk.Box
	manualBy            *gtk.DropDown
	manualWidth         *numField
	manualPercent       *numField
	resultWidth         *gtk.Label
	resultHeight        *gtk.Label
	resultFactor        *gtk.Label

	// Background.
	bgNone                     *gtk.Label
	bgFound                    *gtk.Label
	bgKeep, bgWhiten, bgRemove *gtk.CheckButton
	pageWhite, pageTransparent *gtk.CheckButton

	// Export.
	exportSummary  *gtk.Label
	exportState    *gtk.Label
	exportProgress *gtk.ProgressBar
	exportBar      *gtk.Box
	exportLabel    *gtk.Label
	exportButton   *gtk.Button

	// Placement.
	hDrop   *gtk.DropDown
	vDrop   *gtk.DropDown
	offsetX *numField
	offsetY *numField
	offXRow *gtk.Box
}

var horizontalChoices = []model.HAlign{model.Left, model.CenterH, model.Right, model.CustomH}
var verticalChoices = []model.VAlign{model.Top, model.CenterV}

func (w *window) buildSettingsPanel() gtk.Widgetter {
	s := &settingsPanel{w: w, margin: map[string]*numField{}}
	w.panel = s

	column := gtk.NewBox(gtk.OrientationVertical, 6)
	column.SetMarginTop(12)
	column.SetMarginBottom(12)
	column.SetMarginStart(12)
	column.SetMarginEnd(12)

	column.Append(w.section("panel.source.title", s.buildSource(), true))
	column.Append(w.section("panel.paper.title", s.buildPaper(), true))
	column.Append(w.section("panel.margins.title", s.buildMargins(), true))
	column.Append(w.section("panel.scale.title", s.buildScale(), true))
	column.Append(w.section("panel.placement.title", s.buildPlacement(), false))

	column.Append(w.section("panel.background.title", s.buildBackground(), false))
	column.Append(w.section("panel.export.title", s.buildExport(), true))

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(column)
	scroll.SetSizeRequest(260, -1)
	return scroll
}

// ---- small helpers -----------------------------------------------------------

func (s *settingsPanel) tr() *i18n.Translator { return s.w.texts.tr }

// field makes a numeric field that applies its value through apply.
func (s *settingsPanel) field(min, max, step float64, decimals int, unit string, apply func(v float64)) *numField {
	f := newNumField(s.tr, min, max, step, decimals, unit)
	f.onChange = func(v float64) {
		if !s.updating {
			apply(v)
		}
	}
	return f
}

// caption is a small label above a field, translated.
func (s *settingsPanel) caption(key string) *gtk.Label {
	l := newLabel()
	l.AddCSSClass("caption")
	l.AddCSSClass("dim-label")
	s.w.texts.bind(func(tr *i18n.Translator) { l.SetLabel(tr.T(key)) })
	return l
}

// note is a wrapped hint under a group of fields.
func (s *settingsPanel) note(key string) *gtk.Label {
	l := newLabel()
	l.AddCSSClass("caption")
	l.AddCSSClass("dim-label")
	s.w.texts.bind(func(tr *i18n.Translator) { l.SetLabel(tr.T(key)) })
	return l
}

func (s *settingsPanel) column() *gtk.Box {
	b := gtk.NewBox(gtk.OrientationVertical, 6)
	b.SetMarginTop(4)
	return b
}

// strings makes the model of a dropdown.
func stringList(items []string) *gtk.StringList { return gtk.NewStringList(items) }

// show writes a number in the notation of the language, rounded to d decimals.
func (s *settingsPanel) num(v float64, d int) string {
	return s.tr().Number(round(v, d), -1)
}

// apply carries out a command and shows the result everywhere. A refusal puts
// the widgets back as they were and says why.
func (s *settingsPanel) apply(label, key string, c app.Command) {
	s.w.apply(label, key, c)
}

// ---- source file ------------------------------------------------------------------

func (s *settingsPanel) buildSource() gtk.Widgetter {
	w := s.w
	s.sourceBox = gtk.NewBox(gtk.OrientationVertical, 4)
	s.sourceNone = newLabel()
	s.sourceFile = newLabel()
	s.sourceSize = newLabel()
	s.sourceMM = newLabel()
	s.findingsList = gtk.NewBox(gtk.OrientationVertical, 6)
	s.findingsExpand = gtk.NewExpander("")
	s.findingsExpand.SetChild(s.findingsList)
	for _, l := range []*gtk.Label{s.sourceFile, s.sourceSize, s.sourceMM} {
		s.sourceBox.Append(l)
	}
	s.sourceBox.Append(s.findingsExpand)
	s.sourceBox.SetVisible(false)

	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.Append(s.sourceNone)
	box.Append(s.sourceBox)
	w.texts.bind(func(tr *i18n.Translator) {
		s.sourceNone.SetLabel(tr.T("source.none"))
		s.refreshSource()
	})
	return box
}

// refreshSource shows what is known of the document.
func (s *settingsPanel) refreshSource() {
	tr := s.tr()
	doc := s.w.session.Document()
	s.sourceNone.SetVisible(doc == nil)
	s.sourceBox.SetVisible(doc != nil)
	if doc == nil {
		return
	}
	s.sourceFile.SetLabel(tr.T("source.file", "name", baseName(doc.Path)))
	s.sourceSize.SetLabel(tr.T("source.size", "w", shown(tr, doc.Size.W), "h", shown(tr, doc.Size.H)))
	s.sourceMM.SetLabel(tr.T("source.size.mm", "w", shown(tr, units.PxToMM(doc.Size.W)), "h", shown(tr, units.PxToMM(doc.Size.H))))

	findings := doc.Findings
	switch {
	case len(findings) == 0:
		s.findingsExpand.SetLabel(tr.T("source.findings.none"))
		s.findingsExpand.SetExpanded(false)
		s.findingsExpand.SetChild(nil)
	default:
		s.findingsExpand.SetLabel(tr.N("source.findings", len(findings)))
		s.findingsExpand.SetChild(s.findingsList)
		for c := s.findingsList.FirstChild(); c != nil; c = s.findingsList.FirstChild() {
			s.findingsList.Remove(c)
		}
		for _, it := range findings {
			text := diagText(tr, it.Code, it.Args)
			if it.Ref != "" {
				text += "\n" + tr.T("source.finding.element", "ref", it.Ref)
			}
			l := newLabel()
			l.SetLabel(text)
			l.SetSelectable(true)
			l.SetWrap(true)
			l.SetMaxWidthChars(40)
			l.AddCSSClass("caption")
			s.findingsList.Append(l)
		}
	}
}

// ---- paper ----------------------------------------------------------------------------

func (s *settingsPanel) buildPaper() gtk.Widgetter {
	w := s.w
	box := s.column()

	for _, p := range units.Catalog() {
		s.formats = append(s.formats, p.Name)
	}
	s.formats = append(s.formats, model.CustomFormat)

	s.formatDrop = gtk.NewDropDown(nil, nil)
	s.formatDrop.NotifyProperty("selected", func() {
		if s.updating {
			return
		}
		i := int(s.formatDrop.Selected())
		if i < 0 || i >= len(s.formats) {
			return
		}
		name := s.formats[i]
		s.apply("undo.paper", "paper", func(p model.Project, d layout.Doc) (model.Project, error) {
			q := p.Clone()
			q.Paper.Format = name
			if name == model.CustomFormat && !(q.Paper.CustomWidthMM > 0 && q.Paper.CustomHeightMM > 0) {
				// Start from the sheet the user had.
				if pl := s.w.session.Plan(); pl != nil {
					q.Paper.CustomWidthMM, q.Paper.CustomHeightMM = pl.Sheet.WidthMM, pl.Sheet.HeightMM
				} else if cur, ok := units.Lookup(p.Paper.Format); ok {
					q.Paper.CustomWidthMM, q.Paper.CustomHeightMM = cur.Oriented(p.Paper.Orientation == model.Landscape)
				} else {
					q.Paper.CustomWidthMM, q.Paper.CustomHeightMM = 210, 297
				}
			}
			return q, nil
		})
	})

	s.portrait = gtk.NewToggleButton()
	s.landscape = gtk.NewToggleButton()
	s.landscape.SetGroup(s.portrait)
	orient := gtk.NewBox(gtk.OrientationHorizontal, 0)
	orient.AddCSSClass("linked")
	orient.Append(s.portrait)
	orient.Append(s.landscape)
	for _, b := range []struct {
		btn *gtk.ToggleButton
		o   model.Orientation
	}{{s.portrait, model.Portrait}, {s.landscape, model.Landscape}} {
		b := b
		b.btn.SetHExpand(true)
		b.btn.ConnectToggled(func() {
			if s.updating || !b.btn.Active() {
				return
			}
			s.apply("undo.orientation", "orientation", app.Edit(func(p *model.Project) { p.Paper.Orientation = b.o }))
		})
	}

	s.customW = s.field(10, 5000, 1, 1, "mm", func(v float64) {
		s.apply("undo.paper", "paper-size", app.Edit(func(p *model.Project) { p.Paper.CustomWidthMM = v }))
	})
	s.customH = s.field(10, 5000, 1, 1, "mm", func(v float64) {
		s.apply("undo.paper", "paper-size", app.Edit(func(p *model.Project) { p.Paper.CustomHeightMM = v }))
	})
	s.customBox = gtk.NewBox(gtk.OrientationHorizontal, 6)
	cw, ch := gtk.NewBox(gtk.OrientationVertical, 2), gtk.NewBox(gtk.OrientationVertical, 2)
	cw.SetHExpand(true)
	ch.SetHExpand(true)
	cw.Append(s.caption("paper.custom.width"))
	cw.Append(s.customW.box)
	ch.Append(s.caption("paper.custom.height"))
	ch.Append(s.customH.box)
	s.customBox.Append(cw)
	s.customBox.Append(ch)

	s.paperInfo = newLabel()
	s.usableInfo = newLabel()
	s.paperInfo.AddCSSClass("caption")
	s.usableInfo.AddCSSClass("caption")

	box.Append(s.caption("paper.format"))
	box.Append(s.formatDrop)
	box.Append(s.caption("paper.orientation"))
	box.Append(orient)
	box.Append(s.customBox)
	box.Append(s.paperInfo)
	box.Append(s.usableInfo)

	w.texts.bind(func(tr *i18n.Translator) {
		s.portrait.SetLabel(tr.T("paper.portrait"))
		s.landscape.SetLabel(tr.T("paper.landscape"))
		s.fillFormats()
		s.syncPaper()
	})
	return box
}

// fillFormats writes the names of the formats in the dropdown.
func (s *settingsPanel) fillFormats() {
	tr := s.tr()
	items := make([]string, 0, len(s.formats))
	for _, name := range s.formats {
		if p, ok := units.Lookup(name); ok {
			items = append(items, fmt.Sprintf("%s — %s × %s mm", p.Name, s.num(p.WidthMM, 1), s.num(p.HeightMM, 1)))
		} else {
			items = append(items, tr.T("paper.custom"))
		}
	}
	was := s.updating
	s.updating = true
	s.formatDrop.SetModel(stringList(items))
	s.updating = was
}

func (s *settingsPanel) syncPaper() {
	p := s.w.session.Project()
	tr := s.tr()
	s.updating = true
	defer func() { s.updating = false }()

	sel := len(s.formats) - 1
	for i, name := range s.formats {
		if strings.EqualFold(name, p.Paper.Format) {
			sel = i
		}
	}
	if int(s.formatDrop.Selected()) != sel {
		s.formatDrop.SetSelected(uint(sel))
	}
	s.portrait.SetActive(p.Paper.Orientation != model.Landscape)
	s.landscape.SetActive(p.Paper.Orientation == model.Landscape)
	custom := strings.EqualFold(p.Paper.Format, model.CustomFormat)
	s.customBox.SetVisible(custom)
	s.customW.SetValue(p.Paper.CustomWidthMM)
	s.customH.SetValue(p.Paper.CustomHeightMM)

	if pl := s.w.session.Plan(); pl != nil {
		s.paperInfo.SetLabel(tr.T("paper.size", "w", s.num(pl.Sheet.WidthMM, 1), "h", s.num(pl.Sheet.HeightMM, 1)))
		s.usableInfo.SetLabel(tr.T("paper.usable", "w", s.num(pl.Sheet.UsableWidth, 1), "h", s.num(pl.Sheet.UsableHeight, 1)))
		return
	}
	s.paperInfo.SetLabel("")
	s.usableInfo.SetLabel("")
}

// ---- margins ----------------------------------------------------------------------------

func (s *settingsPanel) buildMargins() gtk.Widgetter {
	box := s.column()

	set := func(side string, p *model.Project, v float64) {
		switch side {
		case "top":
			p.Margins.Top = v
		case "bottom":
			p.Margins.Bottom = v
		case "left":
			p.Margins.Left = v
		case "right":
			p.Margins.Right = v
		}
	}
	s.link = gtk.NewToggleButton()
	s.link.SetIconName("insert-link-symbolic")
	s.link.AddCSSClass("flat")
	s.link.SetVAlign(gtk.AlignCenter)
	s.w.texts.bind(func(tr *i18n.Translator) { s.link.SetTooltipText(tr.T("margins.link")) })

	grid := gtk.NewGrid()
	grid.SetColumnSpacing(8)
	grid.SetRowSpacing(6)
	grid.SetColumnHomogeneous(false)
	cells := []struct {
		side, key string
		col, row  int
	}{{"top", "margins.top", 0, 0}, {"bottom", "margins.bottom", 1, 0}, {"left", "margins.left", 0, 1}, {"right", "margins.right", 1, 1}}
	for _, c := range cells {
		c := c
		f := s.field(0, 1000, 1, 1, "", func(v float64) {
			linked := s.link.Active()
			s.apply("undo.margins", "margins", app.Edit(func(p *model.Project) {
				if linked {
					for _, side := range []string{"top", "bottom", "left", "right"} {
						set(side, p, v)
					}
					return
				}
				set(c.side, p, v)
			}))
		})
		s.margin[c.side] = f
		cell := gtk.NewBox(gtk.OrientationVertical, 2)
		cell.SetHExpand(true)
		cell.Append(s.caption(c.key))
		cell.Append(f.box)
		grid.Attach(cell, c.col, c.row, 1, 1)
	}
	row := gtk.NewBox(gtk.OrientationHorizontal, 4)
	grid.SetHExpand(true)
	row.Append(grid)
	row.Append(s.link)
	box.Append(row)
	box.Append(s.note("margins.hint"))
	s.link.ConnectToggled(func() {
		if s.updating || !s.link.Active() {
			return
		}
		// Linking the margins makes them equal: the top one wins.
		v := s.margin["top"].value
		s.apply("undo.margins", "margins", app.Edit(func(p *model.Project) {
			p.Margins = model.Margins{Top: v, Bottom: v, Left: v, Right: v}
		}))
	})
	return box
}

func (s *settingsPanel) syncMargins() {
	m := s.w.session.Project().Margins
	s.updating = true
	defer func() { s.updating = false }()
	s.margin["top"].SetValue(m.Top)
	s.margin["bottom"].SetValue(m.Bottom)
	s.margin["left"].SetValue(m.Left)
	s.margin["right"].SetValue(m.Right)
}

// ---- scale -----------------------------------------------------------------------------------

func (s *settingsPanel) buildScale() gtk.Widgetter {
	box := s.column()
	w := s.w

	s.modeA = gtk.NewCheckButton()
	s.modeB = gtk.NewCheckButton()
	s.modeC = gtk.NewCheckButton()
	s.modeB.SetGroup(s.modeA)
	s.modeC.SetGroup(s.modeA)
	modes := []struct {
		btn  *gtk.CheckButton
		mode model.ScaleMode
		key  string
	}{{s.modeA, model.FitWidth, "scale.mode.fit_width"}, {s.modeB, model.FitHeight, "scale.mode.fit_height"}, {s.modeC, model.Width, "scale.mode.manual"}}
	for _, m := range modes {
		m := m
		w.texts.bind(func(tr *i18n.Translator) { m.btn.SetLabel(tr.T(m.key)) })
		m.btn.ConnectToggled(func() {
			if s.updating || !m.btn.Active() {
				return
			}
			s.setMode(m.mode)
		})
	}

	// B: the portion.
	s.portion = s.field(0, 1e6, 10, 0, "", func(v float64) {
		s.apply("undo.scale", "scale", app.Edit(func(p *model.Project) { p.Scale.PortionUnits = v }))
	})
	s.portionBox = gtk.NewBox(gtk.OrientationVertical, 4)
	s.portionBox.SetMarginStart(24)
	prow := gtk.NewBox(gtk.OrientationHorizontal, 6)
	prow.Append(s.portion.box)
	unit := gtk.NewLabel("")
	unit.AddCSSClass("dim-label")
	w.texts.bind(func(tr *i18n.Translator) { unit.SetLabel(tr.T("scale.portion.unit")) })
	prow.Append(unit)
	s.portionBox.Append(s.caption("scale.portion"))
	s.portionBox.Append(prow)
	s.portionBox.Append(s.note("scale.portion.hint"))

	// C: a width or a factor.
	s.manualBy = gtk.NewDropDown(nil, nil)
	s.manualBy.NotifyProperty("selected", func() {
		if s.updating {
			return
		}
		if s.manualBy.Selected() == 0 {
			s.setMode(model.Width)
		} else {
			s.setMode(model.Factor)
		}
	})
	w.texts.bind(func(tr *i18n.Translator) {
		was := s.updating
		s.updating = true
		s.manualBy.SetModel(stringList([]string{tr.T("scale.manual.by_width"), tr.T("scale.manual.by_factor")}))
		s.updating = was
		s.syncScale()
	})
	s.manualWidth = s.field(1, 20000, 1, 1, "mm", func(v float64) {
		s.apply("undo.scale", "scale", app.Edit(func(p *model.Project) { p.Scale.Mode, p.Scale.WidthMM = model.Width, v }))
	})
	s.manualPercent = s.field(0.1, 100000, 5, 2, "%", func(v float64) {
		s.apply("undo.scale", "scale", app.Edit(func(p *model.Project) { p.Scale.Mode, p.Scale.Percent = model.Factor, v }))
	})
	s.manualBox = gtk.NewBox(gtk.OrientationVertical, 4)
	s.manualBox.SetMarginStart(24)
	s.manualBox.Append(s.manualBy)
	s.manualBox.Append(s.manualWidth.box)
	s.manualBox.Append(s.manualPercent.box)

	// The result.
	s.resultWidth = newLabel()
	s.resultHeight = newLabel()
	s.resultFactor = newLabel()
	for _, l := range []*gtk.Label{s.resultWidth, s.resultHeight, s.resultFactor} {
		l.SetSelectable(true)
	}
	s.resultWidth.AddCSSClass("heading")

	box.Append(s.modeA)
	box.Append(s.modeB)
	box.Append(s.portionBox)
	box.Append(s.modeC)
	box.Append(s.manualBox)
	box.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
	box.Append(s.resultWidth)
	box.Append(s.resultFactor)
	box.Append(s.resultHeight)
	box.Append(s.note("scale.proportions"))
	return box
}

// setMode changes the mode of the scale. Going to the manual mode starts from
// the scale the drawing has now, so that nothing jumps.
func (s *settingsPanel) setMode(mode model.ScaleMode) {
	pl := s.w.session.Plan()
	s.apply("undo.scale", "scale", app.Edit(func(p *model.Project) {
		p.Scale.Mode = mode
		switch mode {
		case model.Width:
			if pl != nil {
				p.Scale.WidthMM = math.Round(pl.ContentWidthMM*10) / 10
			} else if !(p.Scale.WidthMM > 0) {
				p.Scale.WidthMM = 200
			}
		case model.Factor:
			if pl != nil {
				p.Scale.Percent = math.Round(pl.Percent*100) / 100
			} else if !(p.Scale.Percent > 0) {
				p.Scale.Percent = 100
			}
		}
	}))
}

func (s *settingsPanel) syncScale() {
	if s.resultFactor == nil {
		return // the section is still being built
	}
	p := s.w.session.Project()
	tr := s.tr()
	pl := s.w.session.Plan()
	doc := s.w.session.Document()
	s.updating = true
	defer func() { s.updating = false }()

	manual := p.Scale.Mode == model.Width || p.Scale.Mode == model.Factor
	s.modeA.SetActive(p.Scale.Mode == model.FitWidth)
	s.modeB.SetActive(p.Scale.Mode == model.FitHeight)
	s.modeC.SetActive(manual)
	s.portionBox.SetVisible(p.Scale.Mode == model.FitHeight)
	s.manualBox.SetVisible(manual)
	s.portion.SetValue(p.Scale.PortionUnits)
	if doc != nil {
		s.portion.max = math.Max(doc.Size.H, 1)
	}
	byFactor := p.Scale.Mode == model.Factor
	if (s.manualBy.Selected() == 1) != byFactor {
		if byFactor {
			s.manualBy.SetSelected(1)
		} else {
			s.manualBy.SetSelected(0)
		}
	}
	s.manualWidth.box.SetVisible(!byFactor)
	s.manualPercent.box.SetVisible(byFactor)
	if p.Scale.WidthMM > 0 {
		s.manualWidth.SetValue(p.Scale.WidthMM)
	} else if pl != nil {
		s.manualWidth.SetValue(round(pl.ContentWidthMM, 1))
	}
	if p.Scale.Percent > 0 {
		s.manualPercent.SetValue(p.Scale.Percent)
	} else if pl != nil {
		s.manualPercent.SetValue(round(pl.Percent, 2))
	}

	if pl == nil {
		s.resultWidth.SetLabel(tr.T("scale.no_document"))
		s.resultHeight.SetLabel("")
		s.resultFactor.SetLabel("")
		return
	}
	s.resultWidth.SetLabel(tr.T("scale.result.width", "mm", s.num(pl.ContentWidthMM, 1), "cm", s.num(pl.ContentWidthMM/10, 2)))
	s.resultFactor.SetLabel(tr.T("scale.result.factor", "percent", s.num(pl.Percent, 2)))
	s.resultHeight.SetLabel(tr.T("scale.result.height", "mm", s.num(pl.TotalHeightMM, 1), "cm", s.num(pl.TotalHeightMM/10, 2)))
}

// ---- placement ------------------------------------------------------------------------------------

func (s *settingsPanel) buildPlacement() gtk.Widgetter {
	box := s.column()
	w := s.w

	s.hDrop = gtk.NewDropDown(nil, nil)
	s.hDrop.NotifyProperty("selected", func() {
		i := int(s.hDrop.Selected())
		if s.updating || i < 0 || i >= len(horizontalChoices) {
			return
		}
		h := horizontalChoices[i]
		s.apply("undo.placement", "placement", app.Edit(func(p *model.Project) { p.Placement.Horizontal = h }))
	})
	s.vDrop = gtk.NewDropDown(nil, nil)
	s.vDrop.NotifyProperty("selected", func() {
		i := int(s.vDrop.Selected())
		if s.updating || i < 0 || i >= len(verticalChoices) {
			return
		}
		v := verticalChoices[i]
		s.apply("undo.placement", "placement", app.Edit(func(p *model.Project) { p.Placement.Vertical = v }))
	})
	s.offsetX = s.field(-1000, 1000, 1, 1, "mm", func(v float64) {
		s.apply("undo.placement", "placement-x", app.Edit(func(p *model.Project) { p.Placement.OffsetXMM = v }))
	})
	s.offsetY = s.field(-1000, 1000, 1, 1, "mm", func(v float64) {
		s.apply("undo.placement", "placement-y", app.Edit(func(p *model.Project) { p.Placement.OffsetYMM = v }))
	})
	s.offXRow = gtk.NewBox(gtk.OrientationVertical, 2)
	s.offXRow.Append(s.caption("placement.offset_x"))
	s.offXRow.Append(s.offsetX.box)

	box.Append(s.caption("placement.horizontal"))
	box.Append(s.hDrop)
	box.Append(s.offXRow)
	box.Append(s.caption("placement.vertical"))
	box.Append(s.vDrop)
	box.Append(s.caption("placement.offset_y"))
	box.Append(s.offsetY.box)
	box.Append(s.note("placement.hint"))

	w.texts.bind(func(tr *i18n.Translator) {
		was := s.updating
		s.updating = true
		s.hDrop.SetModel(stringList([]string{tr.T("placement.left"), tr.T("placement.center"), tr.T("placement.right"), tr.T("placement.custom")}))
		s.vDrop.SetModel(stringList([]string{tr.T("placement.top"), tr.T("placement.middle")}))
		s.updating = was
		s.syncPlacement()
	})
	return box
}

func (s *settingsPanel) syncPlacement() {
	p := s.w.session.Project().Placement
	s.updating = true
	defer func() { s.updating = false }()
	h, v := p.Horizontal, p.Vertical
	if h == "" {
		h = model.CenterH
	}
	if v == "" {
		v = model.Top
	}
	for i, c := range horizontalChoices {
		if c == h {
			s.hDrop.SetSelected(uint(i))
		}
	}
	for i, c := range verticalChoices {
		if c == v {
			s.vDrop.SetSelected(uint(i))
		}
	}
	s.offXRow.SetVisible(h == model.CustomH)
	s.offsetX.SetValue(p.OffsetXMM)
	s.offsetY.SetValue(p.OffsetYMM)
	s.offsetY.SetSensitive(v == model.Top)
}

// ---- everything ---------------------------------------------------------------------------------------

// sync shows the project in every widget of the panel.
func (s *settingsPanel) sync() {
	s.refreshSource()
	s.syncPaper()
	s.syncMargins()
	s.syncScale()
	s.syncPlacement()
	s.syncBackground()
	s.syncExport()
}

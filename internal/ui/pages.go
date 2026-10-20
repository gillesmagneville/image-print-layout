// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/svg-print-layout/internal/i18n"
	"github.com/gillesmagneville/svg-print-layout/internal/layout"
	"github.com/gillesmagneville/svg-print-layout/internal/model"
)

// pagesPanel is the right panel: what the plan says of the pages
// (ARCHITECTURE §9), and the proposals when a page does not fit.
type pagesPanel struct {
	w *window

	none    *gtk.Label
	summary *gtk.Box
	count   *gtk.Label
	width   *gtk.Label
	scale   *gtk.Label

	list   *gtk.ListBox
	scroll *gtk.ScrolledWindow
	rows   []*pageRow

	proposals     *gtk.Box
	proposalsHead *gtk.Box
	proposalsList *gtk.Box
}

// pageRow is the row of one page.
type pageRow struct {
	row   *gtk.ListBoxRow
	title *gtk.Label
	state *gtk.Label
	size  *gtk.Label
	scale *gtk.Label
	span  *gtk.Label
	warns *gtk.Box
}

func (w *window) buildPagesPanel() gtk.Widgetter {
	p := &pagesPanel{w: w}
	w.pages = p

	column := gtk.NewBox(gtk.OrientationVertical, 8)
	column.SetMarginTop(12)
	column.SetMarginBottom(12)
	column.SetMarginStart(12)
	column.SetMarginEnd(12)

	title := gtk.NewLabel("")
	title.AddCSSClass("heading")
	title.SetXAlign(0)
	p.none = newLabel()
	p.none.AddCSSClass("dim-label")

	p.count = newLabel()
	p.count.AddCSSClass("title-4")
	p.width = newLabel()
	p.scale = newLabel()
	p.summary = gtk.NewBox(gtk.OrientationVertical, 2)
	p.summary.Append(p.count)
	p.summary.Append(p.width)
	p.summary.Append(p.scale)

	p.list = gtk.NewListBox()
	p.list.SetSelectionMode(gtk.SelectionNone)
	p.list.AddCSSClass("boxed-list")
	p.list.SetVAlign(gtk.AlignStart)
	p.scroll = gtk.NewScrolledWindow()
	p.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	p.scroll.SetVExpand(true)
	p.scroll.SetChild(p.list)

	proposalsTitle := gtk.NewLabel("")
	proposalsTitle.AddCSSClass("heading")
	proposalsTitle.SetXAlign(0)
	proposalsHint := newLabel()
	proposalsHint.AddCSSClass("caption")
	proposalsHint.AddCSSClass("dim-label")
	p.proposalsList = gtk.NewBox(gtk.OrientationVertical, 6)
	p.proposals = gtk.NewBox(gtk.OrientationVertical, 6)
	p.proposals.Append(proposalsTitle)
	p.proposals.Append(proposalsHint)
	p.proposals.Append(p.proposalsList)
	p.proposals.SetVisible(false)

	column.Append(title)
	column.Append(p.none)
	column.Append(p.summary)
	column.Append(p.scroll)
	column.Append(p.proposals)
	column.SetSizeRequest(300, -1)

	w.texts.bind(func(tr *i18n.Translator) {
		title.SetLabel(tr.T("pages.title"))
		p.none.SetLabel(tr.T("pages.none"))
		proposalsTitle.SetLabel(tr.T("corrections.title"))
		proposalsHint.SetLabel(tr.T("corrections.hint"))
		p.update()
	})
	return column
}

// update shows the plan of the session.
func (p *pagesPanel) update() {
	tr := p.w.texts.tr
	plan := p.w.session.Plan()
	has := plan != nil
	p.none.SetVisible(!has)
	p.summary.SetVisible(has)
	p.scroll.SetVisible(has)
	if !has {
		p.proposals.SetVisible(false)
		return
	}
	p.count.SetLabel(tr.N("pages.count", len(plan.Pages)))
	p.width.SetLabel(tr.T("pages.width", "mm", p.num(plan.ContentWidthMM, 1), "cm", p.num(plan.ContentWidthMM/10, 2)))
	p.scale.SetLabel(tr.T("pages.scale", "percent", p.num(plan.Percent, 2)))

	for len(p.rows) < len(plan.Pages) {
		r := p.newRow()
		p.rows = append(p.rows, r)
		p.list.Append(r.row)
	}
	for len(p.rows) > len(plan.Pages) {
		last := p.rows[len(p.rows)-1]
		p.list.Remove(last.row)
		p.rows = p.rows[:len(p.rows)-1]
	}
	for i, page := range plan.Pages {
		p.fill(p.rows[i], plan, page)
	}
	p.updateProposals(plan)
}

func (p *pagesPanel) num(v float64, d int) string {
	return p.w.texts.tr.Number(round(v, d), -1)
}

func (p *pagesPanel) newRow() *pageRow {
	r := &pageRow{
		row: gtk.NewListBoxRow(), title: newLabel(), state: newLabel(),
		size: newLabel(), scale: newLabel(), span: newLabel(), warns: gtk.NewBox(gtk.OrientationVertical, 2),
	}
	r.row.SetActivatable(false)
	r.title.AddCSSClass("heading")
	r.title.SetHExpand(true)
	r.state.SetXAlign(1)
	r.state.AddCSSClass("caption")
	for _, l := range []*gtk.Label{r.size, r.scale, r.span} {
		l.AddCSSClass("caption")
	}
	r.span.AddCSSClass("dim-label")

	head := gtk.NewBox(gtk.OrientationHorizontal, 6)
	head.Append(r.title)
	head.Append(r.state)
	box := gtk.NewBox(gtk.OrientationVertical, 2)
	box.SetMarginTop(8)
	box.SetMarginBottom(8)
	box.SetMarginStart(10)
	box.SetMarginEnd(10)
	box.Append(head)
	box.Append(r.size)
	box.Append(r.scale)
	box.Append(r.span)
	box.Append(r.warns)
	r.row.SetChild(box)
	return r
}

// fill writes a page in its row.
func (p *pagesPanel) fill(r *pageRow, plan *layout.Plan, page layout.Page) {
	tr := p.w.texts.tr
	r.title.SetLabel(tr.T("pages.row.title", "n", page.Number))
	r.state.SetLabel(tr.T("state." + page.State.String()))
	for _, c := range []string{"success", "warning", "error"} {
		r.state.RemoveCSSClass(c)
	}
	switch page.State {
	case layout.Conforming:
		r.state.AddCSSClass("success")
	case layout.MarginOverflow:
		r.state.AddCSSClass("warning")
	default:
		r.state.AddCSSClass("error")
	}
	r.size.SetLabel(tr.T("pages.row.size", "w", p.num(page.ContentW, 1), "h", p.num(page.ContentH, 1)))
	r.scale.SetLabel(tr.T("pages.row.scale", "percent", p.num(plan.Percent, 2)))
	r.span.SetLabel(tr.T("pages.row.range",
		"from", p.num(page.Top, 1), "to", p.num(page.Bottom, 1),
		"fmm", p.num(plan.MM(page.Top), 1), "tmm", p.num(plan.MM(page.Bottom), 1)))

	for c := r.warns.FirstChild(); c != nil; c = r.warns.FirstChild() {
		r.warns.Remove(c)
	}
	for _, it := range page.Items {
		l := newLabel()
		l.SetLabel(diagText(tr, it.Code, it.Args))
		l.AddCSSClass("caption")
		if page.State == layout.SheetOverflow {
			l.AddCSSClass("error")
		} else {
			l.AddCSSClass("warning")
		}
		r.warns.Append(l)
	}
}

// updateProposals lists the corrections the plan offers.
func (p *pagesPanel) updateProposals(plan *layout.Plan) {
	for c := p.proposalsList.FirstChild(); c != nil; c = p.proposalsList.FirstChild() {
		p.proposalsList.Remove(c)
	}
	p.proposals.SetVisible(len(plan.Corrections) > 0)
	for _, c := range plan.Corrections {
		c := c
		b := gtk.NewButton()
		text := newLabel()
		text.SetLabel(p.correctionText(plan, c))
		text.SetWrap(true)
		text.SetMaxWidthChars(34)
		hint := newLabel()
		hint.AddCSSClass("caption")
		hint.AddCSSClass("dim-label")
		key := "correction.improves"
		if c.Resolves {
			key = "correction.resolves"
		}
		hint.SetLabel(p.w.texts.tr.T(key))
		inner := gtk.NewBox(gtk.OrientationVertical, 2)
		inner.Append(text)
		inner.Append(hint)
		b.SetChild(inner)
		b.ConnectClicked(func() { p.w.applyCorrection(c) })
		p.proposalsList.Append(b)
	}
}

// correctionText says what a proposal would do.
func (p *pagesPanel) correctionText(plan *layout.Plan, c layout.Correction) string {
	tr := p.w.texts.tr
	switch c.Kind {
	case layout.CorrectionAddCut:
		return tr.T("correction.add_cut", "mm", p.num(plan.MM(c.Position), 1), "page", c.Page)
	case layout.CorrectionMoveCut:
		n := 0
		for i, cut := range p.w.session.Project().Cuts {
			if cut.ID == c.CutID {
				n = i + 1
			}
		}
		return tr.T("correction.move_cut", "n", n, "mm", p.num(plan.MM(c.Position), 1))
	case layout.CorrectionReduceScale:
		return tr.T("correction.reduce_scale", "mm", p.num(c.WidthMM, 1), "percent", p.num(c.Percent, 1))
	case layout.CorrectionChangePaper:
		orientation := tr.T("paper.portrait")
		if c.Orientation == model.Landscape {
			orientation = tr.T("paper.landscape")
		}
		return tr.T("correction.change_paper", "paper", c.Paper, "orientation", orientation)
	}
	return string(c.Kind)
}

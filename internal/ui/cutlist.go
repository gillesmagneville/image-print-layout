// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/svg-print-layout/internal/i18n"
)

// buildToolbar makes the two actions on the cuts that need no position: add one
// where the view is, and propose cuts.
func (p *pagesPanel) buildToolbar() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 6)
	box.SetHomogeneous(true)

	add := gtk.NewButton()
	auto := gtk.NewButton()
	for _, b := range []*gtk.Button{add, auto} {
		b.SetHExpand(true)
		box.Append(b)
	}
	add.ConnectClicked(p.w.addCutInView)
	auto.ConnectClicked(p.w.autoCuts)

	p.w.texts.bind(func(tr *i18n.Translator) {
		for _, c := range []struct {
			b        *gtk.Button
			text, tt string
		}{{add, "cuts.add", "cuts.add.tooltip"}, {auto, "cuts.auto", "cuts.auto.tooltip"}} {
			label := gtk.NewLabel(tr.T(c.text))
			label.SetWrap(true)
			label.SetJustify(gtk.JustifyCenter)
			c.b.SetChild(label)
			c.b.SetTooltipText(tr.T(c.tt))
		}
	})
	return box
}

// buildCuts makes the section of the cuts, under the pages.
func (p *pagesPanel) buildCuts() *gtk.Box {
	title := gtk.NewLabel("")
	title.AddCSSClass("heading")
	title.SetXAlign(0)
	p.cutsNone = newLabel()
	p.cutsNone.AddCSSClass("dim-label")
	p.cutsNone.AddCSSClass("caption")

	p.cutsList = gtk.NewListBox()
	p.cutsList.SetSelectionMode(gtk.SelectionSingle)
	p.cutsList.AddCSSClass("boxed-list")
	p.cutsList.ConnectRowSelected(func(row *gtk.ListBoxRow) {
		if p.updating {
			return
		}
		id := 0
		if row != nil {
			if i := row.Index(); i >= 0 && i < len(p.cutRows) {
				id = p.cutRows[i].id
			}
		}
		p.w.selectCutFromList(id)
	})

	box := gtk.NewBox(gtk.OrientationVertical, 6)
	box.Append(title)
	box.Append(p.cutsNone)
	box.Append(p.cutsList)
	p.w.texts.bind(func(tr *i18n.Translator) {
		title.SetLabel(tr.T("cuts.title"))
		p.cutsNone.SetLabel(tr.T("cuts.none"))
	})
	return box
}

func (p *pagesPanel) newCutRow() *cutRow {
	r := &cutRow{
		row: gtk.NewListBoxRow(), title: newLabel(), where: newLabel(), warning: newLabel(),
		remove: gtk.NewButtonFromIconName("edit-delete-symbolic"),
	}
	r.title.AddCSSClass("heading")
	r.title.SetHExpand(true)
	r.where.AddCSSClass("caption")
	r.where.AddCSSClass("dim-label")
	r.warning.AddCSSClass("caption")
	r.warning.AddCSSClass("warning")
	r.remove.AddCSSClass("flat")
	r.remove.SetVAlign(gtk.AlignCenter)
	r.remove.ConnectClicked(func() { p.w.deleteCut(r.id) })

	head := gtk.NewBox(gtk.OrientationHorizontal, 6)
	head.Append(r.title)
	head.Append(r.remove)
	box := gtk.NewBox(gtk.OrientationVertical, 2)
	box.SetMarginTop(6)
	box.SetMarginBottom(6)
	box.SetMarginStart(10)
	box.SetMarginEnd(6)
	box.Append(head)
	box.Append(r.where)
	box.Append(r.warning)
	r.row.SetChild(box)
	return r
}

// updateCuts shows the cuts of the session.
func (p *pagesPanel) updateCuts() {
	tr := p.w.texts.tr
	plan := p.w.session.Plan()
	cuts := p.w.cuts
	p.cuts = cuts
	p.cutsNone.SetVisible(len(cuts) == 0)
	p.cutsList.SetVisible(len(cuts) > 0)

	for len(p.cutRows) < len(cuts) {
		r := p.newCutRow()
		p.cutRows = append(p.cutRows, r)
		p.cutsList.Append(r.row)
	}
	for len(p.cutRows) > len(cuts) {
		last := p.cutRows[len(p.cutRows)-1]
		p.cutsList.Remove(last.row)
		p.cutRows = p.cutRows[:len(p.cutRows)-1]
	}
	for i, c := range cuts {
		r := p.cutRows[i]
		r.id = c.ID
		r.title.SetLabel(tr.T("cuts.row.title", "n", c.Number))
		r.remove.SetTooltipText(tr.T("cuts.row.delete", "n", c.Number))
		mm := c.Position
		if plan != nil {
			mm = plan.MM(c.Position)
		}
		r.where.SetLabel(tr.T("cuts.row.position", "mm", p.num(mm, 1), "units", p.num(c.Position, 1)))
		switch {
		case c.Crossed == 0:
			r.warning.SetLabel("")
			r.warning.SetVisible(false)
		case len(c.Names) == 0:
			r.warning.SetLabel(tr.T("cuts.crosses_unnamed"))
			r.warning.SetVisible(true)
		default:
			names := strings.Join(c.Names, ", ")
			if c.Crossed > len(c.Names) {
				names += "…"
			}
			r.warning.SetLabel(tr.N("cuts.crosses", c.Crossed, "names", names))
			r.warning.SetVisible(true)
		}
	}
}

// selectInLists shows the selection of the window in the two lists, and brings
// the selected row into view.
func (p *pagesPanel) selectInLists() {
	w := p.w
	was := p.updating
	p.updating = true
	defer func() { p.updating = was }()

	if w.selPage > 0 && w.selPage <= len(p.rows) {
		p.list.SelectRow(p.rows[w.selPage-1].row)
	} else {
		p.list.UnselectAll()
	}
	cutIndex := -1
	for i, c := range p.cuts {
		if c.ID == w.selCut {
			cutIndex = i
		}
	}
	if cutIndex >= 0 {
		p.cutsList.SelectRow(p.cutRows[cutIndex].row)
	} else {
		p.cutsList.UnselectAll()
	}
}

// scrollToSelection brings the selected row of the lists into view; it waits a
// moment so that the rows have their size.
func (p *pagesPanel) scrollToSelection() {
	glib.IdleAdd(func() {
		var row *gtk.ListBoxRow
		var list *gtk.ListBox
		w := p.w
		switch {
		case w.selPage > 0 && w.selPage <= len(p.rows):
			row, list = p.rows[w.selPage-1].row, p.list
		case w.selCut != 0:
			for i, c := range p.cuts {
				if c.ID == w.selCut && i < len(p.cutRows) {
					row, list = p.cutRows[i].row, p.cutsList
				}
			}
		}
		if row == nil {
			return
		}
		// The position of the row in what the scroll holds: its own, in its
		// list, plus the list's in the section it belongs to.
		y := float64(row.Allocation().Y())
		if list == p.cutsList {
			y += float64(p.cutsList.Allocation().Y() + p.cutsBox.Allocation().Y())
		}
		h := float64(row.Allocation().Height())
		adj := p.scroll.VAdjustment()
		switch {
		case y < adj.Value():
			adj.SetValue(y)
		case y+h > adj.Value()+adj.PageSize():
			adj.SetValue(y + h - adj.PageSize())
		}
	})
}

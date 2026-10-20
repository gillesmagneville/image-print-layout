// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package ui

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gillesmagneville/svg-print-layout/internal/i18n"
)

// typingDelay is how long a value waits after the last keystroke before it is
// applied: there is no Apply button (ARCHITECTURE §9).
const typingDelay = 150 // ms

// debouncer runs a function once, typingDelay after the last call to trigger.
type debouncer struct {
	source glib.SourceHandle
}

func (d *debouncer) trigger(f func()) { d.triggerAfter(typingDelay, f) }

// triggerAfter is trigger with a delay of its own, in milliseconds.
func (d *debouncer) triggerAfter(ms uint, f func()) {
	d.cancel()
	d.source = glib.TimeoutAdd(ms, func() bool {
		d.source = 0
		f()
		return false
	})
}

func (d *debouncer) cancel() {
	if d.source != 0 {
		glib.SourceRemove(d.source)
		d.source = 0
	}
}

// pending tells whether a value is waiting to be applied.
func (d *debouncer) pending() bool { return d.source != 0 }

// numField is a field for a number in the notation of the language of the
// interface. A GtkSpinButton reads what is typed with the locale of the system,
// which is not necessarily the language chosen in the program: a French user
// of an English system could not type 200,5. The field reads either separator.
type numField struct {
	box   *gtk.Box
	entry *gtk.Entry
	unit  *gtk.Label

	tr             func() *i18n.Translator
	min, max, step float64
	decimals       int
	value          float64
	onChange       func(v float64)
	updating       bool
	typing         debouncer
}

// newNumField makes a field for values from min to max. step is what the
// arrow keys add (ten times as much with Shift). decimals is the number of
// decimals shown at most.
func newNumField(tr func() *i18n.Translator, min, max, step float64, decimals int, unit string) *numField {
	f := &numField{tr: tr, min: min, max: max, step: step, decimals: decimals}
	f.box = gtk.NewBox(gtk.OrientationHorizontal, 6)
	f.entry = gtk.NewEntry()
	f.entry.SetWidthChars(8)
	f.entry.SetMaxWidthChars(10)
	f.entry.SetAlignment(1)
	f.entry.SetInputPurpose(gtk.InputPurposeNumber)
	f.entry.SetHExpand(true)
	f.box.Append(f.entry)
	if unit != "" {
		f.unit = gtk.NewLabel(unit)
		f.unit.AddCSSClass("dim-label")
		f.box.Append(f.unit)
	}

	f.entry.ConnectChanged(func() {
		if f.updating {
			return
		}
		f.typing.trigger(f.commit)
	})
	f.entry.ConnectActivate(func() {
		f.typing.cancel()
		f.commit()
	})
	focus := gtk.NewEventControllerFocus()
	focus.ConnectLeave(func() {
		f.typing.cancel()
		f.commit()
		f.show() // canonical notation once the user has left the field
	})
	f.entry.AddController(focus)

	keys := gtk.NewEventControllerKey()
	keys.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		var dir float64
		switch keyval {
		case gdk.KEY_Up:
			dir = 1
		case gdk.KEY_Down:
			dir = -1
		default:
			return false
		}
		step := f.step
		if state&gdk.ShiftMask != 0 {
			step *= 10
		}
		f.typing.cancel()
		v := f.value
		if parsed, ok := f.parse(); ok {
			v = parsed
		}
		f.set(math.Min(math.Max(round(v+dir*step, f.decimals), f.min), f.max), true)
		return true
	})
	f.entry.AddController(keys)
	return f
}

func round(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}

// parse reads the text of the field. It fails on a text that is not a number
// (a half-typed "12," is not worth a message), and on a value out of range.
func (f *numField) parse() (float64, bool) {
	v, err := i18n.ParseNumber(f.entry.Text())
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// commit applies the text of the field if it holds a good value.
func (f *numField) commit() {
	v, ok := f.parse()
	if !ok || v < f.min || v > f.max {
		f.entry.AddCSSClass("error")
		return
	}
	f.entry.RemoveCSSClass("error")
	v = round(v, f.decimals)
	if v == f.value {
		return
	}
	f.value = v
	if f.onChange != nil && !f.updating {
		f.onChange(v)
	}
}

// set puts a value in the field; notify says whether onChange hears of it.
func (f *numField) set(v float64, notify bool) {
	f.value = v
	f.show()
	f.entry.RemoveCSSClass("error")
	if notify && f.onChange != nil {
		f.onChange(v)
	}
}

// show writes the value in the notation of the language. The cursor goes to the
// end: GtkEntry leaves it at the start after SetText.
func (f *numField) show() {
	f.updating = true
	f.entry.SetText(f.tr().Number(round(f.value, f.decimals), -1))
	f.entry.SetPosition(-1)
	f.updating = false
}

// SetValue changes the value from the program: onChange is not called. A field
// the user is typing in is left alone: while a value waits to be applied, and
// once the field already shows the value, in whatever notation (the user typed
// "65", the program says 65 after applying it: writing "65" again would send
// the cursor back to the start, and the next digit would go before the first).
// The canonical notation is written when the user leaves the field.
func (f *numField) SetValue(v float64) {
	if f.typing.pending() {
		return
	}
	if shown, ok := f.parse(); ok && round(shown, f.decimals) == round(v, f.decimals) {
		f.value = v
		return
	}
	f.value = v
	f.show()
	f.entry.RemoveCSSClass("error")
}

// Reformat writes the value again, after a change of language.
func (f *numField) Reformat() { f.show() }

// SetSensitive enables or disables the field.
func (f *numField) SetSensitive(on bool) { f.box.SetSensitive(on) }

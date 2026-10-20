// SPDX-License-Identifier: GPL-3.0-only

package svgdoc

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// element is an element of the document being read: where its start tag sits
// in the source, its attributes, and where it sits in the tree.
type element struct {
	Name  string // as written, prefix included
	Local string
	Attrs []attr
	// Start and TagEnd delimit the start tag, "<" to ">" included. NameEnd is
	// the offset, in the source, just after the name in the tag.
	Start, TagEnd, NameEnd int
	// End is the offset just after the end tag (set when the element ends).
	End int

	Depth  int // 0 for the root
	Index  int // 1-based among the siblings of the same name
	Parent *element
	counts map[string]int

	text strings.Builder // the character data directly inside, for <style>

	// Filled by the analysis as the walk goes down.
	m        matrix // cumulative transformation, own included
	mKnown   bool   // false when a transform in the chain could not be read
	fill     string // inherited fill, the effective one for this element
	hidden   bool   // not drawn: display none, or an opacity of zero, here or above
	inactive bool   // inside a container that draws nothing by itself (defs...)

	container bool // a container of blocks (the root, a layer...), set by findBlocks
}

// path writes the position of the element as a path of indices, the way an
// XPath would: /svg/g[1]/rect[2].
func (e *element) path() string {
	var parts []string
	for n := e; n != nil; n = n.Parent {
		if n.Parent == nil {
			parts = append(parts, n.Local)
		} else {
			parts = append(parts, n.Local+"["+strconv.Itoa(n.Index)+"]")
		}
	}
	var b strings.Builder
	for i := len(parts) - 1; i >= 0; i-- {
		b.WriteByte('/')
		b.WriteString(parts[i])
	}
	return b.String()
}

func (e *element) attr(local string) (string, bool) { return find(e.Attrs, local) }

type eventKind int

const (
	evStart eventKind = iota
	evEnd
	evText
)

// walker reads the elements of an XML document in order, with their position
// in the source. It does not validate: a document that is not well-formed ends
// the walk with an error, after the elements that could be read.
type walker struct {
	src   []byte
	dec   *xml.Decoder
	stack []*element
}

func newWalker(src []byte) *walker {
	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.Strict = false
	return &walker{src: src, dec: dec}
}

// next returns the next event: the element that starts or ends, or, for
// evText, the element the character data belongs to. io.EOF ends the walk.
func (w *walker) next() (eventKind, *element, string, error) {
	for {
		before := int(w.dec.InputOffset())
		tok, err := w.dec.RawToken()
		if err == io.EOF && len(w.stack) > 0 {
			return 0, nil, "", fmt.Errorf("the element <%s> is not closed", w.stack[len(w.stack)-1].Name)
		}
		if err != nil {
			return 0, nil, "", err
		}
		after := int(w.dec.InputOffset())
		switch t := tok.(type) {
		case xml.StartElement:
			start := before
			for start < after && w.src[start] != '<' {
				start++
			}
			name, nameEnd, attrs := scanTag(w.src[start:after])
			e := &element{
				Name: name, Local: localName(name), Attrs: attrs,
				Start: start, TagEnd: after, NameEnd: start + nameEnd,
			}
			if n := len(w.stack); n > 0 {
				p := w.stack[n-1]
				e.Parent, e.Depth = p, p.Depth+1
				if p.counts == nil {
					p.counts = map[string]int{}
				}
				p.counts[e.Local]++
				e.Index = p.counts[e.Local]
			} else {
				e.Index = 1
			}
			w.stack = append(w.stack, e)
			_ = t
			return evStart, e, "", nil
		case xml.EndElement:
			n := len(w.stack)
			if n == 0 {
				continue
			}
			e := w.stack[n-1]
			if t.Name.Local != e.Local {
				return 0, nil, "", fmt.Errorf("the end tag </%s> closes <%s>", t.Name.Local, e.Name)
			}
			w.stack = w.stack[:n-1]
			e.End = after
			return evEnd, e, "", nil
		case xml.CharData:
			if n := len(w.stack); n > 0 {
				return evText, w.stack[n-1], string(t), nil
			}
		}
	}
}

// walk calls visit for every event until the document ends. It returns an
// error only when the document is not well-formed XML (io.EOF is the normal
// end).
func walk(src []byte, visit func(kind eventKind, e *element, text string)) error {
	w := newWalker(src)
	for {
		kind, e, text, err := w.next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		visit(kind, e, text)
	}
}

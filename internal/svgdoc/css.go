// SPDX-License-Identifier: GPL-3.0-only

package svgdoc

import "strings"

// stylesheet holds the simple rules of the <style> elements of a document:
// ".class { ... }" and "tag { ... }", the only selectors the analysis reads.
// Anything more elaborate is left to the rendering engine, and the analysis
// then knows less than the drawing does.
type stylesheet struct {
	byClass map[string]map[string]string
	byTag   map[string]map[string]string
	// webFonts is set when the sheet declares @font-face.
	webFonts bool
	// fontFamilies are the font-family values the sheet declares.
	fontFamilies []string
}

func newStylesheet() *stylesheet {
	return &stylesheet{byClass: map[string]map[string]string{}, byTag: map[string]map[string]string{}}
}

// add reads the text of a <style> element.
func (s *stylesheet) add(css string) {
	css = stripComments(css)
	for len(css) > 0 {
		css = strings.TrimSpace(css)
		if css == "" {
			return
		}
		if css[0] == '@' {
			// An at-rule: with a block ("@font-face {...}") or without
			// ("@import url(...);").
			semi, open := strings.IndexByte(css, ';'), strings.IndexByte(css, '{')
			if open < 0 || (semi >= 0 && semi < open) {
				if semi < 0 {
					return
				}
				css = css[semi+1:]
				continue
			}
			if strings.HasPrefix(strings.ToLower(css), "@font-face") {
				s.webFonts = true
			}
			css = skipBlock(css[open:])
			continue
		}
		open := strings.IndexByte(css, '{')
		if open < 0 {
			return
		}
		selectors, rest := css[:open], css[open+1:]
		closeAt := strings.IndexByte(rest, '}')
		if closeAt < 0 {
			return
		}
		props := styleProps(rest[:closeAt])
		css = rest[closeAt+1:]
		if v, ok := props["font-family"]; ok {
			s.fontFamilies = append(s.fontFamilies, v)
		}
		for _, sel := range strings.Split(selectors, ",") {
			sel = strings.TrimSpace(sel)
			switch {
			case sel == "" || strings.ContainsAny(sel, " >+~[:#*"):
				// not a simple selector
			case sel[0] == '.' && !strings.Contains(sel[1:], "."):
				merge(s.byClass, sel[1:], props)
			case !strings.Contains(sel, "."):
				merge(s.byTag, strings.ToLower(sel), props)
			}
		}
	}
}

func merge(into map[string]map[string]string, key string, props map[string]string) {
	m := into[key]
	if m == nil {
		m = map[string]string{}
		into[key] = m
	}
	for k, v := range props {
		m[k] = v
	}
}

func stripComments(css string) string {
	for {
		i := strings.Index(css, "/*")
		if i < 0 {
			return css
		}
		j := strings.Index(css[i+2:], "*/")
		if j < 0 {
			return css[:i]
		}
		css = css[:i] + " " + css[i+2+j+2:]
	}
}

// skipBlock drops a "{ ... }" block, braces nested, from the start of css.
func skipBlock(css string) string {
	depth := 0
	for i := 0; i < len(css); i++ {
		switch css[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return css[i+1:]
			}
		}
	}
	return ""
}

// props is the part of the style of an element that the analysis reads. An
// empty field is a property that is not set.
type props struct {
	fill, fillOpacity, opacity, display, visibility, filter, fontFamily string
}

// propsOf computes the properties of an element: presentation attributes,
// then the rules of the style sheet (by tag, then by class), then the style
// attribute, each overriding the one before.
func (s *stylesheet) propsOf(e *element) props {
	var p props
	set := func(name, value string) {
		switch name {
		case "fill":
			p.fill = value
		case "fill-opacity":
			p.fillOpacity = value
		case "opacity":
			p.opacity = value
		case "display":
			p.display = value
		case "visibility":
			p.visibility = value
		case "filter":
			p.filter = value
		case "font-family":
			p.fontFamily = value
		}
	}
	for _, a := range e.Attrs {
		if a.Name == a.Local { // unprefixed presentation attributes only
			set(a.Local, strings.TrimSpace(a.Value))
		}
	}
	if len(s.byTag) > 0 {
		for k, v := range s.byTag[strings.ToLower(e.Local)] {
			set(k, v)
		}
	}
	if len(s.byClass) > 0 {
		if classes, ok := e.attr("class"); ok {
			for _, c := range strings.Fields(classes) {
				for k, v := range s.byClass[c] {
					set(k, v)
				}
			}
		}
	}
	if style, ok := e.attr("style"); ok {
		for k, v := range styleProps(style) {
			set(k, v)
		}
	}
	return p
}

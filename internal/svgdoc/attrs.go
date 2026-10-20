// SPDX-License-Identifier: GPL-3.0-only

package svgdoc

import (
	"strings"
)

// attr is an attribute of a start tag, with where its value sits in the tag.
type attr struct {
	Name  string // as written, prefix included
	Local string // without the prefix
	Value string // raw, entities not decoded
	// ValStart and ValEnd delimit the value, quotes excluded, in the bytes of
	// the tag.
	ValStart, ValEnd int
}

// scanTag reads a start tag, given as the bytes from its "<" to its ">".
// It returns the name of the element as written, the offset just after that
// name, and the attributes. It is as lenient as the XML decoder that located
// the tag: it never fails, it stops at what it cannot read.
func scanTag(tag []byte) (name string, nameEnd int, attrs []attr) {
	i := 1
	for i < len(tag) && !isSpace(tag[i]) && tag[i] != '/' && tag[i] != '>' {
		i++
	}
	name, nameEnd = string(tag[1:i]), i
	for i < len(tag) {
		for i < len(tag) && isSpace(tag[i]) {
			i++
		}
		if i >= len(tag) || tag[i] == '/' || tag[i] == '>' {
			break
		}
		start := i
		for i < len(tag) && tag[i] != '=' && !isSpace(tag[i]) && tag[i] != '>' && tag[i] != '/' {
			i++
		}
		an := string(tag[start:i])
		for i < len(tag) && isSpace(tag[i]) {
			i++
		}
		if i >= len(tag) || tag[i] != '=' {
			continue // an attribute with no value: not XML, skipped
		}
		i++
		for i < len(tag) && isSpace(tag[i]) {
			i++
		}
		if i >= len(tag) || (tag[i] != '"' && tag[i] != '\'') {
			break
		}
		quote := tag[i]
		i++
		vs := i
		for i < len(tag) && tag[i] != quote {
			i++
		}
		if i >= len(tag) {
			break
		}
		attrs = append(attrs, attr{Name: an, Local: localName(an), Value: string(tag[vs:i]), ValStart: vs, ValEnd: i})
		i++
	}
	return name, nameEnd, attrs
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func localName(name string) string {
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[i+1:]
	}
	return name
}

// find returns the value of the attribute with that local name, whatever its
// prefix, or "" and false.
func find(attrs []attr, local string) (string, bool) {
	for _, a := range attrs {
		if a.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

// findNS returns the value of an attribute by its full name, prefix included.
func findNS(attrs []attr, name string) (string, bool) {
	for _, a := range attrs {
		if a.Name == name {
			return a.Value, true
		}
	}
	return "", false
}

// styleProps reads the declarations of a style attribute or a CSS rule body
// into a map of lowercase property names. The last declaration wins.
func styleProps(css string) map[string]string {
	props := map[string]string{}
	for _, decl := range strings.Split(css, ";") {
		i := strings.IndexByte(decl, ':')
		if i < 0 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(decl[:i]))
		value := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(decl[i+1:]), "!important"))
		if name != "" {
			props[name] = value
		}
	}
	return props
}

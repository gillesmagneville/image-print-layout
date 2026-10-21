// SPDX-License-Identifier: GPL-3.0-only

package export

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/gillesmagneville/pageslice/internal/diag"
	"github.com/gillesmagneville/pageslice/internal/engine"
	"github.com/gillesmagneville/pageslice/internal/units"
)

// PDF is what is read back of a PDF written by cairo: its pages, in order.
// The reader is not a general PDF reader: it knows the shape of the files the
// engine writes (a page tree, flate streams, object streams) and says so when
// it meets another one.
type PDF struct {
	Pages []PDFPage
}

// PDFPage is a page of a PDF.
type PDFPage struct {
	// WidthPt and HeightPt are the size of the media box, in points.
	WidthPt, HeightPt float64
	// Content is the decoded content stream of the page.
	Content string
}

// pdfObject is an object found in the file: its dictionary as text, and its
// stream, when it has one.
type pdfObject struct {
	dict   string
	stream []byte
}

var (
	objectRe  = regexp.MustCompile(`(?m)^(\d+) (\d+) obj\s*$`)
	refRe     = regexp.MustCompile(`(\d+) \d+ R`)
	mediaRe   = regexp.MustCompile(`/MediaBox\s*\[([^\]]*)\]`)
	kidsRe    = regexp.MustCompile(`/Kids\s*\[([^\]]*)\]`)
	contentRe = regexp.MustCompile(`/Contents\s*(\[[^\]]*\]|\d+ \d+ R)`)
	pagesRe   = regexp.MustCompile(`/Pages\s+(\d+) \d+ R`)
	firstRe   = regexp.MustCompile(`/First\s+(\d+)`)
	countRe   = regexp.MustCompile(`/N\s+(\d+)`)
	typeRe    = regexp.MustCompile(`/Type\s*/(\w+)`)
)

// ReadPDF reads the pages of a PDF written by the engine.
func ReadPDF(data []byte) (*PDF, error) {
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, fmt.Errorf("it does not start with %%PDF-")
	}
	if !bytes.HasSuffix(bytes.TrimRight(data, "\r\n "), []byte("%%EOF")) {
		return nil, fmt.Errorf("it does not end with %%%%EOF: the file is cut short")
	}
	objs, err := readObjects(data)
	if err != nil {
		return nil, err
	}

	var root *pdfObject
	for _, o := range objs {
		o := o
		if typeOf(o.dict) == "Catalog" {
			root = &o
			break
		}
	}
	if root == nil {
		return nil, fmt.Errorf("it has no catalog")
	}
	m := pagesRe.FindStringSubmatch(root.dict)
	if m == nil {
		return nil, fmt.Errorf("its catalog has no page tree")
	}
	top, _ := strconv.Atoi(m[1])

	out := &PDF{}
	if err := walkPages(objs, top, nil, out, 0); err != nil {
		return nil, err
	}
	return out, nil
}

func typeOf(dict string) string {
	if m := typeRe.FindStringSubmatch(dict); m != nil {
		return m[1]
	}
	return ""
}

// readObjects finds every object of the file, those inside object streams
// included.
func readObjects(data []byte) (map[int]pdfObject, error) {
	objs := map[int]pdfObject{}
	locs := objectRe.FindAllSubmatchIndex(data, -1)
	for i, loc := range locs {
		num, _ := strconv.Atoi(string(data[loc[2]:loc[3]]))
		end := len(data)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		body := data[loc[1]:end]
		dict, stream := body, []byte(nil)
		if at := bytes.Index(body, []byte("stream\n")); at >= 0 {
			dict = body[:at]
			rest := body[at+len("stream\n"):]
			if e := bytes.LastIndex(rest, []byte("\nendstream")); e >= 0 {
				stream = rest[:e]
			}
		}
		o := pdfObject{dict: string(dict), stream: stream}
		objs[num] = o
		if typeOf(o.dict) == "ObjStm" {
			inner, err := readObjectStream(o)
			if err != nil {
				return nil, fmt.Errorf("object %d: %w", num, err)
			}
			for n, in := range inner {
				objs[n] = in
			}
		}
	}
	if len(objs) == 0 {
		return nil, fmt.Errorf("it holds no object")
	}
	return objs, nil
}

// decode gives the content of a stream, undone from its flate coding.
func decode(o pdfObject) ([]byte, error) {
	if !strings.Contains(o.dict, "/FlateDecode") {
		return o.stream, nil
	}
	zr, err := zlib.NewReader(bytes.NewReader(o.stream))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// readObjectStream reads the objects packed in an object stream: a header of
// pairs (number, offset), then the objects.
func readObjectStream(o pdfObject) (map[int]pdfObject, error) {
	n, first := 0, 0
	if m := countRe.FindStringSubmatch(o.dict); m != nil {
		n, _ = strconv.Atoi(m[1])
	}
	if m := firstRe.FindStringSubmatch(o.dict); m != nil {
		first, _ = strconv.Atoi(m[1])
	}
	data, err := decode(o)
	if err != nil {
		return nil, err
	}
	if n == 0 || first <= 0 || first > len(data) {
		return nil, fmt.Errorf("an object stream is damaged")
	}
	head := strings.Fields(string(data[:first]))
	if len(head) < 2*n {
		return nil, fmt.Errorf("an object stream header is damaged")
	}
	out := make(map[int]pdfObject, n)
	for i := 0; i < n; i++ {
		num, err1 := strconv.Atoi(head[2*i])
		off, err2 := strconv.Atoi(head[2*i+1])
		if err1 != nil || err2 != nil || first+off > len(data) {
			return nil, fmt.Errorf("an object stream header is damaged")
		}
		end := len(data)
		if i+1 < n {
			if next, err := strconv.Atoi(head[2*i+3]); err == nil && first+next <= len(data) {
				end = first + next
			}
		}
		out[num] = pdfObject{dict: string(data[first+off : end])}
	}
	return out, nil
}

// walkPages adds the pages under a node of the page tree, in order.
func walkPages(objs map[int]pdfObject, node int, inherited []float64, out *PDF, depth int) error {
	if depth > 16 {
		return fmt.Errorf("its page tree is too deep")
	}
	o, ok := objs[node]
	if !ok {
		return fmt.Errorf("a page of its tree is missing (object %d)", node)
	}
	box := inherited
	if m := mediaRe.FindStringSubmatch(o.dict); m != nil {
		v, err := numbers(m[1])
		if err != nil || len(v) != 4 {
			return fmt.Errorf("a media box is unreadable")
		}
		box = v
	}
	switch typeOf(o.dict) {
	case "Pages":
		k := kidsRe.FindStringSubmatch(o.dict)
		if k == nil {
			return fmt.Errorf("a node of its page tree has no children")
		}
		for _, r := range refRe.FindAllStringSubmatch(k[1], -1) {
			kid, _ := strconv.Atoi(r[1])
			if err := walkPages(objs, kid, box, out, depth+1); err != nil {
				return err
			}
		}
	case "Page":
		if box == nil {
			return fmt.Errorf("a page has no media box")
		}
		content, err := pageContent(objs, o)
		if err != nil {
			return err
		}
		out.Pages = append(out.Pages, PDFPage{WidthPt: box[2] - box[0], HeightPt: box[3] - box[1], Content: content})
	default:
		return fmt.Errorf("an unexpected object in its page tree (%d)", node)
	}
	return nil
}

func numbers(s string) ([]float64, error) {
	var v []float64
	for _, f := range strings.Fields(s) {
		x, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil, err
		}
		v = append(v, x)
	}
	return v, nil
}

// pageContent is the decoded content of a page: its stream, or its streams
// one after the other.
func pageContent(objs map[int]pdfObject, page pdfObject) (string, error) {
	m := contentRe.FindStringSubmatch(page.dict)
	if m == nil {
		return "", nil // a blank page
	}
	var sb strings.Builder
	for _, r := range refRe.FindAllStringSubmatch(m[1], -1) {
		n, _ := strconv.Atoi(r[1])
		o, ok := objs[n]
		if !ok {
			return "", fmt.Errorf("the content of a page is missing (object %d)", n)
		}
		data, err := decode(o)
		if err != nil {
			return "", fmt.Errorf("the content of a page cannot be decoded: %w", err)
		}
		sb.Write(data)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// ---- comparing the PDF with the plan --------------------------------------------------------------

const number = `(-?\d+(?:\.\d+)?)`

var (
	clipRe = regexp.MustCompile(number + ` ` + number + ` ` + number + ` ` + number + ` re W n`)
	fillRe = regexp.MustCompile(number + ` ` + number + ` ` + number + ` ` + number + ` re f`)
)

func floats(m []string) []float64 {
	v := make([]float64, len(m))
	for i, s := range m {
		v[i], _ = strconv.ParseFloat(s, 64)
	}
	return v
}

// Verify compares a PDF with the pages that were asked for (ARCHITECTURE §11,
// 8 and 9, and the clip of each page): as many pages as bands; each the size
// of the sheet, to 0.02 mm; each clipped to the window the plan gives, to the
// 1/256 point cairo writes; and, where the document has a rectangle that
// covers it, drawn at the offset the plan gives. It returns the failures.
//
// whitePage tells how the pages were drawn. With a white page the engine
// writes the clip of the band whole at the start of the drawing, whatever the
// document holds, so it must be in the file. Without one, cairo writes a clip
// only where an operation needs it: a document with no rectangle that covers it
// has, instead of the clip of the band, the clips of the elements that cross a
// cut, each cut down to the band. They are then checked one by one: none may
// leave the window of the band.
func Verify(pdf *PDF, specs []engine.PageSpec, docW, docH float64, whitePage bool) []diag.Item {
	var out []diag.Item
	if len(pdf.Pages) != len(specs) {
		return []diag.Item{diag.New("export.pdf_pages", 0, "got", len(pdf.Pages), "want", len(specs))}
	}
	const sizeTolMM = 0.02
	const geomTolPt = 1.0/256 + 0.001
	near := func(got, want float64) bool { return math.Abs(got-want) <= geomTolPt }

	for i, want := range specs {
		n := i + 1
		got := pdf.Pages[i]
		if math.Abs(units.PtToMM(got.WidthPt)-units.PtToMM(want.WidthPt)) > sizeTolMM ||
			math.Abs(units.PtToMM(got.HeightPt)-units.PtToMM(want.HeightPt)) > sizeTolMM {
			out = append(out, diag.New("export.pdf_size", n,
				"w", units.PtToMM(got.WidthPt), "h", units.PtToMM(got.HeightPt),
				"want_w", units.PtToMM(want.WidthPt), "want_h", units.PtToMM(want.HeightPt)))
			continue
		}
		wantW, wantH := docW*want.PtPerUnit, (want.BandBottom-want.BandTop)*want.PtPerUnit

		// The clip of the band, where it is written whole.
		after := -1 // where the content that the clip applies to begins
		var clips [][]float64
		for _, loc := range clipRe.FindAllStringSubmatchIndex(got.Content, -1) {
			c := floats(clipRe.FindStringSubmatch(got.Content[loc[0]:loc[1]])[1:])
			clips = append(clips, c)
			if after < 0 && near(c[0], want.OriginXPt) && near(c[1], want.OriginYPt) && near(c[2], wantW) && near(c[3], wantH) {
				after = loc[1]
			}
		}
		switch {
		case after >= 0:
		case whitePage:
			out = append(out, diag.New("export.pdf_clip", n))
			continue
		default:
			outside := false
			for _, c := range clips {
				if c[0] < want.OriginXPt-geomTolPt || c[1] < want.OriginYPt-geomTolPt ||
					c[0]+c[2] > want.OriginXPt+wantW+geomTolPt || c[1]+c[3] > want.OriginYPt+wantH+geomTolPt {
					outside = true
				}
			}
			if outside {
				out = append(out, diag.New("export.pdf_clip", n))
				continue
			}
			after = 0
		}

		// The offset: a rectangle that covers the whole document, if one is
		// painted, sits where the plan puts the document.
		for _, m := range fillRe.FindAllStringSubmatch(got.Content[after:], -1) {
			f := floats(m[1:])
			if math.Abs(f[2]-docW*want.PtPerUnit) > 0.01 || math.Abs(f[3]-docH*want.PtPerUnit) > 0.01 {
				continue
			}
			if !near(f[0], want.OriginXPt) || !near(f[1], want.OriginYPt-want.BandTop*want.PtPerUnit) {
				out = append(out, diag.New("export.pdf_offset", n))
			}
			break
		}
	}
	return out
}

// SPDX-License-Identifier: GPL-3.0-only

package export

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/gillesmagneville/image-print-layout/internal/diag"
	"github.com/gillesmagneville/image-print-layout/internal/layout"
)

// The HTML export (ARCHITECTURE §6, "HTML page"): the document in one file
// that a browser opens, with nothing to fetch. The picture is written once, as
// the background of a style rule; the page shows it whole, or in one piece for
// each page of the plan, each piece being a box that shows its band of the
// same picture. Everything is written with the numbers of the plan, and read
// back from the file to be compared with it, as the PDF is.

// HTMLSource is the picture a page carries: an SVG (the working copy, with
// the background the project asks for), or a picture a browser reads (a JPEG
// or a PNG as the file was, any other kind written as a PNG).
type HTMLSource struct {
	// MIME is the type of the data: image/svg+xml, image/jpeg or image/png.
	MIME string
	Data []byte
}

// HTMLOptions are the choices of the export and the words of the page.
type HTMLOptions struct {
	// Title is the title of the page: the name of the document.
	Title string
	// Lang is the language of the page, as a BCP 47 tag ("fr").
	Lang string
	// Generator says what wrote the page ("Image Print Layout 1.3.0").
	Generator string
	// Cut shows one piece for each page of the plan, one under the other,
	// instead of the whole picture in one piece.
	Cut bool
	// WhitePage paints the pieces white under the picture; otherwise a
	// transparent part of the picture shows the page of the browser.
	WhitePage bool
	// PageLabel is the caption of a piece, in the language of the page. Nil
	// writes "n / total".
	PageLabel func(n, total int) string
}

// MaxHTMLBytes is the largest page the export writes. A picture is written in
// base64, a third bigger than itself, and a browser opens a page of that size
// slowly; beyond it the export says so and writes nothing.
const MaxHTMLBytes = 256 << 20

// htmlPiece is a band of the picture shown in a box: from the top of the
// picture, with the height it has, in document units.
type htmlPiece struct {
	top, height float64
}

// htmlPieces are the pieces a page shows: the bands of the plan, or the
// picture whole.
func htmlPieces(pl *layout.Plan, cut bool) []htmlPiece {
	if !cut || len(pl.Pages) == 0 {
		return []htmlPiece{{0, pl.Doc.H}}
	}
	out := make([]htmlPiece, len(pl.Pages))
	for i, p := range pl.Pages {
		out[i] = htmlPiece{p.Top, p.Bottom - p.Top}
	}
	return out
}

// htmlNum writes a number for the style of a box: at most six decimals, no
// trailing zero, the same text for the same number on every machine.
func htmlNum(v float64) string {
	s := strconv.FormatFloat(v, 'f', 6, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

// pieceStyle is the style of the box of a piece: its shape (the aspect ratio
// is the band's width to its height, in document units), the size the whole
// picture has inside it (the width of the box, and as high as the picture is
// high in proportion to the band), and which part of the picture is at the top
// of the box. A percentage of a position is a share of the box minus the
// picture, which is negative: a band starting at top, of height h in a picture
// of height H, has the position top / (H - h).
func pieceStyle(docW, docH float64, p htmlPiece) string {
	size := htmlNum(docH / p.height * 100)
	position := "0"
	if gap := docH - p.height; gap > 1e-9 {
		position = htmlNum(p.top / gap * 100)
	}
	return "aspect-ratio:" + htmlNum(docW) + " / " + htmlNum(p.height) +
		";background-size:100% " + size + "%;background-position:0 " + position + "%"
}

// WriteHTML writes the page. It is deterministic: the same plan, picture and
// options give the same bytes.
func WriteHTML(w io.Writer, pl *layout.Plan, src HTMLSource, opt HTMLOptions) error {
	if pl == nil || len(pl.Pages) == 0 {
		return fmt.Errorf("export: no page to write")
	}
	if len(src.Data) == 0 || src.MIME == "" {
		return fmt.Errorf("export: no picture to write")
	}
	if n := base64.StdEncoding.EncodedLen(len(src.Data)); n > MaxHTMLBytes {
		return diag.Fail("export.html_too_large", "mb", n>>20, "max", MaxHTMLBytes>>20)
	}
	label := opt.PageLabel
	if label == nil {
		label = func(n, total int) string { return fmt.Sprintf("%d / %d", n, total) }
	}
	lang := opt.Lang
	if lang == "" {
		lang = "en"
	}
	pieces := htmlPieces(pl, opt.Cut)
	docW, docH := pl.Doc.W, pl.Doc.H

	var b bytes.Buffer
	b.WriteString("<!DOCTYPE html>\n<html lang=\"" + html.EscapeString(lang) + "\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	if opt.Generator != "" {
		b.WriteString("<meta name=\"generator\" content=\"" + html.EscapeString(opt.Generator) + "\">\n")
	}
	b.WriteString("<title>" + html.EscapeString(opt.Title) + "</title>\n<style>\n")
	b.WriteString(htmlStyle(pl.ContentWidthMM))
	// The picture, once.
	b.WriteString(".doc{background-image:url(\"data:" + src.MIME + ";base64,")
	enc := base64.NewEncoder(base64.StdEncoding, &b)
	if _, err := enc.Write(src.Data); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	b.WriteString("\")}\n</style>\n</head>\n<body>\n<main>\n")

	class := "piece doc"
	if opt.WhitePage {
		class += " white"
	}
	for i, p := range pieces {
		name := opt.Title
		if opt.Cut {
			caption := label(i+1, len(pieces))
			b.WriteString("<section class=\"page\">\n<p class=\"label\">" + html.EscapeString(caption) + "</p>\n")
			name = opt.Title + " — " + caption
		}
		b.WriteString("<div class=\"" + class + "\" role=\"img\" aria-label=\"" + html.EscapeString(name) +
			"\" style=\"" + pieceStyle(docW, docH, p) + "\"></div>\n")
		if opt.Cut {
			b.WriteString("</section>\n")
		}
	}
	b.WriteString("</main>\n</body>\n</html>\n")
	_, err := w.Write(b.Bytes())
	return err
}

// htmlStyle is the style of the page, apart from the picture. The picture is
// as wide as the plan says (in millimetres, which a screen shows at the size
// of paper when the window is wide enough) and shrinks with a narrow window.
func htmlStyle(widthMM float64) string {
	return `*{box-sizing:border-box}
html{background:#d9d9de}
body{margin:0;padding:16px;font:14px/1.4 system-ui,sans-serif;color:#333}
main{width:min(100%,` + htmlNum(widthMM) + `mm);margin:0 auto}
.page+.page{margin-top:28px}
.label{margin:0 0 6px;color:#666;font-size:12px}
.piece{display:block;width:100%;background-repeat:no-repeat;image-orientation:from-image}
.white{background-color:#fff}
@media (prefers-color-scheme:dark){html{background:#2b2b30}.label{color:#aaa}}
@media print{html{background:none}body{padding:0}main{width:100%}.label{display:none}.page{break-inside:avoid;break-after:page}.page:last-child{break-after:auto}}
`
}

// ---- reading the page back ---------------------------------------------------------------

// htmlBox is a box of the page as the file describes it.
type htmlBox struct {
	ratioW, ratioH float64
	size, position float64
}

var (
	htmlBoxRe   = regexp.MustCompile(`<div class="piece doc[^"]*"[^>]*style="aspect-ratio:([0-9.]+) / ([0-9.]+);background-size:100% ([0-9.]+)%;background-position:0 ([0-9.]+)%"></div>`)
	htmlWidthRe = regexp.MustCompile(`main\{width:min\(100%,([0-9.]+)mm\)`)
	htmlDataRe  = regexp.MustCompile(`\.doc\{background-image:url\("data:([^;"]+);base64,([A-Za-z0-9+/=]*)"\)\}`)
)

// VerifyHTML reads a page written by WriteHTML and compares what the file says
// with the plan: as many pieces as the plan asks, each showing the band of its
// page, at the width of the plan, over the picture it was given.
func VerifyHTML(data []byte, pl *layout.Plan, src HTMLSource, cut bool) []diag.Item {
	var out []diag.Item
	want := htmlPieces(pl, cut)
	boxes := htmlBoxRe.FindAllSubmatch(data, -1)
	if len(boxes) != len(want) {
		return []diag.Item{diag.New("export.html_pieces", 0, "got", len(boxes), "want", len(want))}
	}
	// The picture is there once, and is the one given.
	if all := htmlDataRe.FindAllSubmatch(data, -1); len(all) != 1 {
		out = append(out, diag.New("export.html_picture", 0))
	} else if dec, err := base64.StdEncoding.DecodeString(string(all[0][2])); err != nil || string(all[0][1]) != src.MIME || !bytes.Equal(dec, src.Data) {
		out = append(out, diag.New("export.html_picture", 0))
	}
	// The width is the one of the plan.
	wantW := pl.ContentWidthMM
	if m := htmlWidthRe.FindSubmatch(data); m == nil || !closeTo(parseHTMLNum(m[1]), wantW, 1e-5*math.Max(1, wantW)) {
		got := math.NaN()
		if m != nil {
			got = parseHTMLNum(m[1])
		}
		out = append(out, diag.New("export.html_width", 0, "w", got, "want", wantW))
	}
	// Each box shows the band of its page.
	for i, m := range boxes {
		rw, rh := parseHTMLNum(m[1]), parseHTMLNum(m[2])
		size, position := parseHTMLNum(m[3]), parseHTMLNum(m[4])
		height := rh
		docH := size / 100 * height
		top := 0.0
		if gap := docH - height; gap > 1e-9 {
			top = position / 100 * gap
		}
		tol := 1e-3 * math.Max(1, pl.Doc.H/1000)
		if !closeTo(rw, pl.Doc.W, tol) || !closeTo(height, want[i].height, tol) ||
			!closeTo(docH, pl.Doc.H, tol) || !closeTo(top, want[i].top, tol) {
			out = append(out, diag.New("export.html_band", i+1))
		}
	}
	return out
}

func parseHTMLNum(b []byte) float64 {
	v, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return math.NaN()
	}
	return v
}

func closeTo(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

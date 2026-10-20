// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package engine

/*
#cgo pkg-config: librsvg-2.0 cairo fontconfig
#include <stdint.h>
#include <stdlib.h>
#include <cairo.h>
#include <cairo-pdf.h>
#include <string.h>
#include <gio/gio.h>
#include <fontconfig/fontconfig.h>
#include <librsvg/rsvg.h>

// Implemented in Go (goCairoWrite below). The declaration must match the one
// cgo generates, hence the non-const data pointer; the cairo callback type is
// reached through an unsafe.Pointer conversion.
extern cairo_status_t goCairoWrite(void *closure, unsigned char *data, unsigned int length);

// librsvg's version numbers are variables exported by the library. They are
// read here, in C, and not through cgo's own access to C variables: on
// Windows, where the library is a DLL, that access reads the import thunk
// instead of the value and returns garbage.
static void svgpl_free(gpointer p) { free(p); }

// svgpl_load reads a document from memory. The data is copied into a stream
// that owns it; base, when not NULL, is the file the document came from, which
// relative links are resolved against. The size limits of the XML parser are
// lifted: a large diagram is not an attack, it is the use case.
static RsvgHandle *svgpl_load(const guint8 *data, gsize len, const char *path, GError **err) {
	guint8 *copy = malloc(len);
	if (copy == NULL) {
		return NULL;
	}
	memcpy(copy, data, len);
	GInputStream *stream = g_memory_input_stream_new_from_data(copy, (gssize)len, svgpl_free);
	GFile *base = path != NULL ? g_file_new_for_path(path) : NULL;
	RsvgHandle *h = rsvg_handle_new_from_stream_sync(stream, base, RSVG_HANDLE_FLAG_UNLIMITED, NULL, err);
	if (base != NULL) {
		g_object_unref(base);
	}
	g_object_unref(stream);
	return h;
}

// svgpl_match_family asks fontconfig which family it would use for a requested
// one, with the substitutions of its configuration. The result is freed by the
// caller, or is NULL.
static char *svgpl_match_family(const char *family) {
	FcPattern *pat = FcPatternCreate();
	if (pat == NULL) {
		return NULL;
	}
	FcPatternAddString(pat, FC_FAMILY, (const FcChar8 *)family);
	FcConfigSubstitute(NULL, pat, FcMatchPattern);
	FcDefaultSubstitute(pat);
	FcResult res;
	FcPattern *match = FcFontMatch(NULL, pat, &res);
	char *out = NULL;
	if (match != NULL) {
		FcChar8 *name = NULL;
		if (FcPatternGetString(match, FC_FAMILY, 0, &name) == FcResultMatch) {
			out = strdup((const char *)name);
		}
		FcPatternDestroy(match);
	}
	FcPatternDestroy(pat);
	return out;
}

static unsigned int svgpl_rsvg_version(int part) {
	switch (part) {
	case 0:
		return rsvg_major_version;
	case 1:
		return rsvg_minor_version;
	default:
		return rsvg_micro_version;
	}
}
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"runtime"
	"runtime/cgo"
	"unsafe"
)

// Doc is a loaded SVG document. It must be closed.
type Doc struct {
	handle *C.RsvgHandle
	w, h   float64
}

// Load parses an SVG document. path is the file it was read from, or empty:
// the images it links to are looked for next to it. It returns an explicit
// error when librsvg rejects the data or when the document has no usable size.
func Load(svg []byte, path string) (*Doc, error) {
	if len(svg) == 0 {
		return nil, errors.New("engine: empty SVG data")
	}
	var cpath *C.char
	if path != "" {
		cpath = C.CString(path)
		defer C.free(unsafe.Pointer(cpath))
	}
	var gerr *C.GError
	h := C.svgpl_load((*C.guint8)(unsafe.Pointer(&svg[0])), C.gsize(len(svg)), cpath, &gerr)
	if h == nil {
		return nil, fmt.Errorf("engine: cannot read the SVG: %s", takeGError(gerr))
	}
	C.rsvg_handle_set_dpi(h, C.double(Dpi))

	var w, ht C.gdouble
	if C.rsvg_handle_get_intrinsic_size_in_pixels(h, &w, &ht) == 0 || w <= 0 || ht <= 0 {
		C.g_object_unref(C.gpointer(unsafe.Pointer(h)))
		return nil, errors.New("engine: the SVG has no usable width and height (and no viewBox to derive them from)")
	}
	d := &Doc{handle: h, w: float64(w), h: float64(ht)}
	runtime.SetFinalizer(d, func(d *Doc) { d.Close() })
	return d, nil
}

// LoadDocument is Load as the Loader type of the package.
func LoadDocument(svg []byte, path string) (Document, error) {
	d, err := Load(svg, path)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// Close releases the document. It is safe to call more than once.
func (d *Doc) Close() {
	if d.handle != nil {
		C.g_object_unref(C.gpointer(unsafe.Pointer(d.handle)))
		d.handle = nil
	}
	runtime.SetFinalizer(d, nil)
}

// Size returns the document size in user units, as librsvg resolves it at Dpi.
func (d *Doc) Size() (w, h float64) { return d.w, d.h }

func takeGError(e *C.GError) string {
	if e == nil {
		return "unknown error"
	}
	msg := C.GoString(e.message)
	C.g_error_free(e)
	return msg
}

func (d *Doc) viewport() C.RsvgRectangle {
	return C.RsvgRectangle{x: 0, y: 0, width: C.double(d.w), height: C.double(d.h)}
}

// Measure returns the ink box of the element with the given id, in document
// units, as it sits in the document (ancestors' transformations applied).
//
// It uses rsvg_handle_get_geometry_for_layer with the whole document as
// viewport. rsvg_handle_get_geometry_for_element would measure the element as
// if extracted alone under an identity transformation, which is not its
// position in the drawing.
func (d *Doc) Measure(id string) (Box, error) {
	cid := C.CString("#" + id)
	defer C.free(unsafe.Pointer(cid))
	vp := d.viewport()
	var ink, logical C.RsvgRectangle
	var gerr *C.GError
	if C.rsvg_handle_get_geometry_for_layer(d.handle, cid, &vp, &ink, &logical, &gerr) == 0 {
		return Box{}, fmt.Errorf("engine: cannot measure #%s: %s", id, takeGError(gerr))
	}
	return Box{float64(ink.x), float64(ink.y), float64(ink.width), float64(ink.height)}, nil
}

// MeasureBlocks measures many elements; the ones that cannot be measured are
// left out (an empty group, an identifier the engine cannot address).
func (d *Doc) MeasureBlocks(ids []string) map[string]Box {
	out := make(map[string]Box, len(ids))
	for _, id := range ids {
		b, err := d.Measure(id)
		if err != nil || math.IsNaN(b.Y) || math.IsNaN(b.H) || math.IsInf(b.Y, 0) || math.IsInf(b.H, 0) {
			continue
		}
		out[id] = b
	}
	return out
}

var _ Document = (*Doc)(nil)

// MatchFont asks fontconfig, which the text of the drawings goes through,
// which installed family it gives for the requested one. It is the name asked
// for when that family is installed, and another one when it is substituted
// (the substitution is the one the export would make, silently). It returns
// the empty string when fontconfig cannot say.
func MatchFont(family string) string {
	cf := C.CString(family)
	defer C.free(unsafe.Pointer(cf))
	m := C.svgpl_match_family(cf)
	if m == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(m))
	return C.GoString(m)
}

func cairoError(op string, s C.cairo_status_t) error {
	return fmt.Errorf("engine: cairo failed during %s: %s", op, C.GoString(C.cairo_status_to_string(s)))
}

// Render draws the window win (document units) at pxPerUnit pixels per unit
// and returns it as an image. A nil background leaves the pixels transparent.
func (d *Doc) Render(win Box, pxPerUnit float64, background color.Color) (*image.RGBA, error) {
	wpx, hpx := int(math.Ceil(win.W*pxPerUnit)), int(math.Ceil(win.H*pxPerUnit))
	if wpx <= 0 || hpx <= 0 || pxPerUnit <= 0 {
		return nil, errors.New("engine: empty render window")
	}
	surf := C.cairo_image_surface_create(C.CAIRO_FORMAT_ARGB32, C.int(wpx), C.int(hpx))
	defer C.cairo_surface_destroy(surf)
	if s := C.cairo_surface_status(surf); s != C.CAIRO_STATUS_SUCCESS {
		return nil, cairoError("image surface creation", s)
	}
	cr := C.cairo_create(surf)
	defer C.cairo_destroy(cr)

	if background != nil {
		r, g, b, a := background.RGBA()
		C.cairo_set_source_rgba(cr, C.double(r)/65535, C.double(g)/65535, C.double(b)/65535, C.double(a)/65535)
		C.cairo_set_operator(cr, C.CAIRO_OPERATOR_SOURCE)
		C.cairo_paint(cr)
		C.cairo_set_operator(cr, C.CAIRO_OPERATOR_OVER)
	}
	C.cairo_scale(cr, C.double(pxPerUnit), C.double(pxPerUnit))
	C.cairo_translate(cr, C.double(-win.X), C.double(-win.Y))

	vp := d.viewport()
	var gerr *C.GError
	if C.rsvg_handle_render_document(d.handle, cr, &vp, &gerr) == 0 {
		return nil, fmt.Errorf("engine: cannot render the SVG: %s", takeGError(gerr))
	}
	C.cairo_surface_flush(surf)

	return copyImage(surf, wpx, hpx), nil
}

// copyImage reads an ARGB32 image surface into an image. cairo's pixels are
// premultiplied and native-endian: bytes B, G, R, A on little-endian machines.
// image.RGBA is premultiplied too, R, G, B, A.
func copyImage(surf *C.cairo_surface_t, wpx, hpx int) *image.RGBA {
	stride := int(C.cairo_image_surface_get_stride(surf))
	src := unsafe.Slice((*byte)(unsafe.Pointer(C.cairo_image_surface_get_data(surf))), stride*hpx)
	img := image.NewRGBA(image.Rect(0, 0, wpx, hpx))
	for y := 0; y < hpx; y++ {
		in := src[y*stride : y*stride+wpx*4]
		out := img.Pix[y*img.Stride : y*img.Stride+wpx*4]
		for x := 0; x < wpx*4; x += 4 {
			out[x], out[x+1], out[x+2], out[x+3] = in[x+2], in[x+1], in[x], in[x+3]
		}
	}
	return img
}

// drawPage draws one sheet on cr: the page background, then the band of the
// document under its clip. The PDF and the preview of a sheet both call it, so
// that what is previewed is what is exported.
func (d *Doc) drawPage(cr *C.cairo_t, p PageSpec, whitePage bool) error {
	if p.BandBottom <= p.BandTop || p.PtPerUnit <= 0 {
		return errors.New("the band is empty or the scale is not positive")
	}
	if whitePage {
		C.cairo_set_source_rgb(cr, 1, 1, 1)
		C.cairo_paint(cr)
	}
	vp := d.viewport()
	// The band rectangle is the clip: it is what materialises the cut.
	C.cairo_save(cr)
	C.cairo_rectangle(cr, C.double(p.OriginXPt), C.double(p.OriginYPt),
		C.double(d.w*p.PtPerUnit), C.double((p.BandBottom-p.BandTop)*p.PtPerUnit))
	C.cairo_clip(cr)
	if whitePage {
		// White on the white of the sheet changes nothing to the picture. It is
		// there for the PDF: cairo writes a clip only for an operation that
		// needs it, so a document with no rectangle that covers it (its
		// background was removed) would leave no trace of the band in the file,
		// and export.Verify could not read the cut there. This operation is as
		// large as the band, so the clip is written whole, first.
		C.cairo_set_source_rgb(cr, 1, 1, 1)
		C.cairo_paint(cr)
	}
	C.cairo_translate(cr, C.double(p.OriginXPt), C.double(p.OriginYPt-p.BandTop*p.PtPerUnit))
	C.cairo_scale(cr, C.double(p.PtPerUnit), C.double(p.PtPerUnit))
	var gerr *C.GError
	if C.rsvg_handle_render_document(d.handle, cr, &vp, &gerr) == 0 {
		C.cairo_restore(cr)
		return fmt.Errorf("cannot render the SVG: %s", takeGError(gerr))
	}
	C.cairo_restore(cr)
	return nil
}

// RenderPage draws a sheet as an image, pxPerPt pixels to the point, with the
// function that writes the PDF. A transparent page stays transparent.
func (d *Doc) RenderPage(p PageSpec, pxPerPt float64, whitePage bool) (*image.RGBA, error) {
	wpx, hpx := int(math.Ceil(p.WidthPt*pxPerPt)), int(math.Ceil(p.HeightPt*pxPerPt))
	if wpx <= 0 || hpx <= 0 || pxPerPt <= 0 {
		return nil, errors.New("engine: empty sheet")
	}
	surf := C.cairo_image_surface_create(C.CAIRO_FORMAT_ARGB32, C.int(wpx), C.int(hpx))
	defer C.cairo_surface_destroy(surf)
	if s := C.cairo_surface_status(surf); s != C.CAIRO_STATUS_SUCCESS {
		return nil, cairoError("image surface creation", s)
	}
	cr := C.cairo_create(surf)
	defer C.cairo_destroy(cr)
	C.cairo_scale(cr, C.double(pxPerPt), C.double(pxPerPt))
	if err := d.drawPage(cr, p, whitePage); err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	C.cairo_surface_flush(surf)
	return copyImage(surf, wpx, hpx), nil
}

type pdfSink struct {
	w   io.Writer
	err error
}

//export goCairoWrite
func goCairoWrite(closure unsafe.Pointer, data *C.uchar, length C.uint) C.cairo_status_t {
	sink := cgo.Handle(*(*C.uintptr_t)(closure)).Value().(*pdfSink)
	if sink.err != nil {
		return C.CAIRO_STATUS_WRITE_ERROR
	}
	if _, err := sink.w.Write(unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length))); err != nil {
		sink.err = err
		return C.CAIRO_STATUS_WRITE_ERROR
	}
	return C.CAIRO_STATUS_SUCCESS
}

// ExportPDF writes one PDF page per PageSpec. Every page draws the whole
// document again, translated to its band and clipped to the band's
// rectangle: a page is the same drawing windowed, never a geometrically cut
// copy, and cairo drops the operations that fall entirely outside the clip.
//
// The document is deliberately not recorded once into a cairo recording
// surface and painted on every page: cairo's PDF backend then emits that
// surface as a single form whose extent is that of its first use, so every
// page after the first would lose the part of the drawing its predecessor's
// clip did not need (measured with cairo 1.18, see docs/M0.md).
func (d *Doc) ExportPDF(out io.Writer, pages []PageSpec, opt PDFOptions) error {
	if len(pages) == 0 {
		return errors.New("engine: no page to export")
	}
	if opt.FixedDate == "" {
		opt.FixedDate = DefaultFixedDate
	}

	sink := &pdfSink{w: out}
	handle := cgo.NewHandle(sink)
	defer handle.Delete()
	slot := (*C.uintptr_t)(C.malloc(C.size_t(unsafe.Sizeof(C.uintptr_t(0)))))
	*slot = C.uintptr_t(handle)
	defer C.free(unsafe.Pointer(slot))

	pdf := C.cairo_pdf_surface_create_for_stream(
		(C.cairo_write_func_t)(unsafe.Pointer(C.goCairoWrite)), unsafe.Pointer(slot),
		C.double(pages[0].WidthPt), C.double(pages[0].HeightPt))
	if s := C.cairo_surface_status(pdf); s != C.CAIRO_STATUS_SUCCESS {
		C.cairo_surface_destroy(pdf)
		return cairoError("PDF surface creation", s)
	}
	date := C.CString(opt.FixedDate)
	defer C.free(unsafe.Pointer(date))
	C.cairo_pdf_surface_set_metadata(pdf, C.CAIRO_PDF_METADATA_CREATE_DATE, date)
	C.cairo_pdf_surface_set_metadata(pdf, C.CAIRO_PDF_METADATA_MOD_DATE, date)

	for i, p := range pages {
		if p.BandBottom <= p.BandTop || p.PtPerUnit <= 0 {
			C.cairo_surface_destroy(pdf)
			return fmt.Errorf("engine: page %d has an empty band or a non-positive scale", i+1)
		}
		C.cairo_pdf_surface_set_size(pdf, C.double(p.WidthPt), C.double(p.HeightPt))
		cr := C.cairo_create(pdf)
		if err := d.drawPage(cr, p, opt.WhitePage); err != nil {
			C.cairo_destroy(cr)
			C.cairo_surface_destroy(pdf)
			return fmt.Errorf("engine: page %d: %w", i+1, err)
		}
		C.cairo_show_page(cr)
		status := C.cairo_status(cr)
		C.cairo_destroy(cr)
		if status != C.CAIRO_STATUS_SUCCESS {
			C.cairo_surface_destroy(pdf)
			return cairoError(fmt.Sprintf("drawing page %d", i+1), status)
		}
		if opt.Progress != nil && !opt.Progress(i+1, len(pages)) {
			C.cairo_surface_destroy(pdf)
			return ErrCancelled
		}
	}

	C.cairo_surface_finish(pdf)
	status := C.cairo_surface_status(pdf)
	C.cairo_surface_destroy(pdf)
	if sink.err != nil {
		return fmt.Errorf("engine: cannot write the PDF: %w", sink.err)
	}
	if status != C.CAIRO_STATUS_SUCCESS {
		return cairoError("PDF finishing", status)
	}
	return nil
}

// Versions returns the librsvg and cairo versions the program runs with:
// byte-identical PDFs are only promised for identical versions.
func Versions() (librsvg, cairo string) {
	return fmt.Sprintf("%d.%d.%d", C.svgpl_rsvg_version(0), C.svgpl_rsvg_version(1), C.svgpl_rsvg_version(2)),
		C.GoString(C.cairo_version_string())
}

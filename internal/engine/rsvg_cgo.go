// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package engine

/*
#cgo pkg-config: librsvg-2.0 cairo
#include <stdint.h>
#include <stdlib.h>
#include <cairo.h>
#include <cairo-pdf.h>
#include <librsvg/rsvg.h>

// Implemented in Go (goCairoWrite below). The declaration must match the one
// cgo generates, hence the non-const data pointer; the cairo callback type is
// reached through an unsafe.Pointer conversion.
extern cairo_status_t goCairoWrite(void *closure, unsigned char *data, unsigned int length);
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

// Dpi is the resolution librsvg uses to turn physical SVG lengths (mm, in,
// pt...) into user units. SVG and CSS say 96; librsvg's own default is not
// guaranteed to be the same, so it is always set explicitly.
const Dpi = 96.0

// Box is a rectangle in document units (SVG user units, origin top left).
type Box struct {
	X, Y, W, H float64
}

// Doc is a loaded SVG document. It must be closed.
type Doc struct {
	handle *C.RsvgHandle
	w, h   float64
}

// Load parses an SVG document. It returns an explicit error when librsvg
// rejects the data or when the document has no usable size.
func Load(svg []byte) (*Doc, error) {
	if len(svg) == 0 {
		return nil, errors.New("engine: empty SVG data")
	}
	var gerr *C.GError
	h := C.rsvg_handle_new_from_data((*C.guint8)(unsafe.Pointer(&svg[0])), C.gsize(len(svg)), &gerr)
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

	// cairo ARGB32 is premultiplied, native-endian: bytes B, G, R, A on
	// little-endian machines. image.RGBA is premultiplied too, R, G, B, A.
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
	return img, nil
}

// PageSpec places one band of the document on one sheet. Sizes are in PDF
// points; the band is in document units.
type PageSpec struct {
	WidthPt, HeightPt    float64 // the sheet
	BandTop, BandBottom  float64 // the part of the document drawn on it
	OriginXPt, OriginYPt float64 // where the band's top-left corner lands
	PtPerUnit            float64 // the single global scale
}

// PDFOptions are the export settings.
type PDFOptions struct {
	// WhitePage paints an opaque white rectangle the size of each sheet
	// before the drawing.
	WhitePage bool
	// FixedDate is written as the creation and modification date, so that two
	// exports of the same input are identical. ISO 8601.
	FixedDate string
}

// DefaultFixedDate is the date written in the PDF metadata.
const DefaultFixedDate = "2000-01-01T00:00:00Z"

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

	vp := d.viewport()
	for i, p := range pages {
		if p.BandBottom <= p.BandTop || p.PtPerUnit <= 0 {
			C.cairo_surface_destroy(pdf)
			return fmt.Errorf("engine: page %d has an empty band or a non-positive scale", i+1)
		}
		C.cairo_pdf_surface_set_size(pdf, C.double(p.WidthPt), C.double(p.HeightPt))
		cr := C.cairo_create(pdf)
		if opt.WhitePage {
			C.cairo_set_source_rgb(cr, 1, 1, 1)
			C.cairo_paint(cr)
		}
		// The band rectangle is the clip: it is what materialises the cut.
		C.cairo_save(cr)
		C.cairo_rectangle(cr, C.double(p.OriginXPt), C.double(p.OriginYPt),
			C.double(d.w*p.PtPerUnit), C.double((p.BandBottom-p.BandTop)*p.PtPerUnit))
		C.cairo_clip(cr)
		C.cairo_translate(cr, C.double(p.OriginXPt), C.double(p.OriginYPt-p.BandTop*p.PtPerUnit))
		C.cairo_scale(cr, C.double(p.PtPerUnit), C.double(p.PtPerUnit))
		var gerr *C.GError
		ok := C.rsvg_handle_render_document(d.handle, cr, &vp, &gerr)
		if ok == 0 {
			C.cairo_destroy(cr)
			C.cairo_surface_destroy(pdf)
			return fmt.Errorf("engine: cannot render the SVG on page %d: %s", i+1, takeGError(gerr))
		}
		C.cairo_restore(cr)
		C.cairo_show_page(cr)
		status := C.cairo_status(cr)
		C.cairo_destroy(cr)
		if status != C.CAIRO_STATUS_SUCCESS {
			C.cairo_surface_destroy(pdf)
			return cairoError(fmt.Sprintf("drawing page %d", i+1), status)
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
	return fmt.Sprintf("%d.%d.%d", C.rsvg_major_version, C.rsvg_minor_version, C.rsvg_micro_version),
		C.GoString(C.cairo_version_string())
}

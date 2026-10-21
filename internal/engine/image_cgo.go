// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package engine

/*
#cgo pkg-config: gdk-pixbuf-2.0 cairo
#include <stdint.h>
#include <string.h>
#include <cairo.h>
#include <gdk-pixbuf/gdk-pixbuf.h>

// What the loader learns before it has decoded anything: the size an image
// announces in its header. An image whose size is over the limit is dropped
// there, before the decoder has allocated the memory for its pixels.
typedef struct {
	gint64 maxpix;
	gint w, h;
	gboolean too_large;
} ipl_probe;

static void ipl_size_prepared(GdkPixbufLoader *loader, gint w, gint h, gpointer data) {
	ipl_probe *p = (ipl_probe *)data;
	p->w = w;
	p->h = h;
	if ((gint64)w * (gint64)h > p->maxpix) {
		p->too_large = TRUE;
	}
}

// ipl_result is what ipl_load gives back. pixbuf is a new reference.
typedef struct {
	GdkPixbuf *pixbuf;
	char *format;
	gboolean animated;
	gboolean unknown_type;
	ipl_probe probe;
} ipl_result;

// ipl_load decodes an image held in memory, in pieces, so that the size it
// announces can be checked first. It returns FALSE with err set when the data
// cannot be read, and FALSE with res->probe.too_large set when the image is
// larger than maxpix pixels.
static gboolean ipl_load(const guchar *data, gsize len, gint64 maxpix, ipl_result *res, GError **err) {
	memset(res, 0, sizeof(*res));
	res->probe.maxpix = maxpix;
	GdkPixbufLoader *loader = gdk_pixbuf_loader_new();
	g_signal_connect(loader, "size-prepared", G_CALLBACK(ipl_size_prepared), &res->probe);
	gboolean ok = TRUE;
	gsize off = 0;
	while (off < len && ok) {
		gsize n = len - off > 65536 ? 65536 : len - off;
		ok = gdk_pixbuf_loader_write(loader, data + off, n, err);
		off += n;
		if (res->probe.too_large) {
			break;
		}
	}
	if (res->probe.too_large || !ok) {
		if (!ok && *err != NULL && (*err)->domain == GDK_PIXBUF_ERROR &&
		    ((*err)->code == GDK_PIXBUF_ERROR_UNKNOWN_TYPE || (*err)->code == GDK_PIXBUF_ERROR_UNSUPPORTED_OPERATION)) {
			res->unknown_type = TRUE;
		}
		gdk_pixbuf_loader_close(loader, NULL);
		g_object_unref(loader);
		return FALSE;
	}
	if (!gdk_pixbuf_loader_close(loader, err)) {
		g_object_unref(loader);
		return FALSE;
	}
	GdkPixbuf *pb = gdk_pixbuf_loader_get_pixbuf(loader);
	if (pb == NULL) {
		g_set_error_literal(err, GDK_PIXBUF_ERROR, GDK_PIXBUF_ERROR_FAILED, "the file holds no picture");
		g_object_unref(loader);
		return FALSE;
	}
	res->pixbuf = g_object_ref(pb);
	GdkPixbufFormat *f = gdk_pixbuf_loader_get_format(loader);
	if (f != NULL) {
		res->format = gdk_pixbuf_format_get_name(f);
	}
	GdkPixbufAnimation *an = gdk_pixbuf_loader_get_animation(loader);
	res->animated = an != NULL && !gdk_pixbuf_animation_is_static_image(an);
	g_object_unref(loader);
	return TRUE;
}

static const char *ipl_option(GdkPixbuf *pb, const char *key) {
	return gdk_pixbuf_get_option(pb, key);
}

static void ipl_free(gpointer p) { g_free(p); }

// ipl_surface turns the pixels of a pixbuf into a cairo image surface: opaque
// (RGB24) when the picture has no transparent pixel, so that the PDF has no
// mask to write, premultiplied ARGB32 when it has. A pixbuf can have an alpha
// channel that is opaque everywhere (the TIFF loader always does): the picture
// is then opaque, and *transparent says so. The pixbuf is not kept.
static cairo_surface_t *ipl_surface(GdkPixbuf *pb, gboolean *transparent) {
	int w = gdk_pixbuf_get_width(pb), h = gdk_pixbuf_get_height(pb);
	int n = gdk_pixbuf_get_n_channels(pb);
	int rowstride = gdk_pixbuf_get_rowstride(pb);
	const guchar *src = gdk_pixbuf_read_pixels(pb);
	gboolean alpha = gdk_pixbuf_get_has_alpha(pb) && n == 4;
	*transparent = FALSE;
	if (alpha) {
		for (int y = 0; y < h && !*transparent; y++) {
			const guchar *in = src + (gsize)y * rowstride + 3;
			for (int x = 0; x < w; x++, in += 4) {
				if (*in != 255) {
					*transparent = TRUE;
					break;
				}
			}
		}
	}
	cairo_surface_t *s = cairo_image_surface_create(*transparent ? CAIRO_FORMAT_ARGB32 : CAIRO_FORMAT_RGB24, w, h);
	if (cairo_surface_status(s) != CAIRO_STATUS_SUCCESS) {
		return s;
	}
	cairo_surface_flush(s);
	guchar *dst = cairo_image_surface_get_data(s);
	int stride = cairo_image_surface_get_stride(s);
	for (int y = 0; y < h; y++) {
		const guchar *in = src + (gsize)y * rowstride;
		uint32_t *out = (uint32_t *)(dst + (gsize)y * stride);
		if (*transparent) {
			for (int x = 0; x < w; x++, in += 4) {
				uint32_t a = in[3];
				uint32_t r = in[0] * a + 128, g = in[1] * a + 128, b = in[2] * a + 128;
				r = (r + (r >> 8)) >> 8;
				g = (g + (g >> 8)) >> 8;
				b = (b + (b >> 8)) >> 8;
				out[x] = (a << 24) | (r << 16) | (g << 8) | b;
			}
		} else if (n >= 3) {
			for (int x = 0; x < w; x++, in += n) {
				out[x] = 0xFF000000u | ((uint32_t)in[0] << 16) | ((uint32_t)in[1] << 8) | in[2];
			}
		}
	}
	cairo_surface_mark_dirty(s);
	return s;
}

// ipl_attach_jpeg gives the surface the JPEG file it was decoded from. The PDF
// writer then embeds the file as it is, instead of the pixels: no second
// compression, no loss, and a file the size of the original.
static cairo_status_t ipl_attach_jpeg(cairo_surface_t *s, const guchar *data, gsize len) {
	guchar *copy = g_malloc(len);
	memcpy(copy, data, len);
	cairo_status_t st = cairo_surface_set_mime_data(s, CAIRO_MIME_TYPE_JPEG, copy, len, ipl_free, copy);
	if (st != CAIRO_STATUS_SUCCESS) {
		g_free(copy);
	}
	return st;
}

// ipl_format_names lists, separated by commas, the formats this installation of
// gdk-pixbuf can read. The result is freed with g_free.
static char *ipl_format_names(void) {
	GSList *formats = gdk_pixbuf_get_formats();
	GString *out = g_string_new("");
	for (GSList *l = formats; l != NULL; l = l->next) {
		GdkPixbufFormat *f = (GdkPixbufFormat *)l->data;
		if (gdk_pixbuf_format_is_disabled(f)) {
			continue;
		}
		gchar *name = gdk_pixbuf_format_get_name(f);
		if (out->len > 0) {
			g_string_append_c(out, ',');
		}
		g_string_append(out, name);
		g_free(name);
	}
	g_slist_free(formats);
	return g_string_free(out, FALSE);
}
// TEMP-DEBUG
static char *ipl_dump(GdkPixbuf *pb) {
	GString *s = g_string_new("");
	g_string_append_printf(s, "size=%dx%d n=%d alpha=%d", gdk_pixbuf_get_width(pb), gdk_pixbuf_get_height(pb),
		gdk_pixbuf_get_n_channels(pb), gdk_pixbuf_get_has_alpha(pb));
	const guchar *p = gdk_pixbuf_read_pixels(pb);
	g_string_append_printf(s, " px00=%d,%d,%d", p[0], p[1], p[2]);
	GHashTable *o = gdk_pixbuf_get_options(pb);
	if (o != NULL) {
		GHashTableIter it;
		gpointer k, v;
		g_hash_table_iter_init(&it, o);
		while (g_hash_table_iter_next(&it, &k, &v)) {
			g_string_append_printf(s, " [%s=%.24s]", (const char *)k, (const char *)v);
		}
		g_hash_table_unref(o);
	}
	return g_string_free(s, FALSE);
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
	"strconv"
	"strings"
	"unsafe"
)

// ImageDoc is a raster image as a document: one picture, drawn as a whole at
// the size its pixels and its resolution give it. It must be closed.
type ImageDoc struct {
	surf *C.cairo_surface_t
	info ImageInfo
	// w and h are the size in document units, as the picture is shown: after
	// its orientation tag.
	w, h float64
	// ux and uy are the document units a pixel of the file covers, across and
	// down the file's own rows.
	ux, uy float64
}

// LoadImage decodes a picture. path is the file it was read from, or empty; it
// is not used, the data is all of the picture. The orientation tag of a
// photograph is honoured by the way it is drawn, and its resolution decides
// its size: the number of pixels at 96 to the inch when the file has none.
func LoadImage(data []byte, path string) (*ImageDoc, error) {
	if len(data) == 0 {
		return nil, &ImageError{Kind: ImageUnreadable, Reason: "empty file"}
	}
	var res C.ipl_result
	var gerr *C.GError
	ok := C.ipl_load((*C.guchar)(unsafe.Pointer(&data[0])), C.gsize(len(data)), C.gint64(MaxImagePixels), &res, &gerr) != 0
	if !ok {
		if res.probe.too_large != 0 {
			return nil, &ImageError{Kind: ImageTooLarge, W: int(res.probe.w), H: int(res.probe.h)}
		}
		reason := takeGError(gerr)
		// A decoder that does not know the data is a missing decoder when the
		// signature is one of a format the program lists, and plain
		// unreadable data otherwise.
		if f := Detect(data); res.unknown_type != 0 && f.IsImage() {
			return nil, &ImageError{Kind: ImageUnsupported, Format: f, Reason: reason}
		}
		return nil, &ImageError{Kind: ImageUnreadable, Reason: reason}
	}
	pb := res.pixbuf
	defer C.g_object_unref(C.gpointer(unsafe.Pointer(pb)))
	var name string
	if res.format != nil {
		name = C.GoString(res.format)
		C.g_free(C.gpointer(unsafe.Pointer(res.format)))
	}

	wpx, hpx := int(C.gdk_pixbuf_get_width(pb)), int(C.gdk_pixbuf_get_height(pb))
	if wpx <= 0 || hpx <= 0 {
		return nil, &ImageError{Kind: ImageUnreadable, Reason: "the picture has no pixels"}
	}
	if wpx > maxSurfaceSide || hpx > maxSurfaceSide || wpx*hpx > MaxImagePixels {
		return nil, &ImageError{Kind: ImageTooLarge, W: wpx, H: hpx}
	}

	info := ImageInfo{
		Format:   formatOfLoader(name, Detect(data)),
		WidthPx:  wpx,
		HeightPx: hpx,
		Animated: res.animated != 0,
	}
	info.Truncated = !Complete(info.Format, data)
	option := func(key string) string {
		ckey := C.CString(key)
		defer C.free(unsafe.Pointer(ckey))
		if v := C.ipl_option(pb, ckey); v != nil {
			return C.GoString(v)
		}
		return ""
	}
	info.Orientation = 1
	if o, err := strconv.Atoi(option("orientation")); err == nil && o >= 1 && o <= 8 {
		info.Orientation = o
	}
	info.MultiPage = option("multipage") == "yes"
	info.XDPI, info.YDPI = resolution(option("x-dpi"), option("y-dpi"))

	var transparent C.gboolean
	surf := C.ipl_surface(pb, &transparent)
	info.HasAlpha = transparent != 0
	if s := C.cairo_surface_status(surf); s != C.CAIRO_STATUS_SUCCESS {
		C.cairo_surface_destroy(surf)
		if s == C.CAIRO_STATUS_NO_MEMORY {
			return nil, &ImageError{Kind: ImageTooLarge, W: wpx, H: hpx}
		}
		return nil, cairoError("image surface creation", s)
	}
	if info.Format == FormatJPEG {
		if h, ok := scanJPEG(data); ok && canEmbedJPEG(h, wpx, hpx) {
			if C.ipl_attach_jpeg(surf, (*C.guchar)(unsafe.Pointer(&data[0])), C.gsize(len(data))) == C.CAIRO_STATUS_SUCCESS {
				info.EmbeddedJPEG = true
			}
		}
	}

	d := &ImageDoc{surf: surf, info: info}
	d.ux, d.uy = Dpi/info.dpiX(), Dpi/info.dpiY()
	d.w, d.h = float64(wpx)*d.ux, float64(hpx)*d.uy
	if info.Orientation >= 5 {
		d.w, d.h = d.h, d.w
	}
	runtime.SetFinalizer(d, func(d *ImageDoc) { d.Close() })
	return d, nil
}

// LoadImageDocument is LoadImage as the Loader type of the package.
func LoadImageDocument(data []byte, path string) (Document, error) {
	d, err := LoadImage(data, path)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// LoadDocument reads a document of either kind: a raster image when the data
// starts like one, an SVG otherwise.
func LoadDocument(data []byte, path string) (Document, error) {
	if Detect(data).IsImage() {
		return LoadImageDocument(data, path)
	}
	d, err := Load(data, path)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// formatOfLoader is the format the decoder says it read, which is the
// signature's when the two agree and the better informed when they do not.
func formatOfLoader(loader string, sniffed Format) Format {
	for _, f := range ImageFormats {
		if string(f) == loader {
			return f
		}
	}
	return sniffed
}

// resolution reads the x-dpi and y-dpi options of a decoded picture. A value
// out of the range a scanner or a camera writes is no resolution at all.
func resolution(x, y string) (xdpi, ydpi float64) {
	parse := func(s string) float64 {
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil || math.IsNaN(v) || v < 10 || v > 100000 {
			return 0
		}
		return v
	}
	xdpi, ydpi = parse(x), parse(y)
	if xdpi == 0 {
		xdpi = ydpi
	}
	if ydpi == 0 {
		ydpi = xdpi
	}
	return xdpi, ydpi
}

// SupportedImageFormats lists the raster formats this installation of
// gdk-pixbuf can read, in the order of ImageFormats. JPEG, PNG, TIFF, GIF and
// BMP come with it; WebP needs the webp-pixbuf-loader package on most Linux
// systems.
func SupportedImageFormats() []Format {
	names := C.ipl_format_names()
	defer C.g_free(C.gpointer(unsafe.Pointer(names)))
	have := map[string]bool{}
	for _, n := range strings.Split(C.GoString(names), ",") {
		have[n] = true
	}
	var out []Format
	for _, f := range ImageFormats {
		if have[string(f)] {
			out = append(out, f)
		}
	}
	return out
}

// Close releases the picture. It is safe to call more than once.
func (d *ImageDoc) Close() {
	if d.surf != nil {
		C.cairo_surface_destroy(d.surf)
		d.surf = nil
	}
	runtime.SetFinalizer(d, nil)
}

// Size is the size of the picture in document units, at Dpi.
func (d *ImageDoc) Size() (w, h float64) { return d.w, d.h }

// ImageInfo describes the file the picture was read from.
func (d *ImageDoc) ImageInfo() ImageInfo { return d.info }

// Measure fails: a picture has no elements.
func (d *ImageDoc) Measure(id string) (Box, error) {
	return Box{}, errors.New("engine: an image has no elements to measure")
}

// MeasureBlocks finds nothing: a picture has no elements.
func (d *ImageDoc) MeasureBlocks(ids []string) map[string]Box { return map[string]Box{} }

var _ Document = (*ImageDoc)(nil)
var _ drawer = (*ImageDoc)(nil)

// orientMatrix maps the pixels of the file, in document units, onto the picture
// as it is seen: its orientation tag. wu and hu are the file's own width and
// height in document units.
func orientMatrix(orientation int, wu, hu float64) C.cairo_matrix_t {
	var m C.cairo_matrix_t
	switch orientation {
	case 2:
		C.cairo_matrix_init(&m, -1, 0, 0, 1, C.double(wu), 0)
	case 3:
		C.cairo_matrix_init(&m, -1, 0, 0, -1, C.double(wu), C.double(hu))
	case 4:
		C.cairo_matrix_init(&m, 1, 0, 0, -1, 0, C.double(hu))
	case 5:
		C.cairo_matrix_init(&m, 0, 1, 1, 0, 0, 0)
	case 6:
		C.cairo_matrix_init(&m, 0, 1, -1, 0, C.double(hu), 0)
	case 7:
		C.cairo_matrix_init(&m, 0, -1, -1, 0, C.double(hu), C.double(wu))
	case 8:
		C.cairo_matrix_init(&m, 0, -1, 1, 0, 0, C.double(wu))
	default:
		C.cairo_matrix_init_identity(&m)
	}
	return m
}

// paint draws the whole picture on cr, whose units are the document's.
func (d *ImageDoc) paint(cr *C.cairo_t) {
	C.cairo_save(cr)
	m := orientMatrix(d.info.Orientation, float64(d.info.WidthPx)*d.ux, float64(d.info.HeightPx)*d.uy)
	C.cairo_transform(cr, &m)
	C.cairo_scale(cr, C.double(d.ux), C.double(d.uy))
	C.cairo_set_source_surface(cr, d.surf, 0, 0)
	// GOOD averages the pixels it shrinks and interpolates the ones it
	// enlarges, and the PDF writer turns it into an "interpolate" flag the
	// viewers honour. EXTEND_NONE, the default, is the one a PDF can say.
	C.cairo_pattern_set_filter(C.cairo_get_source(cr), C.CAIRO_FILTER_GOOD)
	C.cairo_paint(cr)
	C.cairo_restore(cr)
}

// Render draws the window win (document units) at pxPerUnit pixels per unit.
// A nil background leaves the pixels transparent.
func (d *ImageDoc) Render(win Box, pxPerUnit float64, background color.Color) (*image.RGBA, error) {
	wpx, hpx := int(math.Ceil(win.W*pxPerUnit)), int(math.Ceil(win.H*pxPerUnit))
	if wpx <= 0 || hpx <= 0 || pxPerUnit <= 0 {
		return nil, errors.New("engine: empty render window")
	}
	if d.surf == nil {
		return nil, errors.New("engine: the image is closed")
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
	d.paint(cr)
	if s := C.cairo_status(cr); s != C.CAIRO_STATUS_SUCCESS {
		return nil, cairoError("drawing the image", s)
	}
	C.cairo_surface_flush(surf)
	return copyImage(surf, wpx, hpx), nil
}

// drawPage draws one sheet: the page background, then the band of the picture
// under its clip, as the vector document does (see Doc.drawPage).
func (d *ImageDoc) drawPage(cr *C.cairo_t, p PageSpec, whitePage bool) error {
	if p.BandBottom <= p.BandTop || p.PtPerUnit <= 0 {
		return errors.New("the band is empty or the scale is not positive")
	}
	if d.surf == nil {
		return errors.New("the image is closed")
	}
	if whitePage {
		C.cairo_set_source_rgb(cr, 1, 1, 1)
		C.cairo_paint(cr)
	}
	C.cairo_save(cr)
	C.cairo_rectangle(cr, C.double(p.OriginXPt), C.double(p.OriginYPt),
		C.double(d.w*p.PtPerUnit), C.double((p.BandBottom-p.BandTop)*p.PtPerUnit))
	C.cairo_clip(cr)
	if whitePage {
		// As for a vector document: written whole, the clip of the band is
		// there to be read back by the verification of the PDF.
		C.cairo_set_source_rgb(cr, 1, 1, 1)
		C.cairo_paint(cr)
	}
	C.cairo_translate(cr, C.double(p.OriginXPt), C.double(p.OriginYPt-p.BandTop*p.PtPerUnit))
	C.cairo_scale(cr, C.double(p.PtPerUnit), C.double(p.PtPerUnit))
	d.paint(cr)
	C.cairo_restore(cr)
	return nil
}

// RenderPage draws a sheet as an image, pxPerPt pixels to the point, with the
// function that writes the PDF. A transparent page stays transparent.
func (d *ImageDoc) RenderPage(p PageSpec, pxPerPt float64, whitePage bool) (*image.RGBA, error) {
	return renderPage(d, p, pxPerPt, whitePage)
}

// ExportPDF writes one PDF page per PageSpec; see exportPDF. The picture is
// written once and drawn on every page under the clip of its band.
func (d *ImageDoc) ExportPDF(w io.Writer, pages []PageSpec, opt PDFOptions) error {
	return exportPDF(d, w, pages, opt)
}

// TEMP-DEBUG: what the decoder reports for a file.
func DebugOptions(data []byte) string {
	var res C.ipl_result
	var gerr *C.GError
	if C.ipl_load((*C.guchar)(unsafe.Pointer(&data[0])), C.gsize(len(data)), C.gint64(MaxImagePixels), &res, &gerr) == 0 {
		return "load failed: " + takeGError(gerr)
	}
	defer C.g_object_unref(C.gpointer(unsafe.Pointer(res.pixbuf)))
	name := ""
	if res.format != nil {
		name = C.GoString(res.format)
		C.g_free(C.gpointer(unsafe.Pointer(res.format)))
	}
	d := C.ipl_dump(res.pixbuf)
	defer C.g_free(C.gpointer(unsafe.Pointer(d)))
	return fmt.Sprintf("loader=%s animated=%v %s", name, res.animated != 0, C.GoString(d))
}

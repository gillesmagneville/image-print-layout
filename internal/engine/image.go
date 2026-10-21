// SPDX-License-Identifier: GPL-3.0-only

package engine

import "fmt"

// MaxImagePixels is the largest picture the program opens, in pixels. Its
// memory is four bytes a pixel once decoded, and the decoder holds it a
// second time while it works: a hundred million pixels already ask for
// a gigabyte.
const MaxImagePixels = 100_000_000

// maxSurfaceSide is the largest side of a cairo image surface.
const maxSurfaceSide = 32767

// defaultDPI is the resolution of an image that does not give its own: the one
// of the SVG and CSS unit, so that a picture is as large as the same number of
// pixels in a document.
const defaultDPI = Dpi

// ImageErrorKind says why an image could not be opened.
type ImageErrorKind int

const (
	// ImageUnreadable: the data is not a picture the decoder can read.
	ImageUnreadable ImageErrorKind = iota
	// ImageUnsupported: the format is known but this installation has no
	// decoder for it (WebP needs a separate gdk-pixbuf loader on Linux).
	ImageUnsupported
	// ImageTooLarge: more than MaxImagePixels pixels, or a side no image
	// surface can have.
	ImageTooLarge
)

// ImageError is the error of opening an image.
type ImageError struct {
	Kind   ImageErrorKind
	Format Format
	// W and H are the size the image announces, for ImageTooLarge.
	W, H int
	// Reason is the decoder's own message, for ImageUnreadable.
	Reason string
}

func (e *ImageError) Error() string {
	switch e.Kind {
	case ImageUnsupported:
		return fmt.Sprintf("engine: this installation cannot read %s images", e.Format)
	case ImageTooLarge:
		return fmt.Sprintf("engine: the image is too large (%d x %d pixels)", e.W, e.H)
	}
	return "engine: cannot read the image: " + e.Reason
}

// ImageInfo describes the picture a document was read from.
type ImageInfo struct {
	Format Format
	// WidthPx and HeightPx are the pixels of the file, before its orientation
	// tag turns the picture.
	WidthPx, HeightPx int
	// XDPI and YDPI are the resolution the file gives, in pixels to the inch,
	// across and down its rows; zero when it gives none, and the document is
	// then sized at Dpi.
	XDPI, YDPI float64
	// HasAlpha tells that the picture has transparent parts: a channel of
	// opacity that is 255 everywhere does not count.
	HasAlpha bool
	// Orientation is the EXIF orientation tag, 1 to 8; 1 is upright. The
	// picture is drawn turned as the tag says.
	Orientation int
	// MultiPage tells that the file holds several pictures (a TIFF), of which
	// only the first is read.
	MultiPage bool
	// Animated tells that the file holds an animation (a GIF), of which only
	// the first image is read.
	Animated bool
	// Truncated tells that the file ends before the picture does, as far as
	// its end can tell. The decoders read what is there and say nothing: the
	// rest of the picture is blank.
	Truncated bool
	// EmbeddedJPEG tells that the PDF carries the JPEG file as it is.
	EmbeddedJPEG bool
}

func (i ImageInfo) dpiX() float64 {
	if i.XDPI > 0 {
		return i.XDPI
	}
	return defaultDPI
}

func (i ImageInfo) dpiY() float64 {
	if i.YDPI > 0 {
		return i.YDPI
	}
	return defaultDPI
}

// HasResolution tells whether the file gave its resolution.
func (i ImageInfo) HasResolution() bool { return i.XDPI > 0 || i.YDPI > 0 }

// PixelsPerUnit is how many pixels of the file lie in a document unit, across
// and down the file's rows.
func (i ImageInfo) PixelsPerUnit() (x, y float64) { return i.dpiX() / Dpi, i.dpiY() / Dpi }

// ImageDocument is a Document that is a raster image, and can say what its
// file was.
type ImageDocument interface {
	Document
	ImageInfo() ImageInfo
}

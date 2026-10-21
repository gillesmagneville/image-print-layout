// SPDX-License-Identifier: GPL-3.0-only

// Package testimg makes raster pictures for tests, byte for byte the same
// every time: PNG with a resolution, uncompressed TIFF with a resolution and an
// orientation, and the segments that give a JPEG a resolution or an
// orientation. It writes the formats by hand where the standard library has no
// writer for them, so that a test knows exactly what the file says.
//
// Pure Go, no cgo.
package testimg

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
)

// The colours of Quadrants.
var (
	Red   = color.RGBA{R: 255, A: 255}
	Green = color.RGBA{G: 255, A: 255}
	Blue  = color.RGBA{B: 255, A: 255}
	White = color.RGBA{R: 255, G: 255, B: 255, A: 255}
)

// Quadrants is a picture of four flat colours: red top left, green top right,
// blue bottom left, white bottom right. Where they end up says how the picture
// was turned.
func Quadrants(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := Red
			switch {
			case x >= w/2 && y < h/2:
				c = Green
			case x < w/2 && y >= h/2:
				c = Blue
			case x >= w/2 && y >= h/2:
				c = White
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func chunk(kind string, data []byte) []byte {
	b := make([]byte, 0, len(data)+12)
	b = binary.BigEndian.AppendUint32(b, uint32(len(data)))
	b = append(b, kind...)
	b = append(b, data...)
	return binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(b[4:]))
}

// PNG writes img, with a resolution in dots to the inch when xdpi is not zero.
func PNG(img image.Image, xdpi, ydpi float64) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err) // writing to memory
	}
	data := b.Bytes()
	if xdpi == 0 {
		return data
	}
	const afterIHDR = 8 + 25
	phys := binary.BigEndian.AppendUint32(nil, uint32(math.Round(xdpi/0.0254)))
	phys = binary.BigEndian.AppendUint32(phys, uint32(math.Round(ydpi/0.0254)))
	phys = append(phys, 1) // the unit is the metre
	out := append([]byte{}, data[:afterIHDR]...)
	out = append(out, chunk("pHYs", phys)...)
	return append(out, data[afterIHDR:]...)
}

// HugePNG is the start of a PNG that announces w x h pixels and then stops:
// the signature, the header and the first bytes of the pixel data. A reader
// that allocates the picture before it looks at the size would not survive it.
func HugePNG(w, h uint32) []byte {
	ihdr := binary.BigEndian.AppendUint32(nil, w)
	ihdr = binary.BigEndian.AppendUint32(ihdr, h)
	ihdr = append(ihdr, 8, 2, 0, 0, 0) // 8 bits, RGB, no interlace
	b := []byte("\x89PNG\r\n\x1a\n")
	b = append(b, chunk("IHDR", ihdr)...)
	return append(b, chunk("IDAT", []byte{0x78, 0x9c, 0x01, 0x00})...)
}

// TIFF writes img as an uncompressed little endian TIFF of 8-bit RGB, with a
// resolution in the inch and an orientation tag (1 to 8).
func TIFF(img *image.RGBA, xdpi, ydpi float64, orientation int) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	le := binary.LittleEndian
	const (
		entries    = 13
		ifdOffset  = 8
		bpsOffset  = ifdOffset + 2 + entries*12 + 4
		xresOffset = bpsOffset + 6
		yresOffset = xresOffset + 8
		dataOffset = yresOffset + 8
	)
	b := make([]byte, 0, dataOffset+w*h*3)
	b = append(b, 'I', 'I', 42, 0)
	b = le.AppendUint32(b, ifdOffset)
	b = le.AppendUint16(b, entries)
	entry := func(tag, typ uint16, count, value uint32) {
		b = le.AppendUint16(b, tag)
		b = le.AppendUint16(b, typ)
		b = le.AppendUint32(b, count)
		b = le.AppendUint32(b, value)
	}
	const short, long, rational = 3, 4, 5
	entry(256, long, 1, uint32(w))
	entry(257, long, 1, uint32(h))
	entry(258, short, 3, bpsOffset)
	entry(259, short, 1, 1) // no compression
	entry(262, short, 1, 2) // RGB
	entry(273, long, 1, dataOffset)
	entry(274, short, 1, uint32(orientation))
	entry(277, short, 1, 3)
	entry(278, long, 1, uint32(h))
	entry(279, long, 1, uint32(w*h*3))
	entry(282, rational, 1, xresOffset)
	entry(283, rational, 1, yresOffset)
	entry(296, short, 1, 2) // inch
	b = le.AppendUint32(b, 0)
	for i := 0; i < 3; i++ {
		b = le.AppendUint16(b, 8)
	}
	b = le.AppendUint32(b, uint32(math.Round(xdpi*100)))
	b = le.AppendUint32(b, 100)
	b = le.AppendUint32(b, uint32(math.Round(ydpi*100)))
	b = le.AppendUint32(b, 100)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.RGBAAt(x, y)
			b = append(b, c.R, c.G, c.B)
		}
	}
	return b
}

// JPEG writes img at a quality of 90.
func JPEG(img image.Image) []byte {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 90}); err != nil {
		panic(err) // writing to memory
	}
	return b.Bytes()
}

// WithEXIFOrientation inserts an Exif segment that carries only an orientation
// tag (1 to 8) after the start of a JPEG file.
func WithEXIFOrientation(data []byte, orientation int) []byte {
	var seg []byte
	seg = append(seg, "Exif\x00\x00"...)
	seg = append(seg, 'I', 'I', 42, 0, 8, 0, 0, 0) // header, first IFD at 8
	seg = append(seg, 1, 0)                        // one entry
	seg = append(seg, 0x12, 0x01, 3, 0, 1, 0, 0, 0, byte(orientation), 0, 0, 0)
	seg = append(seg, 0, 0, 0, 0) // no next IFD
	out := append([]byte{}, data[:2]...)
	out = append(out, 0xFF, 0xE1, byte((len(seg)+2)>>8), byte(len(seg)+2))
	out = append(out, seg...)
	return append(out, data[2:]...)
}

// WithJFIFDensity inserts a JFIF segment that gives the resolution in dots to
// the inch after the start of a JPEG file.
func WithJFIFDensity(data []byte, xdpi, ydpi int) []byte {
	seg := []byte{0xFF, 0xE0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 1, 1,
		byte(xdpi >> 8), byte(xdpi), byte(ydpi >> 8), byte(ydpi), 0, 0}
	out := append([]byte{}, data[:2]...)
	out = append(out, seg...)
	return append(out, data[2:]...)
}

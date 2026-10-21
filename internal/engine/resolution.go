// SPDX-License-Identifier: GPL-3.0-only

package engine

import (
	"encoding/binary"
	"math"
)

// The resolution of a picture is read from the file here, not from the decoder.
// gdk-pixbuf has more than one decoder behind the same calls (glycin, from
// version 2.43 on, where a distribution has chosen it) and they do not all
// report what a file says about its resolution: a PNG at 300 dots to the inch
// came out with none. What a file says is a fact about the file, so it is read
// the same way everywhere, and the decoder's own figure only fills in for the
// formats read here.

// fileResolution gives the resolution a file states, in dots to the inch, on
// each axis. It is zero when the file states none, when the unit is not an
// absolute length (an aspect ratio only), or when the format is not one that
// is read here (the caller then asks the decoder).
func fileResolution(f Format, data []byte) (xdpi, ydpi float64) {
	switch f {
	case FormatPNG:
		return pngResolution(data)
	case FormatJPEG:
		return jpegResolution(data)
	case FormatTIFF:
		if t, ok := parseTIFF(data); ok {
			return t.resolution()
		}
	}
	return 0, 0
}

// pngResolution reads the pHYs chunk, which is in pixels to the metre when its
// unit byte is 1 and an aspect ratio only when it is 0. A resolution that was
// a whole number of dots to the inch is a fraction once it is in metres (300
// dpi is 11811 to the metre): it is given back to the nearest dot, like the
// figures that come from a metric unit below, so that 96 dpi is 96 and a
// picture is not a fraction of a percent off its natural size.
func pngResolution(data []byte) (xdpi, ydpi float64) {
	forEachPNGChunk(data, func(kind string, body []byte) bool {
		if kind != "pHYs" {
			return true
		}
		if len(body) == 9 && body[8] == 1 {
			x := float64(binary.BigEndian.Uint32(body)) * 0.0254
			y := float64(binary.BigEndian.Uint32(body[4:])) * 0.0254
			xdpi, ydpi = math.Round(x), math.Round(y)
		}
		return false
	})
	return xdpi, ydpi
}

// jpegResolution reads the density of the JFIF segment, and when that is only
// an aspect ratio, the resolution of the Exif segment (a camera writes both).
func jpegResolution(data []byte) (xdpi, ydpi float64) {
	var exif []byte
	forEachJPEGSegment(data, func(marker byte, seg []byte) bool {
		switch {
		case marker == 0xE0 && len(seg) >= 12 && string(seg[:5]) == "JFIF\x00":
			x := float64(binary.BigEndian.Uint16(seg[8:]))
			y := float64(binary.BigEndian.Uint16(seg[10:]))
			switch seg[7] {
			case 1: // dots to the inch
				xdpi, ydpi = x, y
			case 2: // dots to the centimetre
				xdpi, ydpi = math.Round(x*2.54), math.Round(y*2.54)
			}
		case marker == 0xE1 && len(seg) > 6 && string(seg[:6]) == "Exif\x00\x00" && exif == nil:
			exif = seg[6:]
		}
		return true
	})
	if xdpi > 0 && ydpi > 0 {
		return xdpi, ydpi
	}
	if t, ok := parseTIFF(exif); ok {
		return t.resolution()
	}
	return 0, 0
}

// forEachJPEGSegment calls fn with the marker and the payload (without its two
// length bytes) of each segment up to the start of the scan. fn returns false
// to stop.
func forEachJPEGSegment(data []byte, fn func(marker byte, seg []byte) bool) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return
		}
		for i < len(data) && data[i] == 0xFF { // fill bytes
			i++
		}
		if i >= len(data) {
			return
		}
		marker := data[i]
		i++
		switch {
		case marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7):
			continue // no length
		case marker == 0xD9 || marker == 0xDA:
			return // the end, or the scan: no segment of interest after it
		}
		if i+2 > len(data) {
			return
		}
		length := int(data[i])<<8 | int(data[i+1])
		if length < 2 || i+length > len(data) {
			return
		}
		if !fn(marker, data[i+2:i+length]) {
			return
		}
		i += length
	}
}

// tiffTags is what a TIFF file, or the Exif block of a JPEG (which has the same
// layout), says about the resolution and the orientation of its first picture.
type tiffTags struct {
	xres, yres  float64 // as written, in units of unit
	unit        int     // 1 none, 2 inch, 3 centimetre (2 when the tag is missing)
	orientation int     // 1 to 8, or 0 when the tag is missing
	multiPage   bool    // a second directory follows the first
}

// resolution is the resolution in dots to the inch, zero when the file states
// none or states it in no length.
func (t tiffTags) resolution() (xdpi, ydpi float64) {
	switch t.unit {
	case 2:
		return t.xres, t.yres
	case 3:
		return math.Round(t.xres * 2.54), math.Round(t.yres * 2.54)
	}
	return 0, 0
}

// parseTIFF reads the first directory of a classic TIFF structure. A file that
// is not one (or is a BigTIFF) gives ok false.
func parseTIFF(b []byte) (t tiffTags, ok bool) {
	t.unit = 2
	if len(b) < 8 {
		return t, false
	}
	var bo binary.ByteOrder
	switch string(b[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return t, false
	}
	if bo.Uint16(b[2:]) != 42 {
		return t, false
	}
	dir := int64(bo.Uint32(b[4:]))
	if dir < 8 || dir+2 > int64(len(b)) {
		return t, false
	}
	n := int(bo.Uint16(b[dir:]))
	rational := func(entry []byte) float64 {
		const typeRational = 5
		if bo.Uint16(entry[2:]) != typeRational || bo.Uint32(entry[4:]) != 1 {
			return 0
		}
		at := int64(bo.Uint32(entry[8:]))
		if at < 8 || at+8 > int64(len(b)) {
			return 0
		}
		den := bo.Uint32(b[at+4:])
		if den == 0 {
			return 0
		}
		v := float64(bo.Uint32(b[at:])) / float64(den)
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return 0
		}
		return v
	}
	short := func(entry []byte) int {
		const typeShort = 3
		if bo.Uint16(entry[2:]) != typeShort || bo.Uint32(entry[4:]) != 1 {
			return 0
		}
		return int(bo.Uint16(entry[8:]))
	}
	for i := 0; i < n; i++ {
		at := dir + 2 + int64(i)*12
		if at+12 > int64(len(b)) {
			break
		}
		entry := b[at : at+12]
		switch bo.Uint16(entry) {
		case 0x011A:
			t.xres = rational(entry)
		case 0x011B:
			t.yres = rational(entry)
		case 0x0128:
			if u := short(entry); u != 0 {
				t.unit = u
			}
		case 0x0112:
			if o := short(entry); o >= 1 && o <= 8 {
				t.orientation = o
			}
		}
	}
	if next := dir + 2 + int64(n)*12; next+4 <= int64(len(b)) {
		t.multiPage = bo.Uint32(b[next:]) != 0
	}
	return t, true
}

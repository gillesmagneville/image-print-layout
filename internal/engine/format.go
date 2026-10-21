// SPDX-License-Identifier: GPL-3.0-only

package engine

import (
	"bytes"
	"encoding/binary"
	"strings"
)

// Format is the kind of document a file holds, told from its first bytes: the
// extension of a file says what its author hoped, the signature says what it
// is. Pure Go: the application decides which way to open a file without the
// rendering libraries.
type Format string

// The formats the program recognises. FormatSVG is the vector document; the
// others are raster images, read by gdk-pixbuf when the build has it.
const (
	FormatUnknown Format = ""
	FormatSVG     Format = "svg"
	FormatPNG     Format = "png"
	FormatJPEG    Format = "jpeg"
	FormatTIFF    Format = "tiff"
	FormatGIF     Format = "gif"
	FormatBMP     Format = "bmp"
	FormatWebP    Format = "webp"
)

// ImageFormats are the raster formats, in the order the interface lists them.
var ImageFormats = []Format{FormatJPEG, FormatPNG, FormatTIFF, FormatWebP, FormatBMP, FormatGIF}

// IsImage tells whether f is a raster image format.
func (f Format) IsImage() bool {
	for _, g := range ImageFormats {
		if f == g {
			return true
		}
	}
	return false
}

// Label is the name of the format as a person writes it: JPEG, PNG, WebP.
func (f Format) Label() string {
	switch f {
	case FormatWebP:
		return "WebP"
	case FormatUnknown:
		return ""
	}
	return strings.ToUpper(string(f))
}

// MIME is the media type of the format, for the file chooser of the desktop.
func (f Format) MIME() string {
	switch f {
	case FormatSVG:
		return "image/svg+xml"
	case FormatPNG:
		return "image/png"
	case FormatJPEG:
		return "image/jpeg"
	case FormatTIFF:
		return "image/tiff"
	case FormatGIF:
		return "image/gif"
	case FormatBMP:
		return "image/bmp"
	case FormatWebP:
		return "image/webp"
	}
	return ""
}

// Extensions are the file extensions of the format, lower case, with the dot.
func (f Format) Extensions() []string {
	switch f {
	case FormatSVG:
		return []string{".svg"}
	case FormatPNG:
		return []string{".png"}
	case FormatJPEG:
		return []string{".jpg", ".jpeg", ".jpe"}
	case FormatTIFF:
		return []string{".tif", ".tiff"}
	case FormatGIF:
		return []string{".gif"}
	case FormatBMP:
		return []string{".bmp"}
	case FormatWebP:
		return []string{".webp"}
	}
	return nil
}

// Detect tells the format of a file from its first bytes. A file that is no
// raster image and has an <svg element near its start is an SVG; anything else
// is FormatUnknown, and is opened as an SVG all the same, so that its reading
// says why it cannot be one.
func Detect(data []byte) Format {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return FormatPNG
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return FormatJPEG
	case bytes.HasPrefix(data, []byte("II*\x00")), bytes.HasPrefix(data, []byte("MM\x00*")),
		bytes.HasPrefix(data, []byte("II+\x00")), bytes.HasPrefix(data, []byte("MM\x00+")):
		return FormatTIFF
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return FormatGIF
	case isBMP(data):
		return FormatBMP
	case len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return FormatWebP
	}
	head := data
	if len(head) > 4096 {
		head = head[:4096]
	}
	if bytes.Contains(head, []byte("<svg")) {
		return FormatSVG
	}
	return FormatUnknown
}

// isBMP checks the file header and the size of the DIB header that follows
// it: two letters are not enough to call a file a bitmap.
func isBMP(data []byte) bool {
	if len(data) < 18 || data[0] != 'B' || data[1] != 'M' {
		return false
	}
	// Bytes 6 to 9 are reserved and zero; the DIB header size comes at 14.
	if data[6] != 0 || data[7] != 0 || data[8] != 0 || data[9] != 0 {
		return false
	}
	switch size := uint32(data[14]) | uint32(data[15])<<8 | uint32(data[16])<<16 | uint32(data[17])<<24; size {
	case 12, 40, 52, 56, 64, 108, 124:
		return true
	}
	return false
}

// Complete tells whether data reaches the end of a file of format f, as far as
// the end of the format can tell. The decoders of gdk-pixbuf read a PNG or a
// JPEG that stops half way and give the part they have, without an error; the
// program must not print the half of a picture as if it were the whole of it.
// A format that has no such end, or whose decoder refuses a short file itself
// (TIFF), is reported complete.
func Complete(f Format, data []byte) bool {
	switch f {
	case FormatPNG:
		return pngComplete(data)
	case FormatJPEG:
		return jpegComplete(data)
	case FormatGIF:
		return gifComplete(data)
	case FormatBMP:
		// The size of the file is in the header; some writers leave it at 0.
		if len(data) >= 6 {
			if size := binary.LittleEndian.Uint32(data[2:6]); size != 0 && int64(size) > int64(len(data)) {
				return false
			}
		}
	case FormatWebP:
		// The RIFF header gives the size of what follows it.
		if len(data) >= 8 {
			if size := binary.LittleEndian.Uint32(data[4:8]); int64(size)+8 > int64(len(data)) {
				return false
			}
		}
	}
	return true
}

// pngComplete walks the chunks of a PNG up to IEND. It does not read their
// content or their checksum: the decoder does.
func pngComplete(data []byte) bool {
	i := 8
	for i+12 <= len(data) {
		n := int64(binary.BigEndian.Uint32(data[i : i+4]))
		next := int64(i) + 12 + n
		if next > int64(len(data)) {
			return false
		}
		if string(data[i+4:i+8]) == "IEND" {
			return true
		}
		i = int(next)
	}
	return false
}

// jpegComplete follows the markers of a JPEG file to its end of image. Inside
// the compressed data a marker is an FF followed by something other than 0
// (a stuffed byte), a restart marker or more FF.
func jpegComplete(data []byte) bool {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return false
	}
	i := 2
	for {
		j := bytes.IndexByte(data[i:], 0xFF)
		if j < 0 {
			return false
		}
		i += j + 1
		for i < len(data) && data[i] == 0xFF { // fill bytes
			i++
		}
		if i >= len(data) {
			return false
		}
		marker := data[i]
		i++
		switch {
		case marker == 0x00 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD8):
			continue // a stuffed byte, or a marker with no length
		case marker == 0xD9:
			return true
		}
		if i+2 > len(data) {
			return false
		}
		length := int(data[i])<<8 | int(data[i+1])
		if length < 2 || i+length > len(data) {
			return false
		}
		i += length
	}
}

// gifComplete looks for the trailer that ends a GIF, ignoring padding.
func gifComplete(data []byte) bool {
	end := len(data)
	for end > 0 && data[end-1] == 0 {
		end--
	}
	return end > 0 && data[end-1] == 0x3B
}

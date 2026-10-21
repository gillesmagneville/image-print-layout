// SPDX-License-Identifier: GPL-3.0-only

package engine

// jpegHeader is what the PDF needs to know of a JPEG file to take it as it is.
type jpegHeader struct {
	Width, Height int
	// Components is 1 (grey), 3 (colour) or 4 (CMYK).
	Components int
	// Precision is the number of bits of a sample, 8 in nearly every file.
	Precision int
	// AdobeTransform is the colour transform of an Adobe APP14 marker: 0 for
	// samples stored as they are (RGB or CMYK), 1 for YCbCr, 2 for YCCK. It is
	// -1 when the file has no such marker.
	AdobeTransform int
}

// scanJPEG reads the frame header of a JPEG file, and the Adobe marker if
// there is one. ok is false for a file it cannot make sense of. It reads
// segment headers only, never the compressed data.
func scanJPEG(data []byte) (h jpegHeader, ok bool) {
	h.AdobeTransform = -1
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return h, false
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return h, false
		}
		for i < len(data) && data[i] == 0xFF { // fill bytes
			i++
		}
		if i >= len(data) {
			return h, false
		}
		marker := data[i]
		i++
		switch {
		case marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7):
			continue // no length
		case marker == 0xD9:
			return h, false // end of image before any frame
		}
		if i+2 > len(data) {
			return h, false
		}
		length := int(data[i])<<8 | int(data[i+1])
		if length < 2 || i+length > len(data) {
			return h, false
		}
		seg := data[i+2 : i+length]
		switch {
		case marker == 0xEE && len(seg) >= 12 && string(seg[:5]) == "Adobe":
			h.AdobeTransform = int(seg[11])
		case marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC:
			if len(seg) < 6 {
				return h, false
			}
			h.Precision = int(seg[0])
			h.Height = int(seg[1])<<8 | int(seg[2])
			h.Width = int(seg[3])<<8 | int(seg[4])
			h.Components = int(seg[5])
			return h, h.Width > 0 && h.Height > 0 && h.Components > 0
		case marker == 0xDA:
			return h, false // the scan begins and no frame was met
		}
		i += length
	}
	return h, false
}

// canEmbedJPEG tells whether the file can go into the PDF as it is, instead of
// being decoded and written again: the PDF reads baseline and progressive
// JPEG of 8 bits, in grey or colour, with the YCbCr transform its default
// takes for three components. Anything else (CMYK, 12 bits, RGB stored
// untransformed) is written as pixels. The pixels the viewer shows must be
// those of the file's frame: an image that was turned by its orientation tag
// is not embedded.
func canEmbedJPEG(h jpegHeader, widthPx, heightPx int) bool {
	if h.Precision != 8 || h.Width != widthPx || h.Height != heightPx {
		return false
	}
	switch h.Components {
	case 1:
		return true
	case 3:
		return h.AdobeTransform != 0
	}
	return false
}

// SPDX-License-Identifier: GPL-3.0-only

package engine

import (
	"bytes"
	"encoding/binary"
	"regexp"
)

// What a file says about itself, read here for the same reason as its
// resolution (see fileResolution): the decoders behind gdk-pixbuf do not all
// say it. The decoder's own report is kept as well, where it exists: a file
// counts as having several pictures or an animation if either says so.

// forEachPNGChunk calls fn with the type and the data of each chunk, up to the
// first IDAT, where the chunks that describe the picture end. fn returns false
// to stop.
func forEachPNGChunk(data []byte, fn func(kind string, body []byte) bool) {
	const signature = 8
	for i := signature; i+8 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[i:]))
		kind := string(data[i+4 : i+8])
		body := i + 8
		if n < 0 || body+n > len(data) || kind == "IDAT" || kind == "IEND" {
			return
		}
		if !fn(kind, data[body:body+n]) {
			return
		}
		i = body + n + 4 // the data, then its checksum
	}
}

// fileMultiPage tells that a TIFF has a second directory after the first.
func fileMultiPage(f Format, data []byte) bool {
	if f != FormatTIFF {
		return false
	}
	t, ok := parseTIFF(data)
	return ok && t.multiPage
}

// fileAnimated tells that a GIF has more than one image, that a WebP is flagged
// as an animation, or that a PNG has the animation control chunk of APNG. In
// each case the program reads the first image only.
func fileAnimated(f Format, data []byte) bool {
	switch f {
	case FormatGIF:
		return gifImages(data) > 1
	case FormatWebP:
		// RIFF, size, WEBP, then the first chunk: VP8X, whose first byte holds
		// the flags, animation being the second bit.
		return len(data) >= 21 && bytes.Equal(data[12:16], []byte("VP8X")) && data[20]&0x02 != 0
	case FormatPNG:
		animated := false
		forEachPNGChunk(data, func(kind string, _ []byte) bool {
			animated = kind == "acTL"
			return !animated
		})
		return animated
	}
	return false
}

// gifImages counts the image descriptors of a GIF file, walking its blocks.
func gifImages(data []byte) int {
	if len(data) < 13 || (string(data[:6]) != "GIF87a" && string(data[:6]) != "GIF89a") {
		return 0
	}
	skipSubBlocks := func(i int) int {
		for i < len(data) {
			n := int(data[i])
			i++
			if n == 0 {
				return i
			}
			i += n
		}
		return len(data)
	}
	i := 13
	if flags := data[10]; flags&0x80 != 0 {
		i += 3 << (uint(flags&7) + 1) // the global colour table
	}
	images := 0
	for i < len(data) {
		switch data[i] {
		case 0x3B: // trailer
			return images
		case 0x21: // extension: a label, then sub-blocks
			i = skipSubBlocks(i + 2)
		case 0x2C: // image descriptor
			images++
			if i+10 > len(data) {
				return images
			}
			flags := data[i+9]
			i += 10
			if flags&0x80 != 0 {
				i += 3 << (uint(flags&7) + 1) // the local colour table
			}
			i = skipSubBlocks(i + 1) // the code size, then the data
		default:
			return images
		}
	}
	return images
}

var (
	glycinPrefix = regexp.MustCompile(`^Remote error: org\.gnome\.glycin\.Error\.\w+:\s*`)
	sourcePlace  = regexp.MustCompile(`^\S+\.rs(:\d+)*:\s*`)
)

// tidyDecoderMessage takes the noise off what a decoder says about a file it
// cannot read. glycin, which gdk-pixbuf 2.44 and later may use, starts its
// messages with the name of an error of its own and the place in its source
// where it was raised: the user needs the reason, which comes after.
func tidyDecoderMessage(s string) string {
	tidy := glycinPrefix.ReplaceAllString(s, "")
	for {
		t := sourcePlace.ReplaceAllString(tidy, "")
		if t == tidy {
			break
		}
		tidy = t
	}
	if tidy == "" {
		return s
	}
	return tidy
}

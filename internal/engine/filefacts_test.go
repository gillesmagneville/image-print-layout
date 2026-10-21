// SPDX-License-Identifier: GPL-3.0-only

package engine

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color/palette"
	"image/gif"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/testimg"
)

func gifOf(frames int) []byte {
	g := &gif.GIF{}
	for i := 0; i < frames; i++ {
		p := image.NewPaletted(image.Rect(0, 0, 4, 4), palette.Plan9)
		p.SetColorIndex(i, i, uint8(i+1))
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 10)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// apng puts an animation control chunk after the header of a PNG.
func apng(data []byte) []byte {
	body := binary.BigEndian.AppendUint32(nil, 2) // frames
	body = binary.BigEndian.AppendUint32(body, 0) // plays
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	chunk = append(chunk, "acTL"...)
	chunk = append(chunk, body...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
	const afterIHDR = 8 + 25
	out := append([]byte{}, data[:afterIHDR]...)
	out = append(out, chunk...)
	return append(out, data[afterIHDR:]...)
}

func webp(flags byte) []byte {
	b := []byte("RIFF\x1a\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00")
	return append(b, flags, 0, 0, 0, 0, 0, 0, 0, 0, 0)
}

func TestFileAnimatedSeesGIFWebPAndPNGAnimations(t *testing.T) {
	q := testimg.Quadrants(6, 4)
	for _, c := range []struct {
		name   string
		format Format
		data   []byte
		want   bool
	}{
		{"gif of one image", FormatGIF, gifOf(1), false},
		{"gif of three images", FormatGIF, gifOf(3), true},
		{"webp still", FormatWebP, webp(0x10), false},
		{"webp animation", FormatWebP, webp(0x12), true},
		{"png", FormatPNG, testimg.PNG(q, 0, 0), false},
		{"apng", FormatPNG, apng(testimg.PNG(q, 0, 0)), true},
		{"jpeg", FormatJPEG, testimg.JPEG(q), false},
	} {
		if got := fileAnimated(c.format, c.data); got != c.want {
			t.Errorf("%s: fileAnimated = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestGIFImagesSurvivesCutsAndGarbage(t *testing.T) {
	data := gifOf(3)
	for n := 0; n <= len(data); n++ {
		gifImages(data[:n]) // no panic wherever the file stops
	}
	if n := gifImages(data); n != 3 {
		t.Errorf("gifImages = %d, want 3", n)
	}
	junk := append([]byte("GIF89a"), bytes.Repeat([]byte{0xFF}, 40)...)
	gifImages(junk)
}

func TestFileMultiPageSeesASecondDirectoryOfATIFF(t *testing.T) {
	q := testimg.Quadrants(6, 4)
	one := testimg.TIFF(q, 300, 300, 1)
	if fileMultiPage(FormatTIFF, one) {
		t.Error("a TIFF of one picture is said to have several")
	}
	two := append([]byte{}, one...)
	const next = 8 + 2 + 13*12 // the offset of the next directory, after the 13 entries
	binary.LittleEndian.PutUint32(two[next:], 1000)
	if !fileMultiPage(FormatTIFF, two) {
		t.Error("a TIFF with a second directory is not said to have several pictures")
	}
	if fileMultiPage(FormatJPEG, two) {
		t.Error("only a TIFF has pages")
	}
}

func TestTidyDecoderMessageKeepsTheReason(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"Remote error: org.gnome.glycin.Error.LoadingError: glycin-loaders/glycin-image-rs/src/main.rs:228:29: /build/x/image_rs.rs: unexpected end of file", "unexpected end of file"},
		{"Remote error: org.gnome.glycin.Error.LoadingError: src/main.rs:1:2: bad header", "bad header"},
		{"Error reading PNG image file: Not enough image data", "Error reading PNG image file: Not enough image data"},
		{"Remote error: org.gnome.glycin.Error.LoadingError: ", "Remote error: org.gnome.glycin.Error.LoadingError: "}, // nothing after: keep what there is
		{"", ""},
	} {
		if got := tidyDecoderMessage(c.in); got != c.want {
			t.Errorf("tidyDecoderMessage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

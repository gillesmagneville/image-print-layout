// SPDX-License-Identifier: GPL-3.0-only

package engine

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"testing"
)

func TestDetectTellsTheFormatFromTheFirstBytes(t *testing.T) {
	bmp := make([]byte, 30)
	copy(bmp, "BM")
	bmp[14] = 40 // BITMAPINFOHEADER
	cases := []struct {
		name string
		data []byte
		want Format
	}{
		{"png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), FormatPNG},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 16, 'J', 'F', 'I', 'F'}, FormatJPEG},
		{"tiff little endian", []byte("II*\x00\x08\x00\x00\x00"), FormatTIFF},
		{"tiff big endian", []byte("MM\x00*\x00\x00\x00\x08"), FormatTIFF},
		{"bigtiff", []byte("II+\x00\x08\x00\x08\x00"), FormatTIFF},
		{"gif 89a", []byte("GIF89a\x01\x00\x01\x00"), FormatGIF},
		{"gif 87a", []byte("GIF87a\x01\x00\x01\x00"), FormatGIF},
		{"bmp", bmp, FormatBMP},
		{"webp", []byte("RIFF\x24\x00\x00\x00WEBPVP8 "), FormatWebP},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`), FormatSVG},
		{"svg after an xml prolog", []byte("<?xml version=\"1.0\"?>\n<!-- c -->\n<svg xmlns=\"http://www.w3.org/2000/svg\"/>"), FormatSVG},
		{"svg with a byte order mark", []byte("\xEF\xBB\xBF<svg/>"), FormatSVG},
		{"two letters are not a bitmap", []byte("BM is not a picture, just text"), FormatUnknown},
		{"riff that is not webp", []byte("RIFF\x24\x00\x00\x00WAVEfmt "), FormatUnknown},
		{"text", []byte("hello"), FormatUnknown},
		{"empty", nil, FormatUnknown},
	}
	for _, c := range cases {
		if got := Detect(c.data); got != c.want {
			t.Errorf("%s: Detect = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestOnlyRasterFormatsAreImages(t *testing.T) {
	for _, f := range ImageFormats {
		if !f.IsImage() || f.MIME() == "" || len(f.Extensions()) == 0 {
			t.Errorf("%q is listed as an image format but is not described as one", f)
		}
	}
	for _, f := range []Format{FormatSVG, FormatUnknown} {
		if f.IsImage() {
			t.Errorf("%q is not a raster image", f)
		}
	}
	if FormatSVG.MIME() != "image/svg+xml" {
		t.Errorf("the media type of an SVG is %q", FormatSVG.MIME())
	}
}

func TestCompleteSeesAFileThatStopsEarly(t *testing.T) {
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 200, 200))); err != nil {
		t.Fatal(err)
	}
	pngData := pngBuf.Bytes()
	jpegData := encodeJPEG(t, image.NewRGBA(image.Rect(0, 0, 200, 200)))

	whole := map[Format][]byte{
		FormatPNG:  pngData,
		FormatJPEG: jpegData,
		FormatGIF:  []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00,\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"),
	}
	bmp := make([]byte, 60)
	copy(bmp, "BM")
	binary.LittleEndian.PutUint32(bmp[2:], 60)
	whole[FormatBMP] = bmp
	webp := append([]byte("RIFF"), 0, 0, 0, 0)
	webp = append(webp, "WEBPVP8 "...)
	webp = append(webp, make([]byte, 20)...)
	binary.LittleEndian.PutUint32(webp[4:], uint32(len(webp)-8))
	whole[FormatWebP] = webp

	for f, data := range whole {
		if !Complete(f, data) {
			t.Errorf("%s: a whole file is said incomplete", f)
		}
		if Complete(f, data[:len(data)-3]) {
			t.Errorf("%s: a file that lacks its last bytes is said complete", f)
		}
		if Complete(f, data[:len(data)/2]) {
			t.Errorf("%s: half a file is said complete", f)
		}
	}
	// Data after the end of the picture is not a reason to doubt it: a
	// photograph that carries a video or a thumbnail, or padding.
	if !Complete(FormatPNG, append(append([]byte{}, pngData...), "trailing"...)) {
		t.Error("png with trailing bytes")
	}
	if !Complete(FormatJPEG, append(append([]byte{}, jpegData...), "ftypmp42 and the rest of a video"...)) {
		t.Error("jpeg with trailing bytes")
	}
	if !Complete(FormatGIF, append(append([]byte{}, whole[FormatGIF]...), 0, 0, 0)) {
		t.Error("gif with padding")
	}
	// Some writers leave the size of a bitmap at 0.
	binary.LittleEndian.PutUint32(bmp[2:], 0)
	if !Complete(FormatBMP, bmp) {
		t.Error("bmp with no size")
	}
	// TIFF and the formats this function does not know end where they end.
	if !Complete(FormatTIFF, []byte("II*\x00")) || !Complete(FormatSVG, nil) {
		t.Error("formats without an end are complete")
	}
}

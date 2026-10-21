// SPDX-License-Identifier: GPL-3.0-only

package engine

import (
	"bytes"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"testing"
)

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := stdjpeg.Encode(&b, img, &stdjpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestScanJPEGReadsTheFrameHeader(t *testing.T) {
	colour := image.NewRGBA(image.Rect(0, 0, 37, 21))
	h, ok := scanJPEG(encodeJPEG(t, colour))
	if !ok || h.Width != 37 || h.Height != 21 || h.Components != 3 || h.Precision != 8 || h.AdobeTransform != -1 {
		t.Errorf("colour: %+v ok=%v", h, ok)
	}
	grey := image.NewGray(image.Rect(0, 0, 16, 9))
	grey.SetGray(1, 1, color.Gray{Y: 200})
	h, ok = scanJPEG(encodeJPEG(t, grey))
	if !ok || h.Width != 16 || h.Height != 9 || h.Components != 1 {
		t.Errorf("grey: %+v ok=%v", h, ok)
	}
}

// withAdobe inserts an Adobe APP14 marker with the given transform after the
// start of the image: FF EE, a length of 14, then "Adobe", the version, two
// words of flags and the transform byte.
func withAdobe(data []byte, transform byte) []byte {
	seg := []byte{0xFF, 0xEE, 0x00, 0x0E, 'A', 'd', 'o', 'b', 'e', 0x00, 0x64, 0x00, 0x00, 0x00, 0x00, transform}
	out := append([]byte{}, data[:2]...)
	out = append(out, seg...)
	return append(out, data[2:]...)
}

func TestScanJPEGSeesTheAdobeMarker(t *testing.T) {
	data := encodeJPEG(t, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	h, ok := scanJPEG(withAdobe(data, 0))
	if !ok || h.AdobeTransform != 0 {
		t.Fatalf("AdobeTransform = %d (ok=%v), want 0", h.AdobeTransform, ok)
	}
	if canEmbedJPEG(h, 8, 8) {
		t.Error("an RGB JPEG stored untransformed would show wrong colours in a PDF: it must not be embedded")
	}
	h, ok = scanJPEG(withAdobe(data, 1))
	if !ok || h.AdobeTransform != 1 || !canEmbedJPEG(h, 8, 8) {
		t.Errorf("a YCbCr JPEG with an Adobe marker can be embedded: %+v ok=%v", h, ok)
	}
}

func TestScanJPEGRefusesWhatIsNotAFrame(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":     nil,
		"not jpeg":  []byte("\x89PNG\r\n\x1a\n"),
		"truncated": {0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J'},
		"no frame":  {0xFF, 0xD8, 0xFF, 0xD9},
	} {
		if _, ok := scanJPEG(data); ok {
			t.Errorf("%s: scanJPEG accepted it", name)
		}
	}
}

func TestOnlyAFrameThatMatchesThePixelsIsEmbedded(t *testing.T) {
	h, _ := scanJPEG(encodeJPEG(t, image.NewRGBA(image.Rect(0, 0, 30, 20))))
	if !canEmbedJPEG(h, 30, 20) {
		t.Error("a baseline colour JPEG is embedded")
	}
	// A photo turned by its orientation tag has other pixel dimensions than its frame.
	if canEmbedJPEG(h, 20, 30) {
		t.Error("a JPEG whose pixels were turned must not be embedded")
	}
	h.Components = 4
	if canEmbedJPEG(h, 30, 20) {
		t.Error("a CMYK JPEG is written as pixels")
	}
	h.Components, h.Precision = 3, 12
	if canEmbedJPEG(h, 30, 20) {
		t.Error("a 12-bit JPEG is written as pixels")
	}
}

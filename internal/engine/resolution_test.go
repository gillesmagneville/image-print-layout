// SPDX-License-Identifier: GPL-3.0-only

package engine

import (
	"encoding/base64"
	"encoding/binary"
	"math"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/testimg"
)

func closeTo(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestFileResolutionReadsPNGJPEGAndTIFF(t *testing.T) {
	q := testimg.Quadrants(6, 4)
	jpeg := testimg.JPEG(q)
	for _, c := range []struct {
		name         string
		format       Format
		data         []byte
		wantX, wantY float64
	}{
		{"png 300", FormatPNG, testimg.PNG(q, 300, 300), 300, 300},
		{"png two axes", FormatPNG, testimg.PNG(q, 300, 150), 300, 150},
		{"png none", FormatPNG, testimg.PNG(q, 0, 0), 0, 0},
		{"jpeg jfif 300", FormatJPEG, testimg.WithJFIFDensity(jpeg, 300, 300), 300, 300},
		{"jpeg none", FormatJPEG, jpeg, 0, 0},
		{"jpeg aspect ratio only", FormatJPEG, testimg.WithJFIFAspect(jpeg), 0, 0},
		{"jpeg exif 72", FormatJPEG, testimg.WithEXIFResolution(jpeg, 72, 72, 2), 72, 72},
		{"jpeg exif per centimetre", FormatJPEG, testimg.WithEXIFResolution(jpeg, 118, 118, 3), 300, 300},
		{"jpeg exif no unit", FormatJPEG, testimg.WithEXIFResolution(jpeg, 72, 72, 1), 0, 0},
		// A camera writes an aspect ratio in JFIF and the real figure in Exif.
		{"jpeg camera", FormatJPEG, testimg.WithJFIFAspect(testimg.WithEXIFResolution(jpeg, 240, 240, 2)), 240, 240},
		// When both give a length, JFIF is the one that counts.
		{"jpeg both", FormatJPEG, testimg.WithJFIFDensity(testimg.WithEXIFResolution(jpeg, 72, 72, 2), 300, 300), 300, 300},
		{"tiff 300", FormatTIFF, testimg.TIFF(q, 300, 300, 1), 300, 300},
		{"tiff two axes", FormatTIFF, testimg.TIFF(q, 200, 100, 6), 200, 100},
		{"other format", FormatGIF, []byte("GIF89a"), 0, 0},
	} {
		x, y := fileResolution(c.format, c.data)
		if !closeTo(x, c.wantX) || !closeTo(y, c.wantY) {
			t.Errorf("%s: fileResolution = %v, %v; want %v, %v", c.name, x, y, c.wantX, c.wantY)
		}
	}
}

func TestFileResolutionSurvivesDamagedFiles(t *testing.T) {
	q := testimg.Quadrants(6, 4)
	files := map[Format][]byte{
		FormatPNG:  testimg.PNG(q, 300, 300),
		FormatJPEG: testimg.WithEXIFResolution(testimg.WithJFIFDensity(testimg.JPEG(q), 300, 300), 72, 72, 2),
		FormatTIFF: testimg.TIFF(q, 300, 300, 1),
	}
	for f, data := range files {
		// Every prefix: no panic, whatever the cut falls on.
		for n := 0; n <= len(data) && n < 200; n++ {
			fileResolution(f, data[:n])
		}
		// Garbage after the signature.
		bad := append([]byte{}, data...)
		for i := 8; i < len(bad) && i < 120; i++ {
			bad[i] = 0xFF
		}
		fileResolution(f, bad)
	}
	if x, y := fileResolution(FormatPNG, nil); x != 0 || y != 0 {
		t.Errorf("empty PNG: %v, %v", x, y)
	}
}

func TestParseTIFFReadsBothByteOrders(t *testing.T) {
	be := binary.BigEndian
	var b []byte
	b = append(b, 'M', 'M', 0, 42)
	b = be.AppendUint32(b, 8)
	b = be.AppendUint16(b, 3)
	entry := func(tag, typ uint16, count uint32, value []byte) {
		b = be.AppendUint16(b, tag)
		b = be.AppendUint16(b, typ)
		b = be.AppendUint32(b, count)
		b = append(b, value...)
	}
	const end = 8 + 2 + 3*12 + 4
	entry(0x011A, 5, 1, be.AppendUint32(nil, end))
	entry(0x0128, 3, 1, []byte{0, 3, 0, 0}) // centimetres, big endian short
	entry(0x0112, 3, 1, []byte{0, 6, 0, 0})
	b = be.AppendUint32(b, 0)
	b = be.AppendUint32(b, 118) // 118 per centimetre
	b = be.AppendUint32(b, 1)
	tags, ok := parseTIFF(b)
	if !ok {
		t.Fatal("not read")
	}
	if tags.unit != 3 || tags.orientation != 6 || !closeTo(tags.xres, 118) {
		t.Errorf("tags %+v", tags)
	}
	if x, _ := tags.resolution(); !closeTo(x, 300) {
		t.Errorf("x = %v", x)
	}
	// A BigTIFF, or something else, is not read.
	if _, ok := parseTIFF([]byte{'I', 'I', 43, 0, 8, 0, 0, 0}); ok {
		t.Error("a BigTIFF was read as a TIFF")
	}
}

// The images of the self test of the program (cmd/imageprintlayout) carry their
// resolution, 300 dpi, and the engine must read it from the file itself: the
// loaders of gdk-pixbuf 2.44 (glycin) do not report it.
func TestFileResolutionReadsTheImagesOfTheSelfTest(t *testing.T) {
	for name, c := range map[string]struct {
		format Format
		b64    string
	}{
		"PNG":  {FormatPNG, "iVBORw0KGgoAAAANSUhEUgAAAAQAAAAECAIAAAAmkwkpAAAACXBIWXMAAC4jAAAuIwF4pT92AAAAIElEQVR4nGL5zwACjGCKiQEJsDAygAQh8igygAAAAP//ipEEC7T7bwUAAAAASUVORK5CYII="},
		"TIFF": {FormatTIFF, "SUkqAAgAAAANAAABBAABAAAABAAAAAEBBAABAAAABAAAAAIBAwADAAAAqgAAAAMBAwABAAAAAQAAAAYBAwABAAAAAgAAABEBBAABAAAAwAAAABIBAwABAAAAAQAAABUBAwABAAAAAwAAABYBBAABAAAABAAAABcBBAABAAAAMAAAABoBBQABAAAAsAAAABsBBQABAAAAuAAAACgBAwABAAAAAgAAAAAAAAAIAAgACAAwdQAAZAAAADB1AABkAAAA/wAA/wAAAP8AAP8A/wAA/wAAAP8AAP8AAAD/AAD/////////AAD/AAD/////////"},
	} {
		raw, err := base64.StdEncoding.DecodeString(c.b64)
		if err != nil {
			t.Fatal(err)
		}
		x, y := fileResolution(c.format, raw)
		if x != 300 || y != 300 {
			t.Errorf("%s: resolution %v x %v, want 300 x 300", name, x, y)
		}
	}
}

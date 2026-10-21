// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package engine

import (
	"fmt"
	"testing"

	"github.com/gillesmagneville/image-print-layout/internal/testimg"
)

// TEMP-DEBUG: what the decoder of this installation reports.
func TestZZDebugDecoder(t *testing.T) {
	q := testimg.Quadrants(6, 4)
	jpeg := testimg.JPEG(q)
	cases := []struct {
		name string
		data []byte
	}{
		{"png-300dpi", testimg.PNG(q, 300, 300)},
		{"png-nodpi", testimg.PNG(q, 0, 0)},
		{"jpeg-plain", jpeg},
		{"jpeg-jfif-300", testimg.WithJFIFDensity(jpeg, 300, 300)},
		{"jpeg-exif-3", testimg.WithEXIFOrientation(jpeg, 3)},
		{"jpeg-exif-6", testimg.WithEXIFOrientation(jpeg, 6)},
		{"tiff-300-o1", testimg.TIFF(q, 300, 300, 1)},
		{"tiff-300-o2", testimg.TIFF(q, 300, 300, 2)},
		{"tiff-300-o3", testimg.TIFF(q, 300, 300, 3)},
		{"tiff-300-o6", testimg.TIFF(q, 300, 300, 6)},
		{"tiff-300-o8", testimg.TIFF(q, 300, 300, 8)},
	}
	for _, c := range cases {
		fmt.Println("DEBUGINFO", c.name, DebugOptions(c.data))
	}
	t.Error("debug test: always fails so that the CI shows its output")
}

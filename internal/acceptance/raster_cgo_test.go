// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package acceptance

import (
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// need skips a test when a poppler tool is missing, except on CI where a
// missing tool must not turn a check into a silent skip.
func need(t *testing.T, tool string) {
	t.Helper()
	if _, err := exec.LookPath(tool); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s is required on CI", tool)
		}
		t.Skipf("%s not installed", tool)
	}
}

func run(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

// raster is an RGB raster, as pdftoppm draws a page.
type raster struct {
	pix  []uint8 // R, G, B per pixel
	w, h int
}

func (r raster) at(x, y int) [3]int {
	i := (y*r.w + x) * 3
	return [3]int{int(r.pix[i]), int(r.pix[i+1]), int(r.pix[i+2])}
}

func (r raster) luma(x, y int) int {
	c := r.at(x, y)
	return (299*c[0] + 587*c[1] + 114*c[2]) / 1000
}

func (r raster) white(x, y int) bool { return r.at(x, y) == [3]int{255, 255, 255} }

// rasterise draws every page of a PDF with pdftoppm and reads them back, in
// page order.
func rasterise(t *testing.T, pdf string, dpi int) []raster {
	t.Helper()
	need(t, "pdftoppm")
	prefix := filepath.Join(filepath.Dir(pdf), strings.TrimSuffix(filepath.Base(pdf), ".pdf"))
	run(t, "pdftoppm", "-r", strconv.Itoa(dpi), "-png", pdf, prefix)
	files, _ := filepath.Glob(prefix + "-*.png")
	sort.Strings(files)
	var out []raster
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, toRaster(img))
		continue
	}
	return out
}

// mmToPx is the number of pixels of a length in millimetres at a resolution.
func mmToPx(mm float64, dpi int) float64 { return mm / 25.4 * float64(dpi) }

// crop copies a window of the raster.
func (r raster) crop(x, y, w, h int) raster {
	out := raster{pix: make([]uint8, 0, w*h*3), w: w, h: h}
	for j := 0; j < h; j++ {
		out.pix = append(out.pix, r.pix[((y+j)*r.w+x)*3:((y+j)*r.w+x+w)*3]...)
	}
	return out
}

// blocks replaces every n x n square by its mean grey level.
func (r raster) blocks(n int) []uint8 {
	w, h := r.w/n, r.h/n
	out := make([]uint8, w*h)
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			sum := 0
			for dy := 0; dy < n; dy++ {
				for dx := 0; dx < n; dx++ {
					sum += r.luma(i*n+dx, j*n+dy)
				}
			}
			out[j*w+i] = uint8(sum / (n * n))
		}
	}
	return out
}

// differing counts the squares of two equally long lists of means that differ
// by more than tol grey levels.
func differing(a, b []uint8, tol int) (bad int) {
	for i := range a {
		if d := int(a[i]) - int(b[i]); d > tol || d < -tol {
			bad++
		}
	}
	return bad
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// toRaster reads an image into a raster. pdftoppm writes opaque pages, which
// the decoder gives as *image.RGBA: those are copied row by row, the others
// pixel by pixel.
func toRaster(img image.Image) raster {
	b := img.Bounds()
	r := raster{pix: make([]uint8, b.Dx()*b.Dy()*3), w: b.Dx(), h: b.Dy()}
	switch m := img.(type) {
	case *image.RGBA:
		for y := 0; y < r.h; y++ {
			row := m.Pix[y*m.Stride : y*m.Stride+r.w*4]
			out := r.pix[y*r.w*3:]
			for x := 0; x < r.w; x++ {
				out[x*3], out[x*3+1], out[x*3+2] = row[x*4], row[x*4+1], row[x*4+2]
			}
		}
	case *image.NRGBA:
		for y := 0; y < r.h; y++ {
			row := m.Pix[y*m.Stride : y*m.Stride+r.w*4]
			out := r.pix[y*r.w*3:]
			for x := 0; x < r.w; x++ {
				out[x*3], out[x*3+1], out[x*3+2] = row[x*4], row[x*4+1], row[x*4+2]
			}
		}
	default:
		i := 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				cr, cg, cb, _ := img.At(x, y).RGBA()
				r.pix[i], r.pix[i+1], r.pix[i+2] = uint8(cr>>8), uint8(cg>>8), uint8(cb>>8)
				i += 3
			}
		}
	}
	return r
}

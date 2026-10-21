// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package main

import (
	"bytes"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"

	"github.com/gillesmagneville/image-print-layout/internal/engine"
	"github.com/gillesmagneville/image-print-layout/internal/units"
)

// prepareRuntime must run first: it points the engine's libraries at the data
// shipped with the program (Windows packages).
func prepareRuntime() error { return engine.ConfigureRuntime() }

// libraries names the libraries the rendering engine runs with.
func libraries() string {
	rsvg, cairo := engine.Versions()
	return fmt.Sprintf("librsvg %s, cairo %s", rsvg, cairo)
}

// selfTestSVG is a 100 x 60 document with a background, a block, a line of
// text with accents and a small embedded PNG (a red square): enough to
// exercise loading, measuring, rendering, fonts, image decoding and PDF export.
const selfTestSVG = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="100" height="60" viewBox="0 0 100 60">
  <rect id="fond" width="100" height="60" fill="#f4f5f8"/>
  <rect id="bloc" x="10" y="10" width="40" height="20" fill="#2d6cdf"/>
  <text id="texte" x="10" y="48" font-family="sans-serif" font-size="8">Essai é à ç</text>
  <image id="image" x="60" y="10" width="20" height="20" xlink:href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAQAAAAECAIAAAAmkwkpAAAAEElEQVR42mO4aWAARwzEcQAHghORZScVxwAAAABJRU5ErkJggg=="/>
</svg>`

// runSelfTest checks, on the machine and with the libraries it runs on, the
// whole chain an installed package depends on: the SVG library and its
// fonts, the drawing library and the PDF writer. It is what a package
// smoke test, or a user whose export fails, runs first.
func runSelfTest(out io.Writer) error {
	rsvg, cairo := engine.Versions()
	fmt.Fprintf(out, "Moteur de rendu : librsvg %s, cairo %s\n", rsvg, cairo)
	if conf := os.Getenv("FONTCONFIG_FILE"); conf != "" {
		fmt.Fprintf(out, "Configuration des polices : %s\n", conf)
	}
	if cache := os.Getenv("GDK_PIXBUF_MODULE_FILE"); cache != "" {
		fmt.Fprintf(out, "Chargeurs d'images : %s\n", cache)
	}
	ok := func(what string) { fmt.Fprintf(out, "  ok  %s\n", what) }

	d, err := engine.Load([]byte(selfTestSVG), "")
	if err != nil {
		return err
	}
	defer d.Close()
	w, h := d.Size()
	if math.Abs(w-100) > 1e-6 || math.Abs(h-60) > 1e-6 {
		return fmt.Errorf("le SVG d'essai devrait mesurer 100 x 60 unités, le moteur lit %v x %v", w, h)
	}
	ok("chargement du SVG intégré (100 x 60 unités)")

	box, err := d.Measure("bloc")
	if err != nil {
		return err
	}
	if math.Abs(box.X-10) > 0.01 || math.Abs(box.Y-10) > 0.01 || math.Abs(box.W-40) > 0.01 || math.Abs(box.H-20) > 0.01 {
		return fmt.Errorf("la boîte du bloc devrait être (10, 10, 40 x 20), le moteur lit (%v, %v, %v x %v)", box.X, box.Y, box.W, box.H)
	}
	ok("mesure d'un élément dans le repère du document")

	img, err := d.Render(engine.Box{X: 0, Y: 0, W: w, H: h}, 4, color.White)
	if err != nil {
		return err
	}
	blue := color.RGBAModel.Convert(img.At(4*20, 4*20)).(color.RGBA)
	paper := color.RGBAModel.Convert(img.At(4*2, 4*2)).(color.RGBA)
	if blue.B < 180 || blue.R > 100 || paper.R < 230 || paper.G < 230 || paper.B < 230 {
		return fmt.Errorf("le rendu en image est faux : le bloc devrait être bleu (lu %v) et le fond clair (lu %v)", blue, paper)
	}
	red := color.RGBAModel.Convert(img.At(4*70, 4*20)).(color.RGBA)
	if red.R < 180 || red.G > 110 || red.B > 110 {
		return fmt.Errorf("l'image PNG incorporée n'apparaît pas : le carré devrait être rouge (lu %v). Vérifiez que gdk-pixbuf et ses chargeurs d'images sont installés", red)
	}
	ok("rendu en image, image PNG incorporée comprise")

	a4, found := units.Lookup("A4")
	if !found {
		return errors.New("format A4 introuvable")
	}
	sheetW, sheetH := a4.Oriented(false)
	s, err := units.MMPerUnit(150, w)
	if err != nil {
		return err
	}
	page := engine.PageSpec{
		WidthPt: units.MMToPt(sheetW), HeightPt: units.MMToPt(sheetH),
		BandTop: 0, BandBottom: h,
		OriginXPt: units.MMToPt(30), OriginYPt: units.MMToPt(10),
		PtPerUnit: units.MMToPt(s),
	}
	var first, second bytes.Buffer
	for _, buf := range []*bytes.Buffer{&first, &second} {
		if err := d.ExportPDF(buf, []engine.PageSpec{page}, engine.PDFOptions{WhitePage: true}); err != nil {
			return err
		}
	}
	pdf := bytes.TrimRight(first.Bytes(), "\r\n ")
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || !bytes.HasSuffix(pdf, []byte("%%EOF")) {
		return errors.New("le fichier produit n'est pas un PDF complet")
	}
	ok(fmt.Sprintf("export PDF d'une page A4 (%d octets)", first.Len()))

	if !bytes.Contains(pdf, []byte("/FontFile")) {
		return errors.New("aucune police n'a été incorporée au PDF : le texte serait absent. Vérifiez qu'une police est installée et que fontconfig la trouve")
	}
	ok("police incorporée dans le PDF (le texte reste du texte)")

	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		return errors.New("deux exports successifs du même document ne sont pas identiques")
	}
	ok("deux exports successifs identiques")

	fmt.Fprintln(out, "Tout fonctionne.")
	return nil
}

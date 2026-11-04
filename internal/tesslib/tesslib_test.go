// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package tesslib

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/internal/models"
	"github.com/moliri-app/transcribe/internal/ocr"
	"github.com/moliri-app/transcribe/internal/ocr/ocrtest"
)

// TestTestSetLibrary downloads the library and models on first use, as the app does, into
// TRANSCRIBE_OCR_DATA (kept between runs; the test is skipped without it, so a plain go test needs
// no network), and reads the test set with them.
func TestTestSetLibrary(t *testing.T) {
	dir := os.Getenv("TRANSCRIBE_OCR_DATA")
	if dir == "" {
		t.Skip("TRANSCRIBE_OCR_DATA not set")
	}
	if !models.TesseractLibSupported() {
		t.Skip("no Tesseract download on this platform")
	}
	lib := New(dir)
	tess := &ocr.Tesseract{
		TessData:     models.TesseractDataDir(dir),
		Lib:          lib,
		Downloadable: models.TesseractModels(),
		Fetch:        func(ctx context.Context, m string) error { return models.FetchTesseractModel(ctx, dir, m) },
	}
	reg := ocr.NewRegistry("desktop", tess)
	if info := tess.Info(); !info.Usable || info.Boxes != "words" {
		t.Fatalf("info: %+v", info)
	}
	ocrtest.Run(t, reg, "tesseract")
	info := tess.Info()
	if !info.Ready || !strings.HasPrefix(info.Version, "5.") || len(info.Languages) < 2 {
		t.Errorf("after reading: %+v", info)
	}

	// a reading that is cancelled stops Tesseract and says so
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e, o, _ := reg.Prepare("tesseract", ocr.Options{Languages: "deu+frak2021"})
	if _, err := reg.Read(ctx, e, ocrtest.Dir()+"/fraktur.jpg", o, 1); err == nil {
		t.Error("cancelled reading returned a result")
	}
	// a file that is no image
	if _, err := reg.Read(context.Background(), e, ocrtest.Dir()+"/fraktur.gt.txt", o, 1); err == nil {
		t.Error("text file read as an image")
	}
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr_test

import (
	"context"
	"os"
	"testing"

	"github.com/moliri-app/transcribe/ocr"
	"github.com/moliri-app/transcribe/ocr/ocrtest"
	"github.com/moliri-app/transcribe/ort"
)

// PPOCRCases are the bounds of PP-OCRv5 on the test set. It reads typed text better than
// Tesseract and Fraktur worse (no Fraktur model).
var PPOCRCases = []ocrtest.Case{
	{Image: "karteikarte", Languages: "deu", Layout: "lines", MaxCharErrors: 0.02, MinWordsFound: 0.9},
	{Image: "etikett", Languages: "deu", Layout: "block", MaxCharErrors: 0.02, MinWordsFound: 0.9},
	{Image: "fraktur", Languages: "deu", Layout: "auto", MaxCharErrors: 0.06, MinWordsFound: 0.8},
}

// TestTestSetPPOCR reads the test set with PP-OCRv5 when an onnxruntime library
// (TRANSCRIBE_ORT_LIB: its folder, e.g. <data>/lib) and the models (TRANSCRIBE_OCR_MODELS: their
// folder, e.g. <data>/ppocr) are given. TRANSCRIBE_ORT_CUDA=1 runs it on the graphics card (the
// lib-cuda runtime, with the NVIDIA libraries on the PATH).
func TestTestSetPPOCR(t *testing.T) {
	lib, dir := os.Getenv("TRANSCRIBE_ORT_LIB"), os.Getenv("TRANSCRIBE_OCR_MODELS")
	if lib == "" || dir == "" {
		t.Skip("TRANSCRIBE_ORT_LIB and TRANSCRIBE_OCR_MODELS not set")
	}
	cuda := os.Getenv("TRANSCRIBE_ORT_CUDA") != ""
	nets := &ort.Nets{Runtime: func(context.Context) (string, bool, error) { return lib, cuda, nil }}
	p := &ocr.PPOCR{Nets: nets, ModelDir: dir}
	if !p.Info().Ready && !p.Info().Usable {
		t.Fatalf("not usable: %+v", p.Info())
	}
	ocrtest.RunCases(t, ocr.NewRegistry("test", p), "onnx", PPOCRCases)
}

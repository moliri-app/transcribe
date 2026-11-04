// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ort

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRun runs the detection model of PP-OCRv5 on an empty image. It needs an onnxruntime library
// (TRANSCRIBE_ORT_LIB: its folder, e.g. <data>/lib) and the model (TRANSCRIBE_OCR_MODELS: the
// folder holding ch_PP-OCRv5_det_mobile.onnx, e.g. <data>/models/ocr).
func TestRun(t *testing.T) {
	lib, dir := os.Getenv("TRANSCRIBE_ORT_LIB"), os.Getenv("TRANSCRIBE_OCR_MODELS")
	if lib == "" || dir == "" {
		t.Skip("TRANSCRIBE_ORT_LIB and TRANSCRIBE_OCR_MODELS not set")
	}
	rt, err := Load(lib)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("onnxruntime", rt.Version)
	s, err := rt.NewSession(filepath.Join(dir, "ch_PP-OCRv5_det_mobile.onnx"), 2, os.Getenv("TRANSCRIBE_ORT_CUDA") != "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	in := make([]float32, 3*64*96)
	out, shape, err := s.Run(in, []int64{1, 3, 64, 96})
	if err != nil {
		t.Fatal(err)
	}
	if len(shape) != 4 || shape[2] != 64 || shape[3] != 96 || len(out) != 64*96 {
		t.Fatalf("shape %v, %d values", shape, len(out))
	}
	t.Logf("gpu %v, out shape %v", s.GPU, shape)
}

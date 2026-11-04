// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr_test

import (
	"os/exec"
	"testing"

	"github.com/moliri-app/transcribe/internal/ocr"
	"github.com/moliri-app/transcribe/internal/ocr/ocrtest"
)

// TestTestSetProgram reads the test set with the tesseract program when it is installed with the
// models the cases need (deu, frak2021). The library on Windows: internal/tesslib.
func TestTestSetProgram(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract not installed")
	}
	tess := &ocr.Tesseract{}
	if _, err := tess.Clean(ocr.Options{Languages: "deu+frak2021"}); err != nil {
		t.Skip(err)
	}
	ocrtest.Run(t, ocr.NewRegistry("test", tess), "tesseract")
}

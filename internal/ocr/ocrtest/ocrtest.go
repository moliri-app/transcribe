// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package ocrtest runs the OCR test set (testdata/ocr, in Moliri's copy server/transcribe/testdata/ocr)
// through an engine and checks the character errors and words found
// against what the server's Tesseract reaches.
package ocrtest

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/internal/ocr"
)

// Case is one image of the test set read with one setting, with the worst result still accepted.
type Case struct {
	Image, Languages, Layout string
	MaxCharErrors            float64 // 0–1
	MinWordsFound            float64 // 0–1
}

// Cases are the settings Moliri offers (server/transcribe/README.md, "OCR test set"): the server's
// Tesseract 5.3 read them with 2.4 %, 0.0 % and 1.5 % character errors and 88 %, 100 %, 92 % of the
// words; the bounds leave room for another Tesseract version.
var Cases = []Case{
	{"karteikarte", "deu", "lines", 0.05, 0.85},
	{"karteikarte", "deu+frak2021", "lines", 0.05, 0.85},
	{"etikett", "deu+frak2021", "block", 0.03, 0.9},
	{"fraktur", "deu+frak2021", "auto", 0.04, 0.85},
}

// Dir is the directory of the test images and their ground truth: testdata/ocr at the root of the
// module, here and in the server's copy.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "ocr")
}

// Run reads every case with the engine of the registry and checks the result format and quality.
func Run(t *testing.T, reg *ocr.Registry, engine string) {
	t.Helper()
	RunCases(t, reg, engine, Cases)
}

// RunCases reads the given cases, for an engine with bounds of its own.
func RunCases(t *testing.T, reg *ocr.Registry, engine string, cases []Case) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.Image+"/"+c.Languages+"/"+c.Layout, func(t *testing.T) {
			gt, err := os.ReadFile(filepath.Join(Dir(), c.Image+".gt.txt"))
			if err != nil {
				t.Fatal(err)
			}
			e, o, err := reg.Prepare(engine, ocr.Options{Languages: c.Languages, Layout: c.Layout})
			if err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			res, err := reg.Read(context.Background(), e, filepath.Join(Dir(), c.Image+".jpg"), o, 4)
			if err != nil {
				t.Fatal(err)
			}
			cer, found := ocr.Score(string(gt), res.Text())
			t.Logf("%s %s %s: %.1f %% character errors, %.0f %% words found, %d words, %.1f s (%s)",
				c.Image, c.Languages, c.Layout, cer*100, found*100, res.Words(), time.Since(start).Seconds(), res.Engine)
			if cer > c.MaxCharErrors || found < c.MinWordsFound {
				t.Errorf("worse than the server: %.1f %% errors (max %.1f %%), %.0f %% words (min %.0f %%)",
					cer*100, c.MaxCharErrors*100, found*100, c.MinWordsFound*100)
			}
			if len(res.Pages) != 1 || res.Pages[0].Width == 0 {
				t.Errorf("pages: %d", len(res.Pages))
			}
		})
	}
}

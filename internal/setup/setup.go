// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package setup prepares what the app needs beside the engine's models: ffmpeg, the GPU runtime
// and NVIDIA's libraries, Tesseract and PP-OCR for text recognition, EmbeddingGemma for Moliri's
// field search; it lists them all for the settings page and downloads them on request.
package setup

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/moliri-app/transcribe/models"
)

func init() {
	for name, label := range map[string]string{
		"runtime-cuda": "GPU-Runtime (CUDA)",
		"nvidia":       "NVIDIA-Bibliotheken (CUDA, cuDNN)",
		"ffmpeg":       "ffmpeg (Audio/Video)",
		"tesseract":    "Texterkennung (Tesseract)",
		// the Tesseract models, fetched on first use
		"tesseract-deu":      "Texterkennung Deutsch",
		"tesseract-eng":      "Texterkennung Englisch",
		"tesseract-frk":      "Texterkennung Fraktur (frk)",
		"tesseract-Fraktur":  "Texterkennung Fraktur-Schrift",
		"tesseract-frak2021": "Texterkennung Fraktur (frak2021)",
		"ppocr":              "Texterkennung (PP-OCRv5)",
		"embed":              "Suche nach Bedeutung (EmbeddingGemma)",
	} {
		models.SetLabel(name, label)
	}
}

var allMu sync.Mutex

// EnsureAll prepares everything a typical run needs. Whisper turbo (other languages, gap filling) stays on demand.
// Without models (speech is recognised by an external service), that is only ffmpeg. One runs at a time.
func EnsureAll(ctx context.Context, dataDir string, withModels bool) error {
	allMu.Lock()
	defer allMu.Unlock()
	s := models.Set{Dir: filepath.Join(dataDir, "models")}
	parts := []string{"ffmpeg"}
	var names []string
	if withModels {
		names = []string{"ced", "segmentation", "campplus", "silero", "parakeet", "german", "whisper-base"}
		parts = append([]string{"runtime", "ffmpeg"}, names...)
	}
	for _, n := range parts {
		models.Track(n, "pending", 0, 0, nil)
	}
	step := func(name string, f func() error) error {
		models.Track(name, "downloading", 0, 0, nil)
		if err := f(); err != nil {
			models.Track(name, "error", 0, 0, err)
			return err
		}
		models.Track(name, "done", 0, 0, nil)
		return nil
	}
	if withModels {
		if err := step("runtime", func() error { return models.EnsureLib(ctx, filepath.Join(dataDir, "lib"), nil) }); err != nil {
			return err
		}
	}
	if err := step("ffmpeg", func() error { return EnsureFFmpeg(ctx, filepath.Join(dataDir, "bin"), nil) }); err != nil {
		return err
	}
	for _, n := range names {
		n := n
		if err := step(n, func() error { return s.Ensure(ctx, nil, n) }); err != nil {
			return err
		}
	}
	return nil
}

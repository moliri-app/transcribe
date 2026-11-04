// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package models

import (
	"context"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// Status describes one component the app needs (runtime library, ffmpeg, a model).
type Status struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	State string `json:"state"` // pending | downloading | done | error
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Error string `json:"error,omitempty"`
}

var (
	trMu    sync.Mutex
	trItems []*Status
)

var labels = map[string]string{
	"runtime":       "Erkennungs-Runtime (sherpa-onnx)",
	"runtime-cuda":  "GPU-Runtime (CUDA)",
	"nvidia":        "NVIDIA-Bibliotheken (CUDA, cuDNN)",
	"ffmpeg":        "ffmpeg (Audio/Video)",
	"silero":        "Sprachaktivität (Silero VAD)",
	"segmentation":  "Sprechertrennung (pyannote)",
	"campplus":      "Sprecher-Embeddings (CAM++)",
	"ced":           "Geräuscherkennung (CED)",
	"parakeet":      "Spracherkennung (Parakeet v3)",
	"german":        "Deutsch-Modell (FastConformer)",
	"kroko":         "Deutsch live (Kroko)",
	"whisper-base":  "Spracherkennung (Whisper base)",
	"whisper-turbo": "Fremdsprachen (Whisper turbo)",
	"tesseract":     "Texterkennung (Tesseract)",
	// the Tesseract models, fetched on first use
	"tesseract-deu":      "Texterkennung Deutsch",
	"tesseract-eng":      "Texterkennung Englisch",
	"tesseract-frk":      "Texterkennung Fraktur (frk)",
	"tesseract-Fraktur":  "Texterkennung Fraktur-Schrift",
	"tesseract-frak2021": "Texterkennung Fraktur (frak2021)",
	"ppocr":              "Texterkennung (PP-OCRv5)",
}

func track(name, state string, done, total int64, err error) {
	trMu.Lock()
	defer trMu.Unlock()
	var s *Status
	for _, it := range trItems {
		if it.Name == name {
			s = it
		}
	}
	if s == nil {
		l := labels[name]
		if l == "" {
			l = name
		}
		s = &Status{Name: name, Label: l}
		trItems = append(trItems, s)
	}
	s.State = state
	if total > 0 {
		s.Done, s.Total = done, total
	}
	s.Error = ""
	if err != nil {
		s.Error = err.Error()
	}
}

// Snapshot returns the current state of all known components and whether everything is ready.
func Snapshot() (items []Status, ready bool) {
	trMu.Lock()
	defer trMu.Unlock()
	ready = len(trItems) > 0
	for _, it := range trItems {
		items = append(items, *it)
		if it.State != "done" {
			ready = false
		}
	}
	return
}

type countingReader struct {
	r     io.Reader
	name  string
	done  atomic.Int64
	total int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		track(c.name, "downloading", c.done.Add(int64(n)), c.total, nil)
	}
	return n, err
}

func counted(name string, r io.Reader, total int64) io.Reader {
	track(name, "downloading", 0, total, nil)
	return &countingReader{r: r, name: name, total: total}
}

var allMu sync.Mutex

// EnsureAll prepares everything a typical run needs. Whisper turbo (other languages, gap filling) stays on demand.
// Without models (speech is recognised by an external service), that is only ffmpeg. One runs at a time.
func EnsureAll(ctx context.Context, dataDir string, withModels bool) error {
	allMu.Lock()
	defer allMu.Unlock()
	s := Set{Dir: filepath.Join(dataDir, "models")}
	parts := []string{"ffmpeg"}
	var names []string
	if withModels {
		names = []string{"ced", "segmentation", "campplus", "silero", "parakeet", "german", "whisper-base"}
		parts = append([]string{"runtime", "ffmpeg"}, names...)
	}
	for _, n := range parts {
		track(n, "pending", 0, 0, nil)
	}
	step := func(name string, f func() error) error {
		track(name, "downloading", 0, 0, nil)
		if err := f(); err != nil {
			track(name, "error", 0, 0, err)
			return err
		}
		track(name, "done", 0, 0, nil)
		return nil
	}
	if withModels {
		if err := step("runtime", func() error { return EnsureLib(ctx, filepath.Join(dataDir, "lib"), nil) }); err != nil {
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

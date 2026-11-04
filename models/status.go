// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package models

import (
	"io"
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
	"silero":        "Sprachaktivität (Silero VAD)",
	"segmentation":  "Sprechertrennung (pyannote)",
	"campplus":      "Sprecher-Embeddings (CAM++)",
	"ced":           "Geräuscherkennung (CED)",
	"parakeet":      "Spracherkennung (Parakeet v3)",
	"german":        "Deutsch-Modell (FastConformer)",
	"kroko":         "Deutsch live (Kroko)",
	"whisper-base":  "Spracherkennung (Whisper base)",
	"whisper-turbo": "Fremdsprachen (Whisper turbo)",
}

// Label is the name of a component as the settings page shows it.
func Label(name string) string {
	trMu.Lock()
	defer trMu.Unlock()
	return labels[name]
}

// SetLabel names a component of the app beside the engine's own (runtime and models) in Snapshot.
func SetLabel(name, label string) {
	trMu.Lock()
	defer trMu.Unlock()
	labels[name] = label
}

// Track records the state of a component being prepared (pending, downloading, done, error), for Snapshot.
func Track(name, state string, done, total int64, err error) {
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
		Track(c.name, "downloading", c.done.Add(int64(n)), c.total, nil)
	}
	return n, err
}

// Counted wraps the download r of the component name so that Snapshot shows how far it got.
func Counted(name string, r io.Reader, total int64) io.Reader {
	Track(name, "downloading", 0, total, nil)
	return &countingReader{r: r, name: name, total: total}
}

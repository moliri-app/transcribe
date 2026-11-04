// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package models locates (and downloads) the model files the pipeline needs.
package models

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/moliri-app/transcribe/proc"
)

const base = "https://github.com/k2-fsa/sherpa-onnx/releases/download/"

// Set is the models in Dir. Offline: a model that is missing is an error, not a download (the
// server has its models in its image).
type Set struct {
	Dir     string
	Offline bool
}

func (s Set) p(parts ...string) string { return filepath.Join(append([]string{s.Dir}, parts...)...) }

func (s Set) SileroVAD() string    { return s.p("silero_vad.onnx") }
func (s Set) Segmentation() string { return s.p("sherpa-onnx-pyannote-segmentation-3-0", "model.onnx") }
func (s Set) CampPlus() string {
	return s.p("3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx")
}
func (s Set) CEDModel() string {
	return s.p("sherpa-onnx-ced-small-audio-tagging-2024-04-19", "model.int8.onnx")
}
func (s Set) CEDLabels() string {
	return s.p("sherpa-onnx-ced-small-audio-tagging-2024-04-19", "class_labels_indices.csv")
}

type Transducer struct{ Encoder, Decoder, Joiner, Tokens string }

func (s Set) Parakeet() Transducer {
	d := s.p("sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8")
	return Transducer{filepath.Join(d, "encoder.int8.onnx"), filepath.Join(d, "decoder.int8.onnx"), filepath.Join(d, "joiner.int8.onnx"), filepath.Join(d, "tokens.txt")}
}

func (s Set) GermanFastConformer() Transducer {
	d := s.p("sherpa-onnx-nemo-transducer-stt_de_fastconformer_hybrid_large_pc-int8")
	return Transducer{filepath.Join(d, "encoder.int8.onnx"), filepath.Join(d, "decoder.int8.onnx"), filepath.Join(d, "joiner.int8.onnx"), filepath.Join(d, "tokens.txt")}
}

// Kroko is Banafo's streaming Zipformer for German, which live transcription of German shows
// while it is spoken.
func (s Set) Kroko() Transducer {
	d := s.p("sherpa-onnx-streaming-zipformer-de-kroko-2025-08-06")
	return Transducer{filepath.Join(d, "encoder.onnx"), filepath.Join(d, "decoder.onnx"), filepath.Join(d, "joiner.onnx"), filepath.Join(d, "tokens.txt")}
}

type Whisper struct{ Encoder, Decoder, Tokens string }

func (s Set) Whisper(name string) Whisper {
	d := s.p("sherpa-onnx-whisper-" + name)
	return Whisper{filepath.Join(d, name+"-encoder.int8.onnx"), filepath.Join(d, name+"-decoder.int8.onnx"), filepath.Join(d, name+"-tokens.txt")}
}

type item struct {
	check string // file that must exist
	url   string
	dir   bool // tar.bz2 archive, else a single file
}

func (s Set) catalog() map[string]item {
	asr := base + "asr-models/"
	return map[string]item{
		"silero":        {s.SileroVAD(), asr + "silero_vad.onnx", false},
		"parakeet":      {s.Parakeet().Encoder, asr + "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2", true},
		"german":        {s.GermanFastConformer().Encoder, asr + "sherpa-onnx-nemo-transducer-stt_de_fastconformer_hybrid_large_pc-int8.tar.bz2", true},
		"kroko":         {s.Kroko().Encoder, asr + "sherpa-onnx-streaming-zipformer-de-kroko-2025-08-06.tar.bz2", true},
		"whisper-base":  {s.Whisper("base").Encoder, asr + "sherpa-onnx-whisper-base.tar.bz2", true},
		"whisper-turbo": {s.Whisper("turbo").Encoder, asr + "sherpa-onnx-whisper-turbo.tar.bz2", true},
		"segmentation":  {s.Segmentation(), base + "speaker-segmentation-models/sherpa-onnx-pyannote-segmentation-3-0.tar.bz2", true},
		"campplus":      {s.CampPlus(), base + "speaker-recongition-models/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx", false},
		"ced":           {s.CEDModel(), base + "audio-tagging-models/sherpa-onnx-ced-small-audio-tagging-2024-04-19.tar.bz2", true},
	}
}

// Have reports whether the named models have been downloaded. An empty file (the end-to-end
// tests put such placeholders in) is no model.
func (s Set) Have(names ...string) bool {
	cat := s.catalog()
	for _, n := range names {
		it, ok := cat[n]
		if !ok {
			return false
		}
		if fi, err := os.Stat(it.check); err != nil || fi.Size() == 0 {
			return false
		}
	}
	return true
}

// Ensure downloads the named models that are missing. progress receives a status line.
var ensureMu sync.Mutex

func (s Set) Ensure(ctx context.Context, progress func(string), names ...string) error {
	ensureMu.Lock()
	defer ensureMu.Unlock()
	cat := s.catalog()
	for _, n := range names {
		it, ok := cat[n]
		if !ok {
			return fmt.Errorf("unknown model %q", n)
		}
		if _, err := os.Stat(it.check); err == nil {
			continue
		}
		if s.Offline {
			return fmt.Errorf("model %s is not installed (%s missing)", n, it.check)
		}
		if progress != nil {
			progress("downloading " + n)
		}
		if err := os.MkdirAll(s.Dir, 0o755); err != nil {
			return err
		}
		if err := s.fetch(ctx, n, it); err != nil {
			Track(n, "error", 0, 0, err)
			return fmt.Errorf("download %s: %w", n, err)
		}
		Track(n, "done", 0, 0, nil)
		if _, err := os.Stat(it.check); err != nil {
			return fmt.Errorf("download %s: expected %s after unpacking", n, it.check)
		}
	}
	return nil
}

func (s Set) fetch(ctx context.Context, name string, it item) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", it.url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: %s", it.url, resp.Status)
	}
	if !it.dir {
		tmp := it.check + ".part"
		f, err := os.Create(tmp)
		if err != nil {
			return err
		}
		if _, err = io.Copy(f, Counted(name, resp.Body, resp.ContentLength)); err != nil {
			f.Close()
			return err
		}
		f.Close()
		return os.Rename(tmp, it.check)
	}
	// tar is available on Windows 10+, Linux and macOS; it handles bzip2
	cmd := proc.Hide(exec.CommandContext(ctx, "tar", "-xjf", "-", "-C", s.Dir))
	cmd.Stdin = Counted(name, resp.Body, resp.ContentLength)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar: %w: %s", err, out)
	}
	return nil
}

// Installed lists the models present and missing, each sorted by name.
func (s Set) Installed() (present, missing []string) {
	present, missing = []string{}, []string{}
	for n, it := range s.catalog() {
		if _, err := os.Stat(it.check); err == nil {
			present = append(present, n)
		} else {
			missing = append(missing, n)
		}
	}
	sort.Strings(present)
	sort.Strings(missing)
	return present, missing
}

// Locate returns the file that must exist for the named model and the folder (or file) it lives
// in below Dir; ok is false for an unknown name.
func (s Set) Locate(name string) (check, root string, ok bool) {
	it, ok := s.catalog()[name]
	if !ok {
		return "", "", false
	}
	return it.check, s.modelRoot(it), true
}

// modelRoot is the folder (archives) or file (single downloads) that holds a model.
func (s Set) modelRoot(it item) string {
	rel, err := filepath.Rel(s.Dir, it.check)
	if err != nil || strings.HasPrefix(rel, "..") {
		return it.check
	}
	return filepath.Join(s.Dir, strings.Split(rel, string(filepath.Separator))[0])
}

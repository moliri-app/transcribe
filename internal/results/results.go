// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package results keeps the transcripts of the app in <data>/results/<stem>/: the machine result
// (_machine.json), the human corrections (_annotations.json), the cache of language
// re-recognitions (_retranscribed.json) and the final transcript (_final.json). The engine
// (packages annotate, export, pipeline) works on the data; this package reads and writes it.
package results

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moliri-app/transcribe/annotate"
	"github.com/moliri-app/transcribe/export"
	"github.com/moliri-app/transcribe/fileutil"
	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/model"
)

var ErrNoTranscript = errors.New("no transcript yet")

// Dir is the folder of a stem's results.
func Dir(dataDir, stem string) string { return filepath.Join(dataDir, "results", stem) }

// LoadAnnotations reads the annotations of a stem; a missing file means none.
func LoadAnnotations(dataDir, stem string) (*annotate.Annotations, error) {
	b, err := os.ReadFile(filepath.Join(Dir(dataDir, stem), "_annotations.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return annotate.ParseAnnotations(b)
}

// loadCache reads the language re-recognitions of earlier runs; a missing file means none.
func loadCache(dataDir, stem string) (map[string]json.RawMessage, error) {
	cache := map[string]json.RawMessage{}
	if err := fileutil.ReadJSON(filepath.Join(Dir(dataDir, stem), "_retranscribed.json"), &cache); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return cache, nil
}

// saveCache writes the re-recognition cache when it grew or was not there yet.
func saveCache(dataDir, stem string, old, cache map[string]json.RawMessage) error {
	path := filepath.Join(Dir(dataDir, stem), "_retranscribed.json")
	if _, err := os.Stat(path); len(cache) != len(old) || err != nil {
		return fileutil.WriteJSON(path, cache)
	}
	return nil
}

// Apply writes results/<stem>/_final.json: machine result + language re-transcription (if rt is set) + text
// edits + speaker spans + merges + one speaker per sentence. With rt == nil, language regions are skipped
// unless an earlier run left them in _retranscribed.json.
func Apply(ctx context.Context, dataDir, stem string, rt annotate.Retranscriber) error {
	d := Dir(dataDir, stem)
	raw, err := os.ReadFile(filepath.Join(d, "_machine.json"))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("no _machine.json in %s (run the recognition first)", d)
	} else if err != nil {
		return err
	}
	ann, err := LoadAnnotations(dataDir, stem)
	if err != nil {
		return fmt.Errorf("_annotations.json: %w", err)
	}
	cache, err := loadCache(dataDir, stem)
	if err != nil {
		return fmt.Errorf("_retranscribed.json: %w", err)
	}
	in := annotate.Input{Machine: raw, Annotations: ann, Retranscribed: cache}
	if rt != nil {
		in.Audio = media.FindMedia(dataDir, stem)
	}
	final, newCache, err := annotate.Apply(ctx, in, rt)
	if err != nil {
		return err
	}
	if rt != nil && in.Audio == "" && final["languages_pending"] != 0 {
		return fmt.Errorf("audio for %q not found", stem)
	}
	if err := saveCache(dataDir, stem, cache, newCache); err != nil {
		return err
	}
	return fileutil.WriteJSON(filepath.Join(d, "_final.json"), final)
}

// Save writes what a recognition run returned: _machine.json, _final.json and the re-recognition
// cache (old is the cache the run started from).
func Save(dataDir, stem string, res *model.Result, old map[string]json.RawMessage) error {
	d := Dir(dataDir, stem)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	if err := fileutil.WriteJSON(filepath.Join(d, "_machine.json"), res.Machine); err != nil {
		return err
	}
	if err := saveCache(dataDir, stem, old, res.Retranscribed); err != nil {
		return err
	}
	return fileutil.WriteJSON(filepath.Join(d, "_final.json"), res.Final)
}

// Inputs reads what a recognition run of stem puts on top of the machine result: the corrections
// and the re-recognition cache.
func Inputs(dataDir, stem string) (*annotate.Annotations, map[string]json.RawMessage, error) {
	ann, err := LoadAnnotations(dataDir, stem)
	if err != nil {
		return nil, nil, fmt.Errorf("_annotations.json: %w", err)
	}
	cache, err := loadCache(dataDir, stem)
	if err != nil {
		return nil, nil, fmt.Errorf("_retranscribed.json: %w", err)
	}
	return ann, cache, nil
}

// Load reads the final transcript and the annotations of a stem.
func Load(dataDir, stem string) (*export.Final, *annotate.Annotations, error) {
	var final export.Final
	if err := fileutil.ReadJSON(filepath.Join(Dir(dataDir, stem), "_final.json"), &final); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, ErrNoTranscript
		}
		return nil, nil, err
	}
	ann, err := LoadAnnotations(dataDir, stem)
	if err != nil {
		return nil, nil, err
	}
	return &final, ann, nil
}

// Export returns the content, content type and file name of a stem in the given format.
func Export(dataDir, stem, format string) ([]byte, string, string, error) {
	if _, ok := export.Formats[format]; !ok {
		return nil, "", "", fmt.Errorf("unknown format %q", format)
	}
	final, ann, err := Load(dataDir, stem)
	if err != nil {
		return nil, "", "", err
	}
	body, ctype, err := export.Render(format, final, ann, "")
	return body, ctype, media.Title(stem) + "." + format, err
}

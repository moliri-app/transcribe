// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sidecar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/internal/results"
)

// connect makes a data folder of its own that connects folder, as one member of a team would.
func connect(t *testing.T, folder string) string {
	t.Helper()
	data := t.TempDir()
	if _, err := prefs.Update(data, func(p *prefs.Prefs) { p.Folders = []string{folder} }); err != nil {
		t.Fatal(err)
	}
	return data
}

func put(t *testing.T, path, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTwoPeopleShareAFolder(t *testing.T) {
	folder := t.TempDir()
	audio := filepath.Join(folder, "Interviews", "Papierfabrik.mp3")
	put(t, audio, "not really audio")

	anna := connect(t, folder)
	linked := media.LinkedIn(anna)
	if len(linked) != 1 || linked[0].Path != audio || linked[0].Dir != "Interviews" || !media.IsLinked(linked[0].Name) {
		t.Fatalf("linked = %+v", linked)
	}
	stem := media.Stem(linked[0].Name)
	if media.Title(stem) != "Papierfabrik" || media.FindMedia(anna, stem) != audio || media.Path(anna, linked[0].Name) != audio {
		t.Fatalf("title %q, find %q", media.Title(stem), media.FindMedia(anna, stem))
	}

	// Anna transcribes and corrects: her copy goes next to the recording
	d := results.Dir(anna, stem)
	put(t, filepath.Join(d, "_machine.json"), `{"audio": "x", "segments": []}`)
	put(t, filepath.Join(d, "_final.json"), `{"audio": "x", "segments": [{"text": "Guten Tag"}]}`)
	put(t, filepath.Join(d, "_annotations.json"), `{"speakers": {"0": {"name": "Anna"}}, "spans": [], "edits": []}`)
	if err := Write(anna, stem); err != nil {
		t.Fatal(err)
	}
	var f File
	b, err := os.ReadFile(Path(audio))
	if err != nil || json.Unmarshal(b, &f) != nil || f.Format != Format || f.Audio != "Papierfabrik.mp3" || !strings.Contains(string(f.Final), "Guten Tag") {
		t.Fatalf("sidecar %s, %v", b, err)
	}
	if took, err := Read(anna, stem); took || err != nil {
		t.Fatalf("Anna read her own sidecar back: %v %v", took, err)
	}

	// Ben connects the same folder: he finds Anna's transcript without transcribing
	ben := connect(t, folder)
	benStem := media.Stem(media.LinkedIn(ben)[0].Name)
	if took, err := Read(ben, benStem); !took || err != nil {
		t.Fatalf("Ben did not take the sidecar: %v %v", took, err)
	}
	if _, ann, err := results.Load(ben, benStem); err != nil || ann.Speakers["0"].Name != "Anna" {
		t.Fatalf("Ben's copy: %+v %v", ann, err)
	}

	// Ben corrects; Anna gets it, her earlier corrections stay in the history
	put(t, filepath.Join(results.Dir(ben, benStem), "_annotations.json"), `{"speakers": {"0": {"name": "Anna Berg"}}, "spans": [], "edits": []}`)
	time.Sleep(20 * time.Millisecond) // a later time on the sidecar
	if err := Write(ben, benStem); err != nil {
		t.Fatal(err)
	}
	if took, err := Read(anna, stem); !took || err != nil {
		t.Fatalf("Anna did not take Ben's change: %v %v", took, err)
	}
	if _, ann, _ := results.Load(anna, stem); ann.Speakers["0"].Name != "Anna Berg" {
		t.Fatalf("Anna's copy: %+v", ann)
	}
	hist, _ := filepath.Glob(filepath.Join(d, "_annotations_history", "*.json"))
	if len(hist) != 1 {
		t.Fatalf("history %v", hist)
	}
}

func TestNoSidecarWithoutTranscript(t *testing.T) {
	folder := t.TempDir()
	audio := filepath.Join(folder, "Neu.wav")
	put(t, audio, "x")
	data := connect(t, folder)
	stem := media.Stem(media.LinkedIn(data)[0].Name)
	if err := Write(data, stem); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(Path(audio)); !os.IsNotExist(err) {
		t.Fatalf("a sidecar without a transcript: %v", err)
	}
}

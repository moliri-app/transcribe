// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/internal/prefs"
)

func TestNotesOfProgramsNotSaved(t *testing.T) {
	dir := t.TempDir()
	keep := transientKeep
	transientKeep = 50 * time.Millisecond
	defer func() { transientKeep = keep }()
	h := NewHub(dir, func(context.Context, string) (Stream, error) { return &fakeStream{}, nil })
	no := false
	if err := h.SetAPINotes(&no, nil); err != nil {
		t.Fatal(err)
	}
	s, err := h.Audio(context.Background(), "Moliri", func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	for range 15 {
		s.Write(make([]float32, SR/10))
	}
	s.End(nil)

	// readable for a while, then gone; never on disk
	if n := h.Note(s.Note()); n == nil || !n.Transient || n.Text() != "Satz\nRest\n" {
		t.Fatalf("note right after it ended: %+v", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "live", s.Note()+".json")); !os.IsNotExist(err) {
		t.Errorf("note saved: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for h.Note(s.Note()) != nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if h.Note(s.Note()) != nil || len(h.Notes()) != 0 {
		t.Error("the note stays")
	}
}

func TestOldNotesOfProgramsDeleted(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-50 * time.Hour)
	ended := old.Add(time.Minute)
	recent := time.Now().Add(-time.Hour)
	for _, n := range []*Note{
		{ID: old.Format(idTime) + "-1", Kind: KindAudio, Started: old, Ended: &ended, Lines: []Line{}},
		{ID: old.Format(idTime) + "-2", Kind: KindMic, Started: old, Ended: &ended, Lines: []Line{}},
		{ID: old.Format(idTime) + "-3", Kind: KindDauer, Started: old, Ended: &ended, Lines: []Line{}},
		{ID: old.Format(idTime) + "-4", Kind: KindDiktat, Started: old, Ended: &ended, Lines: []Line{}},
		{ID: recent.Format(idTime) + "-5", Kind: KindAudio, Started: recent, Ended: &recent, Lines: []Line{}},
	} {
		if err := saveNote(dir, n); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHub(dir, nil)
	h.prune() // keeps everything by default
	if len(h.Notes()) != 5 {
		t.Fatalf("pruned without being asked: %d notes", len(h.Notes()))
	}
	hours := 24
	if err := h.SetAPINotes(nil, &hours); err != nil {
		t.Fatal(err)
	}
	h.prune()
	var left []string
	for _, n := range h.Notes() {
		left = append(left, n.Kind)
	}
	if len(left) != 3 || left[0] != KindDauer || left[1] != KindDiktat || left[2] != KindAudio {
		t.Errorf("left: %v", left)
	}
	if got := len(noteFiles(dir)); got != 3 {
		t.Errorf("%d files left", got)
	}
	if p := prefs.Load(dir); p.LiveAPIKeepHours != 24 || p.LiveAPINoSave {
		t.Errorf("settings: %+v", p)
	}
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/moliri-app/transcribe/jobs"
)

// Deleting a recording in the library takes its file and its results along.
func TestDeleteRecording(t *testing.T) {
	s, d := newServer(t)
	path := func(p ...string) string { return filepath.Join(append([]string{d}, p...)...) }
	_ = os.WriteFile(path("audio", "talk.wav"), []byte("RIFF"), 0o644) // results/talk/_final.json exists
	del := func(name string, hdr map[string]string) int {
		return do(s, "POST", "/api/delete?name="+url.QueryEscape(name), "", hdr).Code
	}

	if c := del("talk.wav", map[string]string{"Origin": "https://evil.example"}); c != 403 {
		t.Errorf("foreign page: %d", c)
	}
	for _, name := range []string{"", "../talk.wav", `..\talk.wav`, "notes.txt"} {
		if c := del(name, nil); c != 400 {
			t.Errorf("%q: %d", name, c)
		}
	}
	if c := del("missing.wav", nil); c != 404 {
		t.Errorf("missing: %d", c)
	}

	if c := del("talk.wav", nil); c != 200 {
		t.Fatalf("delete: %d", c)
	}
	for _, p := range []string{path("audio", "talk.wav"), path("results", "talk")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Error("still there:", p)
		}
	}
}

// A recording waiting in the queue is cancelled there; one being transcribed is not deleted.
func TestDeleteQueuedRecording(t *testing.T) {
	d := t.TempDir()
	_ = os.MkdirAll(filepath.Join(d, "audio"), 0o755)
	for _, n := range []string{"a.mp3", "b.mp3"} {
		_ = os.WriteFile(filepath.Join(d, "audio", n), []byte("x"), 0o644)
	}
	started, release := make(chan struct{}), make(chan struct{})
	run := func(ctx context.Context, _, audio string, _ jobs.Options, _ jobs.Progress) error {
		close(started)
		<-release
		return nil
	}
	s := New(Config{DataDir: d, Runner: run})
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	if _, err := s.Queue.Add([]string{"a.mp3", "b.mp3"}, nil); err != nil {
		t.Fatal(err)
	}
	s.Start(ctx)
	<-started // a.mp3 runs, b.mp3 waits

	if w := do(s, "POST", "/api/delete?name=a.mp3", "", nil); w.Code != 409 {
		t.Errorf("running: %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(filepath.Join(d, "audio", "a.mp3")); err != nil {
		t.Error("the running recording went:", err)
	}
	if w := do(s, "POST", "/api/delete?name=b.mp3", "", nil); w.Code != 200 {
		t.Errorf("queued: %d %s", w.Code, w.Body)
	}
	for _, j := range s.Queue.List() {
		if j.Audio == "b.mp3" {
			t.Error("its job stayed:", j.Status)
		}
	}
	close(release)
	stop()
	s.Queue.Wait()
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/embed"
	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/jobs"
)

// newEmbedServer has an embedding model whose files are missing; fetch is its download.
func newEmbedServer(t *testing.T, fetch func(context.Context) error) *Server {
	t.Helper()
	run := func(context.Context, string, string, jobs.Options, jobs.Progress) error { return nil }
	noLive := func(context.Context, string) (live.Stream, error) { return nil, errors.New("no live in tests") }
	return New(Config{DataDir: t.TempDir(), Runner: run, Live: noLive, Embed: &embed.Model{Dir: t.TempDir()}, EmbedFetch: fetch})
}

func TestEmbedInfo(t *testing.T) {
	s := newEmbedServer(t, nil)
	w := do(s, "GET", "/api/embed", "", map[string]string{"Origin": "http://localhost:5173"})
	var info struct {
		OK        bool              `json:"ok"`
		Model     string            `json:"model"`
		Dtype     string            `json:"dtype"`
		Installed bool              `json:"installed"`
		Prefixes  map[string]string `json:"prefixes"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &info) != nil || !info.OK || info.Model != embed.ModelID ||
		info.Dtype != "q8" || info.Installed || info.Prefixes["query"] != embed.QueryPrefix {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("CORS: %v", w.Header())
	}
}

// TestEmbedOrigins: pages from elsewhere need the approval of the Live page, as for text recognition.
func TestEmbedOrigins(t *testing.T) {
	s := newEmbedServer(t, nil)
	museum := map[string]string{"Origin": "https://museum.moliri.app"}
	for _, path := range []string{"/api/embed", "/api/embed/download"} {
		w := do(s, "POST", path, `{"texts":["Künstler"],"kind":"query"}`, museum)
		if w.Code != 403 || !strings.Contains(w.Body.String(), "Suche nach Bedeutung") || !strings.Contains(w.Body.String(), "https://museum.moliri.app") {
			t.Errorf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	_ = s.Live.Allow("https://museum.moliri.app", true)
	if w := do(s, "GET", "/api/embed", "", museum); w.Code != 200 {
		t.Errorf("allowed: %d %s", w.Code, w.Body)
	}
}

func TestEmbedRequests(t *testing.T) {
	s := newEmbedServer(t, nil)
	for _, c := range []struct {
		body string
		code int
	}{
		{`{"texts":["Künstler"],"kind":"query"}`, 503}, // the model is not downloaded
		{`{"texts":["Künstler"],"kind":"title"}`, 400},
		{`{"texts":[],"kind":"document"}`, 400},
		{`{"texts":["` + strings.Repeat("x", maxEmbedText+1) + `"],"kind":"document"}`, 400},
		{`kein JSON`, 400},
	} {
		if w := do(s, "POST", "/api/embed", c.body, nil); w.Code != c.code {
			t.Errorf("%.40s: %d %s, want %d", c.body, w.Code, w.Body, c.code)
		}
	}
}

func TestEmbedDownload(t *testing.T) {
	started := make(chan struct{})
	s := newEmbedServer(t, func(context.Context) error { close(started); return errors.New("offline") })
	w := do(s, "POST", "/api/embed/download", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"downloading"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	<-started
}

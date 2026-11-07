// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/internal/llm"
	"github.com/moliri-app/transcribe/jobs"
)

// fakeModel answers with answer once release is closed (nil: at once), reading the prompt in
// two steps first.
type fakeModel struct {
	ready   bool
	answer  string
	release chan struct{}

	mu   sync.Mutex
	reqs []llm.Request
}

func (f *fakeModel) Ready() bool                { return f.ready }
func (f *fakeModel) Loaded() bool               { return false }
func (f *fakeModel) Warm(context.Context) error { return nil }
func (f *fakeModel) Chat(ctx context.Context, req llm.Request, progress func(llm.Progress)) (llm.Answer, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()
	progress(llm.Progress{Stage: "reading", Done: 512, Total: 1024})
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return llm.Answer{}, ctx.Err()
		}
	}
	return llm.Answer{Content: f.answer, Seconds: 61}, nil
}

func newFilterServer(t *testing.T, m *fakeModel) *Server {
	t.Helper()
	run := func(context.Context, string, string, jobs.Options, jobs.Progress) error { return nil }
	noLive := func(context.Context, string) (live.Stream, error) { return nil, errors.New("no live in tests") }
	return New(Config{DataDir: t.TempDir(), Runner: run, Live: noLive, Filter: m, FilterModel: "Qwen", FilterSizeMB: 4490})
}

const filterQuestion = `{"text": "Gemälde ohne Standort", "catalog": "Feldkatalog:\n- ` + "`Objektart`" + `\n- ` + "`Dauerstandort`" + `", "fields": ["Objektart", "Dauerstandort"]}`

// state polls the question until it leaves the waiting states or the time is up.
func state(t *testing.T, s *Server, id string, until func(map[string]any) bool) map[string]any {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; {
		w := do(s, "GET", "/api/filter?id="+id, "", nil)
		var out map[string]any
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		if until(out) || time.Now().After(deadline) {
			return out
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func ask(t *testing.T, s *Server, body string) string {
	t.Helper()
	w := do(s, "POST", "/api/filter", body, map[string]string{"Origin": "http://localhost:5173"})
	var out struct {
		OK bool   `json:"ok"`
		ID string `json:"id"`
	}
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &out) != nil || !out.OK || out.ID == "" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	return out.ID
}

func TestFilterAnswer(t *testing.T) {
	m := &fakeModel{ready: true, answer: `{"conditions": [
		{"join": "AND", "field": "Objektart", "comparison": "equals", "value": "Gemälde"},
		{"join": "AND", "field": "Dauerstandort", "comparison": "empty", "value": ""},
		{"join": "AND", "field": "Künstler", "comparison": "contains", "value": "Monet"}], "not_understood": []}`}
	s := newFilterServer(t, m)
	id := ask(t, s, filterQuestion)
	out := state(t, s, id, func(o map[string]any) bool { return o["status"] == "done" || o["status"] == "failed" })
	if out["status"] != "done" || out["model"] != "Qwen" || out["seconds"] != 61.0 {
		t.Fatalf("%v", out)
	}
	conds, _ := out["conditions"].([]any)
	if len(conds) != 2 || conds[1].(map[string]any)["comparison"] != "empty" {
		t.Errorf("conditions %v", out["conditions"])
	}
	if d, _ := out["dropped"].([]any); len(d) != 1 || d[0] != "Künstler" {
		t.Errorf("dropped %v", out["dropped"])
	}
	// The prompt is Moliri's, with the sentence last, and the catalog's beginning is kept.
	req := m.reqs[0]
	if c := req.Messages[1].Content; !req.JSON || req.CacheKey == "" || strings.Index(c, "Suchanfrage: „Gemälde ohne Standort“") < strings.Index(c, "Feldkatalog") {
		t.Errorf("%+v", req)
	}
}

// TestFilterQueue: one question at a time; the next one says how many are ahead and can be dropped.
func TestFilterQueue(t *testing.T) {
	m := &fakeModel{ready: true, answer: `{"conditions": []}`, release: make(chan struct{})}
	s := newFilterServer(t, m)
	first := ask(t, s, filterQuestion)
	out := state(t, s, first, func(o map[string]any) bool { return o["stage"] == "reading" })
	if p, _ := out["progress"].(map[string]any); out["status"] != "processing" || p["done"] != 512.0 || p["total"] != 1024.0 {
		t.Fatalf("%v", out)
	}
	second := ask(t, s, filterQuestion)
	if out := state(t, s, second, func(map[string]any) bool { return true }); out["status"] != "pending" || out["ahead"] != 1.0 {
		t.Errorf("second %v", out)
	}
	do(s, "POST", "/api/filter/cancel", `{"id": "`+second+`"}`, nil)
	if out := state(t, s, second, func(map[string]any) bool { return true }); out["status"] != "failed" || out["error_kind"] != "cancelled" {
		t.Errorf("dropped %v", out)
	}
	close(m.release)
	if out := state(t, s, first, func(o map[string]any) bool { return o["status"] == "done" }); out["status"] != "done" {
		t.Errorf("first %v", out)
	}
	if len(m.reqs) != 1 {
		t.Errorf("asked %d times", len(m.reqs))
	}
}

func TestFilterRefusals(t *testing.T) {
	s := newFilterServer(t, &fakeModel{ready: false})
	if w := do(s, "POST", "/api/filter", filterQuestion, nil); w.Code != 503 {
		t.Errorf("not installed: %d %s", w.Code, w.Body)
	}
	s = newFilterServer(t, &fakeModel{ready: true})
	if w := do(s, "POST", "/api/filter", `{"text": "", "catalog": "x", "fields": ["a"]}`, nil); w.Code != 400 {
		t.Errorf("no text: %d %s", w.Code, w.Body)
	}
	if w := do(s, "GET", "/api/filter?id=nope", "", nil); w.Code != 404 {
		t.Errorf("unknown: %d %s", w.Code, w.Body)
	}
	museum := map[string]string{"Origin": "https://museum.moliri.app"}
	for _, path := range []string{"/api/filter", "/api/filter/download", "/api/filter/cancel"} {
		w := do(s, "POST", path, filterQuestion, museum)
		if w.Code != 403 || !strings.Contains(w.Body.String(), "Filter in eigenen Worten") {
			t.Errorf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	_ = s.Live.Allow("https://museum.moliri.app", true)
	if w := do(s, "GET", "/api/filter", "", museum); w.Code != 200 || !strings.Contains(w.Body.String(), `"size":4490`) {
		t.Errorf("allowed: %d %s", w.Code, w.Body)
	}
}

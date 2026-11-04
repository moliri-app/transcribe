// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/internal/queue"
	"github.com/moliri-app/transcribe/jobs"
)

func newServer(t *testing.T) (*Server, string) {
	t.Helper()
	d := t.TempDir()
	for _, p := range []string{"audio", filepath.Join("results", "talk")} {
		_ = os.MkdirAll(filepath.Join(d, p), 0o755)
	}
	_ = os.WriteFile(filepath.Join(d, "audio", "Über uns ä.mp3"), []byte("0123456789"), 0o644)
	_ = os.WriteFile(filepath.Join(d, "results", "talk", "_final.json"), []byte(`{"segments":[]}`), 0o644)
	run := func(context.Context, string, string, jobs.Options, jobs.Progress) error { return nil }
	return New(Config{DataDir: d, Runner: run}), d
}

func do(s *Server, method, target string, body string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Host = "localhost:8765"
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestRange(t *testing.T) {
	s, _ := newServer(t)
	w := do(s, "GET", "/audio/"+pyQuote("Über uns ä.mp3"), "", map[string]string{"Range": "bytes=2-5"})
	if w.Code != 206 || w.Body.String() != "2345" || w.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Errorf("range: %d %q %v", w.Code, w.Body.String(), w.Header())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("missing no-store")
	}
	if w := do(s, "GET", "/audio/"+pyQuote("Über uns ä.mp3"), "", nil); w.Code != 200 || w.Body.Len() != 10 {
		t.Errorf("full: %d", w.Code)
	}
}

func TestPathTraversal(t *testing.T) {
	s, d := newServer(t)
	_ = os.WriteFile(filepath.Join(d, "secret.txt"), []byte("s"), 0o644)
	for _, p := range []string{
		"/audio/..%2Fsecret.txt", "/audio/%2e%2e%5csecret.txt", "/audio/..%5Csecret.txt", "/results/..%2Fsecret.txt",
		"/results/%2e%2e/secret.txt", "/audio/", "/results/talk/", "/audio/C:%5Cwindows%5Cwin.ini",
	} {
		if w := do(s, "GET", p, "", nil); w.Code == 200 {
			t.Errorf("%s served", p)
		}
	}
	for _, stem := range []string{"..", "..%2Ftalk", "a%5Cb", ""} {
		for _, ep := range []string{"/api/peaks", "/api/thumbs", "/api/export"} {
			if w := do(s, "GET", ep+"?format=txt&stem="+stem, "", nil); w.Code != 400 {
				t.Errorf("%s stem=%q: %d", ep, stem, w.Code)
			}
		}
		for _, ep := range []string{"/api/annotations", "/api/apply"} {
			if w := do(s, "POST", ep+"?stem="+stem, "{}", nil); w.Code != 400 {
				t.Errorf("%s stem=%q: %d", ep, stem, w.Code)
			}
		}
	}
}

func TestOriginCheck(t *testing.T) {
	s, _ := newServer(t)
	bad := []map[string]string{{"Origin": "http://evil.example"}, {"Origin": "null"}, {"Origin": ""}}
	for _, h := range bad {
		if w := do(s, "POST", "/api/queue/cancel-all", "", h); w.Code != 403 {
			t.Errorf("origin %v: %d", h, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/api/queue/cancel-all", nil)
	r.Host = "evil.example:8765"
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Errorf("host: %d", w.Code)
	}
	for _, h := range []map[string]string{nil, {"Origin": "http://localhost:8765"}, {"Origin": "http://127.0.0.1:1"}} {
		if w := do(s, "POST", "/api/queue/cancel-all", "", h); w.Code != 200 {
			t.Errorf("origin %v: %d", h, w.Code)
		}
	}
}

func TestIndexAndExport(t *testing.T) {
	s, d := newServer(t)
	_ = os.WriteFile(filepath.Join(d, "audio", "talk.mp3"), []byte("x"), 0o644)
	w := do(s, "GET", "/api/index", "", nil)
	var items []struct {
		Name, Stem, URL string
		Engines         []map[string]any
	}
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil || len(items) != 2 {
		t.Fatalf("index: %v %s", err, w.Body)
	}
	if items[0].URL != "audio/talk.mp3" || items[1].URL != "audio/%C3%9Cber%20uns%20%C3%A4.mp3" {
		t.Errorf("urls: %+v", items)
	}
	if len(items[0].Engines) != 1 || items[0].Engines[0]["final"] != true {
		t.Errorf("engines: %v", items[0].Engines)
	}
	w = do(s, "GET", "/api/export?stem=talk&format=vtt", "", nil)
	if w.Code != 200 || !strings.HasPrefix(w.Body.String(), "WEBVTT") ||
		!strings.Contains(w.Header().Get("Content-Disposition"), "UTF-8''talk.vtt") {
		t.Errorf("export: %d %v", w.Code, w.Header())
	}
	if w := do(s, "GET", "/api/export?stem=talk&format=doc", "", nil); w.Code != 400 {
		t.Errorf("bad format: %d", w.Code)
	}
	if w := do(s, "GET", "/api/export?stem=none&format=txt", "", nil); w.Code != 404 {
		t.Errorf("no transcript: %d", w.Code)
	}
}

func TestIndexSummary(t *testing.T) {
	s, d := newServer(t)
	_ = os.WriteFile(filepath.Join(d, "audio", "talk.mp3"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(d, "results", "talk", "_final.json"), []byte(`{"duration": 61.5, "segments": [
		{"start": 0, "end": 2, "speaker": 1, "text": "Guten Tag."},
		{"start": 2, "end": 4, "speaker": 0, "text": " Hallo. "},
		{"start": 4, "end": 6, "speaker": 2, "text": "Wie geht es?"}],
		"speakers": {"1": {"name": "Anna"}}}`), 0o644)
	_ = os.WriteFile(filepath.Join(d, "results", "talk", "_annotations.json"),
		[]byte(`{"speakers": {"0": {"name": "Ben"}, "2": {"merge_into": "1"}}, "spans": [], "edits": [{"start": 0, "end": 1, "text": "x"}],
			"clips": [{"id": "a"}, {"id": "b"}]}`), 0o644)
	var items []struct {
		Name    string
		Added   int64
		Summary *summary
	}
	if err := json.Unmarshal(do(s, "GET", "/api/index", "", nil).Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	var talk, other = items[0], items[1]
	if talk.Name != "talk.mp3" || talk.Summary == nil || talk.Added == 0 {
		t.Fatalf("talk: %+v", talk)
	}
	want := summary{Duration: 61.5, Speakers: []speaker{{1, "Anna"}, {0, "Ben"}}, Preview: "Guten Tag. Hallo. Wie geht es?", Edits: 1, Clips: 2,
		Done: talk.Summary.Done}
	if !reflect.DeepEqual(*talk.Summary, want) {
		t.Errorf("summary %+v, want %+v", *talk.Summary, want)
	}
	if other.Summary != nil {
		t.Errorf("no transcript, but a summary: %+v", other.Summary)
	}
}

func TestCut(t *testing.T) {
	if got := cut("eins zwei drei vier", 12); got != "eins zwei …" {
		t.Errorf("cut: %q", got)
	}
	if got := cut("kurz", 12); got != "kurz" {
		t.Errorf("cut: %q", got)
	}
}

func TestUploadAndQueue(t *testing.T) {
	s, d := newServer(t)
	up := func(name, body string) *httptest.ResponseRecorder {
		return do(s, "POST", "/api/upload?name="+name, body, nil)
	}
	if w := up("..%5C..%5Cevil.exe", "x"); w.Code != 400 {
		t.Errorf("bad type: %d", w.Code)
	}
	w := up("..%2F..%2Fa.mp3", "abc")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"a.mp3"`) {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	if w := up("a.mp3", "def"); !strings.Contains(w.Body.String(), `"a (2).mp3"`) {
		t.Errorf("collision: %s", w.Body)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "audio", "a.mp3")); string(b) != "abc" {
		t.Errorf("content %q", b)
	}

	if w := do(s, "POST", "/api/queue/add", `{"audio":[]}`, nil); w.Code != 400 {
		t.Errorf("empty add: %d", w.Code)
	}
	if w := do(s, "POST", "/api/queue/add", `{"audio":["a.mp3"],"threads":0}`, nil); w.Code != 400 {
		t.Errorf("threads 0: %d", w.Code)
	}
	w = do(s, "POST", "/api/queue/add", `{"audio":["a.mp3"],"speakers":"2"}`, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"added":1`) {
		t.Fatalf("add: %d %s", w.Code, w.Body)
	}
	var q struct{ Jobs []queue.Job }
	_ = json.Unmarshal(do(s, "GET", "/api/queue", "", nil).Body.Bytes(), &q)
	if len(q.Jobs) != 1 || q.Jobs[0].Options.Speakers != 2 {
		t.Fatalf("queue: %+v", q)
	}
	if w := do(s, "POST", "/api/queue/nope/cancel", "", nil); w.Code != 404 {
		t.Errorf("unknown job: %d", w.Code)
	}
	if w := do(s, "POST", "/api/queue/"+q.Jobs[0].ID+"/retry", "", nil); w.Code != 400 {
		t.Errorf("retry active: %d", w.Code)
	}
	if w := do(s, "POST", "/api/queue/"+q.Jobs[0].ID+"/cancel", "", nil); w.Code != 200 {
		t.Errorf("cancel: %d", w.Code)
	}
	if w := do(s, "POST", "/api/queue/"+q.Jobs[0].ID+"/remove", "", nil); w.Code != 200 {
		t.Errorf("remove: %d", w.Code)
	}
}

func TestAnnotationsSave(t *testing.T) {
	s, d := newServer(t)
	body := `{"stem":"talk","speakers":{"0":{"name":"Ä","x":1}},"edits":[{"start":1,"end":2,"text":"<a>","mode":"replace","extra":true}]}`
	if w := do(s, "POST", "/api/annotations?stem=talk", body, nil); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	b, _ := os.ReadFile(filepath.Join(d, "results", "talk", "_annotations.json"))
	for _, want := range []string{`"Ä"`, `"x": 1`, `"<a>"`, `"extra": true`, `"spans": []`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %s in %s", want, b)
		}
	}
	if w := do(s, "POST", "/api/annotations?stem=talk", `{"stem":"other"}`, nil); w.Code != 409 {
		t.Errorf("mismatch: %d", w.Code)
	}
	do(s, "POST", "/api/annotations?stem=talk", body, nil)
	if files, _ := filepath.Glob(filepath.Join(d, "results", "talk", "_annotations_history", "*.json")); len(files) != 1 {
		t.Errorf("history: %v", files)
	}
}

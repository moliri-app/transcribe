// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/jobs"
	"github.com/moliri-app/transcribe/ocr"
)

// ocrFake reads every image as one line and remembers what it was given.
type ocrFake struct {
	image   []byte
	options ocr.Options
}

func (f *ocrFake) Info() ocr.Info {
	return ocr.Info{ID: "tesseract", Name: "Tesseract", Usable: true, Ready: true, Languages: []string{"deu", "eng"}, Layouts: []string{"auto", "block"}, Boxes: "words"}
}

func (f *ocrFake) Clean(o ocr.Options) (ocr.Options, error) {
	return ocr.Clean(o, []string{"deu", "eng"})
}

func (f *ocrFake) Read(_ context.Context, path string, o ocr.Options, _ int) (*ocr.Result, error) {
	f.image, _ = os.ReadFile(path)
	f.options = o
	if string(f.image) == "kaputt" {
		return nil, ocr.ErrImage
	}
	p := 0.9
	return &ocr.Result{Engine: "tesseract-5.0.0", Language: o.Languages, Layout: o.Layout, Pages: []ocr.Page{{Width: 10, Height: 5,
		Lines: []ocr.Line{{BBox: ocr.Box{0, 0, 10, 5}, Text: "Zinnteller", Words: []ocr.Word{{W: "Zinnteller", BBox: ocr.Box{0, 0, 10, 5}, P: &p}}}}}}}, nil
}

func newOCRServer(t *testing.T) (*Server, *ocrFake) {
	t.Helper()
	f := &ocrFake{}
	run := func(context.Context, string, string, jobs.Options, jobs.Progress) error { return nil }
	noLive := func(context.Context, string) (live.Stream, error) { return nil, errors.New("no live in tests") }
	return New(Config{DataDir: t.TempDir(), Runner: run, Live: noLive, OCR: ocr.NewRegistry("desktop", f)}), f
}

func TestOCRReadRaw(t *testing.T) {
	s, f := newOCRServer(t)
	w := do(s, "POST", "/api/ocr?languages=DEU%2Beng&layout=block", "jpeg bytes", map[string]string{"Content-Type": "image/jpeg"})
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var res ocr.Result
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || res.Format != 1 || res.Engine != "desktop:tesseract-5.0.0" || res.Words() != 1 {
		t.Errorf("result: %v %s", err, w.Body)
	}
	if string(f.image) != "jpeg bytes" || f.options != (ocr.Options{Languages: "deu+eng", Layout: "block"}) {
		t.Errorf("engine got %q %+v", f.image, f.options)
	}
}

func TestOCRReadMultipart(t *testing.T) {
	s, f := newOCRServer(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("engine", "tesseract")
	_ = mw.WriteField("languages", "eng")
	fw, _ := mw.CreateFormFile("image", "seite.png")
	_, _ = fw.Write([]byte("png bytes"))
	_ = mw.Close()
	w := do(s, "POST", "/api/ocr", body.String(), map[string]string{"Content-Type": mw.FormDataContentType()})
	if w.Code != 200 || string(f.image) != "png bytes" || f.options.Languages != "eng" || f.options.Layout != "auto" {
		t.Errorf("%d %s, engine got %q %+v", w.Code, w.Body, f.image, f.options)
	}

	// the Live page lists the request, with what came back
	notes := s.Live.Notes()
	if len(notes) != 1 {
		t.Fatalf("%d notes on the Live page", len(notes))
	}
	n := notes[0]
	if n.Kind != live.KindOCR || n.Model != "Tesseract" || !strings.HasPrefix(n.Request, "Bild: seite.png") ||
		!strings.Contains(n.Request, "Sprachen: eng") || n.Response != "Zinnteller\n" || n.Ended == nil || len(n.Facts) == 0 {
		t.Errorf("logged: %+v", n)
	}
}

func TestOCRErrors(t *testing.T) {
	s, _ := newOCRServer(t)
	for _, c := range []struct {
		target, body string
		code         int
		says         string
	}{
		{"/api/ocr?engine=onnx", "x", 404, "engines: tesseract"},
		{"/api/ocr?languages=lat", "x", 400, "installed: deu, eng"},
		{"/api/ocr?layout=spalten", "x", 400, "layout"},
		{"/api/ocr", "", 400, "Bild fehlt"},
		{"/api/ocr", "kaputt", 422, "unreadable image"},
	} {
		w := do(s, "POST", c.target, c.body, nil)
		if w.Code != c.code || !strings.Contains(w.Body.String(), c.says) {
			t.Errorf("%s %q: %d %s, want %d %q", c.target, c.body, w.Code, w.Body, c.code, c.says)
		}
	}
	if w := do(s, "GET", "/api/ocr", "", nil); w.Code != 405 {
		t.Errorf("GET /api/ocr: %d", w.Code)
	}
}

// TestOCROrigins: the approval of the Live page holds for text recognition too. A page turned away
// can read why and is listed there to be allowed; a foreign host name (DNS rebinding) gets nothing.
func TestOCROrigins(t *testing.T) {
	s, f := newOCRServer(t)
	museum := map[string]string{"Origin": "https://museum.moliri.app"}

	pre := do(s, "OPTIONS", "/api/ocr", "", map[string]string{"Origin": "https://museum.moliri.app",
		"Access-Control-Request-Method": "POST", "Access-Control-Request-Private-Network": "true"})
	if pre.Code != 204 || pre.Header().Get("Access-Control-Allow-Origin") != "https://museum.moliri.app" ||
		pre.Header().Get("Access-Control-Allow-Private-Network") != "true" {
		t.Errorf("preflight: %d %v", pre.Code, pre.Header())
	}

	w := do(s, "POST", "/api/ocr", "img", museum)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "https://museum.moliri.app") ||
		w.Header().Get("Access-Control-Allow-Origin") != "https://museum.moliri.app" || f.image != nil {
		t.Errorf("refused page: %d %s %v", w.Code, w.Body, w.Header())
	}
	if w := do(s, "GET", "/api/ocr/engines", "", museum); w.Code != 403 {
		t.Errorf("engines for a refused page: %d", w.Code)
	}
	snap, _, cancel := s.Live.Subscribe()
	cancel()
	if !strings.Contains(string(snap), "museum.moliri.app") {
		t.Errorf("refused page not listed on the Live page: %s", snap)
	}

	if err := s.Live.Allow("https://museum.moliri.app", true); err != nil {
		t.Fatal(err)
	}
	if w := do(s, "POST", "/api/ocr", "img", museum); w.Code != 200 {
		t.Errorf("allowed page: %d %s", w.Code, w.Body)
	}
	if w := do(s, "POST", "/api/ocr", "img", map[string]string{"Origin": "http://localhost:5173"}); w.Code != 200 {
		t.Errorf("page on this computer: %d %s", w.Code, w.Body)
	}

	r := httptest.NewRequest("POST", "/api/ocr", strings.NewReader("img"))
	r.Host = "evil.example:8765"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	if rec.Code != 403 {
		t.Errorf("foreign host name: %d", rec.Code)
	}
}

func TestOCREnginesAndSchema(t *testing.T) {
	s, _ := newOCRServer(t)
	w := do(s, "GET", "/api/ocr/engines", "", nil)
	var got struct {
		Format  int        `json:"format"`
		Place   string     `json:"place"`
		Accel   string     `json:"accel"`
		Default string     `json:"default"`
		Engines []ocr.Info `json:"engines"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Format != 1 || got.Place != "desktop" || got.Accel != "cpu" ||
		got.Default != "tesseract" || len(got.Engines) != 1 || got.Engines[0].ID != "tesseract" || got.Engines[0].GPU {
		t.Errorf("engines: %v %s", err, w.Body)
	}
	w = do(s, "GET", "/api/ocr/schema", "", nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), ocr.Schema) || w.Header().Get("Content-Type") != "application/schema+json" {
		t.Errorf("schema: %d %v", w.Code, w.Header())
	}
}

// The AI service of text recognition: a key of its own, never shown, apart from the speech
// service's; the default engine of requests is one of the app's.
func TestOCRServicePrefs(t *testing.T) {
	s, _ := newOCRServer(t)
	b := do(s, "POST", "/api/prefs", `{"ocrApi":{"provider":"anthropic"}}`, nil).Body.String()
	if !strings.Contains(b, `"ocrApi":{"provider":"anthropic","url":"https://api.anthropic.com/v1","model":"claude-opus-5-5","keySet":false}`) ||
		!strings.Contains(b, `"ocrApiProviders":[{"id":"openai"`) || !strings.Contains(b, `"api":{"provider":"openai"`) {
		t.Fatalf("service: %s", b)
	}
	w := do(s, "POST", "/api/prefs", `{"ocrApi":{"key":" sk-ant-0123456789abcd ","model":" claude-sonnet-5-5 "}}`, nil)
	if b := w.Body.String(); w.Code != 200 || strings.Contains(b, "sk-ant") || !strings.Contains(b, `"model":"claude-sonnet-5-5","keySet":true,"keyHint":"abcd"`) ||
		!strings.Contains(b, `"api":{"provider":"openai","url":"https://api.openai.com/v1","model":"whisper-1","keySet":false}`) {
		t.Fatalf("key: %d %s", w.Code, b)
	}
	if b := do(s, "GET", "/api/prefs", "", nil).Body.String(); strings.Contains(b, "sk-ant") || !strings.Contains(b, `"keyHint":"abcd"`) {
		t.Errorf("shown: %s", b)
	}
	if p := prefs.Load(s.cfg.DataDir); p.OCRAPI.Key != "sk-ant-0123456789abcd" || p.API.Key != "" {
		t.Errorf("stored: %+v %+v", p.OCRAPI, p.API)
	}
	for body, code := range map[string]int{
		`{"ocrApi":{"provider":"nope"}}`: 400,
		`{"ocrEngine":"nope"}`:           400,
		`{"ocrEngine":"Tesseract"}`:      200,
	} {
		if w := do(s, "POST", "/api/prefs", body, nil); w.Code != code {
			t.Errorf("%s: %d %s", body, w.Code, w.Body)
		}
	}
	if b := do(s, "GET", "/api/prefs", "", nil).Body.String(); !strings.Contains(b, `"ocrEngine":"tesseract"`) {
		t.Errorf("default engine: %s", b)
	}
	// another service: the key of the one before is gone
	b = do(s, "POST", "/api/prefs", `{"ocrApi":{"provider":"gemini"}}`, nil).Body.String()
	if !strings.Contains(b, `"ocrApi":{"provider":"gemini","url":"https://generativelanguage.googleapis.com/v1beta","model":"gemini-2.5-pro","keySet":false}`) {
		t.Errorf("other service: %s", b)
	}
}

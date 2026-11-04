// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocrapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/internal/ocr"
)

// service is a fake AI service: it keeps the requests and answers with the next of its answers.
type service struct {
	mu       sync.Mutex
	requests []seen
	answers  []answer
	delay    time.Duration
}

type seen struct {
	path string
	hdr  http.Header
	body map[string]any
}

type answer struct {
	status int
	body   string
}

func (s *service) start(t *testing.T) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		s.mu.Lock()
		s.requests = append(s.requests, seen{r.URL.Path, r.Header.Clone(), body})
		a := answer{500, "no answer left"}
		if len(s.answers) > 0 {
			a, s.answers = s.answers[0], s.answers[1:]
		}
		s.mu.Unlock()
		if s.delay > 0 {
			select {
			case <-time.After(s.delay):
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(a.status)
		_, _ = io.WriteString(w, a.body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/v1"
}

// testImage is a page of 3200×1000 pixels: sent as 1600×500.
func testImage(t *testing.T) string {
	img := image.NewGray(image.Rect(0, 0, 3200, 1000))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for x := 400; x < 2800; x++ {
		img.SetGray(x, 500, color.Gray{})
	}
	path := filepath.Join(t.TempDir(), "seite.png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func engine(c Config) *Engine { return &Engine{Config: func() Config { return c }} }

const linesJSON = `{"lines":[{"text":"Zinnteller, um 1800","box":[100,400,900,500],"para":0},{"text":"Inv. 905"}]}`

// sentImage decodes a base64 JPEG and gives its size.
func sentImage(t *testing.T, data string) (int, int) {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

func get(m any, path ...any) any {
	for _, p := range path {
		switch k := p.(type) {
		case string:
			mm, _ := m.(map[string]any)
			m = mm[k]
		case int:
			l, _ := m.([]any)
			if k >= len(l) {
				return nil
			}
			m = l[k]
		}
	}
	return m
}

func checkResult(t *testing.T, res *ocr.Result, err error, engineName string) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	pg := res.Pages[0]
	if res.Engine != engineName || pg.Width != 3200 || pg.Height != 1000 || len(pg.Lines) != 2 {
		t.Fatalf("result: %+v", res)
	}
	if pg.Lines[0].Text != "Zinnteller, um 1800" || pg.Lines[0].BBox != (ocr.Box{320, 400, 2560, 100}) || len(pg.Lines[0].Words) != 3 {
		t.Errorf("line 0: %+v", pg.Lines[0])
	}
	if pg.Lines[1].BBox != (ocr.Box{320, 500, 2560, 100}) || pg.Lines[1].Para != 0 {
		t.Errorf("line 1: %+v", pg.Lines[1])
	}
}

func TestOpenAI(t *testing.T) {
	s := &service{answers: []answer{{200, `{"choices":[{"message":{"role":"assistant","content":` + quote(linesJSON) + `}}]}`}}}
	url := s.start(t)
	e := engine(Config{Provider: "openai", URL: url, Model: "gpt-5-mini", Key: "sk-test"})
	o, err := e.Clean(ocr.Options{Languages: "DEU+frak2021", Layout: "Lines"})
	if err != nil || o != (ocr.Options{Languages: "deu+frak2021", Layout: "lines"}) {
		t.Fatalf("clean: %v %+v", err, o)
	}
	res, err := e.Read(context.Background(), testImage(t), o, 0)
	checkResult(t, res, err, "api:gpt-5-mini")
	r := s.requests[0]
	if r.path != "/v1/chat/completions" || r.hdr.Get("Authorization") != "Bearer sk-test" || r.hdr.Get("Content-Type") != "application/json" {
		t.Errorf("request: %s %v", r.path, r.hdr)
	}
	b := r.body
	if b["model"] != "gpt-5-mini" || b["temperature"] != 0.0 || b["max_tokens"] != 8192.0 || b["stream"] != false ||
		get(b, "response_format", "type") != "json_object" || get(b, "messages", 0, "role") != "user" {
		t.Errorf("body: %v", b)
	}
	if text, _ := get(b, "messages", 0, "content", 0, "text").(string); get(b, "messages", 0, "content", 0, "type") != "text" ||
		text != Prompt("deu+frak2021", "lines") || !strings.Contains(text, "label and its value") {
		t.Errorf("text part: %v", get(b, "messages", 0, "content", 0))
	}
	dataURL, _ := get(b, "messages", 0, "content", 1, "image_url", "url").(string)
	if get(b, "messages", 0, "content", 1, "type") != "image_url" || !strings.HasPrefix(dataURL, "data:image/jpeg;base64,") {
		t.Fatalf("image part: %.80s", dataURL)
	}
	if w, h := sentImage(t, strings.TrimPrefix(dataURL, "data:image/jpeg;base64,")); w != 1600 || h != 500 {
		t.Errorf("sent %d×%d, want 1600×500", w, h)
	}
}

// Reasoning models (GPT-5 and later) refuse temperature and max_tokens: asked again without them.
func TestOpenAIReasoningModel(t *testing.T) {
	s := &service{answers: []answer{
		{400, `{"error":{"message":"Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead.","type":"invalid_request_error"}}`},
		{200, `{"choices":[{"message":{"content":"Erste Zeile\nZweite Zeile"}}]}`},
	}}
	e := engine(Config{Provider: "openai", URL: s.start(t), Model: "gpt-5", Key: "k"})
	res, err := e.Read(context.Background(), testImage(t), ocr.Options{Languages: "deu", Layout: "auto"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.requests) != 2 {
		t.Fatalf("%d requests", len(s.requests))
	}
	b := s.requests[1].body
	if _, ok := b["temperature"]; ok || b["max_tokens"] != nil || b["max_completion_tokens"] != 8192.0 {
		t.Errorf("second request: %v", b)
	}
	if l := res.Pages[0].Lines; len(l) != 2 || l[1].Text != "Zweite Zeile" || l[0].BBox != (ocr.Box{160, 0, 2880, 500}) {
		t.Errorf("plain text: %+v", l)
	}
}

func TestAnthropic(t *testing.T) {
	s := &service{answers: []answer{{200, `{"content":[{"type":"thinking","thinking":"…"},{"type":"text","text":` +
		quote("```json\n"+linesJSON+"\n```") + `}],"stop_reason":"end_turn"}`}}}
	e := engine(Config{Provider: "anthropic", URL: s.start(t), Model: "claude-opus-5-5", Key: "sk-ant"})
	res, err := e.Read(context.Background(), testImage(t), ocr.Options{Languages: "deu", Layout: "auto"}, 0)
	checkResult(t, res, err, "api:claude-opus-5-5")
	r := s.requests[0]
	if r.path != "/v1/messages" || r.hdr.Get("x-api-key") != "sk-ant" || r.hdr.Get("anthropic-version") != "2023-06-01" || r.hdr.Get("Authorization") != "" {
		t.Errorf("request: %s %v", r.path, r.hdr)
	}
	b := r.body
	if _, ok := b["temperature"]; ok || b["model"] != "claude-opus-5-5" || b["max_tokens"] != 8192.0 {
		t.Errorf("body: %v", b)
	}
	img := get(b, "messages", 0, "content", 1)
	if get(b, "messages", 0, "content", 0, "type") != "text" || get(img, "type") != "image" ||
		get(img, "source", "type") != "base64" || get(img, "source", "media_type") != "image/jpeg" {
		t.Errorf("parts: %v", get(b, "messages", 0, "content"))
	}
	data, _ := get(img, "source", "data").(string)
	if w, h := sentImage(t, data); w != 1600 || h != 500 {
		t.Errorf("sent %d×%d", w, h)
	}
}

func TestGemini(t *testing.T) {
	s := &service{answers: []answer{{200, `{"candidates":[{"content":{"role":"model","parts":[{"text":"Ich lese die Karte …","thought":true},{"text":` +
		quote(linesJSON) + `}]},"finishReason":"STOP"}]}`}}}
	e := engine(Config{Provider: "gemini", URL: s.start(t), Model: "models/gemini-2.5-pro", Key: "g-key"})
	res, err := e.Read(context.Background(), testImage(t), ocr.Options{Languages: "deu", Layout: "auto"}, 0)
	checkResult(t, res, err, "api:models/gemini-2.5-pro")
	r := s.requests[0]
	if r.path != "/v1/models/gemini-2.5-pro:generateContent" || r.hdr.Get("x-goog-api-key") != "g-key" {
		t.Errorf("request: %s %v", r.path, r.hdr)
	}
	b := r.body
	if get(b, "generationConfig", "temperature") != 0.0 || get(b, "generationConfig", "responseMimeType") != "application/json" ||
		get(b, "generationConfig", "maxOutputTokens") != nil || get(b, "contents", 0, "role") != "user" {
		t.Errorf("body: %v", b)
	}
	if text, _ := get(b, "contents", 0, "parts", 0, "text").(string); !strings.HasPrefix(text, "Transcribe all text") {
		t.Errorf("text part: %v", get(b, "contents", 0, "parts", 0))
	}
	if get(b, "contents", 0, "parts", 1, "inline_data", "mime_type") != "image/jpeg" {
		t.Errorf("image part: %v", get(b, "contents", 0, "parts", 1))
	}
	data, _ := get(b, "contents", 0, "parts", 1, "inline_data", "data").(string)
	if w, h := sentImage(t, data); w != 1600 || h != 500 {
		t.Errorf("sent %d×%d", w, h)
	}
}

// An own server (Ollama) needs no key and may answer in pixels of the image it saw.
func TestOwnServerPixels(t *testing.T) {
	s := &service{answers: []answer{{200, `{"choices":[{"message":{"content":"{\"lines\":[{\"text\":\"Inv. 905\",\"box\":[100,400,1500,450]}]}"}}]}`}}}
	e := engine(Config{Provider: "custom", URL: s.start(t), Model: "qwen2.5vl:3b"})
	if info := e.Info(); !info.Usable || !info.Ready || info.Version != "qwen2.5vl:3b" || info.Boxes != "lines" {
		t.Errorf("info: %+v", info)
	}
	res, err := e.Read(context.Background(), testImage(t), ocr.Options{Languages: "deu", Layout: "auto"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if s.requests[0].hdr.Get("Authorization") != "" {
		t.Error("a key was sent")
	}
	if b := res.Pages[0].Lines[0].BBox; b != (ocr.Box{200, 800, 2800, 100}) {
		t.Errorf("pixel box: %v", b)
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		provider string
		answer   answer
		kind     string
		says     string
	}{
		{"openai", answer{401, `{"error":{"message":"Incorrect API key provided","type":"invalid_request_error"}}`}, KindAuth, "hat den API-Schlüssel abgelehnt. Incorrect API key"},
		{"anthropic", answer{429, `{"type":"error","error":{"type":"rate_limit_error","message":"Number of requests has exceeded your rate limit"}}`}, KindRateLimit, "nimmt gerade nichts an"},
		{"gemini", answer{429, `{"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota).","status":"RESOURCE_EXHAUSTED"}}`}, KindRateLimit, "Guthaben"},
		{"custom", answer{404, `{"error":{"message":"model \"qwen9\" not found, try pulling it first"}}`}, KindModel, "kennt das Modell „m“ nicht"},
		{"openai", answer{200, `<html><body>Login</body></html>`}, KindEndpoint, "keine passende Schnittstelle. Stimmt die Adresse?"},
		{"openai", answer{404, `404 page not found`}, KindEndpoint, "(Fehler 404)"},
		{"anthropic", answer{529, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`}, KindServer, "Fehler gemeldet (529). Overloaded"},
		{"anthropic", answer{200, `{"content":[],"stop_reason":"refusal"}`}, KindRefused, "hat das Bild abgelehnt: refusal"},
		{"gemini", answer{200, `{"promptFeedback":{"blockReason":"PROHIBITED_CONTENT"}}`}, KindRefused, "PROHIBITED_CONTENT"},
		{"gemini", answer{200, `{"candidates":[{"finishReason":"RECITATION"}]}`}, KindRefused, "RECITATION"},
		{"openai", answer{200, `{"object":"list"}`}, KindBadRequest, "Fehler 200"},
	}
	img := testImage(t)
	for _, c := range cases {
		s := &service{answers: []answer{c.answer}}
		e := engine(Config{Provider: c.provider, URL: s.start(t), Model: "m", Key: "k"})
		_, err := e.Read(context.Background(), img, ocr.Options{Languages: "deu", Layout: "auto"}, 0)
		var se *ServiceError
		if !errors.As(err, &se) || se.Kind != c.kind || !strings.Contains(err.Error(), c.says) || !errors.Is(err, ocr.ErrService) {
			t.Errorf("%s %d: got %v (%+v), want %s %q", c.provider, c.answer.status, err, se, c.kind, c.says)
		}
	}
}

func TestTimeoutAndCancel(t *testing.T) {
	img := testImage(t)
	s := &service{delay: 2 * time.Second, answers: []answer{{200, `{}`}, {200, `{}`}}}
	url := s.start(t)
	e := &Engine{Config: func() Config { return Config{Provider: "openai", URL: url, Model: "m", Key: "k"} }, Timeout: 100 * time.Millisecond}
	_, err := e.Read(context.Background(), img, ocr.Options{Languages: "deu", Layout: "auto"}, 0)
	var se *ServiceError
	if !errors.As(err, &se) || se.Kind != KindTimeout || !strings.Contains(err.Error(), "nicht rechtzeitig") {
		t.Errorf("timeout: %v", err)
	}
	e.Timeout = time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := e.Read(ctx, img, ocr.Options{Languages: "deu", Layout: "auto"}, 0); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("cancelled by the caller: %v", err)
	}
}

func TestInfoAndClean(t *testing.T) {
	e := engine(Config{})
	info := e.Info()
	if info.ID != "api" || info.Usable || info.Ready || info.Error != ErrNoKey.Error() || info.Version != "gpt-5-mini" ||
		!strings.Contains(info.Name, "OpenAI") || len(info.Layouts) != 4 || info.GPU {
		t.Errorf("without a key: %+v", info)
	}
	for _, want := range []string{"deu", "eng", "frak2021"} {
		if !strings.Contains(strings.Join(info.Languages, " "), want) {
			t.Errorf("languages %v lack %s", info.Languages, want)
		}
	}
	if _, err := e.Read(context.Background(), "x", ocr.Options{}, 0); !errors.Is(err, ocr.ErrUnavailable) {
		t.Errorf("read without a key: %v", err)
	}
	if !engine(Config{Provider: "gemini", Key: "k"}).Info().Usable {
		t.Error("gemini with a key is not usable")
	}
	if !e.ByChoice() {
		t.Error("the AI service must be chosen")
	}
	for in, want := range map[string]string{"": "deu", "deu+eng+DEU": "deu+eng", "Fraktur": "fraktur", "lat+de-DE": "lat+de-de"} {
		if o, err := e.Clean(ocr.Options{Languages: in}); err != nil || o.Languages != want || o.Layout != "auto" {
			t.Errorf("clean %q: %v %+v", in, err, o)
		}
	}
	for _, bad := range []ocr.Options{{Languages: "deu+"}, {Languages: "d e"}, {Layout: "spalten"}} {
		if _, err := e.Clean(bad); !errors.Is(err, ocr.ErrOptions) {
			t.Errorf("clean %+v: %v", bad, err)
		}
	}
	if _, err := PrepareImage(filepath.Join(t.TempDir(), "x.jpg"), MaxSide); err == nil {
		t.Error("missing image")
	}
	broken := filepath.Join(t.TempDir(), "kaputt.jpg")
	_ = os.WriteFile(broken, []byte("kaputt"), 0o644)
	if _, err := PrepareImage(broken, MaxSide); !errors.Is(err, ocr.ErrImage) {
		t.Errorf("broken image: %v", err)
	}
}

// In the registry: never the engine of a request without one, unless the settings make it the
// default; its result passes the format and Moliri's check of the engine name.
func TestRegistry(t *testing.T) {
	s := &service{answers: []answer{{200, `{"choices":[{"message":{"content":` + quote(linesJSON) + `}}]}`}}}
	api := engine(Config{Provider: "openai", URL: s.start(t), Model: "gpt-5-mini", Key: "k"})
	r := ocr.NewRegistry("desktop", api)
	if _, err := r.Get(""); !errors.Is(err, ocr.ErrUnavailable) {
		t.Errorf("chosen without being asked: %v", err)
	}
	def := "api"
	r.Default = func() string { return def }
	e, o, err := r.Prepare("", ocr.Options{Languages: "deu+frak2021", Layout: "lines"})
	if err != nil || e != ocr.Engine(api) {
		t.Fatalf("default: %v", err)
	}
	res, err := r.Read(context.Background(), e, testImage(t), o, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[a-z]+:\S+$`).MatchString(res.Engine) || res.Engine != "desktop:api:gpt-5-mini" || len(res.Engine) > 100 ||
		res.Language != "deu+frak2021" || res.Layout != "lines" {
		t.Errorf("result: %s %s %s", res.Engine, res.Language, res.Layout)
	}
	if got := r.DefaultEngine(); got != "api" {
		t.Errorf("default engine: %q", got)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

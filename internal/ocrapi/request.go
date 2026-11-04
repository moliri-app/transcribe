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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/moliri-app/transcribe/ocr"
)

// The request of one page and the answer, in each interface's own shape, as Moliri's
// aiBuildRequest and aiParseResponse (static/api/_func/ai_account.php) send and read them.

// MaxTokens is the room for the answer: a dense page has some 60 lines of 15 tokens with their
// boxes; with Anthropic it also covers the model's thinking.
const MaxTokens = 8192

type request struct {
	url  string
	hdr  http.Header
	body any
}

// buildRequest is the HTTP request for the prompt and the image. compat: an OpenAI model that
// refused temperature or max_tokens (the reasoning models, GPT-5 and later) gets neither, but
// max_completion_tokens.
func buildRequest(c Config, prompt string, img *Prepared, compat bool) request {
	data := base64.StdEncoding.EncodeToString(img.Data)
	hdr := http.Header{}
	hdr.Set("Content-Type", "application/json")
	hdr.Set("Accept", "application/json")
	switch c.Kind() {
	case Anthropic:
		if c.Key != "" {
			hdr.Set("x-api-key", c.Key)
		}
		hdr.Set("anthropic-version", "2023-06-01")
		// no temperature: current Claude models refuse sampling parameters
		return request{c.base() + "/messages", hdr, map[string]any{
			"model":      c.Model,
			"max_tokens": MaxTokens,
			"messages": []any{map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "text", "text": prompt},
				map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": img.Mime, "data": data}},
			}}},
		}}
	case Gemini:
		if c.Key != "" {
			hdr.Set("x-goog-api-key", c.Key)
		}
		model := strings.TrimPrefix(c.Model, "models/")
		// no maxOutputTokens: the thinking of Gemini 2.5 and later counts against it
		return request{c.base() + "/models/" + url.PathEscape(model) + ":generateContent", hdr, map[string]any{
			"contents": []any{map[string]any{"role": "user", "parts": []any{
				map[string]any{"text": prompt},
				map[string]any{"inline_data": map[string]any{"mime_type": img.Mime, "data": data}},
			}}},
			"generationConfig": map[string]any{"temperature": 0, "responseMimeType": "application/json"},
		}}
	}
	if c.Key != "" {
		hdr.Set("Authorization", "Bearer "+c.Key)
	}
	body := map[string]any{
		"model": c.Model,
		"messages": []any{map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "text", "text": prompt},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + img.Mime + ";base64," + data}},
		}}},
		"stream":          false,
		"response_format": map[string]any{"type": "json_object"},
	}
	if compat {
		body["max_completion_tokens"] = MaxTokens
	} else {
		body["temperature"] = 0
		body["max_tokens"] = MaxTokens
	}
	return request{c.base() + "/chat/completions", hdr, body}
}

// Ask sends the prompt and the image and returns the text of the answer. Errors of the service are
// *ServiceError (wrapping ocr.ErrService); a cancelled ctx gives its error.
func Ask(ctx context.Context, c Config, prompt string, img *Prepared, timeout time.Duration) (string, error) {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	content, err := send(tctx, c, buildRequest(c, prompt, img, false), timeout)
	var se *ServiceError
	if c.Kind() == OpenAI && errors.As(err, &se) && se.Status == http.StatusBadRequest && reasoningRefusal.MatchString(se.Detail) {
		content, err = send(tctx, c, buildRequest(c, prompt, img, true), timeout)
	}
	if err != nil && ctx.Err() != nil {
		return "", ctx.Err() // the caller went away
	}
	return content, err
}

var reasoningRefusal = regexp.MustCompile(`(?i)max_tokens|temperature|max_completion_tokens`)

func send(ctx context.Context, c Config, r request, timeout time.Duration) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r.body); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, &buf)
	if err != nil {
		return "", &ServiceError{Kind: KindEndpoint, Label: c.Label(), URL: c.URL, Detail: err.Error()}
	}
	req.Header = r.hdr
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", &ServiceError{Kind: KindTimeout, Label: c.Label(), Detail: fmt.Sprintf("%.0f s", timeout.Seconds())}
		}
		return "", &ServiceError{Kind: KindUnreachable, Label: c.Label(), URL: c.URL, Detail: err.Error()}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", &ServiceError{Kind: KindTimeout, Label: c.Label(), Detail: fmt.Sprintf("%.0f s", timeout.Seconds())}
		}
		return "", &ServiceError{Kind: KindUnreachable, Label: c.Label(), URL: c.URL, Detail: err.Error()}
	}
	return parseResponse(c, resp.StatusCode, raw)
}

// parseResponse reads the text of an answer, or the service's error.
func parseResponse(c Config, status int, raw []byte) (string, error) {
	var decoded map[string]any
	isJSON := json.Unmarshal(raw, &decoded) == nil && decoded != nil
	fail := func(kind, detail string) error {
		return &ServiceError{Kind: kind, Status: status, Label: c.Label(), URL: c.URL, Model: c.Model, Detail: detail}
	}
	if status != http.StatusOK || !isJSON {
		msg := providerMessage(decoded, raw)
		return "", fail(errorKind(status, msg, decoded), msg)
	}
	var text strings.Builder
	switch c.Kind() {
	case Anthropic:
		if s, _ := decoded["stop_reason"].(string); s == "refusal" {
			detail := "refusal"
			if d, ok := decoded["stop_details"].(map[string]any); ok {
				if e, ok := d["explanation"].(string); ok && e != "" {
					detail = e
				}
			}
			return "", fail(KindRefused, detail)
		}
		blocks, _ := decoded["content"].([]any)
		for _, b := range blocks {
			if m, ok := b.(map[string]any); ok && m["type"] == "text" {
				s, _ := m["text"].(string)
				text.WriteString(s)
			}
		}
	case Gemini:
		var candidate map[string]any
		if list, ok := decoded["candidates"].([]any); ok && len(list) > 0 {
			candidate, _ = list[0].(map[string]any)
		}
		blocked := ""
		if fb, ok := decoded["promptFeedback"].(map[string]any); ok {
			blocked, _ = fb["blockReason"].(string)
		}
		finish, _ := candidate["finishReason"].(string)
		switch {
		case blocked != "":
			return "", fail(KindRefused, blocked)
		case finish == "SAFETY" || finish == "PROHIBITED_CONTENT" || finish == "BLOCKLIST" || finish == "RECITATION":
			return "", fail(KindRefused, finish)
		}
		var parts []any
		if content, ok := candidate["content"].(map[string]any); ok {
			parts, _ = content["parts"].([]any)
		}
		for _, p := range parts {
			m, ok := p.(map[string]any)
			if !ok || truthy(m["thought"]) {
				continue // the model's thinking, not its answer
			}
			s, _ := m["text"].(string)
			text.WriteString(s)
		}
	default:
		choices, ok := decoded["choices"].([]any)
		if !ok {
			return "", fail(KindBadRequest, providerMessage(decoded, raw))
		}
		if len(choices) > 0 {
			if ch, ok := choices[0].(map[string]any); ok {
				if msg, ok := ch["message"].(map[string]any); ok {
					s, _ := msg["content"].(string)
					text.WriteString(s)
				}
			}
		}
	}
	return text.String(), nil
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != "" && x != "0"
	case float64:
		return x != 0
	}
	return true
}

var tags = regexp.MustCompile(`<[^>]*>`)

// providerMessage is the service's own words about an error, out of the shapes the three
// interfaces (and Ollama, Mistral) use, else the start of the answer.
func providerMessage(decoded map[string]any, raw []byte) string {
	if decoded != nil {
		switch e := decoded["error"].(type) {
		case map[string]any:
			if m, ok := e["message"].(string); ok {
				return m
			}
		case string:
			if e != "" {
				return e
			}
		}
		if m, ok := decoded["message"].(string); ok {
			return m
		}
		if d, ok := decoded["detail"].(string); ok {
			return d
		}
	}
	s := []rune(tags.ReplaceAllString(string(raw), ""))
	if len(s) > 300 {
		s = s[:300]
	}
	return strings.TrimSpace(string(s))
}

// The kinds of failure, as Moliri's aiErrorKind tells them apart.
const (
	KindUnreachable = "unreachable"
	KindTimeout     = "timeout"
	KindAuth        = "auth"
	KindModel       = "model"
	KindEndpoint    = "endpoint"
	KindRateLimit   = "rate_limit"
	KindServer      = "server"
	KindRefused     = "refused"
	KindBadRequest  = "bad_request"
)

var (
	authWords  = regexp.MustCompile(`api[ _-]?key|unauthori[sz]ed|(invalid|incorrect|expired) (api |access |auth(entication)? )?(key|token)`)
	quotaWords = regexp.MustCompile(`quota|rate limit`)
	modelWords = regexp.MustCompile(`not (be )?found|does not exist|unknown|not supported|invalid model|try pulling`)
)

func errorKind(status int, message string, decoded map[string]any) string {
	typ := ""
	if e, ok := decoded["error"].(map[string]any); ok {
		for _, k := range []string{"type", "status", "code"} {
			if s, ok := e[k].(string); ok && s != "" {
				typ = strings.ToLower(s)
				break
			}
		}
	}
	text := strings.ToLower(message)
	switch {
	case status == 401 || status == 403 || strings.Contains(typ, "authentication") || strings.Contains(typ, "permission") || authWords.MatchString(text):
		return KindAuth
	case status == 429 || strings.Contains(typ, "rate_limit") || strings.Contains(typ, "resource_exhausted") || quotaWords.MatchString(text):
		return KindRateLimit
	case strings.Contains(text, "model") && (status == 404 || status == 400 || modelWords.MatchString(text)):
		return KindModel
	case status == 404 || status == 405:
		return KindEndpoint
	case status >= 500:
		return KindServer
	case status == 200:
		return KindEndpoint // an answer, but no JSON: a web page rather than an API
	}
	return KindBadRequest
}

// ServiceError is a failure of the service, said for people (in German, as the app speaks).
type ServiceError struct {
	Kind   string // KindAuth, …
	Status int    // HTTP status, 0 when there was no answer
	Label  string // the service
	URL    string
	Model  string
	Detail string // the service's own words
}

func (e *ServiceError) Unwrap() error { return ocr.ErrService }

func (e *ServiceError) Error() string {
	detail := strings.TrimSpace(e.Detail)
	with := func(s string) string {
		if detail == "" {
			return s
		}
		return s + " " + detail
	}
	switch e.Kind {
	case KindUnreachable:
		return with(fmt.Sprintf("%s ist nicht erreichbar (%s).", e.Label, e.URL))
	case KindTimeout:
		return fmt.Sprintf("%s hat nicht rechtzeitig geantwortet (%s).", e.Label, detail)
	case KindAuth:
		return with(fmt.Sprintf("%s hat den API-Schlüssel abgelehnt.", e.Label))
	case KindRateLimit:
		return with(fmt.Sprintf("%s nimmt gerade nichts an (Guthaben oder Anfragen aufgebraucht).", e.Label))
	case KindModel:
		return with(fmt.Sprintf("%s kennt das Modell „%s“ nicht, oder es liest keine Bilder.", e.Label, e.Model))
	case KindEndpoint:
		if e.Status != 0 && e.Status != http.StatusOK {
			return with(fmt.Sprintf("Unter %s antwortet keine passende Schnittstelle (Fehler %d). Stimmt die Adresse?", e.URL, e.Status))
		}
		return with(fmt.Sprintf("Unter %s antwortet keine passende Schnittstelle. Stimmt die Adresse?", e.URL))
	case KindServer:
		return with(fmt.Sprintf("%s hat einen Fehler gemeldet (%d).", e.Label, e.Status))
	case KindRefused:
		return with(fmt.Sprintf("%s hat das Bild abgelehnt:", e.Label))
	}
	return with(fmt.Sprintf("%s: Fehler %d:", e.Label, e.Status))
}

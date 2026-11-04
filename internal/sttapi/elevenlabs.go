// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sttapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ElevenLabs (Scribe): whole recordings over HTTP, with speakers and sounds; live over a WebSocket.

// elevenLabsLiveModel is the model for live transcription; the one in the settings is for recordings.
const elevenLabsLiveModel = "scribe_v2_realtime"

type elWord struct {
	Text      string  `json:"text"`
	Type      string  `json:"type"` // word, spacing or audio_event
	Logprob   float64 `json:"logprob"`
	Start     float64 `json:"start"`
	End       float64 `json:"end"`
	SpeakerID string  `json:"speaker_id"`
}

type elResult struct {
	LanguageCode string   `json:"language_code"`
	Words        []elWord `json:"words"`
	Duration     float64  `json:"audio_duration_secs"`
}

func elevenLabsTranscribe(ctx context.Context, c Config, audio io.Reader, name string, o Request) (*Result, error) {
	fields := map[string]string{
		"model_id":               c.Model,
		"timestamps_granularity": "word",
		"tag_audio_events":       "true",
		"diarize":                strconv.FormatBool(o.Diarize),
	}
	if o.Language != "" {
		fields["language_code"] = o.Language
	}
	if o.Diarize && o.Speakers > 0 {
		fields["num_speakers"] = strconv.Itoa(o.Speakers)
	}
	body, err := post(ctx, c.base()+"/v1/speech-to-text", http.Header{"xi-api-key": {c.Key}}, fields, audio, name, c.Label())
	if err != nil {
		return nil, err
	}
	var r elResult
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("%s: unerwartete Antwort: %w", c.Label(), err)
	}
	res := &Result{Language: shortLang(r.LanguageCode), Duration: r.Duration}
	for _, w := range r.Words {
		switch w.Type {
		case "audio_event":
			res.Words = append(res.Words, Word{Text: strings.Trim(w.Text, "()[] "), Start: w.Start, End: w.End, P: 1, Event: true})
		case "word":
			if t := strings.TrimSpace(w.Text); t != "" {
				res.Words = append(res.Words, Word{Text: t, Start: w.Start, End: w.End, P: prob(w.Logprob), Speaker: w.SpeakerID})
			}
		}
	}
	return res, nil
}

// post sends a multipart form with the audio as its file and returns the answer of a 200.
func post(ctx context.Context, target string, hdr http.Header, fields map[string]string, audio io.Reader, name, label string) ([]byte, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var err error
		for k, v := range fields {
			for _, one := range strings.Split(v, "\x00") { // a field given several times
				if err == nil {
					err = mw.WriteField(k, one)
				}
			}
		}
		if err == nil {
			var fw io.Writer
			if fw, err = mw.CreateFormFile("file", name); err == nil {
				_, err = io.Copy(fw, audio)
			}
		}
		if err == nil {
			err = mw.Close()
		}
		pw.CloseWithError(err)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, pr)
	if err != nil {
		pr.Close()
		return nil, err
	}
	for k, v := range hdr {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%s ist nicht erreichbar: %w", label, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &apiError{status: resp.StatusCode, msg: errorMessage(body), label: label}
	}
	return body, nil
}

// apiError is a refusal of the service.
type apiError struct {
	status int
	msg    string
	label  string
}

func (e *apiError) Error() string {
	switch e.status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Sprintf("%s hat den API-Schlüssel abgelehnt. %s", e.label, e.msg)
	case http.StatusPaymentRequired, http.StatusTooManyRequests:
		return fmt.Sprintf("%s nimmt gerade nichts an (Guthaben oder Anfragen aufgebraucht). %s", e.label, e.msg)
	}
	return fmt.Sprintf("%s: Fehler %d: %s", e.label, e.status, e.msg)
}

// errorMessage reads what a service says about an error: {"detail": ...} (ElevenLabs),
// {"error": {"message": ...}} (OpenAI) or plain text.
func errorMessage(body []byte) string {
	var e struct {
		Detail json.RawMessage `json:"detail"`
		Error  json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil {
		for _, raw := range []json.RawMessage{e.Detail, e.Error} {
			if len(raw) == 0 {
				continue
			}
			var d struct{ Message string }
			if json.Unmarshal(raw, &d) == nil && d.Message != "" {
				return d.Message
			}
			var s string
			if json.Unmarshal(raw, &s) == nil {
				return s
			}
			return string(raw)
		}
	}
	return strings.TrimSpace(string(body))
}

// elevenLabsLiveURL is the WebSocket address for live transcription of 16 kHz audio in lang ("" for any).
func elevenLabsLiveURL(c Config, lang string) string {
	u := c.base()
	switch {
	case strings.HasPrefix(u, "https://"):
		u = "wss://" + strings.TrimPrefix(u, "https://")
	case strings.HasPrefix(u, "http://"):
		u = "ws://" + strings.TrimPrefix(u, "http://")
	}
	q := url.Values{
		"model_id":                   {elevenLabsLiveModel},
		"audio_format":               {"pcm_16000"},
		"commit_strategy":            {"vad"},
		"vad_silence_threshold_secs": {"0.6"},
	}
	if lang != "" {
		q.Set("language_code", lang)
	}
	return u + "/v1/speech-to-text/realtime?" + q.Encode()
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package sttapitest is a stand-in for speech recognition services, for tests and demos without an
// account. It speaks both protocols internal/sttapi knows: the OpenAI-compatible one under /v1
// (its base address is <url>/v1) and ElevenLabs' (base address <url>). A recording is answered with
// the transcript of the staged interview in e2e/testdata; a short one (live transcription, a passage
// recognised again) with the interview's next sentence; ElevenLabs' live transcription with its
// sentences, a word for every half second of audio.
package sttapitest

import (
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

//go:embed interview.json
var interview []byte // as ElevenLabs answers, with speakers and a laugh

// BadKey is a key the stand-in refuses.
const BadKey = "falsch"

// Fake is the stand-in; it remembers what it was asked.
type Fake struct {
	mu       sync.Mutex
	Requests []map[string]string // the form fields of each recording sent, "path" its address
	next     int                 // the sentence for the next short recording
}

func (f *Fake) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/speech-to-text", f.elevenLabs)
	mux.HandleFunc("/v1/speech-to-text/realtime", f.realtime)
	mux.HandleFunc("/v1/audio/transcriptions", f.openAI)
	return mux
}

func refused(w http.ResponseWriter, r *http.Request) bool {
	k := r.Header.Get("xi-api-key")
	if a := r.Header.Get("Authorization"); a != "" {
		k = strings.TrimPrefix(a, "Bearer ")
	}
	if k == BadKey || r.URL.Path != "/v1/audio/transcriptions" && k == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"Invalid API key"},"detail":{"status":"invalid_api_key","message":"Invalid API key"}}`)
		return true
	}
	return false
}

// form reads and remembers a recording sent; size is the length of its file in bytes.
func (f *Fake) form(w http.ResponseWriter, r *http.Request) (fields map[string]string, size int64, ok bool) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return nil, 0, false
	}
	if refused(w, r) {
		return nil, 0, false
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), 400)
		return nil, 0, false
	}
	fields = map[string]string{"path": r.URL.Path}
	for k, v := range r.MultipartForm.Value {
		fields[k] = strings.Join(v, ",")
	}
	if fh := r.MultipartForm.File["file"]; len(fh) > 0 {
		fields["file"] = fh[0].Filename
		size = fh[0].Size
	}
	f.mu.Lock()
	f.Requests = append(f.Requests, fields)
	f.mu.Unlock()
	return fields, size, true
}

// short: less than 20 s of 16 kHz WAV, or of the test recordings' MP3 (which are much smaller)
func short(fields map[string]string, size int64) bool {
	return strings.HasSuffix(fields["file"], ".wav") && size < 44+20*32000
}

func (f *Fake) nextSentence() []word {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := sentences()
	out := s[f.next%len(s)]
	f.next++
	return out
}

func (f *Fake) elevenLabs(w http.ResponseWriter, r *http.Request) {
	fields, size, ok := f.form(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if short(fields, size) {
		var out []map[string]any
		t := 0.3
		for _, wd := range f.nextSentence() {
			out = append(out, map[string]any{"text": wd.Text, "type": "word", "start": t, "end": t + 0.4, "logprob": -0.05})
			t += 0.5
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"language_code": "deu", "words": out, "audio_duration_secs": t})
		return
	}
	_, _ = w.Write(interview)
}

// openAI answers as OpenAI's whisper-1 does with verbose_json: words without punctuation, the
// text with it, segments with how sure it was.
func (f *Fake) openAI(w http.ResponseWriter, r *http.Request) {
	fields, size, ok := f.form(w, r)
	if !ok {
		return
	}
	ws := all()
	if short(fields, size) {
		ws = f.nextSentence()
		t := 0.3
		for i := range ws {
			ws[i].Start, ws[i].End = t, t+0.4
			t += 0.5
		}
	}
	var text []string
	var words []map[string]any
	for _, wd := range ws {
		text = append(text, wd.Text)
		words = append(words, map[string]any{"word": strings.Trim(wd.Text, ".,?!"), "start": wd.Start, "end": wd.End})
	}
	res := map[string]any{"text": strings.Join(text, " ")}
	if fields["response_format"] == "verbose_json" {
		res["language"] = "german"
		res["duration"] = 35.136
		res["words"] = words
		if len(ws) > 0 {
			res["segments"] = []map[string]any{{"start": ws[0].Start, "end": ws[len(ws)-1].End, "text": res["text"], "avg_logprob": -0.1}}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

type word struct {
	Text       string
	Type       string
	Start, End float64
}

// all are the interview's words.
func all() []word {
	var res struct{ Words []word }
	_ = json.Unmarshal(interview, &res)
	var out []word
	for _, w := range res.Words {
		if w.Type == "word" {
			out = append(out, w)
		}
	}
	return out
}

// sentences are the interview's, as live transcription hears them.
func sentences() [][]word {
	var out [][]word
	var cur []word
	for _, w := range all() {
		cur = append(cur, w)
		if strings.ContainsAny(w.Text[len(w.Text)-1:], ".?!") {
			out = append(out, cur)
			cur = nil
		}
	}
	return out
}

func (f *Fake) realtime(w http.ResponseWriter, r *http.Request) {
	if refused(w, r) {
		return
	}
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	ctx := r.Context()
	if wsjson.Write(ctx, c, map[string]any{"message_type": "session_started", "session_id": "fake"}) != nil {
		return
	}
	script := sentences()
	sentence, said, heard := 0, 0, 0 // the sentence going on, its words said so far, samples since the last word
	text := func() string {
		var t []string
		for _, w := range script[sentence%len(script)][:said] {
			t = append(t, w.Text)
		}
		return strings.Join(t, " ")
	}
	commit := func() error {
		if said == 0 {
			return nil
		}
		t := text()
		sentence, said = sentence+1, 0
		return wsjson.Write(ctx, c, map[string]any{"message_type": "committed_transcript", "text": t})
	}
	for {
		var m struct {
			Type   string `json:"message_type"`
			Audio  string `json:"audio_base_64"`
			Commit bool   `json:"commit"`
		}
		if wsjson.Read(ctx, c, &m) != nil {
			return
		}
		heard += len(m.Audio) * 3 / 4 / 2
		if heard >= 8000 { // half a second of 16 kHz audio: one more word
			heard = 0
			said++
			if wsjson.Write(ctx, c, map[string]any{"message_type": "partial_transcript", "text": text()}) != nil {
				return
			}
			if said == len(script[sentence%len(script)]) && commit() != nil {
				return
			}
		}
		if m.Commit && commit() != nil {
			return
		}
	}
}

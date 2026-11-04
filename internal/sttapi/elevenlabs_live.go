// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sttapi

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/moliri-app/transcribe/internal/live"
)

// message is what the service sends during live transcription.
type message struct {
	Type  string `json:"message_type"`
	Text  string `json:"text"`
	Error string `json:"error"`
}

// session is one WebSocket connection to the live transcription.
type session struct {
	c      *websocket.Conn
	cancel context.CancelFunc
	in     chan message // closed when the connection ends
}

func dial(ctx context.Context, cfg Config, lang string) (*session, error) {
	dctx, stop := context.WithTimeout(ctx, 15*time.Second)
	defer stop()
	c, resp, err := websocket.Dial(dctx, elevenLabsLiveURL(cfg, lang), &websocket.DialOptions{HTTPHeader: http.Header{"xi-api-key": {cfg.Key}}})
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return nil, errors.New("ElevenLabs hat den API-Schlüssel abgelehnt.")
		}
		return nil, fmt.Errorf("ElevenLabs ist nicht erreichbar: %w", err)
	}
	c.SetReadLimit(1 << 20)
	// the session ends with Close, not with the context of whoever opened it
	sctx, cancel := context.WithCancel(context.Background())
	s := &session{c: c, cancel: cancel, in: make(chan message, 64)}
	// the first message tells whether the service accepted the session
	var first message
	if err := wsjson.Read(dctx, c, &first); err != nil {
		s.close()
		return nil, fmt.Errorf("ElevenLabs antwortet nicht: %w", err)
	}
	if first.Type != "session_started" {
		s.close()
		return nil, fmt.Errorf("ElevenLabs lehnt ab: %s", refusal(first))
	}
	go func() {
		defer close(s.in)
		for {
			var m message
			if err := wsjson.Read(sctx, c, &m); err != nil {
				if sctx.Err() == nil {
					log.Println("live (ElevenLabs): connection ended:", err)
				}
				return
			}
			s.in <- m
		}
	}()
	return s, nil
}

// refusal explains an error message of the service.
func refusal(m message) string {
	switch m.Type {
	case "auth_error":
		return "Der API-Schlüssel wurde nicht angenommen."
	case "quota_exceeded", "rate_limited":
		return "Guthaben oder Anfragen sind aufgebraucht."
	}
	if m.Error != "" {
		return m.Error
	}
	return m.Type
}

// send gives the service 16 kHz samples; commit ends the utterance going on.
func (s *session) send(pcm []float32, commit bool) error {
	b := make([]byte, 2*len(pcm))
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(int16(math.Round(float64(max(-1, min(1, v)))*32767))))
	}
	ctx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	return wsjson.Write(ctx, s.c, map[string]any{
		"message_type":  "input_audio_chunk",
		"audio_base_64": base64.StdEncoding.EncodeToString(b),
		"commit":        commit,
		"sample_rate":   live.SR,
	})
}

func (s *session) close() {
	s.cancel()
	_ = s.c.Close(websocket.StatusNormalClosure, "")
}

// elevenLabsLive opens live transcription at ElevenLabs for lang ("" for any).
func elevenLabsLive(ctx context.Context, cfg Config, lang string) (live.Stream, error) {
	s, err := dial(ctx, cfg, lang)
	if err != nil {
		return nil, err
	}
	return &stream{cfg: cfg, lang: lang, s: s}, nil
}

// stream is live.Stream over a session. The service decides itself where an utterance ends;
// its times are the samples sent until then, as the service gives none of its own.
type stream struct {
	cfg   Config
	lang  string
	s     *session // nil after the connection broke, until it is opened again
	retry time.Time

	pos     int    // samples sent
	from    int    // sample where the utterance going on began
	partial string // what the service has heard of it so far
	shown   string // partial at the last preview
}

func (st *stream) Accept(pcm []float32) []live.Event {
	if st.s == nil && time.Now().After(st.retry) {
		if s, err := dial(context.Background(), st.cfg, st.lang); err == nil {
			log.Println("live (ElevenLabs): connected again")
			st.s = s
		} else {
			log.Println("live (ElevenLabs):", err)
			st.retry = time.Now().Add(5 * time.Second)
		}
	}
	if st.s != nil {
		if err := st.s.send(pcm, false); err != nil {
			log.Println("live (ElevenLabs): sending audio:", err)
			st.broke()
		}
	}
	st.pos += len(pcm)
	return st.take(0)
}

// broke drops a connection that no longer works; the next Accept opens a new one.
func (st *stream) broke() {
	if st.s != nil {
		st.s.close()
		st.s = nil
	}
	st.from = st.pos
	st.partial, st.shown = "", ""
}

// take collects what the service sent; with wait > 0 it waits that long for an utterance to end.
func (st *stream) take(wait time.Duration) []live.Event {
	var out []live.Event
	var deadline <-chan time.Time
	if wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		deadline = t.C
	}
	for st.s != nil {
		var m message
		var ok bool
		if deadline == nil {
			select {
			case m, ok = <-st.s.in:
			default:
				return out
			}
		} else {
			select {
			case m, ok = <-st.s.in:
			case <-deadline:
				return out
			}
		}
		if !ok {
			st.broke()
			return out
		}
		switch m.Type {
		case "partial_transcript":
			st.partial = strings.TrimSpace(m.Text)
		case "committed_transcript":
			if t := strings.TrimSpace(m.Text); t != "" {
				out = append(out, live.Event{Final: true, Text: t, Start: secs(st.from), End: secs(st.pos)})
			}
			st.from, st.partial, st.shown = st.pos, "", ""
			if deadline != nil {
				return out
			}
		case "session_started", "committed_transcript_with_timestamps", "committed_transcript_entities":
		default:
			log.Println("live (ElevenLabs):", refusal(m))
		}
	}
	return out
}

func secs(samples int) float64 { return math.Round(float64(samples)/live.SR*100) / 100 }

func (st *stream) Preview() (live.Event, bool) {
	if st.partial == "" || st.partial == st.shown {
		return live.Event{}, false
	}
	st.shown = st.partial
	return live.Event{Text: st.partial, Start: secs(st.from), End: secs(st.pos)}, true
}

// Flush asks the service to end the utterance going on and waits a moment for it.
func (st *stream) Flush() []live.Event {
	out := st.take(0)
	if st.s == nil || st.partial == "" {
		st.from = st.pos
		return out
	}
	// a tenth of a second of silence to carry the commit
	pad := make([]float32, live.SR/10)
	if err := st.s.send(pad, true); err != nil {
		st.broke()
		return out
	}
	st.pos += len(pad)
	partial := st.partial
	got := st.take(5 * time.Second)
	if len(got) == 0 && partial != "" { // no answer in time: what was heard so far stands
		got = []live.Event{{Final: true, Text: partial, Start: secs(st.from), End: secs(st.pos)}}
		st.from, st.partial, st.shown = st.pos, "", ""
	}
	return append(out, got...)
}

func (st *stream) Close() {
	if st.s != nil {
		st.s.close()
		st.s = nil
	}
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sttapi_test

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/internal/sttapi"
	"github.com/moliri-app/transcribe/internal/sttapi/sttapitest"
)

func fake(t *testing.T) (*sttapitest.Fake, string) {
	f := &sttapitest.Fake{}
	srv := httptest.NewServer(f.Handler())
	t.Cleanup(srv.Close)
	return f, srv.URL
}

func TestElevenLabs(t *testing.T) {
	f, url := fake(t)
	c := sttapi.Config{Provider: "elevenlabs", URL: url, Model: "scribe_v2", Key: "key"}
	res, err := sttapi.Transcribe(context.Background(), c, strings.NewReader("audio"), "a.mp3",
		sttapi.Request{Language: "de", Speakers: 2, Diarize: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Language != "de" || len(res.Words) == 0 || res.Words[0].Text != "Guten" || res.Words[0].Speaker != "speaker_0" {
		t.Errorf("result: %+v", res)
	}
	laugh := false
	for _, w := range res.Words {
		laugh = laugh || w.Event && w.Text == "laughter"
	}
	if !laugh {
		t.Error("no laughter")
	}
	got := f.Requests[0]
	if got["model_id"] != "scribe_v2" || got["language_code"] != "de" || got["num_speakers"] != "2" ||
		got["diarize"] != "true" || got["timestamps_granularity"] != "word" || got["file"] != "a.mp3" {
		t.Errorf("request: %v", got)
	}

	c.Key = sttapitest.BadKey
	if _, err := sttapi.Transcribe(context.Background(), c, strings.NewReader("x"), "a.mp3", sttapi.Request{}); err == nil ||
		!strings.Contains(err.Error(), "abgelehnt") {
		t.Errorf("bad key: %v", err)
	}
	c.Key = ""
	if _, err := sttapi.Transcribe(context.Background(), c, strings.NewReader("x"), "a.mp3", sttapi.Request{}); err != sttapi.ErrNoKey {
		t.Errorf("no key: %v", err)
	}
}

func TestOpenAI(t *testing.T) {
	f, url := fake(t)
	// a server of one's own needs no key
	c := sttapi.Config{Provider: "custom", URL: url + "/v1/", Model: "whisper-1"}
	res, err := sttapi.Transcribe(context.Background(), c, strings.NewReader("audio"), "a.mp3", sttapi.Request{Language: "de"})
	if err != nil {
		t.Fatal(err)
	}
	// the words get their punctuation back from the text; speakers there are none
	if res.Language != "de" || len(res.Words) < 10 || res.Words[1].Text != "Tag." || res.Words[0].Speaker != "" || res.Words[0].P < 0.85 {
		t.Errorf("result: %+v", res.Words[:2])
	}
	got := f.Requests[0]
	if got["path"] != "/v1/audio/transcriptions" || got["model"] != "whisper-1" || got["language"] != "de" ||
		got["response_format"] != "verbose_json" || got["timestamp_granularities[]"] != "word,segment" {
		t.Errorf("request: %v", got)
	}
	c.Provider, c.Key = "openai", ""
	if _, err := sttapi.Transcribe(context.Background(), c, strings.NewReader("x"), "a.mp3", sttapi.Request{}); err != sttapi.ErrNoKey {
		t.Errorf("OpenAI without a key: %v", err)
	}
}

func TestElevenLabsLive(t *testing.T) {
	_, url := fake(t)
	c := sttapi.Config{Provider: "elevenlabs", URL: url, Model: "scribe_v2", Key: sttapitest.BadKey}
	if _, err := sttapi.OpenLive(context.Background(), c, "de"); err == nil {
		t.Fatal("a refused key opens a stream")
	}
	c.Key = "key"
	s, err := sttapi.OpenLive(context.Background(), c, "auto")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var finals []live.Event
	preview := ""
	chunk := make([]float32, live.SR/10)
	// two seconds: the stand-in says "Guten Tag." after one and starts the next sentence
	for range 20 {
		finals = append(finals, s.Accept(chunk)...)
		time.Sleep(20 * time.Millisecond) // its answers arrive meanwhile
		if ev, ok := s.Preview(); ok {
			preview = ev.Text
		}
	}
	finals = append(finals, s.Accept(chunk)...)
	finals = append(finals, s.Flush()...)
	if len(finals) != 2 || finals[0].Text != "Guten Tag." || !finals[0].Final || finals[1].Text != "Herr Berger." {
		t.Fatalf("finals: %+v", finals)
	}
	if finals[0].Start != 0 || finals[0].End <= 0.9 || finals[1].Start != finals[0].End || finals[1].End < 2 {
		t.Errorf("times: %+v", finals)
	}
	if preview == "" {
		t.Error("no preview")
	}
}

// Live over the OpenAI-compatible API: each utterance, ended by a pause, goes as a recording of its own.
func TestPiecesLive(t *testing.T) {
	f, url := fake(t)
	s, err := sttapi.OpenLive(context.Background(), sttapi.Config{Provider: "custom", URL: url + "/v1", Model: "m"}, "de")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tone := func(secs float64, amp float32) []float32 {
		x := make([]float32, int(secs*live.SR))
		for i := range x {
			if i%20 < 10 {
				x[i] = amp
			} else {
				x[i] = -amp
			}
		}
		return x
	}
	var finals []live.Event
	for _, part := range [][]float32{tone(1, 0.001), tone(1.2, 0.3), tone(1, 0.001), tone(0.8, 0.3)} {
		for i := 0; i < len(part); i += live.SR / 10 {
			finals = append(finals, s.Accept(part[i:min(len(part), i+live.SR/10)])...)
		}
	}
	time.Sleep(300 * time.Millisecond)
	finals = append(finals, s.Accept(tone(0.1, 0.001))...)
	finals = append(finals, s.Flush()...)
	if len(finals) != 2 || finals[0].Text != "Guten Tag." || finals[1].Text != "Herr Berger." {
		t.Fatalf("finals: %+v", finals)
	}
	if finals[0].Start < 0.6 || finals[0].Start > 1 || finals[1].Start < 2.8 {
		t.Errorf("times: %+v", finals)
	}
	if len(f.Requests) != 2 || f.Requests[0]["language"] != "de" {
		t.Errorf("requests: %v", f.Requests)
	}
}

func TestWAV(t *testing.T) {
	b := sttapi.WAV([]float32{0, 1, -1})
	if len(b) != 50 || !bytes.HasPrefix(b, []byte("RIFF")) || b[46] != 0xff || b[47] != 0x7f {
		t.Errorf("wav: % x", b)
	}
}

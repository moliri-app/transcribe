// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package annotate

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/model"
)

const machine = `{"engine":"x","duration":10,"extra":{"keep":1},"segments":[{"words":[
 {"w":"Hallo","start":0,"end":1,"speaker":0},{"w":"Welt.","start":1,"end":2,"speaker":0},
 {"w":"Bonjour","start":3,"end":4,"speaker":1},{"w":"monde.","start":4,"end":5,"speaker":1}]}]}`

func input(t *testing.T, ann string) Input {
	t.Helper()
	a, err := ParseAnnotations([]byte(ann))
	if err != nil {
		t.Fatal(err)
	}
	return Input{Machine: json.RawMessage(machine), Annotations: a, Audio: "talk.mp3"}
}

// run applies in and returns the final transcript as it goes over the wire.
func run(t *testing.T, in Input, rt Retranscriber) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	t.Helper()
	final, cache, err := Apply(context.Background(), in, rt)
	if err != nil {
		return nil, nil, err
	}
	b, _ := json.Marshal(final)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m, cache, nil
}

func TestApplyNilRetranscriber(t *testing.T) {
	m, _, err := run(t, input(t, `{"speakers":{"0":{"name":"Anna"},"1":{"merge_into":"0","language":"fr"}},
	  "spans":[],"edits":[{"start":0,"end":1,"text":"Servus","mode":"replace"}]}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(m["extra"]) != `{"keep":1}` && !json.Valid(m["extra"]) {
		t.Errorf("extra field lost: %s", m["extra"])
	}
	var sp map[string]map[string]string
	_ = json.Unmarshal(m["speakers"], &sp)
	if len(sp) != 1 || sp["0"]["name"] != "Anna" {
		t.Errorf("speakers: %v", sp)
	}
	if string(m["annotated"]) != "true" || string(m["edited_words"]) != "1" || string(m["engine"]) != `"★ Ergebnis"` {
		t.Errorf("flags: %s %s %s", m["annotated"], m["edited_words"], m["engine"])
	}
}

type fakeRT struct{ calls int }

func (f *fakeRT) Retranscribe(_ context.Context, audio string, a, b float64, lang string) ([]model.Word, error) {
	f.calls++
	return []model.Word{{W: "Bonjour", Start: 3, End: 4.5}}, nil
}

func TestApplyLanguageSpan(t *testing.T) {
	in := input(t, `{"speakers":{},"spans":[{"start":2.5,"end":5.5,"language":"fr"}],"edits":[]}`)
	rt := &fakeRT{}
	_, cache, err := run(t, in, rt)
	if err != nil || rt.calls != 1 {
		t.Fatalf("apply: %v calls=%d", err, rt.calls)
	}
	in.Retranscribed = cache
	m, _, err := run(t, in, rt)
	if err != nil || rt.calls != 1 {
		t.Fatalf("second apply should use the cache: %v calls=%d", err, rt.calls)
	}
	if string(m["languages_pending"]) != "0" {
		t.Errorf("languages_pending with the cache: %s", m["languages_pending"])
	}
	in.Audio = ""
	if m, _, err := run(t, in, rt); err != nil || rt.calls != 1 || string(m["languages_pending"]) != "0" {
		t.Fatalf("without audio the cache still applies: %v calls=%d pending=%s", err, rt.calls, m["languages_pending"])
	}
	in.Retranscribed = nil
	if m, _, err := run(t, in, rt); err != nil || rt.calls != 1 || string(m["languages_pending"]) != "1" {
		t.Fatalf("without audio nothing is recognised again, the region is pending: %v calls=%d pending=%s", err, rt.calls, m["languages_pending"])
	}
	var segs []struct{ Text string }
	_ = json.Unmarshal(m["segments"], &segs)
	if len(segs) == 0 {
		t.Error("no segments")
	}
}

func TestApplyLanguageWhisperDoesNotKnow(t *testing.T) {
	rt := &fakeRT{}
	m, _, err := run(t, input(t, `{"speakers":{"0":{"language":"nds"}},"spans":[{"start":2.5,"end":5.5,"language":"gsw"}],"edits":[]}`), rt)
	if err != nil || rt.calls != 0 {
		t.Fatalf("a language Whisper does not know is only noted: %v calls=%d", err, rt.calls)
	}
	if !strings.Contains(string(m["segments"]), "monde.") {
		t.Error("words were replaced")
	}
}

func TestApplyErrors(t *testing.T) {
	if _, _, err := Apply(context.Background(), Input{}, nil); err == nil {
		t.Error("expected error without a machine result")
	}
	if _, _, err := Apply(context.Background(), input(t, `{"spans":[{"start":2.5,"end":5.5,"language":"fr"}]}`), failRT{}); err == nil {
		t.Error("retranscriber error swallowed")
	}
}

type failRT struct{}

func (failRT) Retranscribe(context.Context, string, float64, float64, string) ([]model.Word, error) {
	return nil, errors.New("nope")
}

// A passage recognised again keeps the speaker its sentence showed, not that of a stray machine word.
func TestApplyLanguageSpanKeepsTheSentenceSpeaker(t *testing.T) {
	const m = `{"segments":[{"words":[
	 {"w":"Aber","start":0,"end":0.2,"speaker":1},{"w":"so","start":0.2,"end":0.3,"speaker":1},
	 {"w":"geht","start":0.3,"end":2,"speaker":0},{"w":"das.","start":2,"end":4,"speaker":0}]}]}`
	a, _ := ParseAnnotations([]byte(`{"spans":[{"start":0,"end":0.3,"language":"de"}]}`))
	final, _, err := Apply(context.Background(), Input{Machine: json.RawMessage(m), Annotations: a, Audio: "x"}, &fakeRT{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range final["segments"].([]Word) {
		if Speaker(s) != 0 {
			t.Errorf("segment %q has speaker %d, want 0", s["text"], Speaker(s))
		}
	}
}

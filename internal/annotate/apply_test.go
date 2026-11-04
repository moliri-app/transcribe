// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package annotate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/internal/model"
)

const machine = `{"engine":"x","duration":10,"extra":{"keep":1},"segments":[{"words":[
 {"w":"Hallo","start":0,"end":1,"speaker":0},{"w":"Welt.","start":1,"end":2,"speaker":0},
 {"w":"Bonjour","start":3,"end":4,"speaker":1},{"w":"monde.","start":4,"end":5,"speaker":1}]}]}`

func setup(t *testing.T, ann string) string {
	t.Helper()
	d := t.TempDir()
	rd := filepath.Join(d, "results", "talk")
	_ = os.MkdirAll(rd, 0o755)
	_ = os.MkdirAll(filepath.Join(d, "audio"), 0o755)
	_ = os.WriteFile(filepath.Join(d, "audio", "talk.mp3"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(rd, "_machine.json"), []byte(machine), 0o644)
	if ann != "" {
		_ = os.WriteFile(filepath.Join(rd, "_annotations.json"), []byte(ann), 0o644)
	}
	return d
}

func readFinal(t *testing.T, d string) map[string]json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(d, "results", "talk", "_final.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestApplyNilRetranscriber(t *testing.T) {
	d := setup(t, `{"speakers":{"0":{"name":"Anna"},"1":{"merge_into":"0","language":"fr"}},
	  "spans":[],"edits":[{"start":0,"end":1,"text":"Servus","mode":"replace"}]}`)
	if err := Apply(context.Background(), d, "talk", nil); err != nil {
		t.Fatal(err)
	}
	m := readFinal(t, d)
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
	d := setup(t, `{"speakers":{},"spans":[{"start":2.5,"end":5.5,"language":"fr"}],"edits":[]}`)
	rt := &fakeRT{}
	if err := Apply(context.Background(), d, "talk", rt); err != nil || rt.calls != 1 {
		t.Fatalf("apply: %v calls=%d", err, rt.calls)
	}
	if err := Apply(context.Background(), d, "talk", rt); err != nil || rt.calls != 1 {
		t.Fatalf("second apply should use the cache: %v calls=%d", err, rt.calls)
	}
	var segs []struct{ Text string }
	_ = json.Unmarshal(readFinal(t, d)["segments"], &segs)
	if len(segs) == 0 {
		t.Error("no segments")
	}
}

func TestApplyLanguageWhisperDoesNotKnow(t *testing.T) {
	d := setup(t, `{"speakers":{"0":{"language":"nds"}},"spans":[{"start":2.5,"end":5.5,"language":"gsw"}],"edits":[]}`)
	rt := &fakeRT{}
	if err := Apply(context.Background(), d, "talk", rt); err != nil || rt.calls != 0 {
		t.Fatalf("a language Whisper does not know is only noted: %v calls=%d", err, rt.calls)
	}
	if !strings.Contains(string(readFinal(t, d)["segments"]), "monde.") {
		t.Error("words were replaced")
	}
}

func TestApplyErrors(t *testing.T) {
	d := t.TempDir()
	_ = os.MkdirAll(filepath.Join(d, "results", "talk"), 0o755)
	if err := Apply(context.Background(), d, "talk", nil); err == nil {
		t.Error("expected error without _machine.json")
	}
	d = setup(t, `{"spans":[{"start":2.5,"end":5.5,"language":"fr"}]}`)
	if err := Apply(context.Background(), d, "talk", failRT{}); err == nil {
		t.Error("retranscriber error swallowed")
	}
}

type failRT struct{}

func (failRT) Retranscribe(context.Context, string, float64, float64, string) ([]model.Word, error) {
	return nil, errors.New("nope")
}

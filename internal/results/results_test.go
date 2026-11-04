// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package results

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/model"
)

const machine = `{"engine":"x","duration":10,"extra":{"keep":1},"segments":[{"words":[
 {"w":"Hallo","start":0,"end":1,"speaker":0},{"w":"Welt.","start":1,"end":2,"speaker":0},
 {"w":"Bonjour","start":3,"end":4,"speaker":1},{"w":"monde.","start":4,"end":5,"speaker":1}]}]}`

func setup(t *testing.T, ann string) string {
	t.Helper()
	d := t.TempDir()
	rd := Dir(d, "talk")
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
	b, err := os.ReadFile(filepath.Join(Dir(d, "talk"), "_final.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

type fakeRT struct {
	calls int
	audio string
}

func (f *fakeRT) Retranscribe(_ context.Context, audio string, a, b float64, lang string) ([]model.Word, error) {
	f.calls++
	f.audio = audio
	return []model.Word{{W: "Bonjour", Start: 3, End: 4.5}}, nil
}

func TestApplyWritesFinal(t *testing.T) {
	d := setup(t, `{"speakers":{"0":{"name":"Anna"},"1":{"merge_into":"0","language":"fr"}},
	  "spans":[],"edits":[{"start":0,"end":1,"text":"Servus","mode":"replace"}]}`)
	if err := Apply(context.Background(), d, "talk", nil); err != nil {
		t.Fatal(err)
	}
	m := readFinal(t, d)
	var sp map[string]map[string]string
	_ = json.Unmarshal(m["speakers"], &sp)
	if len(sp) != 1 || sp["0"]["name"] != "Anna" || string(m["edited_words"]) != "1" || !json.Valid(m["extra"]) {
		t.Errorf("final: speakers %v, edited %s, extra %s", sp, m["edited_words"], m["extra"])
	}
}

// The re-recognition cache lives beside the transcript: written once, then read instead of
// recognising again; without a retranscriber a region not in it is pending.
func TestApplyKeepsTheCache(t *testing.T) {
	d := setup(t, `{"speakers":{},"spans":[{"start":2.5,"end":5.5,"language":"fr"}],"edits":[]}`)
	rt := &fakeRT{}
	if err := Apply(context.Background(), d, "talk", rt); err != nil || rt.calls != 1 || filepath.Base(rt.audio) != "talk.mp3" {
		t.Fatalf("apply: %v calls=%d audio=%q", err, rt.calls, rt.audio)
	}
	cache := filepath.Join(Dir(d, "talk"), "_retranscribed.json")
	if _, err := os.Stat(cache); err != nil {
		t.Fatal(err)
	}
	if err := Apply(context.Background(), d, "talk", rt); err != nil || rt.calls != 1 {
		t.Fatalf("second apply should use the cache: %v calls=%d", err, rt.calls)
	}
	if err := Apply(context.Background(), d, "talk", nil); err != nil || string(readFinal(t, d)["languages_pending"]) != "0" {
		t.Fatalf("without a retranscriber the cache still applies: %v", err)
	}
	_ = os.Remove(cache)
	if err := Apply(context.Background(), d, "talk", nil); err != nil || string(readFinal(t, d)["languages_pending"]) != "1" {
		t.Fatalf("not in the cache, the region is pending: %v %s", err, readFinal(t, d)["languages_pending"])
	}
}

func TestApplyErrors(t *testing.T) {
	d := t.TempDir()
	_ = os.MkdirAll(Dir(d, "talk"), 0o755)
	if err := Apply(context.Background(), d, "talk", nil); err == nil {
		t.Error("expected error without _machine.json")
	}
	d = setup(t, `{"spans":[{"start":2.5,"end":5.5,"language":"fr"}]}`)
	_ = os.Remove(filepath.Join(d, "audio", "talk.mp3"))
	if err := Apply(context.Background(), d, "talk", &fakeRT{}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("a region to recognise without the recording: %v", err)
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

func TestExport(t *testing.T) {
	d := setup(t, "")
	if _, _, _, err := Export(d, "talk", "txt"); !errors.Is(err, ErrNoTranscript) {
		t.Errorf("before apply: %v", err)
	}
	if err := Apply(context.Background(), d, "talk", nil); err != nil {
		t.Fatal(err)
	}
	body, ctype, name, err := Export(d, "talk", "txt")
	if err != nil || !strings.Contains(string(body), "Bonjour") || !strings.HasPrefix(ctype, "text/plain") || name != "talk.txt" {
		t.Errorf("export: %v %q %q %q", err, body, ctype, name)
	}
}

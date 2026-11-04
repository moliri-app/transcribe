// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package runner

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/internal/sttapi/sttapitest"
	"github.com/moliri-app/transcribe/jobs"
	"github.com/moliri-app/transcribe/model"
)

// apiData is a data directory with the staged interview, its recognition set to the service.
func apiData(t *testing.T, api prefs.Service) (string, *sttapitest.Fake) {
	t.Helper()
	f := &sttapitest.Fake{}
	srv := httptest.NewServer(f.Handler())
	t.Cleanup(srv.Close)
	api.URL = strings.Replace(api.URL, "FAKE", srv.URL, 1)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "audio"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "e2e", "testdata", "audio", "Papierfabrik Interview.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "audio", "a.mp3"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := prefs.Update(dir, func(p *prefs.Prefs) { p.Recognition, p.API = prefs.API, api }); err != nil {
		t.Fatal(err)
	}
	return dir, f
}

func machine(t *testing.T, dir string) model.Machine {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "results", "a", "_machine.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m model.Machine
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// A recording sent to ElevenLabs as it is: words, speakers and sounds land in _machine.json,
// without any model on this computer.
func TestRunElevenLabs(t *testing.T) {
	dir, f := apiData(t, prefs.Service{Provider: "elevenlabs", URL: "FAKE", Key: "key"})
	r := New(dir)
	o := jobs.Options{Preset: "interview", Language: "auto", Engine: prefs.API}
	if err := r.Run(context.Background(), dir, "a.mp3", o, nil); err != nil {
		t.Fatal(err)
	}
	if got := f.Requests[0]; got["num_speakers"] != "2" || got["language_code"] != "" || got["model_id"] != "scribe_v2" || got["file"] != "a.mp3" {
		t.Errorf("request: %v", got)
	}
	m := machine(t, dir)
	if m.EngineUsed != "api-elevenlabs~scribe_v2" || m.Duration != 35.14 || len(m.Segments) < 8 {
		t.Fatalf("machine: %s %v %d", m.EngineUsed, m.Duration, len(m.Segments))
	}
	first, second := m.Segments[0], m.Segments[1]
	if first.Text != "Guten Tag." || second.Text != "Herr Berger." || *first.Words[0].Speaker != 0 || first.Words[0].Lid != "" {
		t.Errorf("segments: %+v / %+v", first, second)
	}
	spk := map[int]bool{}
	for _, s := range m.Segments {
		spk[*s.Words[0].Speaker] = true
	}
	if len(spk) != 2 {
		t.Errorf("speakers: %v", spk)
	}
	if len(m.Events) != 1 || m.Events[0].Label != "Lachen" {
		t.Errorf("events: %+v", m.Events)
	}
	if _, err := os.Stat(filepath.Join(dir, "results", "a", "_final.json")); err != nil {
		t.Error("no _final.json:", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "models")); err == nil {
		t.Error("models were fetched")
	}

	// recognition on this computer is switched off: a local job is refused, not started
	if err := r.Run(context.Background(), dir, "a.mp3", jobs.Options{Preset: "auto"}, nil); err == nil || !strings.Contains(err.Error(), "ausgeschaltet") {
		t.Errorf("local job: %v", err)
	}
}

// A service with the OpenAI-compatible API gets the recording as WAV; it tells no speakers apart.
func TestRunOpenAI(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not on the PATH")
	}
	dir, f := apiData(t, prefs.Service{Provider: "custom", URL: "FAKE/v1"})
	r := New(dir)
	if err := r.Run(context.Background(), dir, "a.mp3", jobs.Options{Preset: "interview", Language: "de", Engine: prefs.API}, nil); err != nil {
		t.Fatal(err)
	}
	if got := f.Requests[0]; got["model"] != "whisper-1" || got["language"] != "de" || got["file"] != "teil-1.wav" {
		t.Errorf("request: %v", got)
	}
	m := machine(t, dir)
	if m.EngineUsed != "api-custom~whisper-1" || m.SpeakersVariant != nil || len(m.Segments) < 8 || m.Segments[1].Text != "Herr Berger." {
		t.Fatalf("machine: %s %v %+v", m.EngineUsed, m.SpeakersVariant, m.Segments)
	}
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

// fakeStream "recognises" one utterance per second of audio and previews the second going on.
type fakeStream struct {
	n, said int
	closed  bool
}

func (f *fakeStream) Accept(pcm []float32) []Event {
	f.n += len(pcm)
	var out []Event
	for f.said+SR <= f.n {
		f.said += SR
		out = append(out, Event{Final: true, Text: "Satz", Start: float64(f.said-SR) / SR, End: float64(f.said) / SR})
	}
	return out
}

func (f *fakeStream) Preview() (Event, bool) {
	if f.n == f.said {
		return Event{}, false
	}
	return Event{Text: "Sa", Start: float64(f.said) / SR, End: float64(f.n) / SR}, true
}

func (f *fakeStream) Flush() []Event {
	if f.n == f.said {
		return nil
	}
	ev := Event{Final: true, Text: "Rest", Start: float64(f.said) / SR, End: float64(f.n) / SR}
	f.said = f.n
	return []Event{ev}
}

func (f *fakeStream) Close() { f.closed = true }

func TestAudioSessionKeepsANote(t *testing.T) {
	dir := t.TempDir()
	stream := &fakeStream{}
	h := NewHub(dir, func(context.Context, string) (Stream, error) { return stream, nil })
	var mu sync.Mutex
	var got []Event
	s, err := h.Audio(context.Background(), "Moliri", func(ev Event) {
		mu.Lock()
		got = append(got, ev)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	if !h.Busy() {
		t.Error("a running session must keep the app busy")
	}
	for range 25 { // 2.5 s
		s.Write(make([]float32, SR/10))
	}
	s.End(nil)
	if h.Busy() || !stream.closed {
		t.Errorf("after End: busy %v, stream closed %v", h.Busy(), stream.closed)
	}
	var finals []string
	for _, ev := range got {
		if ev.Final {
			finals = append(finals, ev.Text)
		}
	}
	if len(finals) != 3 || finals[2] != "Rest" {
		t.Errorf("finals sent: %v", finals)
	}

	// the note is on disk and comes back after a restart
	if _, err := os.Stat(filepath.Join(dir, "live", s.Note()+".json")); err != nil {
		t.Fatal(err)
	}
	n := NewHub(dir, nil).Note(s.Note())
	if n == nil || n.Kind != KindAudio || n.Client != "Moliri" || n.Ended == nil || n.Text() != "Satz\nSatz\nRest\n" {
		t.Fatalf("note after restart: %+v", n)
	}
	if err := h.Delete(s.Note()); err != nil {
		t.Fatal(err)
	}
	if NewHub(dir, nil).Note(s.Note()) != nil {
		t.Error("deleted note is back")
	}
}

// While live transcriptions run, the transcription of whole recordings is held, once for all of them.
func TestSessionsHoldTheQueue(t *testing.T) {
	h := NewHub(t.TempDir(), func(context.Context, string) (Stream, error) { return &fakeStream{}, nil })
	holds, releases := 0, 0
	h.Hold = func() func() {
		holds++
		return func() { releases++ }
	}
	a, err := h.Audio(context.Background(), "Moliri", func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	b, err := h.Audio(context.Background(), "Skript", func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	if holds != 1 || releases != 0 {
		t.Errorf("two sessions: %d holds, %d releases", holds, releases)
	}
	a.End(nil)
	if releases != 0 {
		t.Error("released while a session still runs")
	}
	b.End(nil)
	if holds != 1 || releases != 1 {
		t.Errorf("after both: %d holds, %d releases", holds, releases)
	}
}

func TestAllowedOrigins(t *testing.T) {
	h := NewHub(t.TempDir(), nil)
	for origin, want := range map[string]bool{
		"":                        true, // a program, not a web page
		"http://localhost:5173":   true,
		"http://127.0.0.1:8080":   true,
		"https://moliri.example":  false,
		"null":                    false,
		"http://localhost.evil.x": false,
	} {
		if got := h.Allowed(origin); got != want {
			t.Errorf("Allowed(%q) = %v, want %v", origin, got, want)
		}
	}
	h.Refused("https://moliri.example", "Moliri")
	if err := h.Allow("https://Moliri.example/", true); err != nil {
		t.Fatal(err)
	}
	if !h.Allowed("https://moliri.example") {
		t.Error("allowed origin is still refused")
	}
	if b := h.access()["blocked"].([]Refusal); len(b) != 0 {
		t.Errorf("allowed origin still listed as refused: %v", b)
	}
	if err := h.Allow("https://moliri.example", false); err != nil {
		t.Fatal(err)
	}
	if h.Allowed("https://moliri.example") {
		t.Error("origin is still allowed after taking it back")
	}
}

func TestMixedLinesTellTheirSource(t *testing.T) {
	r := &run{source: SourceBoth}
	for range 10 { // 1 s of the own voice, then 1 s of the others
		r.loud = append(r.loud, [2]float32{0.1, 0.001})
	}
	for range 10 {
		r.loud = append(r.loud, [2]float32{0.002, 0.2})
	}
	if s := r.whose(0.1, 0.9); s != SourceMic {
		t.Errorf("first second: %q", s)
	}
	if s := r.whose(0.8, 1.9); s != SourceSystem {
		t.Errorf("second second: %q", s)
	}
	n := &Note{Lines: []Line{{Text: "Frage?", Source: SourceMic}, {Text: "Antwort.", Source: SourceSystem}, {Text: "Mehr.", Source: SourceSystem}}}
	if got, want := n.Text(), "Mikrofon:\nFrage?\n\nSystemaudio:\nAntwort.\nMehr.\n"; got != want {
		t.Errorf("text %q, want %q", got, want)
	}
	if s := (&run{}).whose(0, 1); s != "" {
		t.Errorf("one source only: %q", s)
	}
}

func TestRecordingWaitsForSound(t *testing.T) {
	dir := t.TempDir()
	h := NewHub(dir, nil)
	r := newRun(h, &fakeStream{}, 8, SourceMic)
	n := h.newNote(KindDauer, "")
	tp := &tap{note: n, ended: make(chan struct{})}
	r.do(func() { r.taps[tp] = struct{}{}; h.waiting(n) })
	if c := h.Note(n.ID); c.Loading || !c.Waiting {
		t.Fatalf("before the first sound: loading %v, waiting %v", c.Loading, c.Waiting)
	}
	before := h.Note(n.ID).Started
	r.audio <- chunk{pcm: make([]float32, SR/10)}
	r.do(func() {})
	if c := h.Note(n.ID); c.Waiting || !c.Started.After(before) {
		t.Errorf("after the first sound: waiting %v, started %v (was %v)", c.Waiting, c.Started, before)
	}
	close(r.audio)
	<-r.done
}

func TestEmptyRecordingIsNotKept(t *testing.T) {
	dir := t.TempDir()
	h := NewHub(dir, nil)
	path := filepath.Join(dir, "leer.wav")
	w, err := createWav(path)
	if err != nil {
		t.Fatal(err)
	}
	n := h.newNote(KindDauer, "")
	n.Audio = "leer.wav"
	h.finish(&tap{note: n, wav: w, ended: make(chan struct{})}, nil)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the empty recording is still there: %v", err)
	}
	if c := h.Note(n.ID); c.Audio != "" || c.Error == "" {
		t.Errorf("note of an empty recording: audio %q, error %q", c.Audio, c.Error)
	}
}

// On a busy computer recognition falls behind the recording; what was said before Stop still
// counts.
func TestStopRecognisesWhatWaits(t *testing.T) {
	h := NewHub(t.TempDir(), nil)
	r := newRun(h, &fakeStream{}, 64, SourceMic)
	r.cancel = func() {}
	h.mic = r
	var mu sync.Mutex
	var finals []string
	tp := &tap{note: h.newNote(KindDiktat, ""), ended: make(chan struct{}), send: func(ev Event) {
		if ev.Final {
			mu.Lock()
			finals = append(finals, ev.Text)
			mu.Unlock()
		}
	}}
	r.do(func() { r.taps[tp] = struct{}{} })
	hold, held := make(chan struct{}), make(chan struct{})
	go r.do(func() { close(held); <-hold }) // recognition is busy
	<-held
	for range 25 { // 2.5 s wait for it
		r.audio <- chunk{pcm: make([]float32, SR/10)}
	}
	go func() { close(hold) }()
	h.untap(tp, nil)
	mu.Lock()
	if len(finals) != 3 || finals[2] != "Rest" {
		t.Errorf("finals sent: %v, want two seconds and the rest", finals)
	}
	mu.Unlock()
	close(r.audio)
	<-r.done
}

// hearingStream hears speech from the start; its preview covers what was said up to then.
type hearingStream struct {
	fakeStream
	covered int
}

func (f *hearingStream) Preview() (Event, bool) {
	ev, ok := f.fakeStream.Preview()
	if ok {
		f.covered = f.n
	}
	return ev, ok
}

func (f *hearingStream) Pending() (from, to float64, ok bool) {
	return float64(f.covered) / SR, float64(f.n) / SR, true
}

// The Live page learns how much speech waits for recognition and, on its own stream, that the
// microphone sends only telephone quality; a program gets neither.
func TestLivePageHearsWhatWaits(t *testing.T) {
	h := NewHub(t.TempDir(), nil)
	_, updates, cancel := h.Subscribe()
	defer cancel()
	r := newRun(h, &hearingStream{}, 64, SourceMic)
	r.cancel = func() {}
	h.mic = r
	var mu sync.Mutex
	var sent []Event
	tp := &tap{note: h.newNote(KindMic, "Moliri"), ended: make(chan struct{}), send: func(ev Event) {
		mu.Lock()
		sent = append(sent, ev)
		mu.Unlock()
	}}
	r.do(func() { r.taps[tp] = struct{}{} })
	narrow := make([]float32, SR/10)
	for i := range narrow {
		narrow[i] = float32(0.1 * math.Sin(2*math.Pi*500*float64(i)/SR))
	}
	for range 40 { // 4 s of speech that has nothing above 4.5 kHz
		r.audio <- chunk{pcm: narrow, micPCM: narrow}
	}
	r.do(r.drain)
	hearing, input := 0, ""
	for len(updates) > 0 {
		var m struct {
			Type    string
			Pending *float64
			Mic     MicState
		}
		_ = json.Unmarshal(<-updates, &m)
		switch m.Type {
		case "hearing":
			if m.Pending == nil {
				t.Error("hearing without pending")
			}
			hearing++
		case "mic":
			input = m.Mic.Input
		}
	}
	if hearing == 0 {
		t.Error("the Live page heard nothing of the speech waiting")
	}
	if s := h.micState(); s.Input != InputNarrow {
		t.Errorf("microphone state: %+v, want narrow; last sent %q", s, input)
	}
	h.InputChanged()
	if s := h.micState(); s.Input != "" {
		t.Errorf("after choosing another microphone: %+v", s)
	}
	mu.Lock()
	for _, ev := range sent {
		if ev.Text == "" {
			t.Errorf("the program got an empty event: %+v", ev)
		}
	}
	mu.Unlock()
	close(r.audio)
	<-r.done
}

// A mark said at the start of an utterance ends the line before.
func TestRecordAttachesSpokenMarks(t *testing.T) {
	h := NewHub(t.TempDir(), nil)
	n := h.newNote(KindDauer, "")
	for _, text := range []string{"Ich komme morgen", ". Dann gehen wir", "\n\n", "Gruß Anna"} {
		h.record(n, Event{Final: true, Text: text})
	}
	var got []string
	for _, l := range h.Note(n.ID).Lines {
		got = append(got, l.Text)
	}
	if want := []string{"Ich komme morgen.", "Dann gehen wir\n\n", "Gruß Anna"}; !slices.Equal(got, want) {
		t.Errorf("lines %q, want %q", got, want)
	}
}

// Off, „Im Hintergrund bereit“ only turns away programs that want the app to record the
// microphone; audio a program sends itself is still transcribed.
func TestOffTurnsAwayOnlyTheMicrophone(t *testing.T) {
	h := NewHub(t.TempDir(), func(context.Context, string) (Stream, error) { return &fakeStream{}, nil })
	if err := h.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Listen(context.Background(), "Moliri", func(Event) {}); err != errOff {
		t.Errorf("listening while off: %v, want errOff", err)
	}
	var got []string
	s, err := h.Audio(context.Background(), "Moliri", func(ev Event) {
		if ev.Final {
			got = append(got, ev.Text)
		}
	})
	if err != nil {
		t.Fatalf("sending audio while off: %v", err)
	}
	s.Write(make([]float32, SR))
	s.End(nil)
	if len(got) != 1 || got[0] != "Satz" {
		t.Errorf("finals sent: %v", got)
	}
}

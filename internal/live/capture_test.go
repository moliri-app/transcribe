// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/prefs"
)

func writeTone(t *testing.T, path string, hz float64) {
	w, err := createWav(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(tone(hz, SR, SR*5)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// record runs capture for d and returns what it handed out.
func recordFor(t *testing.T, source string, d time.Duration) (pcm, mic, sys []float32) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	var mu sync.Mutex
	err := capture(ctx, source, "", func(p, m, s []float32) {
		mu.Lock()
		defer mu.Unlock()
		if len(p) != SR/10 || (m != nil && len(m) != len(p)) || (s != nil && len(s) != len(p)) {
			t.Errorf("pieces of %d, %d, %d samples; want 100 ms", len(p), len(m), len(s))
		}
		pcm, mic, sys = append(pcm, p...), append(mic, m...), append(sys, s...)
	})
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestBothSourcesAreMixed(t *testing.T) {
	if _, err := exec.LookPath(media.Tool("ffmpeg")); err != nil {
		t.Skip("no ffmpeg")
	}
	dir := t.TempDir()
	writeTone(t, filepath.Join(dir, "mic.wav"), 440)
	writeTone(t, filepath.Join(dir, "system.wav"), 1000)
	t.Setenv("TRANSCRIBE_LIVE_INPUT", filepath.Join(dir, "mic.wav"))
	t.Setenv("TRANSCRIBE_LIVE_SYSTEM_INPUT", filepath.Join(dir, "system.wav"))

	pcm, mic, sys := recordFor(t, SourceBoth, 3*time.Second)
	if len(pcm) < SR {
		t.Fatalf("%d samples in 3 s", len(pcm))
	}
	// the system sound comes in after the mixing buffer has filled
	end := len(pcm) - SR/10
	if r := rms(mic[SR/2 : end]); r < 0.3 {
		t.Errorf("microphone RMS %.3f", r)
	}
	if r := rms(sys[SR/2 : end]); r < 0.3 {
		t.Errorf("system sound RMS %.3f", r)
	}
	for i := range pcm {
		if pcm[i] != mic[i]+sys[i] {
			t.Fatalf("sample %d: %v is not %v + %v", i, pcm[i], mic[i], sys[i])
		}
	}

	_, mic, sys = recordFor(t, SourceSystem, 2*time.Second)
	if mic != nil || len(sys) < SR/2 || rms(sys[SR/10:]) < 0.3 {
		t.Errorf("system sound alone: %d samples of the microphone, %d of the system (RMS %.3f)", len(mic), len(sys), rms(sys))
	}
}

// stepper is a recorder for mix that hands out a piece each time the test says so.
type stepper struct {
	step chan []float32
	done chan struct{}
}

func newStepper() *stepper { return &stepper{make(chan []float32), make(chan struct{})} }

func (s *stepper) record(ctx context.Context, piece func([]float32)) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case p := <-s.step:
			piece(p)
			s.done <- struct{}{}
		}
	}
}

func (s *stepper) give(p []float32) {
	s.step <- p
	<-s.done
}

// A microphone that gives no sound for longer than the mix used to wait (a Bluetooth headset that
// stalls) loses nothing of the system sound: it goes on with silence for the microphone, in order,
// and the log tells.
func TestMixKeepsTheSystemSoundWhileTheMicrophoneStalls(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)

	mic, sys := newStepper(), newStepper()
	ctx, cancel := context.WithCancel(context.Background())
	var pcm, micOut, sysOut []float32
	done := make(chan error)
	go func() {
		done <- mix(ctx, mic.record, sys.record, func(p, m, s []float32) {
			pcm, micOut, sysOut = append(pcm, p...), append(micOut, m...), append(sysOut, s...)
		})
	}()

	// the system sound counts its samples, in pieces of 10 ms; the microphone gives -1
	next := float32(0)
	system := func(pieces int) {
		for range pieces {
			p := make([]float32, SR/100)
			for i := range p {
				next++
				p[i] = next
			}
			sys.give(p)
		}
	}
	speak := func(seconds int) {
		for range seconds * 10 {
			system(10)
			p := make([]float32, SR/10)
			for i := range p {
				p[i] = -1
			}
			mic.give(p)
		}
	}
	speak(3)
	system(1500) // the microphone stalls for 15 s, for longer than the mix used to keep the system sound
	speak(3)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	var got []float32
	for _, v := range sysOut {
		if v != 0 {
			got = append(got, v)
		}
	}
	for i, v := range got {
		if v != float32(i+1) {
			t.Fatalf("system sample %d is %v: the system sound lost some of itself", i+1, v)
		}
	}
	if left := int(next) - len(got); left > mixDelay+SR/10 {
		t.Errorf("%.1f s of the system sound not handed out", float64(left)/SR)
	}
	for i := range pcm {
		if pcm[i] != micOut[i]+sysOut[i] {
			t.Fatalf("sample %d: %v is not %v + %v", i, pcm[i], micOut[i], sysOut[i])
		}
	}
	spoken := 0
	for _, v := range micOut {
		if v == -1 {
			spoken++
		}
	}
	if spoken != 6*SR {
		t.Errorf("%.1f s of the microphone handed out, want 6 s", float64(spoken)/SR)
	}
	for _, want := range []string{"the system audio goes on with silence for it", "the microphone gives sound again"} {
		if !strings.Contains(logged.String(), want) {
			t.Errorf("the log does not say %q:\n%s", want, logged.String())
		}
	}
}

// TestLoopback records this computer's output for two seconds; set TRANSCRIBE_TEST_LOOPBACK=1.
// While nothing plays, it is silence, without gaps.
func TestLoopback(t *testing.T) {
	if runtime.GOOS != "windows" || os.Getenv("TRANSCRIBE_TEST_LOOPBACK") == "" {
		t.Skip("set TRANSCRIBE_TEST_LOOPBACK=1 on Windows")
	}
	pcm, _, sys := recordFor(t, SourceSystem, 2*time.Second)
	t.Logf("%d samples, RMS %.4f", len(pcm), rms(sys))
	if len(pcm) < SR*12/10 {
		t.Errorf("%d samples in 2 s", len(pcm))
	}
}

func TestDictationRecordsTheMicrophone(t *testing.T) {
	if _, err := exec.LookPath(media.Tool("ffmpeg")); err != nil {
		t.Skip("no ffmpeg")
	}
	dir := t.TempDir()
	writeTone(t, filepath.Join(dir, "mic.wav"), 440)
	writeTone(t, filepath.Join(dir, "system.wav"), 1000)
	t.Setenv("TRANSCRIBE_LIVE_INPUT", filepath.Join(dir, "mic.wav"))
	t.Setenv("TRANSCRIBE_LIVE_SYSTEM_INPUT", filepath.Join(dir, "system.wav"))
	if _, err := prefs.Update(dir, func(p *prefs.Prefs) { p.LiveSource = SourceBoth }); err != nil {
		t.Fatal(err)
	}
	h := NewHub(dir, func(context.Context, string) (Stream, error) { return &fakeStream{}, nil })

	// the Live page records both, a dictation only the microphone
	var mu sync.Mutex
	var finals []string
	l, err := h.Dictate(context.Background(), func(ev Event) {
		if ev.Final {
			mu.Lock()
			finals = append(finals, ev.Text)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-l.Heard():
	case <-time.After(30 * time.Second): // ffmpeg starts slowly on a busy computer
		t.Fatal("no sound came")
	}
	time.Sleep(1500 * time.Millisecond)
	l.Stop()
	n := h.Note(l.Note())
	if n.Kind != KindDiktat || n.Source != SourceMic || n.Ended == nil {
		t.Errorf("note: kind %q, source %q, ended %v", n.Kind, n.Source, n.Ended)
	}
	mu.Lock()
	if len(finals) == 0 || len(n.Lines) != len(finals) {
		t.Errorf("finals sent: %v, lines kept: %d", finals, len(n.Lines))
	}
	mu.Unlock()

	// while only the system sound is recorded, there is nothing to dictate from
	if _, err := prefs.Update(dir, func(p *prefs.Prefs) { p.LiveSource = SourceSystem }); err != nil {
		t.Fatal(err)
	}
	if err := h.StartDauer(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	defer h.StopDauer()
	if _, err := h.Dictate(context.Background(), func(Event) {}); err != errSystemOnly {
		t.Errorf("dictating during a recording of the system sound: %v", err)
	}
}

// Once the recording on the Live page is stopped, the page is told so last: an older state of the
// microphone, still naming the recording, never comes after it.
func TestStopDauerTellsTheLivePageLast(t *testing.T) {
	if _, err := exec.LookPath(media.Tool("ffmpeg")); err != nil {
		t.Skip("no ffmpeg")
	}
	dir := t.TempDir()
	writeTone(t, filepath.Join(dir, "mic.wav"), 440)
	t.Setenv("TRANSCRIBE_LIVE_INPUT", filepath.Join(dir, "mic.wav"))
	if err := os.MkdirAll(media.AudioDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	h := NewHub(dir, func(context.Context, string) (Stream, error) { return &fakeStream{}, nil })
	for range 5 {
		if err := h.StartDauer(context.Background(), true); err != nil {
			t.Fatal(err)
		}
		if err := h.StartDauer(context.Background(), true); err == nil {
			t.Fatal("a second recording started")
		}
		_, updates, cancel := h.Subscribe()
		time.Sleep(300 * time.Millisecond)
		h.StopDauer()
		if d := h.micState().Dauer; d != "" {
			t.Fatalf("still recording %s once stopped", d)
		}
		last := ""
		for quiet := false; !quiet; {
			select {
			case b := <-updates:
				var m struct {
					Type string
					Mic  MicState
				}
				if json.Unmarshal(b, &m) == nil && m.Type == "mic" {
					last = m.Mic.Dauer
				}
			case <-time.After(300 * time.Millisecond):
				quiet = true
			}
		}
		cancel()
		if last != "" {
			t.Fatalf("the Live page was told last that %s still records", last)
		}
	}
}

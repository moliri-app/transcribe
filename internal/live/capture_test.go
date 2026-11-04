// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/internal/audio"
	"github.com/moliri-app/transcribe/internal/models"
	"github.com/moliri-app/transcribe/internal/sherpa"
)

// The clustering of a long recording stops when its job is cancelled.
func TestAverageLinkageStopsOnCancel(t *testing.T) {
	x := make([][]float32, 50)
	for i := range x {
		x[i] = normalize([]float32{float32(i), 1, float32(i % 7)})
	}
	if m, err := averageLinkage(context.Background(), x); err != nil || len(m) != len(x)-1 {
		t.Fatalf("%d merges, %v", len(m), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := averageLinkage(ctx, x); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
}

// A job cancelled while the speakers are found leaves DiarizeAHC2 within moments, also while the
// sherpa diarizer runs over the recording.
// go test -run DiarizeStops -v ./internal/pipeline, with SHERPA_LIB, TRANSCRIBE_MODELS and
// TRANSCRIBE_AUDIO (a recording of some minutes).
func TestDiarizeStopsOnCancel(t *testing.T) {
	lib := sherpa.FindLib(os.Getenv("SHERPA_LIB"))
	dir, file := os.Getenv("TRANSCRIBE_MODELS"), os.Getenv("TRANSCRIBE_AUDIO")
	if lib == "" || dir == "" || file == "" {
		t.Skip("set SHERPA_LIB, TRANSCRIBE_MODELS, TRANSCRIBE_AUDIO (a recording of some minutes)")
	}
	sherpa.SetLibDir(lib)
	m := models.Set{Dir: dir}
	pcm, err := audio.Decode(context.Background(), file)
	if err != nil {
		t.Skip(err)
	}
	for _, after := range []time.Duration{3 * time.Second, 20 * time.Second} {
		ctx, cancel := context.WithCancel(context.Background())
		var canceled time.Time
		time.AfterFunc(after, func() { canceled = time.Now(); cancel() })
		_, err := DiarizeAHC2(ctx, pcm, m.Segmentation(), m.CampPlus(), 0, 0.6, 4)
		took := time.Since(canceled)
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled after %v: %v", after, err)
		}
		t.Logf("cancelled after %v: returned %v later", after, took.Round(time.Millisecond))
		if took > 2*time.Second {
			t.Errorf("cancelled after %v: returned only %v later", after, took)
		}
	}
}

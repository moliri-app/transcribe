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

	"github.com/moliri-app/transcribe/audio"
	"github.com/moliri-app/transcribe/models"
	"github.com/moliri-app/transcribe/sherpa"
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

// Two speakers asked for, a few seconds of intro music do not take the place of the second voice:
// cut into exactly two, the tree splits off the music and leaves both voices in one cluster.
func TestFixedSpeakersSkipScraps(t *testing.T) {
	var x [][]float32
	var spans []span
	add := func(n int, v []float32) {
		for range n {
			s := float64(len(spans)) * 0.75
			x = append(x, normalize(v))
			spans = append(spans, span{s, s + 1.5})
		}
	}
	add(5, []float32{0, 0, 1})     // music: 3.75 s
	add(100, []float32{1, 0, 0})   // guest: 75 s
	add(30, []float32{1, 1.7, 0})  // host, 60° from the guest: 22.5 s
	add(100, []float32{1, 0, 0.1}) // guest again
	merges, err := averageLinkage(context.Background(), x)
	if err != nil {
		t.Fatal(err)
	}
	if l := cut(len(x), merges, 2, 0); l[5] != l[105] {
		t.Fatalf("the test no longer shows the problem: cut into two parts the voices")
	}
	k, scraps := clustersFor(spans, merges, 2, minSpeaker)
	if k != 3 || !scraps {
		t.Fatalf("clustersFor = %d, %v; want 3, true", k, scraps)
	}
	labels := cut(len(x), merges, k, 0)
	mergeSmallClusters(x, spans, labels, minSpeaker)
	if labels[5] == labels[105] || labels[5] != labels[135] || labels[0] != labels[5] {
		t.Fatalf("labels music %d, guest %d, host %d, guest %d", labels[0], labels[5], labels[105], labels[135])
	}

	// too short for two speakers of minSpeaker each: the clusters stay as cut into two
	if k, scraps := clustersFor(spans[:10], merges[:0], 2, minSpeaker); k != 2 || scraps {
		t.Fatalf("short: clustersFor = %d, %v; want 2, false", k, scraps)
	}
}

// A job cancelled while the speakers are found leaves DiarizeAHC2 within moments, also while the
// sherpa diarizer runs over the recording.
// go test -run DiarizeStops -v ./pipeline, with SHERPA_LIB, TRANSCRIBE_MODELS and
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

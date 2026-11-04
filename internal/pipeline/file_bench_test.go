// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/internal/audio"
	"github.com/moliri-app/transcribe/internal/sherpa"
)

// TestFileBench measures the text step of the full pipeline (Engine.Transcribe, German) on the
// excerpts of TestLiveBench, whole as a file: without speakers, as for one speaker; with the
// speaker cuts the full pipeline finds (DiarizeAHC2, number of speakers found); and with them and
// Whisper turbo for the gaps, as the full pipeline runs. For comparison it measures the live
// stream with Parakeet over the same audio, as fast as it can.
//
// It needs what TestLiveBench needs, and the speaker and Whisper models in TRANSCRIBE_MODELS.
// go test -run FileBench -v -timeout 2h ./internal/pipeline
// TRANSCRIBE_FILE_VARIANTS (see below) narrows it, TRANSCRIBE_LIVE_TEXT=1 shows the text.
func TestFileBench(t *testing.T) {
	m, excerpts := loadBench(t)
	for _, p := range []string{m.Segmentation(), m.CampPlus(), m.Whisper("turbo").Encoder, m.Whisper("base").Encoder} {
		if _, err := os.Stat(p); err != nil {
			t.Skip(err)
		}
	}
	ctx := context.Background()
	eng := NewEngine(m, 4, nil)
	defer eng.Close()
	type variant struct {
		name     string
		speakers bool
		fill     bool
		flush    bool // the stream ends its line at each speaker turn, not split afterwards
		live     bool // the live stream with Parakeet, not the file pipeline
	}
	variants := []variant{
		{name: "live stream", live: true},
		{name: "file"},
		{name: "file+speakers", speakers: true},
		{name: "file+speakers flushed", speakers: true, flush: true},
		{name: "file+speakers+fill", speakers: true, fill: true},
	}
	if v := os.Getenv("TRANSCRIBE_FILE_VARIANTS"); v != "" {
		var pick []variant
		for _, name := range strings.Split(v, ",") {
			for _, fv := range variants {
				if fv.name == name {
					pick = append(pick, fv)
				}
			}
		}
		variants = pick
	}
	recs := map[string]*sherpa.Recognizer{}
	recognizer := func(key string) *sherpa.Recognizer {
		if recs[key] == nil {
			recs[key] = benchRecognizer(t, m, key)
		}
		return recs[key]
	}
	speakers := map[string][]SpeakerSeg{}
	diarize := func(key string, pcm []float32) []SpeakerSeg {
		if speakers[key] == nil {
			spk, err := DiarizeAHC2(ctx, pcm, m.Segmentation(), m.CampPlus(), 0, 0.6, 4)
			if err != nil {
				t.Fatal(err)
			}
			speakers[key] = spk
		}
		return speakers[key]
	}
	var summary []string
	for _, kind := range []string{"clean", "headset"} {
		for _, v := range variants {
			var sum werResult
			var took, secs float64
			for _, e := range excerpts {
				pcm := e.pcm[kind]
				var text []string
				t0 := time.Now()
				if v.live {
					segs, err := decodeStream(ctx, recognizer("parakeet"), recognizer("german"), m.SileroVAD(), liveTune, voice{}, pcm, nil)
					if err != nil {
						t.Fatal(err)
					}
					for _, sg := range segs {
						text = append(text, sg.Text)
					}
				} else {
					o := TextOptions{Fill: v.fill, Lang: "de"}
					if v.speakers {
						o.Speakers = diarize(kind+e.name, pcm)
						t0 = time.Now() // the speakers are not the text step's
					}
					segs, err := eng.transcribe(ctx, pcm, o, fileTune, v.flush)
					if err != nil {
						t.Fatal(err)
					}
					for _, sg := range segs {
						text = append(text, sg.Text)
					}
				}
				took += time.Since(t0).Seconds()
				secs += float64(len(pcm)) / audio.SR
				w := wordErrors(e.ref, benchWords(strings.Join(text, " ")))
				t.Logf("%-8s %-21s WER %5.1f %% (%3d of %d words: %2d missing, %2d wrong, %2d extra)",
					kind+e.name, v.name, 100*w.rate(), w.sub+w.del+w.ins, w.n, w.del, w.sub, w.ins)
				if os.Getenv("TRANSCRIBE_LIVE_TEXT") != "" {
					t.Logf("  missing: %s", strings.Join(w.missing, " "))
					t.Logf("  text:\n%s", strings.Join(text, "\n"))
				}
				sum.n, sum.sub, sum.del, sum.ins = sum.n+w.n, sum.sub+w.sub, sum.del+w.del, sum.ins+w.ins
			}
			summary = append(summary, fmt.Sprintf("%-8s %-21s WER %5.1f %% (%2d missing, %2d wrong, %2d extra), %.2f s computing per second of audio",
				kind, v.name, 100*sum.rate(), sum.del, sum.sub, sum.ins, took/secs))
		}
	}
	t.Log("all excerpts:\n" + strings.Join(summary, "\n"))
}

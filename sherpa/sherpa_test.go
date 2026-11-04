// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sherpa

import (
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func decode(t *testing.T, path string) []float32 {
	out, err := exec.Command("ffmpeg", "-v", "error", "-i", path, "-ac", "1", "-ar", "16000", "-f", "s16le", "-").Output()
	if err != nil {
		t.Skip("ffmpeg/audio not available:", err)
	}
	pcm := make([]float32, len(out)/2)
	for i := range pcm {
		pcm[i] = float32(int16(binary.LittleEndian.Uint16(out[2*i:]))) / 32768
	}
	return pcm
}

func setup(t *testing.T) (models, audio string) {
	lib := FindLib(os.Getenv("SHERPA_LIB"))
	models = os.Getenv("TRANSCRIBE_MODELS")
	audio = os.Getenv("TRANSCRIBE_AUDIO")
	if lib == "" || models == "" || audio == "" {
		t.Skip("set SHERPA_LIB, TRANSCRIBE_MODELS, TRANSCRIBE_AUDIO")
	}
	SetLibDir(lib)
	return
}

func TestParakeet(t *testing.T) {
	models, audio := setup(t)
	pcm := decode(t, audio)
	if len(pcm) > 16000*30 {
		pcm = pcm[:16000*30]
	}
	segs, err := SegmentSpeech(VADConfig{Model: filepath.Join(models, "silero_vad.onnx"), Threshold: 0.5, MinSilence: 0.3, MinSpeech: 0.25, MaxSpeech: 15, Threads: 2}, pcm)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d VAD segments", len(segs))
	d := filepath.Join(models, "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8")
	r, err := NewRecognizer(RecognizerConfig{Kind: "transducer", Encoder: filepath.Join(d, "encoder.int8.onnx"), Decoder: filepath.Join(d, "decoder.int8.onnx"), Joiner: filepath.Join(d, "joiner.int8.onnx"), Tokens: filepath.Join(d, "tokens.txt"), Threads: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, s := range segs[:min(3, len(segs))] {
		res := r.Decode(s.Samples)
		t.Logf("@%.1fs %q tokens=%d", float64(s.Start)/16000, res.Text, len(res.Tokens))
		for _, k := range res.Tokens[:min(4, len(res.Tokens))] {
			if math.IsNaN(k.TS) {
				t.Fatal("nan")
			}
			t.Logf("  %q ts=%.2f dur=%.2f lp=%.3f", k.Text, k.TS, k.Dur, k.LogP)
		}
	}
}

// TestKroko streams the first 20 s of the audio through the German streaming model in pieces of
// 100 ms, as live transcription does, and checks that its running result grows with timed tokens.
func TestKroko(t *testing.T) {
	models, audio := setup(t)
	d := filepath.Join(models, "sherpa-onnx-streaming-zipformer-de-kroko-2025-08-06")
	if _, err := os.Stat(d); err != nil {
		t.Skip(err)
	}
	pcm := decode(t, audio)
	pcm = pcm[:min(len(pcm), 16000*20)]
	r, err := NewOnlineRecognizer(OnlineConfig{Encoder: filepath.Join(d, "encoder.onnx"), Decoder: filepath.Join(d, "decoder.onnx"), Joiner: filepath.Join(d, "joiner.onnx"), Tokens: filepath.Join(d, "tokens.txt"), Threads: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	s := r.NewStream()
	defer s.Close()
	var last string
	var lag float64 // from a token's time to the piece that brought it
	n := 0
	for i := 0; i < len(pcm); i += 1600 {
		j := min(len(pcm), i+1600)
		s.Accept(pcm[i:j])
		if res := s.Result(); res.Text != last {
			last = res.Text
			for _, k := range res.Tokens[n:] {
				lag += float64(j)/16000 - k.TS
			}
			n = len(res.Tokens)
		}
	}
	t.Logf("a token shows %.2f s after its time, on average", lag/float64(max(1, n)))
	s.Finish(0.5)
	res := s.Result()
	t.Logf("final %q", res.Text)
	if len(res.Tokens) == 0 {
		t.Fatal("no tokens")
	}
	prev := -1.0
	for _, k := range res.Tokens {
		if k.TS < prev || k.LogP > 0 {
			t.Fatalf("token %+v after %.2f", k, prev)
		}
		prev = k.TS
	}
	t.Logf("first tokens %+v", res.Tokens[:min(5, len(res.Tokens))])
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sherpa

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestHotwords decodes each wav in HOTWORDS_WAVS with both NeMo transducers greedily, with
// modified beam search and a hotword that is not said (what the beam costs), and with the
// hotwords in HOTWORDS (comma-separated): the text and the time per decoding. Opt-in, with
// SHERPA_LIB and TRANSCRIBE_MODELS.
func TestHotwords(t *testing.T) {
	lib, models, wavs := FindLib(os.Getenv("SHERPA_LIB")), os.Getenv("TRANSCRIBE_MODELS"), os.Getenv("HOTWORDS_WAVS")
	if lib == "" || models == "" || wavs == "" {
		t.Skip("set SHERPA_LIB, TRANSCRIBE_MODELS, HOTWORDS_WAVS")
	}
	SetLibDir(lib)
	hot := strings.Split(os.Getenv("HOTWORDS"), ",")
	var score float32
	fmt.Sscan(os.Getenv("HOTWORDS_SCORE"), &score)
	files, _ := filepath.Glob(filepath.Join(wavs, "*.wav"))
	for _, dir := range []string{"sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8", "sherpa-onnx-nemo-transducer-stt_de_fastconformer_hybrid_large_pc-int8"} {
		d := filepath.Join(models, dir)
		for _, v := range []struct {
			name string
			hot  []string
		}{{"greedy", nil}, {"beam", []string{"Zebrastreifen"}}, {"hotwords", hot}} {
			r, err := NewRecognizer(RecognizerConfig{Kind: "transducer", Encoder: filepath.Join(d, "encoder.int8.onnx"), Decoder: filepath.Join(d, "decoder.int8.onnx"), Joiner: filepath.Join(d, "joiner.int8.onnx"), Tokens: filepath.Join(d, "tokens.txt"), Threads: 4, Hotwords: v.hot, HotwordsScore: score})
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range files {
				pcm := decode(t, f)
				r.Decode(pcm) // warm
				t0 := time.Now()
				const n = 3
				var res Result
				for range n {
					res = r.Decode(pcm)
				}
				t.Logf("%-10.10s %-8s %-8s %4.0f ms  %s", dir[16:], v.name, filepath.Base(f), float64(time.Since(t0).Milliseconds())/n, res.Text)
			}
			r.Close()
		}
	}
}

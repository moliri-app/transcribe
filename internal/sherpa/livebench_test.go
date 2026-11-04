// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sherpa

// Benchmark of speech recognition models for live transcription. Opt-in:
//
//	SHERPA_LIB=<dir with sherpa-onnx-c-api.dll> TRANSCRIBE_MODELS=<data/models> \
//	BENCH_MODELS=<dir with extra candidate models> BENCH_WAVS=<dir with 16 kHz wavs> \
//	BENCH_OUT=<result.json> go test ./internal/sherpa -run TestLiveBench -v -timeout 2h
//
// For every model it decodes each wav once (the text that is kept) and, for the offline models,
// replays what the Live page does while someone speaks: decoding the growing utterance again every
// 0.4 s. Streaming models get the audio in 0.1 s pieces instead. BENCH_VAD=1 treats the wavs as
// long recordings and cuts them into utterances with Silero first, as Live does; BENCH_ONLY picks
// models by name, BENCH_N limits the number of wavs. Results are merged into BENCH_OUT per model.
// BENCH_REF=<reference.txt> logs each model's word error rate over all wavs in order against it
// (one recording cut with BENCH_VAD, say): case and punctuation do not count.
// Findings: docs/live-models.md.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
	"unsafe"
)

type benchModel struct {
	Name    string
	offline *RecognizerConfig
	online  *onlineRecognizerCfg
	lang    string
	keep    *keeper
}

type benchResult struct {
	Model      string
	Load       float64           // seconds to create the recognizer
	RTF        float64           // compute time / audio time, decoding each wav once
	LiveRTF    float64           // compute time / audio time, as Live decodes while someone speaks
	MaxLatency float64           // longest single decode while speaking (s): how long the preview lags
	Final10s   float64           // mean time to decode a whole utterance of about 10 s (offline)
	Texts      map[string]string // wav -> text
}

func TestLiveBench(t *testing.T) {
	models := os.Getenv("TRANSCRIBE_MODELS")
	extra, wavDir, outPath := os.Getenv("BENCH_MODELS"), os.Getenv("BENCH_WAVS"), os.Getenv("BENCH_OUT")
	lib := FindLib(os.Getenv("SHERPA_LIB"))
	if lib == "" || models == "" || extra == "" || wavDir == "" || outPath == "" {
		t.Skip("set SHERPA_LIB, TRANSCRIBE_MODELS, BENCH_MODELS, BENCH_WAVS, BENCH_OUT")
	}
	SetLibDir(lib)
	if err := Load(lib); err != nil {
		t.Fatal(err)
	}
	wavs, _ := filepath.Glob(filepath.Join(wavDir, "*.wav"))
	sort.Strings(wavs)
	if n := os.Getenv("BENCH_N"); n != "" {
		var k int
		for _, c := range n {
			k = k*10 + int(c-'0')
		}
		wavs = wavs[:min(k, len(wavs))]
	}
	pcms := map[string][]float32{}
	for _, w := range wavs {
		pcms[filepath.Base(w)] = decode(t, w)
	}
	// BENCH_VAD: the wavs are long recordings; cut them into utterances as Live does
	if os.Getenv("BENCH_VAD") != "" {
		vadCfg := VADConfig{Model: filepath.Join(models, "silero_vad.onnx"), Threshold: 0.5, MinSilence: 0.5, MinSpeech: 0.25, MaxSpeech: 15, Threads: 1}
		cut := map[string][]float32{}
		var names []string
		for _, w := range wavs {
			segs, err := SegmentSpeech(vadCfg, pcms[filepath.Base(w)])
			if err != nil {
				t.Fatal(err)
			}
			for i, s := range segs {
				n := fmt.Sprintf("%s#%03d", filepath.Base(w), i)
				cut[n] = s.Samples
				names = append(names, n)
			}
		}
		pcms, wavs = cut, names
	}
	only := os.Getenv("BENCH_ONLY")

	tr := func(dir string) (string, string, string, string) {
		return filepath.Join(dir, "encoder.int8.onnx"), filepath.Join(dir, "decoder.int8.onnx"), filepath.Join(dir, "joiner.int8.onnx"), filepath.Join(dir, "tokens.txt")
	}
	var list []benchModel
	add := func(name string, c RecognizerConfig) {
		c.Threads = 4
		list = append(list, benchModel{Name: name, offline: &c})
	}
	e, d, j, k := tr(filepath.Join(models, "sherpa-onnx-nemo-transducer-stt_de_fastconformer_hybrid_large_pc-int8"))
	add("german-fastconformer", RecognizerConfig{Kind: "transducer", Encoder: e, Decoder: d, Joiner: j, Tokens: k})
	e, d, j, k = tr(filepath.Join(models, "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8"))
	add("parakeet-v3", RecognizerConfig{Kind: "transducer", Encoder: e, Decoder: d, Joiner: j, Tokens: k})
	for _, w := range []string{"base", "turbo"} {
		dir := filepath.Join(models, "sherpa-onnx-whisper-"+w)
		add("whisper-"+w, RecognizerConfig{Kind: "whisper", Encoder: filepath.Join(dir, w+"-encoder.int8.onnx"), Decoder: filepath.Join(dir, w+"-decoder.int8.onnx"), Tokens: filepath.Join(dir, w+"-tokens.txt"), Language: "de"})
	}
	if dir := filepath.Join(extra, "sherpa-onnx-nemo-canary-180m-flash-en-es-de-fr-int8"); exists(dir) {
		add("canary-180m-flash", RecognizerConfig{Kind: "canary", Encoder: filepath.Join(dir, "encoder.int8.onnx"), Decoder: filepath.Join(dir, "decoder.int8.onnx"), Tokens: filepath.Join(dir, "tokens.txt"), Language: "de"})
	}
	onl := func(name, dir, lang string) {
		if !exists(dir) {
			return
		}
		var kp keeper
		e, d, j, k := tr(dir)
		if !exists(e) {
			e, d, j = filepath.Join(dir, "encoder.onnx"), filepath.Join(dir, "decoder.onnx"), filepath.Join(dir, "joiner.onnx")
		}
		c := onlineRecognizerCfg{Feat: featureConfig{16000, 80}, DecodingMethod: kp.s("greedy_search")}
		c.Model.Transducer = onlineTransducerCfg{kp.s(e), kp.s(d), kp.s(j)}
		c.Model.Tokens = kp.s(k)
		c.Model.NumThreads = 4
		c.Model.Provider = kp.s("cpu")
		list = append(list, benchModel{Name: name, online: &c, lang: lang, keep: &kp})
	}
	onl("nemotron-3.5-560ms", filepath.Join(extra, "sherpa-onnx-nemotron-3.5-asr-streaming-0.6b-560ms-int8-2026-06-11"), "de")
	onl("nemotron-3.5-160ms", filepath.Join(extra, "sherpa-onnx-nemotron-3.5-asr-streaming-0.6b-160ms-int8-2026-06-11"), "de")
	onl("kroko-de-streaming", filepath.Join(extra, "sherpa-onnx-streaming-zipformer-de-kroko-2025-08-06"), "")

	var results []benchResult
	if b, err := os.ReadFile(outPath); err == nil {
		json.Unmarshal(b, &results)
	}
	for _, m := range list {
		if only != "" && !strings.Contains(","+only+",", ","+m.Name+",") {
			continue
		}
		var r benchResult
		if m.offline != nil {
			r = benchOffline(t, m, wavs, pcms)
		} else {
			r = benchOnline(t, m, wavs, pcms)
		}
		t.Logf("%-22s load %.1fs  RTF %.3f  live RTF %.3f  max lag %.2fs  final(10s) %.2fs", r.Model, r.Load, r.RTF, r.LiveRTF, r.MaxLatency, r.Final10s)
		if ref := os.Getenv("BENCH_REF"); ref != "" {
			t.Logf("%-22s WER %.1f %%", r.Model, 100*benchWER(t, ref, r.Texts))
		}
		kept := results[:0]
		for _, o := range results {
			if o.Model != r.Model {
				kept = append(kept, o)
			}
		}
		results = append(kept, r)
		b, _ := json.MarshalIndent(results, "", " ")
		os.WriteFile(outPath, b, 0o644)
		runtime.GC()
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// benchWER is the word error rate of the texts (by wav, in the order of their names) against the
// reference text in the file ref.
func benchWER(t *testing.T, ref string, texts map[string]string) float64 {
	b, err := os.ReadFile(ref)
	if err != nil {
		t.Fatal(err)
	}
	words := func(s string) []string {
		var out []string
		for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ' ' || r == '\n' || r == '\r' || r == '\t' || r == '-' }) {
			if f = strings.Trim(f, ".,;:!?\"'„“”‚‘’()[]…"); f != "" {
				out = append(out, f)
			}
		}
		return out
	}
	names := make([]string, 0, len(texts))
	for n := range texts {
		names = append(names, n)
	}
	sort.Strings(names)
	var hyp []string
	for _, n := range names {
		hyp = append(hyp, words(texts[n])...)
	}
	r := words(string(b))
	d := make([]int, len(hyp)+1) // Levenshtein over words, one row at a time
	for j := range d {
		d[j] = j
	}
	for i := 1; i <= len(r); i++ {
		prev := d[0]
		d[0] = i
		for j := 1; j <= len(hyp); j++ {
			c := 1
			if r[i-1] == hyp[j-1] {
				c = 0
			}
			prev, d[j] = d[j], min(prev+c, d[j]+1, d[j-1]+1)
		}
	}
	return float64(d[len(hyp)]) / float64(max(1, len(r)))
}

func benchOffline(t *testing.T, m benchModel, wavs []string, pcms map[string][]float32) benchResult {
	t0 := time.Now()
	var rec *Recognizer
	var err error
	if m.offline.Kind == "canary" {
		rec, err = newCanary(*m.offline)
	} else {
		rec, err = NewRecognizer(*m.offline)
	}
	if err != nil {
		t.Fatal(m.Name, err)
	}
	defer rec.Close()
	r := benchResult{Model: m.Name, Load: time.Since(t0).Seconds(), Texts: map[string]string{}}
	rec.Decode(pcms[filepath.Base(wavs[0])][:16000]) // warm up
	var audio, once, live float64
	var n10 int
	for _, w := range wavs {
		pcm := pcms[filepath.Base(w)]
		sec := float64(len(pcm)) / 16000
		audio += sec
		t1 := time.Now()
		r.Texts[filepath.Base(w)] = rec.Decode(pcm).Text
		d := time.Since(t1).Seconds()
		once += d
		if sec > 8 && sec < 12 {
			r.Final10s += d
			n10++
		}
	}
	// the preview while speaking: every 0.4 s of new audio the utterance so far again (live.go);
	// Whisper always works on a 30 s window, so one recording shows its cost
	liveN := 5
	if m.offline.Kind == "whisper" {
		liveN = 1
	}
	for _, w := range wavs[:min(liveN, len(wavs))] {
		pcm := pcms[filepath.Base(w)]
		for end := 16000 * 2 / 5; end <= len(pcm); end += 16000 * 2 / 5 {
			t1 := time.Now()
			rec.Decode(pcm[:end])
			d := time.Since(t1).Seconds()
			live += d
			r.MaxLatency = max(r.MaxLatency, d)
		}
	}
	var liveAudio float64
	for _, w := range wavs[:min(liveN, len(wavs))] {
		liveAudio += float64(len(pcms[filepath.Base(w)])) / 16000
	}
	r.RTF = once / audio
	r.LiveRTF = live / liveAudio
	if n10 > 0 {
		r.Final10s /= float64(n10)
	}
	return r
}

func benchOnline(t *testing.T, m benchModel, wavs []string, pcms map[string][]float32) benchResult {
	t0 := time.Now()
	h := onCreateRecF(unsafe.Pointer(m.online))
	runtime.KeepAlive(m.keep)
	if h == 0 {
		t.Fatal(m.Name, "cannot create online recognizer")
	}
	defer onDestroyRecF(h)
	r := benchResult{Model: m.Name, Load: time.Since(t0).Seconds(), Texts: map[string]string{}}
	var audio, total float64
	tail := make([]float32, 16000) // flushes the last chunk out of the model
	for _, w := range wavs {
		pcm := pcms[filepath.Base(w)]
		audio += float64(len(pcm)) / 16000
		s := onCreateStreamF(h)
		if m.lang != "" {
			onSetOptionF(s, "language", m.lang)
		}
		feed := func(x []float32) {
			t1 := time.Now()
			onlineAcceptF(s, 16000, unsafe.Pointer(&x[0]), int32(len(x)))
			for onReadyF(h, s) != 0 {
				onDecodeF(h, s)
			}
			d := time.Since(t1).Seconds()
			total += d
			r.MaxLatency = max(r.MaxLatency, d)
		}
		for i := 0; i < len(pcm); i += 1600 {
			feed(pcm[i:min(i+1600, len(pcm))])
		}
		feed(tail)
		onlineFinishedF(s)
		for onReadyF(h, s) != 0 {
			onDecodeF(h, s)
		}
		rp := onResultF(h, s)
		r.Texts[filepath.Base(w)] = goString(uintptr(unsafe.Pointer((*onlineResult)(unsafe.Pointer(rp)).Text)))
		onDestroyResF(rp)
		onlineDestroyF(s)
	}
	r.RTF = total / audio
	r.LiveRTF = r.RTF
	return r
}

func newCanary(c RecognizerConfig) (*Recognizer, error) {
	var k keeper
	cfg := offlineRecognizerConfig{}
	cfg.Feat = featureConfig{16000, 80}
	cfg.Model.Tokens = k.s(c.Tokens)
	cfg.Model.NumThreads = int32(c.Threads)
	cfg.Model.Provider = k.s("cpu")
	cfg.DecodingMethod = k.s("greedy_search")
	cfg.Model.Canary = canaryCfg{Encoder: k.s(c.Encoder), Decoder: k.s(c.Decoder), SrcLang: k.s(c.Language), TgtLang: k.s(c.Language), UsePnc: 1}
	h := createRecF(unsafe.Pointer(&cfg))
	runtime.KeepAlive(&k)
	if h == 0 {
		return nil, os.ErrInvalid
	}
	return &Recognizer{h: h}, nil
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package sherpa binds the sherpa-onnx C API at runtime via purego (no cgo).
package sherpa

import (
	"cmp"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

type cstr = *byte

type keeper struct{ b [][]byte }

func (k *keeper) s(v string) cstr {
	if v == "" {
		return nil
	}
	b := append([]byte(v), 0)
	k.b = append(k.b, b)
	return &b[0]
}

func goString(p uintptr) string {
	if p == 0 {
		return ""
	}
	var n int
	for *(*byte)(unsafe.Pointer(p + uintptr(n))) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(p)), n))
}

type featureConfig struct{ SampleRate, FeatureDim int32 }
type transducerCfg struct{ Encoder, Decoder, Joiner cstr }
type modelOnly struct{ Model cstr }
type whisperCfg struct {
	Encoder, Decoder, Language, Task cstr
	TailPaddings, TokenTS, SegmentTS int32
}
type senseVoiceCfg struct {
	Model, Language cstr
	UseITN          int32
}
type moonshineCfg struct{ A, B, C, D, E cstr }
type fireRedCfg struct{ Encoder, Decoder cstr }
type canaryCfg struct {
	Encoder, Decoder, SrcLang, TgtLang cstr
	UsePnc                             int32
}
type funasrCfg struct {
	EncoderAdaptor, LLM, Embedding, Tokenizer, SystemPrompt, UserPrompt cstr
	MaxNewTokens                                                        int32
	Temperature, TopP                                                   float32
	Seed                                                                int32
	Language                                                            cstr
	ITN                                                                 int32
	Hotwords                                                            cstr
}
type qwen3Cfg struct {
	ConvFrontend, Encoder, Decoder, Tokenizer cstr
	MaxTotalLen, MaxNewTokens                 int32
	Temperature, TopP                         float32
	Seed                                      int32
	Hotwords                                  cstr
}
type cohereCfg struct {
	Encoder, Decoder, Language cstr
	UsePunct, UseITN           int32
}

type offlineModelConfig struct {
	Transducer                        transducerCfg
	Paraformer, NemoCtc               modelOnly
	Whisper                           whisperCfg
	Tdnn                              modelOnly
	Tokens                            cstr
	NumThreads, Debug                 int32
	Provider, ModelType, ModelingUnit cstr
	BpeVocab, TelespeechCtc           cstr
	SenseVoice                        senseVoiceCfg
	Moonshine                         moonshineCfg
	FireRed                           fireRedCfg
	Dolphin, ZipformerCtc             modelOnly
	Canary                            canaryCfg
	WenetCtc, Omnilingual, Medasr     modelOnly
	FunasrNano                        funasrCfg
	FireRedCtc                        modelOnly
	Qwen3                             qwen3Cfg
	Cohere                            cohereCfg
}

type offlineLMConfig struct {
	Model cstr
	Scale float32
}

type hrConfig struct{ DictDir, Lexicon, RuleFsts cstr }

type offlineRecognizerConfig struct {
	Feat           featureConfig
	Model          offlineModelConfig
	LM             offlineLMConfig
	DecodingMethod cstr
	MaxActivePaths int32
	HotwordsFile   cstr
	HotwordsScore  float32
	RuleFsts       cstr
	RuleFars       cstr
	BlankPenalty   float32
	HR             hrConfig
}

type offlineResult struct {
	Text          cstr
	Timestamps    *float32
	Count         int32
	Tokens        cstr
	TokensArr     **byte
	JSON          cstr
	Lang, Emotion cstr
	Event         cstr
	Durations     *float32
	YsLogProbs    *float32
	SegTimestamps *float32
	SegDurations  *float32
	SegTexts      cstr
	SegTextsArr   **byte
	SegCount      int32
}

type sileroCfg struct {
	Model                            cstr
	Threshold, MinSilence, MinSpeech float32
	WindowSize                       int32
	MaxSpeech                        float32
}
type vadConfig struct {
	Silero     sileroCfg
	SampleRate int32
	NumThreads int32
	Provider   cstr
	Debug      int32
	Ten        sileroCfg
}
type speechSegment struct {
	Start   int32
	Samples *float32
	N       int32
}

var (
	loadErr        error
	createRecF     func(unsafe.Pointer) uintptr
	createStreamF  func(uintptr) uintptr
	acceptWaveF    func(uintptr, int32, unsafe.Pointer, int32)
	decodeF        func(uintptr, uintptr)
	getResultF     func(uintptr) uintptr
	destroyResultF func(uintptr)
	destroyStreamF func(uintptr)
	destroyRecF    func(uintptr)
	createVadF     func(unsafe.Pointer, float32) uintptr
	vadAcceptF     func(uintptr, unsafe.Pointer, int32)
	vadFlushF      func(uintptr)
	vadEmptyF      func(uintptr) int32
	vadDetectedF   func(uintptr) int32
	vadResetF      func(uintptr)
	vadFrontF      func(uintptr) uintptr
	vadPopF        func(uintptr)
	vadDestroySegF func(uintptr)
	vadDestroyF    func(uintptr)
)

// Load opens the sherpa-onnx C API from dir. A failed attempt can be retried with another dir.
func Load(dir string) error {
	loadMu.Lock()
	defer loadMu.Unlock()
	if loaded {
		return nil
	}
	if loadErr = load(dir); loadErr == nil {
		loaded = true
	}
	return loadErr
}

var (
	loadMu   sync.Mutex
	loaded   bool
	provider = "cpu"
)

// Loaded reports whether the runtime is open (it cannot be swapped afterwards).
func Loaded() bool {
	loadMu.Lock()
	defer loadMu.Unlock()
	return loaded
}

// SetProvider selects the ONNX execution provider ("cpu" or "cuda") used for speech recognition.
func SetProvider(p string) { provider = p }

// Provider returns the execution provider speech recognition runs on.
func Provider() string { return provider }

func libFile() string {
	switch runtime.GOOS {
	case "darwin":
		return "libsherpa-onnx-c-api.dylib"
	case "linux":
		return "libsherpa-onnx-c-api.so"
	}
	return "sherpa-onnx-c-api.dll"
}

// FindLib returns the first directory containing the C API library.
func FindLib(candidates ...string) string {
	for _, d := range candidates {
		if d == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(d, libFile())); err == nil {
			return d
		}
	}
	return ""
}

// unusableLib explains, in the words the app shows, a library file that is there but cannot be
// loaded: an empty placeholder (as the test app has) or a damaged file. Other errors stay as they are.
func unusableLib(dir string, err error) error {
	path := filepath.Join(dir, libFile())
	if st, serr := os.Stat(path); serr == nil && st.Size() == 0 {
		return fmt.Errorf("Spracherkennung nicht verfügbar: %s ist leer, nur ein Platzhalter wie in der Test-App", libFile())
	}
	if badFormat(err) {
		return fmt.Errorf("Spracherkennung nicht verfügbar: %s ist beschädigt oder für einen anderen Rechnertyp gebaut. Die Datei löschen, dann lädt die App sie beim nächsten Start neu", path)
	}
	return fmt.Errorf("sherpa-onnx-c-api: %w", err)
}

func load(dir string) error {
	if _, err := openLib(dir, "onnxruntime"); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("onnxruntime: %w", err)
	}
	h, err := openLib(dir, "sherpa-onnx-c-api")
	if err != nil {
		return unusableLib(dir, err)
	}
	reg := func(f any, name string) { purego.RegisterLibFunc(f, h, name) }
	reg(&createRecF, "SherpaOnnxCreateOfflineRecognizer")
	reg(&createStreamF, "SherpaOnnxCreateOfflineStream")
	reg(&acceptWaveF, "SherpaOnnxAcceptWaveformOffline")
	reg(&decodeF, "SherpaOnnxDecodeOfflineStream")
	reg(&getResultF, "SherpaOnnxGetOfflineStreamResult")
	reg(&destroyResultF, "SherpaOnnxDestroyOfflineRecognizerResult")
	reg(&destroyStreamF, "SherpaOnnxDestroyOfflineStream")
	reg(&destroyRecF, "SherpaOnnxDestroyOfflineRecognizer")
	reg(&createVadF, "SherpaOnnxCreateVoiceActivityDetector")
	reg(&vadAcceptF, "SherpaOnnxVoiceActivityDetectorAcceptWaveform")
	reg(&vadFlushF, "SherpaOnnxVoiceActivityDetectorFlush")
	reg(&vadEmptyF, "SherpaOnnxVoiceActivityDetectorEmpty")
	reg(&vadDetectedF, "SherpaOnnxVoiceActivityDetectorDetected")
	reg(&vadResetF, "SherpaOnnxVoiceActivityDetectorReset")
	reg(&vadFrontF, "SherpaOnnxVoiceActivityDetectorFront")
	reg(&vadPopF, "SherpaOnnxVoiceActivityDetectorPop")
	reg(&vadDestroySegF, "SherpaOnnxDestroySpeechSegment")
	reg(&vadDestroyF, "SherpaOnnxDestroyVoiceActivityDetector")
	loadOnline(h)
	return loadAux(h)
}

// Word-level result of one decode.
type Token struct {
	Text string
	TS   float64
	Dur  float64
	LogP float64
}

type Result struct {
	Text   string
	Lang   string
	Tokens []Token
}

type RecognizerConfig struct {
	Kind                             string // "transducer" | "whisper"
	Encoder, Decoder, Joiner, Tokens string
	Language                         string
	Threads                          int
	TokenTimestamps                  bool
	UseGPU                           bool // run on the selected provider; small or int8 models are faster on the CPU
	// Hotwords are words and phrases a transducer favours, decoding with modified beam search
	// rather than greedily; HotwordsScore is how much (0: 1.5 per token).
	Hotwords      []string
	HotwordsScore float32
}

type Recognizer struct {
	h  uintptr
	mu sync.Mutex
}

func NewRecognizer(c RecognizerConfig) (*Recognizer, error) {
	if err := Load(libDir); err != nil {
		return nil, err
	}
	var k keeper
	cfg := offlineRecognizerConfig{}
	cfg.Feat = featureConfig{16000, 80}
	cfg.Model.Tokens = k.s(c.Tokens)
	cfg.Model.NumThreads = int32(c.Threads)
	cfg.Model.Provider = k.s("cpu")
	if c.UseGPU {
		cfg.Model.Provider = k.s(provider)
	}
	cfg.DecodingMethod = k.s("greedy_search")
	switch c.Kind {
	case "transducer":
		cfg.Model.Transducer = transducerCfg{k.s(c.Encoder), k.s(c.Decoder), k.s(c.Joiner)}
		cfg.Model.ModelType = k.s("nemo_transducer")
		if len(c.Hotwords) > 0 {
			vocab, words, err := hotwordFiles(c.Tokens, c.Hotwords)
			if err != nil {
				return nil, err
			}
			defer os.Remove(vocab)
			defer os.Remove(words)
			cfg.DecodingMethod = k.s("modified_beam_search")
			cfg.MaxActivePaths = 4
			cfg.Model.ModelingUnit = k.s("bpe")
			cfg.Model.BpeVocab = k.s(vocab)
			cfg.HotwordsFile = k.s(words)
			cfg.HotwordsScore = cmp.Or(c.HotwordsScore, 1.5)
		}
	case "whisper":
		cfg.Model.Whisper = whisperCfg{Encoder: k.s(c.Encoder), Decoder: k.s(c.Decoder), Language: k.s(c.Language), Task: k.s("transcribe"), TailPaddings: -1}
		if c.TokenTimestamps {
			cfg.Model.Whisper.TokenTS = 1
		}
	default:
		return nil, fmt.Errorf("unknown recognizer kind %q", c.Kind)
	}
	h := createRecF(unsafe.Pointer(&cfg))
	runtime.KeepAlive(&k)
	runtime.KeepAlive(&cfg)
	if h == 0 {
		return nil, fmt.Errorf("sherpa: cannot create recognizer (%s)", c.Kind)
	}
	return &Recognizer{h: h}, nil
}

func (r *Recognizer) Close() { destroyRecF(r.h) }

// Decode runs one utterance (16 kHz mono float32).
func (r *Recognizer) Decode(samples []float32) Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := createStreamF(r.h)
	defer destroyStreamF(s)
	if len(samples) > 0 {
		acceptWaveF(s, 16000, unsafe.Pointer(&samples[0]), int32(len(samples)))
	}
	decodeF(r.h, s)
	rp := getResultF(s)
	defer destroyResultF(rp)
	res := (*offlineResult)(unsafe.Pointer(rp))
	out := Result{Text: goString(uintptr(unsafe.Pointer(res.Text))), Lang: goString(uintptr(unsafe.Pointer(res.Lang)))}
	n := int(res.Count)
	if n > 0 && res.TokensArr != nil {
		toks := unsafe.Slice(res.TokensArr, n)
		var ts, du, lp []float32
		if res.Timestamps != nil {
			ts = unsafe.Slice(res.Timestamps, n)
		}
		if res.Durations != nil {
			du = unsafe.Slice(res.Durations, n)
		}
		if res.YsLogProbs != nil {
			lp = unsafe.Slice(res.YsLogProbs, n)
		}
		for i := 0; i < n; i++ {
			t := Token{Text: goString(uintptr(unsafe.Pointer(toks[i])))}
			if ts != nil {
				t.TS = float64(ts[i])
			}
			if du != nil {
				t.Dur = float64(du[i])
			}
			if lp != nil {
				t.LogP = float64(lp[i])
			}
			out.Tokens = append(out.Tokens, t)
		}
	}
	return out
}

type VADConfig struct {
	Model                            string
	Threshold                        float32
	MinSilence, MinSpeech, MaxSpeech float32
	Threads                          int
}

type Speech struct {
	Start   int // sample offset
	Samples []float32
}

// SegmentSpeech runs Silero VAD over the whole signal.
func SegmentSpeech(c VADConfig, pcm []float32) ([]Speech, error) {
	v, err := NewVAD(c, float32(len(pcm)/16000+10))
	if err != nil {
		return nil, err
	}
	defer v.Close()
	var out []Speech
	const win = 512
	for i := 0; i < len(pcm); i += win {
		e := min(i+win, len(pcm))
		v.Accept(pcm[i:e])
		out = append(out, v.Pop()...)
	}
	v.Flush()
	return append(out, v.Pop()...), nil
}

// VAD is Silero voice activity detection fed bit by bit. Not safe for concurrent use.
type VAD struct{ h uintptr }

// NewVAD keeps up to bufferSeconds of audio for the speech it has not handed out yet.
func NewVAD(c VADConfig, bufferSeconds float32) (*VAD, error) {
	if err := Load(libDir); err != nil {
		return nil, err
	}
	var k keeper
	cfg := vadConfig{SampleRate: 16000, NumThreads: int32(c.Threads), Provider: k.s("cpu")}
	cfg.Silero = sileroCfg{Model: k.s(c.Model), Threshold: c.Threshold, MinSilence: c.MinSilence, MinSpeech: c.MinSpeech, WindowSize: 512, MaxSpeech: c.MaxSpeech}
	v := createVadF(unsafe.Pointer(&cfg), bufferSeconds)
	runtime.KeepAlive(&k)
	runtime.KeepAlive(&cfg)
	if v == 0 {
		return nil, fmt.Errorf("sherpa: cannot create VAD")
	}
	return &VAD{h: v}, nil
}

// Accept takes the next samples (16 kHz mono), best in windows of 512.
func (v *VAD) Accept(pcm []float32) {
	if len(pcm) > 0 {
		vadAcceptF(v.h, unsafe.Pointer(&pcm[0]), int32(len(pcm)))
	}
}

// Speaking reports whether speech is going on at the end of what was accepted.
func (v *VAD) Speaking() bool { return vadDetectedF(v.h) != 0 }

// Pop hands out the speech segments that have ended; Start counts samples since the first Accept.
func (v *VAD) Pop() []Speech {
	var out []Speech
	for vadEmptyF(v.h) == 0 {
		sp := (*speechSegment)(unsafe.Pointer(vadFrontF(v.h)))
		cp := make([]float32, sp.N)
		copy(cp, unsafe.Slice(sp.Samples, sp.N))
		out = append(out, Speech{Start: int(sp.Start), Samples: cp})
		vadDestroySegF(uintptr(unsafe.Pointer(sp)))
		vadPopF(v.h)
	}
	return out
}

// Flush ends the speech going on, so that the next Pop hands it out. Only at the end: audio
// accepted after it can come out as broken segments.
func (v *VAD) Flush() { vadFlushF(v.h) }

// Reset forgets everything; Start then counts from the next Accept.
func (v *VAD) Reset() { vadResetF(v.h) }

func (v *VAD) Close() { vadDestroyF(v.h) }

var libDir string

// SetLibDir sets where Load looks for the shared libraries.
func SetLibDir(d string) { libDir = d }

// hotwordFiles writes what modified beam search needs for hotwords, two files to remove after
// use: the model's vocabulary for splitting a hotword into its tokens, and the hotwords, one per
// line. tokens.txt has no scores; NeMo's SentencePiece models list their pieces roughly from the
// most frequent on, so a piece scores as a Zipf distribution gives it by its rank. Splitting
// "Moliri" by these scores gives the pieces the models themselves use for it (▁Mo li ri).
func hotwordFiles(tokens string, hotwords []string) (vocab, words string, err error) {
	b, err := os.ReadFile(tokens)
	if err != nil {
		return "", "", err
	}
	var v, w strings.Builder
	rank := 0
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		tok, _, ok := strings.Cut(strings.TrimRight(line, "\r"), " ")
		if !ok {
			return "", "", fmt.Errorf("sherpa: unexpected line in %s: %q", tokens, line)
		}
		score := -1000.0 // <unk>, <blk> and the like, which no text splits into
		if !strings.HasPrefix(tok, "<") {
			rank++
			score = -math.Log(float64(rank))
		}
		fmt.Fprintf(&v, "%s\t%g\n", tok, score)
	}
	for _, h := range hotwords {
		if h = strings.Join(strings.Fields(h), " "); h != "" {
			fmt.Fprintln(&w, h)
		}
	}
	if vocab, err = tempFile("vocab-*.txt", v.String()); err != nil {
		return "", "", err
	}
	if words, err = tempFile("hotwords-*.txt", w.String()); err != nil {
		os.Remove(vocab)
		return "", "", err
	}
	return vocab, words, nil
}

func tempFile(pattern, content string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

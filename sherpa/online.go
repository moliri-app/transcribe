// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sherpa

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ---- Streaming recognition (online transducer, e.g. Kroko's Zipformer) ----

type onlineTransducerCfg struct{ Encoder, Decoder, Joiner cstr }
type onlineParaformerCfg struct{ Encoder, Decoder cstr }
type onlineModelCfg struct {
	Transducer                        onlineTransducerCfg
	Paraformer                        onlineParaformerCfg
	Zipformer2Ctc                     modelOnly
	Tokens                            cstr
	NumThreads                        int32
	Provider                          cstr
	Debug                             int32
	ModelType, ModelingUnit, BpeVocab cstr
	TokensBuf                         cstr
	TokensBufSize                     int32
	NemoCtc, ToneCtc                  modelOnly
}
type onlineFstCfg struct {
	Graph     cstr
	MaxActive int32
}
type onlineRecognizerCfg struct {
	Feat                featureConfig
	Model               onlineModelCfg
	DecodingMethod      cstr
	MaxActivePaths      int32
	EnableEndpoint      int32
	Rule1, Rule2, Rule3 float32
	HotwordsFile        cstr
	HotwordsScore       float32
	Fst                 onlineFstCfg
	RuleFsts, RuleFars  cstr
	BlankPenalty        float32
	HotwordsBuf         cstr
	HotwordsBufSize     int32
	HR                  hrConfig
}
type onlineResult struct {
	Text       cstr
	Tokens     cstr
	TokensArr  **byte
	Timestamps *float32
	Count      int32
	JSON       cstr
}

var (
	onCreateRecF    func(unsafe.Pointer) uintptr
	onDestroyRecF   func(uintptr)
	onCreateStreamF func(uintptr) uintptr
	onSetOptionF    func(uintptr, string, string)
	onReadyF        func(uintptr, uintptr) int32
	onDecodeF       func(uintptr, uintptr)
	onResultF       func(uintptr, uintptr) uintptr
	onDestroyResF   func(uintptr)
)

func loadOnline(h uintptr) {
	reg := func(f any, name string) { purego.RegisterLibFunc(f, h, name) }
	reg(&onCreateRecF, "SherpaOnnxCreateOnlineRecognizer")
	reg(&onDestroyRecF, "SherpaOnnxDestroyOnlineRecognizer")
	reg(&onCreateStreamF, "SherpaOnnxCreateOnlineStream")
	reg(&onSetOptionF, "SherpaOnnxOnlineStreamSetOption")
	reg(&onReadyF, "SherpaOnnxIsOnlineStreamReady")
	reg(&onDecodeF, "SherpaOnnxDecodeOnlineStream")
	reg(&onResultF, "SherpaOnnxGetOnlineStreamResult")
	reg(&onDestroyResF, "SherpaOnnxDestroyOnlineRecognizerResult")
}

type OnlineConfig struct {
	Encoder, Decoder, Joiner, Tokens string
	Threads                          int
}

// OnlineRecognizer is a streaming model: its streams take audio bit by bit and keep a running
// result, without decoding what came before again. Its streams may be used from several
// goroutines, one each.
type OnlineRecognizer struct {
	h  uintptr
	mu sync.Mutex
}

func NewOnlineRecognizer(c OnlineConfig) (*OnlineRecognizer, error) {
	if err := Load(libDir); err != nil {
		return nil, err
	}
	var k keeper
	cfg := onlineRecognizerCfg{Feat: featureConfig{16000, 80}, DecodingMethod: k.s("greedy_search")}
	cfg.Model.Transducer = onlineTransducerCfg{k.s(c.Encoder), k.s(c.Decoder), k.s(c.Joiner)}
	cfg.Model.Tokens = k.s(c.Tokens)
	cfg.Model.NumThreads = int32(c.Threads)
	cfg.Model.Provider = k.s("cpu")
	h := onCreateRecF(unsafe.Pointer(&cfg))
	runtime.KeepAlive(&k)
	runtime.KeepAlive(&cfg)
	if h == 0 {
		return nil, fmt.Errorf("sherpa: cannot create online recognizer")
	}
	return &OnlineRecognizer{h: h}, nil
}

func (r *OnlineRecognizer) Close() { onDestroyRecF(r.h) }

// Decode recognises one utterance (16 kHz mono) as a whole, as Recognizer.Decode does.
func (r *OnlineRecognizer) Decode(samples []float32) Result {
	s := r.NewStream()
	defer s.Close()
	s.Accept(samples)
	s.Finish(0.5)
	return s.Result()
}

// OnlineStream is one utterance (or more) fed to an OnlineRecognizer. Not safe for concurrent use.
type OnlineStream struct {
	r *OnlineRecognizer
	h uintptr
}

func (r *OnlineRecognizer) NewStream() *OnlineStream {
	return &OnlineStream{r: r, h: onCreateStreamF(r.h)}
}

// Accept takes the next samples (16 kHz mono) and decodes as far as they go.
func (s *OnlineStream) Accept(pcm []float32) {
	if len(pcm) > 0 {
		onlineAcceptF(s.h, 16000, unsafe.Pointer(&pcm[0]), int32(len(pcm)))
	}
	s.decode()
}

// Finish ends the input and decodes what is left. The model decodes in chunks: what came last
// is only recognised with some silence after it, which tail adds (seconds).
func (s *OnlineStream) Finish(tail float64) {
	if n := int(tail * 16000); n > 0 {
		z := make([]float32, n)
		onlineAcceptF(s.h, 16000, unsafe.Pointer(&z[0]), int32(n))
	}
	onlineFinishedF(s.h)
	s.decode()
}

// decode decodes the chunks that are ready, one at a time: a stream that decodes a whole utterance
// (Decode) holds up another one for a chunk at most.
func (s *OnlineStream) decode() {
	for {
		s.r.mu.Lock()
		ready := onReadyF(s.r.h, s.h) != 0
		if ready {
			onDecodeF(s.r.h, s.h)
		}
		s.r.mu.Unlock()
		if !ready {
			return
		}
	}
}

// Result is what the stream recognised so far; a token's TS counts seconds from its first sample,
// LogP is how sure the model was of it.
func (s *OnlineStream) Result() Result {
	rp := onResultF(s.r.h, s.h)
	if rp == 0 {
		return Result{}
	}
	defer onDestroyResF(rp)
	res := (*onlineResult)(unsafe.Pointer(rp))
	out := Result{Text: goString(uintptr(unsafe.Pointer(res.Text)))}
	n := int(res.Count)
	if n == 0 || res.TokensArr == nil {
		return out
	}
	var extra struct {
		YsProbs []float64 `json:"ys_probs"`
	}
	json.Unmarshal([]byte(goString(uintptr(unsafe.Pointer(res.JSON)))), &extra)
	toks := unsafe.Slice(res.TokensArr, n)
	var ts []float32
	if res.Timestamps != nil {
		ts = unsafe.Slice(res.Timestamps, n)
	}
	for i := 0; i < n; i++ {
		t := Token{Text: goString(uintptr(unsafe.Pointer(toks[i])))}
		if ts != nil {
			t.TS = float64(ts[i])
		}
		if i < len(extra.YsProbs) {
			t.LogP = extra.YsProbs[i]
		}
		out.Tokens = append(out.Tokens, t)
	}
	return out
}

func (s *OnlineStream) Close() { onlineDestroyF(s.h) }

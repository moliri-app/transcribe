// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sherpa

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

type zipformerTag struct{ Model cstr }
type tagModelCfg struct {
	Zipformer  zipformerTag
	Ced        cstr
	NumThreads int32
	Debug      int32
	Provider   cstr
}
type tagCfg struct {
	Model  tagModelCfg
	Labels cstr
	TopK   int32
}
type audioEvent struct {
	Name  cstr
	Index int32
	Prob  float32
}

type embCfg struct {
	Model      cstr
	NumThreads int32
	Debug      int32
	Provider   cstr
}

type pyannoteCfg struct {
	Model            cstr
	WindowShiftRatio float32
}
type segModelCfg struct {
	Pyannote   pyannoteCfg
	NumThreads int32
	Debug      int32
	Provider   cstr
}
type clusterCfg struct {
	NumClusters       int32
	Threshold         float32
	ComputeConfidence int32
}
type diarCfg struct {
	Seg                      segModelCfg
	Emb                      embCfg
	Clustering               clusterCfg
	MinDurationOn, MinDurOff float32
}
type diarSegment struct {
	Start, End float32
	Speaker    int32
	Confidence float32
}

type lidWhisper struct {
	Encoder, Decoder cstr
	TailPaddings     int32
}
type lidCfg struct {
	Whisper    lidWhisper
	NumThreads int32
	Debug      int32
	Provider   cstr
}

var (
	createTagF      func(unsafe.Pointer) uintptr
	tagStreamF      func(uintptr) uintptr
	tagComputeF     func(uintptr, uintptr, int32) uintptr
	tagFreeF        func(uintptr)
	tagDestroyF     func(uintptr)
	createEmbF      func(unsafe.Pointer) uintptr
	embDimF         func(uintptr) int32
	embStreamF      func(uintptr) uintptr
	embIsReadyF     func(uintptr, uintptr) int32
	embComputeF     func(uintptr, uintptr) uintptr
	embDestroyVecF  func(uintptr)
	embDestroyF     func(uintptr)
	onlineAcceptF   func(uintptr, int32, unsafe.Pointer, int32)
	onlineFinishedF func(uintptr)
	onlineDestroyF  func(uintptr)
	createDiarF     func(unsafe.Pointer) uintptr
	diarProcessF    func(uintptr, unsafe.Pointer, int32) uintptr
	diarNumF        func(uintptr) int32
	diarSortF       func(uintptr) uintptr
	diarDestroySegF func(uintptr)
	diarDestroyResF func(uintptr)
	diarDestroyF    func(uintptr)
	createLidF      func(unsafe.Pointer) uintptr
	lidStreamF      func(uintptr) uintptr
	lidComputeF     func(uintptr, uintptr) uintptr
	lidDestroyResF  func(uintptr)
	lidDestroyF     func(uintptr)
)

func loadAux(h uintptr) error {
	reg := func(f any, name string) { purego.RegisterLibFunc(f, h, name) }
	reg(&createTagF, "SherpaOnnxCreateAudioTagging")
	reg(&tagStreamF, "SherpaOnnxAudioTaggingCreateOfflineStream")
	reg(&tagComputeF, "SherpaOnnxAudioTaggingCompute")
	reg(&tagFreeF, "SherpaOnnxAudioTaggingFreeResults")
	reg(&tagDestroyF, "SherpaOnnxDestroyAudioTagging")
	reg(&createEmbF, "SherpaOnnxCreateSpeakerEmbeddingExtractor")
	reg(&embDimF, "SherpaOnnxSpeakerEmbeddingExtractorDim")
	reg(&embStreamF, "SherpaOnnxSpeakerEmbeddingExtractorCreateStream")
	reg(&embIsReadyF, "SherpaOnnxSpeakerEmbeddingExtractorIsReady")
	reg(&embComputeF, "SherpaOnnxSpeakerEmbeddingExtractorComputeEmbedding")
	reg(&embDestroyVecF, "SherpaOnnxSpeakerEmbeddingExtractorDestroyEmbedding")
	reg(&embDestroyF, "SherpaOnnxDestroySpeakerEmbeddingExtractor")
	reg(&onlineAcceptF, "SherpaOnnxOnlineStreamAcceptWaveform")
	reg(&onlineFinishedF, "SherpaOnnxOnlineStreamInputFinished")
	reg(&onlineDestroyF, "SherpaOnnxDestroyOnlineStream")
	reg(&createDiarF, "SherpaOnnxCreateOfflineSpeakerDiarization")
	reg(&diarProcessF, "SherpaOnnxOfflineSpeakerDiarizationProcess")
	reg(&diarNumF, "SherpaOnnxOfflineSpeakerDiarizationResultGetNumSegments")
	reg(&diarSortF, "SherpaOnnxOfflineSpeakerDiarizationResultSortByStartTime")
	reg(&diarDestroySegF, "SherpaOnnxOfflineSpeakerDiarizationDestroySegment")
	reg(&diarDestroyResF, "SherpaOnnxOfflineSpeakerDiarizationDestroyResult")
	reg(&diarDestroyF, "SherpaOnnxDestroyOfflineSpeakerDiarization")
	reg(&createLidF, "SherpaOnnxCreateSpokenLanguageIdentification")
	reg(&lidStreamF, "SherpaOnnxSpokenLanguageIdentificationCreateOfflineStream")
	reg(&lidComputeF, "SherpaOnnxSpokenLanguageIdentificationCompute")
	reg(&lidDestroyResF, "SherpaOnnxDestroySpokenLanguageIdentificationResult")
	reg(&lidDestroyF, "SherpaOnnxDestroySpokenLanguageIdentification")
	return nil
}

// ---- Audio tagging (CED) ----

type Tagger struct{ h uintptr }
type TagEvent struct {
	Name string
	Prob float64
}

func NewTagger(model, labels string, threads int) (*Tagger, error) {
	if err := Load(libDir); err != nil {
		return nil, err
	}
	var k keeper
	cfg := tagCfg{Model: tagModelCfg{Ced: k.s(model), NumThreads: int32(threads), Provider: k.s("cpu")}, Labels: k.s(labels), TopK: 5}
	h := createTagF(unsafe.Pointer(&cfg))
	runtime.KeepAlive(&k)
	runtime.KeepAlive(&cfg)
	if h == 0 {
		return nil, fmt.Errorf("sherpa: cannot create audio tagger")
	}
	return &Tagger{h}, nil
}

func (t *Tagger) Close() { tagDestroyF(t.h) }

func (t *Tagger) Compute(samples []float32, topK int) []TagEvent {
	if len(samples) == 0 {
		return nil
	}
	s := tagStreamF(t.h)
	defer destroyStreamF(s)
	acceptWaveF(s, 16000, unsafe.Pointer(&samples[0]), int32(len(samples)))
	arr := tagComputeF(t.h, s, int32(topK))
	if arr == 0 {
		return nil
	}
	defer tagFreeF(arr)
	var out []TagEvent
	for i := 0; i < topK; i++ {
		p := *(*uintptr)(unsafe.Pointer(arr + uintptr(i)*unsafe.Sizeof(uintptr(0))))
		if p == 0 {
			break
		}
		ev := (*audioEvent)(unsafe.Pointer(p))
		out = append(out, TagEvent{goString(uintptr(unsafe.Pointer(ev.Name))), float64(ev.Prob)})
	}
	return out
}

// ---- Speaker embeddings ----

type Embedder struct {
	h   uintptr
	Dim int
}

func NewEmbedder(model string, threads int) (*Embedder, error) {
	if err := Load(libDir); err != nil {
		return nil, err
	}
	var k keeper
	cfg := embCfg{Model: k.s(model), NumThreads: int32(threads), Provider: k.s("cpu")}
	h := createEmbF(unsafe.Pointer(&cfg))
	runtime.KeepAlive(&k)
	runtime.KeepAlive(&cfg)
	if h == 0 {
		return nil, fmt.Errorf("sherpa: cannot create embedding extractor")
	}
	return &Embedder{h: h, Dim: int(embDimF(h))}, nil
}

func (e *Embedder) Close() { embDestroyF(e.h) }

// Embed returns nil when the audio is too short.
func (e *Embedder) Embed(samples []float32) []float32 {
	if len(samples) == 0 {
		return nil
	}
	s := embStreamF(e.h)
	defer onlineDestroyF(s)
	onlineAcceptF(s, 16000, unsafe.Pointer(&samples[0]), int32(len(samples)))
	onlineFinishedF(s)
	if embIsReadyF(e.h, s) == 0 {
		return nil
	}
	p := embComputeF(e.h, s)
	if p == 0 {
		return nil
	}
	defer embDestroyVecF(p)
	out := make([]float32, e.Dim)
	copy(out, unsafe.Slice((*float32)(unsafe.Pointer(p)), e.Dim))
	return out
}

// ---- Diarization (segmentation only; clustering is done in Go) ----

type DiarSegment struct {
	Start, End float64
	Speaker    int
}

type Diarizer struct{ h uintptr }

func NewDiarizer(segModel, embModel string, threads int) (*Diarizer, error) {
	const windowShift = 0.1
	if err := Load(libDir); err != nil {
		return nil, err
	}
	var k keeper
	cfg := diarCfg{
		Seg:           segModelCfg{Pyannote: pyannoteCfg{k.s(segModel), windowShift}, NumThreads: int32(threads), Provider: k.s("cpu")},
		Emb:           embCfg{Model: k.s(embModel), NumThreads: int32(threads), Provider: k.s("cpu")},
		Clustering:    clusterCfg{NumClusters: -1, Threshold: 0.5},
		MinDurationOn: 0.3, MinDurOff: 0.5,
	}
	h := createDiarF(unsafe.Pointer(&cfg))
	runtime.KeepAlive(&k)
	runtime.KeepAlive(&cfg)
	if h == 0 {
		return nil, fmt.Errorf("sherpa: cannot create diarizer")
	}
	return &Diarizer{h}, nil
}

func (d *Diarizer) Close() { diarDestroyF(d.h) }

func (d *Diarizer) Process(pcm []float32) []DiarSegment {
	if len(pcm) == 0 {
		return nil
	}
	res := diarProcessF(d.h, unsafe.Pointer(&pcm[0]), int32(len(pcm)))
	if res == 0 {
		return nil
	}
	defer diarDestroyResF(res)
	n := int(diarNumF(res))
	segs := diarSortF(res)
	if segs == 0 {
		return nil
	}
	defer diarDestroySegF(segs)
	out := make([]DiarSegment, n)
	for i, s := range unsafe.Slice((*diarSegment)(unsafe.Pointer(segs)), n) {
		out[i] = DiarSegment{float64(s.Start), float64(s.End), int(s.Speaker)}
	}
	return out
}

// ---- Spoken language identification ----

type LangID struct{ h uintptr }

func NewLangID(encoder, decoder string, threads int) (*LangID, error) {
	if err := Load(libDir); err != nil {
		return nil, err
	}
	var k keeper
	cfg := lidCfg{Whisper: lidWhisper{k.s(encoder), k.s(decoder), -1}, NumThreads: int32(threads), Provider: k.s("cpu")}
	h := createLidF(unsafe.Pointer(&cfg))
	runtime.KeepAlive(&k)
	runtime.KeepAlive(&cfg)
	if h == 0 {
		return nil, fmt.Errorf("sherpa: cannot create language id")
	}
	return &LangID{h}, nil
}

func (l *LangID) Close() { lidDestroyF(l.h) }

// Detect returns an ISO code such as "de".
func (l *LangID) Detect(samples []float32) string {
	s := lidStreamF(l.h)
	defer destroyStreamF(s)
	acceptWaveF(s, 16000, unsafe.Pointer(&samples[0]), int32(len(samples)))
	r := lidComputeF(l.h, s)
	if r == 0 {
		return ""
	}
	defer lidDestroyResF(r)
	return goString(*(*uintptr)(unsafe.Pointer(r)))
}

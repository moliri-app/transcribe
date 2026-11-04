// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/moliri-app/transcribe/internal/annotate"
	"github.com/moliri-app/transcribe/internal/audio"
	"github.com/moliri-app/transcribe/internal/fileutil"
	"github.com/moliri-app/transcribe/internal/jobs"
	"github.com/moliri-app/transcribe/internal/model"
	"github.com/moliri-app/transcribe/internal/models"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/internal/sherpa"
)

const eventsVariant = "ced-w2h0.25-mid-g5"

// Preset fixes what is known about a recording; recognition rules are identical everywhere.
var presets = map[string]struct {
	Speakers int // 0 = auto
}{
	"auto":      {0},
	"interview": {2},
	"gespraech": {0},
	"vortrag":   {1},
	"musik":     {0},
}

// quality is what recognition on this computer always does: "best" fills the gaps the main
// model leaves (singing, other languages) with Whisper turbo. The quality a job asks for is no
// longer heeded.
const quality = "best"

// Runner is the pipeline bound to a data directory; it implements jobs.Runner and annotate.Retranscriber.
type Runner struct {
	DataDir string
	LibDir  string
}

func (r *Runner) models() models.Set { return models.Set{Dir: filepath.Join(r.DataDir, "models")} }

func (r *Runner) init(ctx context.Context) error {
	if r.LibDir == "" {
		if dir := r.cudaLib(); dir != "" {
			sherpa.SetLibDir(dir)
			sherpa.SetProvider("cuda")
			if err := sherpa.Load(dir); err == nil {
				r.LibDir = dir
				log.Println("GPU: CUDA runtime loaded from", dir)
				return nil
			} else {
				log.Println("GPU: CUDA runtime failed, using CPU:", err)
				sherpa.SetProvider("cpu")
			}
		}
		r.LibDir = r.findLib()
		if r.LibDir == "" {
			r.LibDir = filepath.Join(r.DataDir, "lib")
			if err := models.EnsureLib(ctx, r.LibDir, nil); err != nil {
				r.LibDir = ""
				return err
			}
		}
	}
	sherpa.SetLibDir(r.LibDir)
	return sherpa.Load(r.LibDir)
}

// Runtime loads the speech recognition runtime, downloading it when it is missing, and returns its
// folder and whether it runs on the graphics card. Text recognition (internal/ort) loads the
// onnxruntime of the same folder: a process holds only one, and speech recognition would else find
// the wrong one loaded.
func (r *Runner) Runtime(ctx context.Context) (string, bool, error) {
	if err := r.init(ctx); err != nil {
		return "", false, err
	}
	return r.LibDir, sherpa.Provider() == "cuda", nil
}

// RuntimePresent reports whether Runtime can load the runtime without downloading it. Unlike
// libPresent it does not ask for the GPU (nvidia-smi takes seconds): it is asked for every list of
// the text recognition engines.
func (r *Runner) RuntimePresent() bool {
	return r.LibDir != "" || r.findLib() != "" || models.FindLib(models.CudaLibDir(r.DataDir)) != ""
}

// WantsGPU reports whether the runtime loads (or would load) on the graphics card: the user chose
// it and its CUDA build is installed.
func (r *Runner) WantsGPU() bool {
	if sherpa.Loaded() {
		return sherpa.Provider() == "cuda"
	}
	return prefs.Load(r.DataDir).Accel == "cuda" && models.FindLib(models.CudaLibDir(r.DataDir)) != ""
}

// findLib is the folder of the speech recognition runtime on the CPU; "" if it has not been
// downloaded yet.
func (r *Runner) findLib() string {
	exe, _ := os.Executable()
	return models.FindLib(os.Getenv("SHERPA_LIB"), filepath.Dir(exe), filepath.Join(r.DataDir, "lib"))
}

// libPresent reports whether init can load the runtime without downloading it.
func (r *Runner) libPresent() bool { return r.LibDir != "" || r.cudaLib() != "" || r.findLib() != "" }

// cudaLib returns the CUDA runtime folder if the user chose GPU acceleration and everything it needs is present.
func (r *Runner) cudaLib() string {
	if prefs.Load(r.DataDir).Accel != "cuda" {
		return ""
	}
	dir := models.FindLib(models.CudaLibDir(r.DataDir))
	if dir == "" {
		return ""
	}
	g := models.DetectGPU(r.DataDir)
	if !g.Ready {
		log.Println("GPU: CUDA selected but not usable (missing:", g.Missing, "), using CPU")
		return ""
	}
	models.UseLibDirs(g.Dirs)
	return dir
}

func speakerOf(segs []SpeakerSeg, t, maxDist float64) (int, bool) {
	i := sort.Search(len(segs), func(i int) bool { return segs[i].Start > t }) - 1
	best, dist, found := 0, maxDist, false
	for k := max(0, i); k < min(len(segs), i+2); k++ {
		s := segs[k]
		d := 0.0
		if t < s.Start {
			d = s.Start - t
		} else if t > s.End {
			d = t - s.End
		}
		if d <= dist {
			best, dist, found = s.Speaker, d, true
		}
	}
	return best, found
}

// Run executes events -> speakers -> text -> annotate and writes results/<stem>/_final.json.
func (r *Runner) Run(ctx context.Context, dataDir, audioFile string, o jobs.Options, p jobs.Progress) error {
	if o.Engine == prefs.API {
		return r.runAPI(ctx, dataDir, audioFile, o, p)
	}
	if !prefs.Load(r.DataDir).UsesModels() {
		return errors.New("Die Erkennung auf diesem Rechner ist ausgeschaltet (Einstellungen, Erkennung: nur externe API).")
	}
	if err := r.init(ctx); err != nil {
		return err
	}
	preset, ok := presets[o.Preset]
	if !ok {
		preset = presets["auto"]
	}
	n := o.Speakers
	if n == 0 {
		n = preset.Speakers
	}
	lang := o.Language
	if lang == "" {
		lang = "auto"
	}
	// several languages: language identification chooses among them; without German, Whisper turbo
	// recognises each piece in the language found for it
	langs := jobs.Languages(lang)
	otherLang := len(langs) > 0 && !slices.Contains(langs, "de")
	threads := o.Threads
	if threads <= 0 {
		threads = 4
	}
	total := 3
	if n != 1 {
		total = 4
	}
	step := 0
	next := func(label string) {
		step++
		if p != nil {
			p(step, total, label)
		}
	}

	stem := strings.TrimSuffix(audioFile, filepath.Ext(audioFile))
	outDir := filepath.Join(dataDir, "results", stem)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	m := models.Set{Dir: filepath.Join(dataDir, "models")}
	say := func(string) {}
	if err := m.Ensure(ctx, say, "ced", "segmentation", "campplus", "silero"); err != nil {
		return err
	}
	pcm, err := audio.Decode(ctx, filepath.Join(dataDir, "audio", audioFile))
	if err != nil {
		return err
	}
	if err := tooShort(pcm); err != nil {
		return err
	}
	duration := round(float64(len(pcm))/audio.SR, 2)

	next("analyze events")
	tg, err := sherpa.NewTagger(m.CEDModel(), m.CEDLabels(), threads)
	if err != nil {
		return err
	}
	events, err := TagEvents(ctx, pcm, tg, 2.0, 0.25, 5)
	tg.Close()
	if err != nil {
		return err
	}

	var spk []SpeakerSeg
	var spkVariant *string
	if n != 1 {
		next("analyze speakers")
		spk, err = DiarizeAHC2(ctx, pcm, m.Segmentation(), m.CampPlus(), n, 0.6, threads)
		if err != nil {
			return err
		}
		v := "ahc2-campplus-t0.6"
		if n > 0 {
			v = fmt.Sprintf("ahc2-campplus-n%d", n)
		}
		spkVariant = &v
	}

	next("transcribe")
	eng := NewEngine(m, threads, say)
	if len(langs) > 1 {
		eng.Langs = langs
	}
	defer eng.Close()
	var segs []tseg
	engineUsed := "parakeet-v3@live+cutahc2cs+defbr5"
	switch {
	case otherLang:
		engineUsed = "sherpa-whisper-turbo~" + lang
		segs, err = eng.TranscribeWhisper(ctx, pcm, langs)
	default:
		if n == 1 {
			engineUsed = "parakeet-v3@live+defbr5"
		} else if quality == "best" {
			engineUsed += "+fill6"
		}
		segs, err = eng.Transcribe(ctx, pcm, TextOptions{Speakers: spk, Fill: quality == "best" && n != 1, Lang: "de"})
	}
	if err != nil {
		return err
	}

	out := make([]model.Segment, 0, len(segs))
	for _, sg := range segs {
		for i := range sg.Words {
			w := &sg.Words[i]
			if spk != nil {
				md := 1.0
				if w.Fill {
					md = 0
				}
				if s, ok := speakerOf(spk, (w.Start+w.End)/2, md); ok {
					s := s
					w.Speaker = &s
				}
			} else {
				z := 0
				w.Speaker = &z
			}
			if len(sg.Lid) > 0 {
				var l []any
				if json.Unmarshal(sg.Lid, &l) == nil && len(l) > 0 {
					w.Lid, _ = l[0].(string)
				}
			}
		}
		out = append(out, sg.Segment)
	}
	var given *int
	if n > 0 {
		given = &n
	}
	mach := model.Machine{Audio: audioFile, Preset: o.Preset, SpeakersGiven: given, Quality: quality, Language: lang,
		EngineUsed: engineUsed, SpeakersVariant: spkVariant, EventsVariant: eventsVariant, Duration: duration,
		Segments: out, Events: events}
	if err := fileutil.WriteJSON(filepath.Join(outDir, "_machine.json"), mach); err != nil {
		return err
	}

	next("annotate")
	return annotate.Apply(ctx, dataDir, stem, localRetranscriber{r})
}

// minAudio is the shortest recording worth transcribing; the models fail on much less.
const minAudio = audio.SR / 2

// tooShort explains a recording with no sound in it, or too little to recognise anything.
func tooShort(pcm []float32) error {
	if len(pcm) == 0 {
		return errors.New("Die Aufnahme ist leer: Es ist kein Ton darin.")
	}
	if len(pcm) < minAudio {
		return fmt.Errorf("Die Aufnahme ist zu kurz (%s s), um darin etwas zu erkennen.", strings.Replace(fmt.Sprintf("%.1f", float64(len(pcm))/audio.SR), ".", ",", 1))
	}
	return nil
}

// TranscribeWhisper handles languages without German: VAD pieces of at most 25 s through Whisper turbo,
// forced to the one language, or of several to the one language identification finds for the piece.
func (e *Engine) TranscribeWhisper(ctx context.Context, pcm []float32, langs []string) ([]tseg, error) {
	if err := e.M.Ensure(ctx, e.say, "silero"); err != nil {
		return nil, err
	}
	vad := sherpa.VADConfig{Model: e.M.SileroVAD(), Threshold: 0.5, MinSilence: 0.3, MinSpeech: 0.25, MaxSpeech: 25, Threads: 1}
	speech, err := sherpa.SegmentSpeech(vad, pcm)
	if err != nil {
		return nil, err
	}
	var segs []tseg
	for _, sp0 := range speech {
		for _, sp := range splitLong(sp0, 25) {
			a := float64(sp.Start) / audio.SR
			b := a + float64(len(sp.Samples))/audio.SR
			lang := langs[0]
			if len(langs) > 1 {
				if l := e.spokenLanguage(ctx, pcm, a, b, 1.0); slices.Contains(langs, l) {
					lang = l
				}
			}
			ws, err := e.whisperSnippet(ctx, pcm, a, b, lang)
			if err != nil {
				return nil, err
			}
			if len(ws) > 0 {
				sg := tseg{Segment: model.Segment{Start: round(a, 3), End: round(b, 3), Text: joinWords(ws), Words: ws}}
				if len(langs) > 1 {
					sg.Lid = []byte(fmt.Sprintf(`[%q,0,0]`, lang))
				}
				segs = append(segs, sg)
			}
		}
	}
	return segs, nil
}

var (
	rtMu    sync.Mutex
	rtEng   *Engine
	rtPath  string
	rtAudio []float32
)

// Retranscribe implements annotate.Retranscriber for passages corrected in the editor: with
// whatever recognises whole recordings by default.
func (r *Runner) Retranscribe(ctx context.Context, audioPath string, a, b float64, lang string) ([]model.Word, error) {
	if p := prefs.Load(r.DataDir); p.Engine("", p.FileEngine) == prefs.API {
		return apiRetranscriber{r}.Retranscribe(ctx, audioPath, a, b, lang)
	}
	return r.retranscribeLocal(ctx, audioPath, a, b, lang)
}

// retranscribeLocal is [a, b] with Whisper turbo forced to lang, in chunks of <= 25 s.
func (r *Runner) retranscribeLocal(ctx context.Context, audioPath string, a, b float64, lang string) ([]model.Word, error) {
	if err := r.init(ctx); err != nil {
		return nil, err
	}
	rtMu.Lock()
	defer rtMu.Unlock()
	if rtEng == nil {
		rtEng = NewEngine(r.models(), 4, nil)
	}
	if rtPath != audioPath {
		pcm, err := audio.Decode(ctx, audioPath)
		if err != nil {
			return nil, err
		}
		rtPath, rtAudio = audioPath, pcm
	}
	const chunk = 25.0
	cuts := []float64{a}
	for b-cuts[len(cuts)-1] > chunk {
		cuts = append(cuts, snapToPause(rtAudio, cuts[len(cuts)-1]+chunk-3, 3))
	}
	cuts = append(cuts, b)
	var out []model.Word
	for i := 0; i+1 < len(cuts); i++ {
		ws, err := rtEng.whisperSnippet(ctx, rtAudio, cuts[i], cuts[i+1], lang)
		if err != nil {
			return nil, err
		}
		for _, w := range ws {
			w.Lang, w.Redone = lang, true
			out = append(out, w)
		}
	}
	return out, nil
}

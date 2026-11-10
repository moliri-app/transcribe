// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/moliri-app/transcribe/annotate"
	"github.com/moliri-app/transcribe/audio"
	"github.com/moliri-app/transcribe/fileutil"
	"github.com/moliri-app/transcribe/jobs"
	"github.com/moliri-app/transcribe/model"
	"github.com/moliri-app/transcribe/models"
	"github.com/moliri-app/transcribe/sherpa"
)

const eventsVariant = "ced-w2h0.25-mid-g5"

// Preset fixes what is known about a recording; recognition rules are identical everywhere.
var presets = map[string]struct {
	Speakers int // 0 = auto
	Quality  string
}{
	"auto":      {0, "fast"},
	"interview": {2, "fast"},
	"gespraech": {0, "fast"},
	"vortrag":   {1, "fast"},
	"musik":     {0, "best"},
}

// Speakers is the number of speakers a job asks for: its own, else its preset's; 0 = automatic.
func Speakers(o jobs.Options) int {
	if o.Speakers != 0 {
		return o.Speakers
	}
	preset, ok := presets[o.Preset]
	if !ok {
		preset = presets["auto"]
	}
	return preset.Speakers
}

// Runtime loads the speech recognition runtime for a Runner that brings its own way of doing so
// (the desktop app: the GPU build when the user chose it, a download when it is missing).
type Runtime interface {
	// Load loads the runtime (sherpa.Load) and returns the folder it loaded it from.
	Load(ctx context.Context) (string, error)
	// Present reports whether Load can load it without downloading anything.
	Present() bool
}

// Runner is the pipeline bound to its models and runtime; it implements annotate.Retranscriber.
type Runner struct {
	Models models.Set // where the models live; with Offline, a missing model is an error, not a download
	LibDir string     // where the sherpa-onnx runtime lives, when Runtime is nil
	// Runtime, when set, loads the runtime instead of LibDir.
	Runtime Runtime
	// Voice is what the user set for their voice in live transcription (their own words, spoken
	// punctuation), read whenever a live stream opens; nil = nothing.
	Voice func() Voice
}

func (r *Runner) models() models.Set { return r.Models }

func (r *Runner) init(ctx context.Context) error {
	if r.Runtime != nil {
		_, err := r.Runtime.Load(ctx)
		return err
	}
	dir := models.FindLib(r.LibDir)
	if dir == "" {
		return fmt.Errorf("sherpa-onnx runtime not found in %q", r.LibDir)
	}
	sherpa.SetLibDir(dir)
	return sherpa.Load(dir)
}

// libPresent reports whether init can load the runtime without downloading it.
func (r *Runner) libPresent() bool {
	if r.Runtime != nil {
		return r.Runtime.Present()
	}
	return models.FindLib(r.LibDir) != ""
}

// Check loads the runtime once, so a broken installation fails at start and not with the first job.
func (r *Runner) Check() error { return r.init(context.Background()) }

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

// Run executes events -> speakers -> text -> annotate on the recording at audioPath. ann and cache are the
// human corrections and the re-recognition cache to put on top (both may be nil).
func (r *Runner) Run(ctx context.Context, audioPath string, o jobs.Options, ann *annotate.Annotations, cache map[string]json.RawMessage, p jobs.Progress) (*model.Result, error) {
	if err := r.init(ctx); err != nil {
		return nil, err
	}
	preset, ok := presets[o.Preset]
	if !ok {
		preset = presets["auto"]
	}
	n := Speakers(o)
	quality := o.Quality
	if quality == "" {
		quality = preset.Quality
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

	m := r.models()
	say := func(string) {}
	if err := m.Ensure(ctx, say, "ced", "segmentation", "campplus", "silero"); err != nil {
		return nil, err
	}
	pcm, err := audio.Decode(ctx, audioPath)
	if err != nil {
		return nil, err
	}
	if err := TooShort(pcm); err != nil {
		return nil, err
	}
	duration := round(float64(len(pcm))/audio.SR, 2)
	jobs.SetAudio(ctx, duration)
	jobs.SetThreads(ctx, threads)

	next("analyze events")
	tg, err := sherpa.NewTagger(m.CEDModel(), m.CEDLabels(), threads)
	if err != nil {
		return nil, err
	}
	events, err := TagEvents(ctx, pcm, tg, 2.0, 0.25, 5)
	tg.Close()
	if err != nil {
		return nil, err
	}

	var spk []SpeakerSeg
	var spkVariant *string
	if n != 1 {
		next("analyze speakers")
		spk, err = DiarizeAHC2(ctx, pcm, m.Segmentation(), m.CampPlus(), n, 0.6, threads)
		if err != nil {
			return nil, err
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
		return nil, err
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
	mach := model.Machine{Audio: filepath.Base(audioPath), Preset: o.Preset, SpeakersGiven: given, Quality: quality, Language: lang,
		EngineUsed: engineUsed, SpeakersVariant: spkVariant, EventsVariant: eventsVariant, Duration: duration,
		Segments: out, Events: events}
	raw, err := fileutil.JSON(mach, "")
	if err != nil {
		return nil, err
	}

	next("annotate")
	final, cache, err := annotate.Apply(ctx, annotate.Input{Machine: raw, Annotations: ann, Retranscribed: cache, Audio: audioPath}, r)
	if err != nil {
		return nil, err
	}
	return &model.Result{Machine: mach, Final: final, Retranscribed: cache}, nil
}

// Apply puts corrections on a machine result; with in.Audio set, language regions are recognised again.
func (r *Runner) Apply(ctx context.Context, in annotate.Input) (map[string]any, map[string]json.RawMessage, error) {
	return annotate.Apply(ctx, in, r)
}

// minAudio is the shortest recording worth transcribing; the models fail on much less.
const minAudio = audio.SR / 2

// TooShort explains a recording with no sound in it, or too little to recognise anything.
func TooShort(pcm []float32) error {
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
			if err := jobs.Hold(ctx); err != nil {
				return nil, err
			}
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
				sg := tseg{Segment: model.Segment{Start: round(a, 3), End: round(b, 3), Text: JoinWords(ws), Words: ws}}
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

// Retranscribe implements annotate.Retranscriber: [a, b] with Whisper turbo forced to lang, in chunks of <= 25 s.
func (r *Runner) Retranscribe(ctx context.Context, audioPath string, a, b float64, lang string) ([]model.Word, error) {
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
		cuts = append(cuts, SnapToPause(rtAudio, cuts[len(cuts)-1]+chunk-3, 3))
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

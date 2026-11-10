// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/moliri-app/transcribe/audio"
	"github.com/moliri-app/transcribe/model"
	"github.com/moliri-app/transcribe/models"
	"github.com/moliri-app/transcribe/sherpa"
)

var (
	enWords = map[string]bool{}
	// hallucinations Whisper produces on silence/music
	hallucinations = []string{"untertitel", "amara.org", "vielen dank", "copyright", "swr 20", "zdf"}
	fillHalluc     = []string{"wdr", "ard text", "ard-text", "mediagroup", "im auftrag", "funk.net", "musik",
		"abonnier", "bis zum nächsten mal", "danke fürs zuschauen"}
	subtitleCredits = []string{"titrage", "sous-titre", "st' 501", "subtitles by", "untertitel"}
)

func init() {
	for _, w := range strings.Fields("the and you is it that to of this what we they are have with not but for my me " +
		"your he she his her them there been would could should about just very i") {
		enWords[w] = true
	}
}

func englishRatio(text string) float64 {
	toks := strings.Fields(text)
	n, hit := 0, 0
	for _, t := range toks {
		t = strings.ToLower(strings.Trim(t, ".,!?;:\"'"))
		if t == "" {
			continue
		}
		n++
		if enWords[t] {
			hit++
		}
	}
	return float64(hit) / float64(max(1, n))
}

func normWord(w string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(w) {
		if hasAlnum(string(r)) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// tseg is a transcript segment plus internal markers.
type tseg struct {
	model.Segment
	fallback bool
}

// Engine lazily loads the sherpa models.
type Engine struct {
	M       models.Set
	Threads int
	Status  func(string)
	// Langs, when two or more, are the languages spoken in the recording: language identification
	// chooses only among them, so a passage is not taken for a third language.
	Langs []string

	mu      sync.Mutex
	rec     map[string]*sherpa.Recognizer
	lid     *sherpa.LangID
	missing map[string]bool
}

func NewEngine(m models.Set, threads int, status func(string)) *Engine {
	return &Engine{M: m, Threads: threads, Status: status, rec: map[string]*sherpa.Recognizer{}}
}

func (e *Engine) Close() {
	for _, r := range e.rec {
		r.Close()
	}
	if e.lid != nil {
		e.lid.Close()
	}
}

func (e *Engine) say(s string) {
	if e.Status != nil {
		e.Status(s)
	}
}

// recognizer gives the recognizer of key, loading it the first time. There is one per key: Whisper
// turbo serves every language, set per decode (sherpa.Recognizer.DecodeIn), as a copy of it per
// language would take another 1.6 GB each.
func (e *Engine) recognizer(ctx context.Context, key string) (*sherpa.Recognizer, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if r := e.rec[key]; r != nil {
		return r, nil
	}
	var cfg sherpa.RecognizerConfig
	switch key {
	case "parakeet":
		if err := e.M.Ensure(ctx, e.say, "parakeet"); err != nil {
			return nil, err
		}
		t := e.M.Parakeet()
		cfg = sherpa.RecognizerConfig{Kind: "transducer", Encoder: t.Encoder, Decoder: t.Decoder, Joiner: t.Joiner, Tokens: t.Tokens}
	case "german":
		if err := e.M.Ensure(ctx, e.say, "german"); err != nil {
			return nil, err
		}
		t := e.M.GermanFastConformer()
		cfg = sherpa.RecognizerConfig{Kind: "transducer", Encoder: t.Encoder, Decoder: t.Decoder, Joiner: t.Joiner, Tokens: t.Tokens}
	case "turbo":
		if err := e.M.Ensure(ctx, e.say, "whisper-turbo"); err != nil {
			return nil, err
		}
		w := e.M.Whisper("turbo")
		cfg = sherpa.RecognizerConfig{Kind: "whisper", Encoder: w.Encoder, Decoder: w.Decoder, Tokens: w.Tokens, UseGPU: true}
	default:
		return nil, fmt.Errorf("unknown recognizer %q", key)
	}
	cfg.Threads = e.Threads
	if os.Getenv("TRANSCRIBE_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "load recognizer %s\n", key)
	}
	r, err := sherpa.NewRecognizer(cfg)
	if err != nil {
		return nil, err
	}
	e.rec[key] = r
	return r, nil
}

func (e *Engine) langID(ctx context.Context) (*sherpa.LangID, error) {
	if e.lid != nil {
		return e.lid, nil
	}
	if err := e.M.Ensure(ctx, e.say, "whisper-base"); err != nil {
		return nil, err
	}
	w := e.M.Whisper("base")
	enc := w.Encoder
	if len(e.Langs) > 1 {
		// languages Whisper does not know leave nothing to restrict: then it chooses among all
		if r, err := e.M.LangIDEncoder(e.Langs); err == nil {
			enc = r
		} else if os.Getenv("TRANSCRIBE_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "language id among %v: %v\n", e.Langs, err)
		}
	}
	l, err := sherpa.NewLangID(enc, w.Decoder, e.Threads)
	if err != nil {
		return nil, err
	}
	e.lid = l
	return l, nil
}

// spokenLanguage detects the language of pcm[a-pad : b+pad]; "?" if too short.
func (e *Engine) spokenLanguage(ctx context.Context, pcm []float32, a, b, pad float64) string {
	x := audio.Slice(pcm, math.Max(0, a-pad), b+pad)
	if len(x) < audio.SR/2 {
		return "?"
	}
	l, err := e.langID(ctx)
	if err != nil {
		return "?"
	}
	if lang := l.Detect(x); lang != "" {
		return lang
	}
	return "?"
}

// TextOptions mirrors the engine option set used by the Python pipeline (defbr5 [+ fill6]).
type TextOptions struct {
	Speakers []SpeakerSeg // nil: no speaker cuts (VAD chunks only)
	Fill     bool
	Lang     string
}

// fileTune is how the text step decodes a whole recording: as live transcription does, with the
// seconds before each line as context, quiet input raised, from where the line before ended
// (TestFileBench). Doubtful lines are left to deFallback, which knows the language too.
var fileTune = liveTune

// Transcribe runs Parakeet over the recording as the live stream does, splits its lines at the
// speaker turns and applies the post-processing.
func (e *Engine) Transcribe(ctx context.Context, pcm []float32, o TextOptions) ([]tseg, error) {
	return e.transcribe(ctx, pcm, o, fileTune, false)
}

// transcribe is Transcribe with the stream tuned as tune; with flush the stream ends its line at
// each speaker turn rather than having it split afterwards.
func (e *Engine) transcribe(ctx context.Context, pcm []float32, o TextOptions, tune liveTuning, flush bool) ([]tseg, error) {
	dur := float64(len(pcm)) / audio.SR
	var cuts []float64
	if o.Speakers != nil {
		chunks := speakerCuts(o.Speakers, dur, 4.0, pcm, 1.5)
		for _, c := range chunks[:len(chunks)-1] {
			cuts = append(cuts, c[1])
		}
	}
	rec, err := e.recognizer(ctx, "parakeet")
	if err != nil {
		return nil, err
	}
	if err := e.M.Ensure(ctx, e.say, "silero"); err != nil {
		return nil, err
	}
	var segs []tseg
	if flush {
		segs, err = decodeStream(ctx, rec, nil, e.M.SileroVAD(), tune, voice{}, pcm, cuts)
	} else {
		segs, err = decodeStream(ctx, rec, nil, e.M.SileroVAD(), tune, voice{}, pcm, nil)
		segs = splitSegments(segs, cuts)
	}
	if err != nil {
		return nil, err
	}
	if os.Getenv("TRANSCRIBE_DEBUG") != "" {
		for _, sg := range segs {
			fmt.Fprintf(os.Stderr, "seg %.1f-%.1f: %q\n", sg.Start, sg.End, sg.Text)
		}
	}
	segs, err = e.deFallback(ctx, segs, pcm, o.Lang)
	if err != nil {
		return nil, err
	}
	if o.Fill {
		segs, err = e.fillGaps(ctx, segs, pcm, o)
		if err != nil {
			return nil, err
		}
	}
	return segs, nil
}

// splitSegments splits segments at the cuts (seconds, in order), each word going with its start.
func splitSegments(segs []tseg, cuts []float64) []tseg {
	var out []tseg
	for _, sg := range segs {
		i := sort.SearchFloat64s(cuts, sg.Start)
		for len(sg.Words) > 0 && i < len(cuts) && cuts[i] < sg.End {
			k := 0
			for k < len(sg.Words) && sg.Words[k].Start < cuts[i] {
				k++
			}
			if k > 0 && k < len(sg.Words) {
				head := sg
				head.End, head.Words = cuts[i], sg.Words[:k]
				head.Text = JoinWords(head.Words)
				out = append(out, head)
				sg.Start, sg.Words = cuts[i], sg.Words[k:]
				sg.Text = JoinWords(sg.Words)
			}
			i++
		}
		out = append(out, sg)
	}
	return out
}

func meanP(ws []model.Word) (float64, bool) {
	if len(ws) == 0 {
		return 1, false
	}
	var s float64
	for _, w := range ws {
		s += w.P
	}
	return s / float64(len(ws)), true
}

// deFallback redoes suspicious segments with the German FastConformer (rule en+conf, language ID,
// only when more confident than the original, number words as digits).
func (e *Engine) deFallback(ctx context.Context, segs []tseg, pcm []float32, lang string) ([]tseg, error) {
	var out []tseg
	for _, sg := range segs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n := len(strings.Fields(sg.Text))
		conf, _ := meanP(sg.Words)
		suspicious := (n >= 3 && (englishRatio(sg.Text) > 0.15 || conf < 0.7)) || (n < 3 && conf < 0.4)
		if suspicious {
			if l := e.spokenLanguage(ctx, pcm, sg.Start, sg.End, 1.0); l != lang {
				suspicious = false
				sg.Lid = []byte(fmt.Sprintf(`[%q,0,0]`, l))
			}
		}
		if suspicious {
			de, err := e.recognizer(ctx, "german")
			if err != nil {
				return nil, err
			}
			res := de.Decode(audio.Slice(pcm, sg.Start, sg.End))
			words, drop := germanCheck(sg.Text, sg.Words, tokensToWords(res.Tokens, sg.Start, sg.End))
			if drop {
				continue // noise the main model turned into a word
			}
			if words != nil {
				sg.Words, sg.Text, sg.fallback = words, JoinWords(words), true
			}
		}
		out = append(out, sg)
	}
	return out, nil
}

// germanCheck weighs what the German model heard (de) in a suspicious segment against the words
// of the main model: de's words (numbers as digits) when it is surer and heard about as many, drop
// when it heard nothing in a short one, else nil to keep the segment as it is.
func germanCheck(text string, words, de []model.Word) (out []model.Word, drop bool) {
	n := len(strings.Fields(text))
	conf, _ := meanP(words)
	rejected := false
	if len(de) > 0 && englishRatio(text) <= 0.15 {
		if pNew, _ := meanP(de); pNew <= conf {
			de, rejected = nil, true
		}
	}
	switch {
	case len(de) > 0 && float64(len(de)) >= 0.6*float64(n):
		for i := range de {
			de[i].W = toDigits(de[i].W)
		}
		return de, false
	case len(de) == 0 && n < 3 && !rejected:
		return nil, true
	}
	return nil, false
}

// whisperSnippet transcribes pcm[a:b] with Whisper turbo forced to lang. sherpa's Whisper gives no
// word times or confidences, so words are spread over the detected speech by length (p fixed).
func (e *Engine) whisperSnippet(ctx context.Context, pcm []float32, a, b float64, lang string) ([]model.Word, error) {
	r, err := e.recognizer(ctx, "turbo")
	if err != nil {
		return nil, err
	}
	const pad = 0.5
	x := audio.Slice(pcm, a, math.Min(b, a+28))
	piece := make([]float32, 0, len(x)+2*int(pad*audio.SR))
	piece = append(piece, make([]float32, int(pad*audio.SR))...)
	piece = append(piece, x...)
	piece = append(piece, make([]float32, int(pad*audio.SR))...)
	text := strings.TrimSpace(r.DecodeIn(lang, piece).Text)
	low := strings.ToLower(text)
	for _, h := range hallucinations {
		if strings.Contains(low, h) {
			return nil, nil
		}
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil, nil
	}
	t0, t1 := a, math.Min(b, a+28)
	// trim leading/trailing silence so the words sit where the speech is
	if err := e.M.Ensure(ctx, e.say, "silero"); err == nil {
		vad := sherpa.VADConfig{Model: e.M.SileroVAD(), Threshold: 0.5, MinSilence: 0.3, MinSpeech: 0.1, MaxSpeech: 30, Threads: 1}
		if sp, err := sherpa.SegmentSpeech(vad, x); err == nil && len(sp) > 0 {
			last := sp[len(sp)-1]
			t0 = a + float64(sp[0].Start)/audio.SR
			t1 = a + float64(last.Start+len(last.Samples))/audio.SR
		}
	}
	total := 0
	for _, f := range fields {
		total += len([]rune(f)) + 1
	}
	words := make([]model.Word, 0, len(fields))
	t := t0
	for _, f := range fields {
		d := (t1 - t0) * float64(len([]rune(f))+1) / float64(total)
		words = append(words, model.Word{W: f, Start: round(t, 3), End: round(t+d, 3), P: 0.8})
		t += d
	}
	return words, nil
}

type iv struct{ a, b float64 }

// fillGaps transcribes speech the engine has no words for (fill6: edge trimming, language ID, loose repeat filter).
func (e *Engine) fillGaps(ctx context.Context, segs []tseg, pcm []float32, o TextOptions) ([]tseg, error) {
	dur := float64(len(pcm)) / audio.SR
	turns := speakerTurns(o.Speakers, dur)
	var words []model.Word
	for _, sg := range segs {
		words = append(words, sg.Words...)
	}
	sort.SliceStable(words, func(i, j int) bool { return words[i].Start < words[j].Start })
	var covered []iv
	for _, w := range words {
		covered = append(covered, iv{w.Start - 0.15, w.End + 0.15})
	}
	for _, sg := range segs {
		if sg.fallback {
			covered = append(covered, iv{sg.Start, sg.End})
		}
	}
	sort.Slice(covered, func(i, j int) bool { return covered[i].a < covered[j].a })

	var lv []float64
	for i := 0; i < len(words); i += 5 {
		if words[i].End > words[i].Start {
			lv = append(lv, rms(audio.Slice(pcm, words[i].Start, words[i].End)))
		}
	}
	ref := 0.05
	if len(lv) > 0 {
		sort.Float64s(lv)
		ref = lv[len(lv)/2]
		if len(lv)%2 == 0 {
			ref = (lv[len(lv)/2-1] + lv[len(lv)/2]) / 2
		}
	}
	var gaps []iv
	for _, tr := range turns {
		t := tr[0]
		for _, c := range covered {
			if c.b <= t || c.a >= tr[1] {
				continue
			}
			if c.a-t >= 0.4 {
				gaps = append(gaps, iv{t, c.a})
			}
			t = math.Max(t, c.b)
		}
		if tr[1]-t >= 0.4 {
			gaps = append(gaps, iv{t, tr[1]})
		}
	}
	added := []tseg{}
	for _, g := range gaps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if speechActivity(pcm, g.a, g.b, ref) < 0.25 {
			continue
		}
		lang := o.Lang
		if l := e.spokenLanguage(ctx, pcm, g.a, g.b, 3.0); l != "?" && l != o.Lang {
			lang = l
		}
		ws, err := e.whisperSnippet(ctx, pcm, g.a, g.b, lang)
		if err != nil {
			return nil, err
		}
		var before, after []string
		for _, w := range words {
			if w.End >= g.a-3 && w.End <= g.a+0.3 {
				before = append(before, normWord(w.W))
			}
			if w.Start >= g.b-0.3 && w.Start <= g.b+3 {
				after = append(after, normWord(w.W))
			}
		}
		if len(before) > 4 {
			before = before[len(before)-4:]
		}
		if len(after) > 4 {
			after = after[:4]
		}
		for len(ws) > 0 && contains(before, normWord(ws[0].W)) {
			ws = ws[1:]
		}
		for len(ws) > 0 && contains(after, normWord(ws[len(ws)-1].W)) {
			ws = ws[:len(ws)-1]
		}
		joined := strings.ToLower(JoinWords(ws))
		for _, h := range fillHalluc {
			if strings.Contains(joined, h) {
				ws = nil
				break
			}
		}
		if len(ws) < 2 {
			ws = nil
		}
		if len(ws) > 0 && isRepeatNoise(ws, joined) {
			ws = nil
		}
		if len(ws) == 0 {
			continue
		}
		for i := range ws {
			ws[i].Fill = true
		}
		added = append(added, tseg{Segment: model.Segment{Start: ws[0].Start, End: ws[len(ws)-1].End, Text: JoinWords(ws), Words: ws}})
	}
	segs = append(segs, added...)
	sort.SliceStable(segs, func(i, j int) bool { return segs[i].Start < segs[j].Start })
	return segs, nil
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func JoinWords(ws []model.Word) string {
	p := make([]string, len(ws))
	for i, w := range ws {
		p[i] = w.W
	}
	return strings.Join(p, " ")
}

// isRepeatNoise is fill6 "loose": one repeated word, >= 4 words with one over half, or subtitle credits.
func isRepeatNoise(ws []model.Word, joined string) bool {
	var toks []string
	count := map[string]int{}
	top, topN := "", 0
	for _, w := range ws {
		if t := normWord(w.W); t != "" {
			toks = append(toks, t)
			count[t]++
			if count[t] > topN {
				top, topN = t, count[t]
			}
		}
	}
	_ = top
	noise := len(toks) > 0 && (len(count) == 1 || (len(toks) >= 4 && float64(topN)/float64(len(toks)) > 0.5))
	if noise {
		return true
	}
	for _, h := range subtitleCredits {
		if strings.Contains(joined, h) {
			return true
		}
	}
	return false
}

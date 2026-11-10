// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"log"
	"math"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/moliri-app/transcribe/jobs"
	"github.com/moliri-app/transcribe/model"
	"github.com/moliri-app/transcribe/sherpa"
	"github.com/moliri-app/transcribe/stream"
)

// Live transcription: Silero VAD cuts the arriving audio into utterances, Parakeet (which tells
// some 25 languages apart itself) decodes the one going on again and again for the preview and
// once more when it has ended: with the seconds before it as context, from where the line before
// stopped, so that nothing between two lines is lost, and, when it stopped mid-sentence, once the
// speaker has not gone on for a moment (liveTuning; each of these was measured with
// TestLiveBench). German goes to Kroko's streaming model for the preview and to Parakeet for the
// final text instead (livekroko.go). All live streams of a model share one recognizer, kept a
// while after the last stream closed so the next one starts at once.
var (
	liveMu    sync.Mutex
	liveRec   = map[string]interface{ Close() }{} // *sherpa.Recognizer, or *sherpa.OnlineRecognizer for kroko
	liveUsers = map[string]int{}
	liveIdle  = map[string]*time.Timer{}
)

// liveModel is the model live transcription uses for lang ("auto" or a code like de).
func liveModel(lang string) string {
	if lang == "de" {
		return "kroko"
	}
	return "parakeet"
}

const liveKeep = 5 * time.Minute

// recKey names a live recognizer: its model, and after it the user's words it favours, a line
// each; only Parakeet favours them (hotwords).
func recKey(model string, words []string) string {
	if model != "parakeet" {
		return model
	}
	return strings.Join(append([]string{model}, words...), "\n")
}

func (r *Runner) liveRecognizer(ctx context.Context, key string) (*sherpa.Recognizer, error) {
	rec, err := r.takeLive(ctx, key)
	if err != nil {
		return nil, err
	}
	return rec.(*sherpa.Recognizer), nil
}

// liveOnline is liveRecognizer for the streaming model (kroko).
func (r *Runner) liveOnline(ctx context.Context, key string) (*sherpa.OnlineRecognizer, error) {
	rec, err := r.takeLive(ctx, key)
	if err != nil {
		return nil, err
	}
	return rec.(*sherpa.OnlineRecognizer), nil
}

// takeLive gives the recognizer of key, loading it (and downloading its model) if no stream holds
// it; each call is one user until releaseLiveRecognizer.
func (r *Runner) takeLive(ctx context.Context, key string) (interface{ Close() }, error) {
	liveMu.Lock()
	defer liveMu.Unlock()
	if t := liveIdle[key]; t != nil {
		t.Stop()
		delete(liveIdle, key)
	}
	if liveRec[key] == nil {
		name, words, _ := strings.Cut(key, "\n")
		m := r.models()
		if err := m.Ensure(ctx, nil, name); err != nil {
			return nil, err
		}
		var rec interface{ Close() }
		var err error
		switch name {
		case "kroko":
			rec, err = newKroko(m)
		default:
			t := m.Parakeet()
			if name == "german" {
				t = m.GermanFastConformer()
			}
			var hot []string
			if words != "" {
				hot = hotwords(strings.Split(words, "\n"))
			}
			rec, err = sherpa.NewRecognizer(sherpa.RecognizerConfig{Kind: "transducer", Encoder: t.Encoder, Decoder: t.Decoder, Joiner: t.Joiner, Tokens: t.Tokens, Threads: 4, Hotwords: hot, HotwordsScore: hotwordScore})
		}
		if err != nil {
			return nil, err
		}
		liveRec[key] = rec
	}
	liveUsers[key]++
	return liveRec[key], nil
}

func releaseLiveRecognizer(key string) { releaseLiveRecognizerAfter(key, liveKeep) }

// releaseLiveRecognizerAfter closes the recognizer keep after its last user let it go, unless
// another one takes it before; at once for 0.
func releaseLiveRecognizerAfter(key string, keep time.Duration) {
	liveMu.Lock()
	defer liveMu.Unlock()
	if liveUsers[key]--; liveUsers[key] > 0 || liveRec[key] == nil {
		return
	}
	closeIdle := func() {
		if liveUsers[key] == 0 && liveRec[key] != nil {
			liveRec[key].Close()
			delete(liveRec, key)
		}
	}
	if keep == 0 {
		closeIdle()
		return
	}
	liveIdle[key] = time.AfterFunc(keep, func() {
		liveMu.Lock()
		defer liveMu.Unlock()
		closeIdle()
	})
}

// The live mode keeps live recognition loaded while it is on (the desktop app's live.Warmer).
var (
	warmMu   sync.Mutex
	warmKeys []string // the recognizers it holds
)

// KeepWarm implements the desktop app's live.Warmer: it loads the recognizers live transcription of lang needs, as
// OpenLive would, and holds them until the next call; "" lets them go at once. It downloads
// nothing: what has not been downloaded yet is loaded by the first recording.
func (r *Runner) KeepWarm(lang string) bool {
	warmMu.Lock()
	defer warmMu.Unlock()
	var want []string
	ok := lang != ""
	if ok {
		m := r.models()
		name := liveModel(lang)
		words := r.voice().words
		ok = m.Have("silero", name) && r.libPresent() && r.init(context.Background()) == nil
		if ok {
			want = append(want, recKey(name, words))
			switch {
			case name == "kroko":
				if m.Have("parakeet") { // for the final text
					want = append(want, recKey("parakeet", words))
				}
			case germanMayBeSpoken(lang) && m.Have("german"):
				want = append(want, recKey("german", words))
			}
		}
	}
	// the new ones first, so that one held before and now is not closed in between
	var got []string
	for _, key := range want {
		if _, err := r.takeLive(context.Background(), key); err != nil {
			log.Println("live: keeping recognition loaded:", err)
			ok = false
			continue
		}
		got = append(got, key)
	}
	for _, key := range warmKeys {
		releaseLiveRecognizerAfter(key, 0)
	}
	warmKeys = got
	return ok
}

// OpenLive implements stream.Opener: live transcription of lang ("auto" or codes like de or de,en).
func (r *Runner) OpenLive(ctx context.Context, lang string) (stream.Stream, error) {
	if err := r.init(ctx); err != nil {
		return nil, err
	}
	m := r.models()
	if err := m.Ensure(ctx, nil, "silero"); err != nil {
		return nil, err
	}
	v := r.voice()
	name := liveModel(lang)
	if name == "kroko" {
		return r.openKroko(ctx, v)
	}
	key := recKey(name, v.words)
	rec, err := r.liveRecognizer(ctx, key)
	if err != nil {
		return nil, err
	}
	// Parakeet's doubtful utterances are checked with the German model, unless another language was
	// chosen; live transcription uses it only when it is there already, it downloads nothing for it
	var de *sherpa.Recognizer
	deKey := recKey("german", v.words)
	if name != "german" && germanMayBeSpoken(lang) {
		if _, err := os.Stat(m.GermanFastConformer().Encoder); err == nil {
			if de, err = r.liveRecognizer(ctx, deKey); err != nil {
				log.Println("live: German model for checking:", err)
				de = nil
			}
		}
	}
	s, err := newLiveStream(rec, de, key, m.SileroVAD(), liveTune, v)
	if err != nil {
		releaseLiveRecognizer(key)
		if de != nil {
			releaseLiveRecognizer(deKey)
		}
		return nil, err
	}
	s.deKey = deKey
	return s, nil
}

// germanMayBeSpoken: lang is auto, or a list of codes with de among them.
func germanMayBeSpoken(lang string) bool {
	return lang == "" || lang == "auto" || slices.Contains(strings.Split(lang, ","), "de")
}

// liveTuning is how the live stream cuts and decodes; TestLiveBench measures each of them.
type liveTuning struct {
	threshold  float32 // Silero's probability of speech
	minSilence float32 // seconds of silence that end an utterance
	pad        float64 // seconds decoded before and after an utterance for its final
	context    float64 // seconds before an utterance decoded with it; only the words after its start are kept
	recheck    bool    // short or doubtful utterances go to the German model rather than being dropped
	gain       bool    // quiet input is raised towards a level for speech
	// a final decodes from where the one before ended, so that nothing between two is skipped
	// (where the VAD cut long speech short it dropped sound), not from the start the VAD found
	seamless bool
	// seconds after which speech going on is ended: after its last finished sentence, else at
	// its quietest moment; 0 leaves it to the VAD, which cuts after 15 s wherever it is
	split float64
	// seconds an utterance that stops mid-sentence waits for the speaker to go on, so that its
	// line is the whole sentence (needs seamless)
	hold float64
}

// liveTune is what TestLiveBench found best. A longer silence before an utterance ends (0.8 or
// 1 s) made it worse; hold waits for the end of a sentence instead.
var liveTune = liveTuning{threshold: 0.35, minSilence: 0.5, pad: 0.3, context: 4, recheck: true, gain: true, seamless: true, split: 12, hold: 1}

func newLiveStream(rec, de *sherpa.Recognizer, key, vadModel string, tune liveTuning, v voice) (*liveStream, error) {
	maxSpeech := float32(15)
	if tune.split > 0 {
		maxSpeech = 40
	}
	vad, err := sherpa.NewVAD(sherpa.VADConfig{Model: vadModel, Threshold: tune.threshold, MinSilence: tune.minSilence, MinSpeech: 0.25, MaxSpeech: maxSpeech, Threads: 1}, 60)
	if err != nil {
		return nil, err
	}
	s := &liveStream{rec: rec, de: de, key: key, vad: vad, tune: tune, voice: v, spoke: -1}
	if de != nil {
		s.check = de.Decode
	}
	return s, nil
}

// decodeStream runs pcm through the live stream with rec (and de for doubtful utterances), in
// pieces of 100 ms as a recording does, and gives its finals with their words, with the user's
// voice v (the zero voice for a recording transcribed in full). At each cut
// (seconds, in order) the line going on ends, as at the end of a recording; the seconds before
// it stay context for the next.
func decodeStream(ctx context.Context, rec, de *sherpa.Recognizer, vadModel string, tune liveTuning, v voice, pcm []float32, cuts []float64) ([]tseg, error) {
	s, err := newLiveStream(rec, de, "", vadModel, tune, v)
	if err != nil {
		return nil, err
	}
	defer s.vad.Close() // the recognizers are the caller's, not released by s.Close
	var out []tseg
	s.gave = func(ev stream.Event, ws []model.Word) {
		// a line starts with its first word: its utterance may have begun with words decoded
		// with the line before, or with sound none was heard in (which fillGaps may fill)
		out = append(out, tseg{Segment: model.Segment{Start: ws[0].Start, End: ev.End, Text: ev.Text, Words: ws}})
	}
	const piece = stream.SR / 10
	i := 0
	for _, stop := range append(slices.Clone(cuts), float64(len(pcm))/stream.SR) {
		for end := min(len(pcm), int(stop*stream.SR)); i < end; {
			if err := jobs.Hold(ctx); err != nil {
				return nil, err
			}
			j := min(end, i+piece)
			s.Accept(pcm[i:j])
			i = j
		}
		s.Flush()
	}
	return out, nil
}

const (
	liveWin = 512
	// what is kept before speech starts, so the preview does not cut off its first sound
	liveLead = stream.SR / 2
	// new audio before the preview is decoded again
	liveStep = stream.SR * 2 / 5
	// how far the VAD's start of speech lies before the moment it says so: two windows and the
	// shortest speech
	liveOnset = 2*liveWin + stream.SR/4
	// how far back a long utterance may be ended after a sentence or a comma
	liveSplitBack = 5 * stream.SR
)

type liveStream struct {
	rec   *sherpa.Recognizer
	de    *sherpa.Recognizer // the German model for checking doubtful utterances; nil if none
	key   string             // the recognizer of rec (recKey)
	deKey string             // and of de
	// decodes a doubtful utterance again: de, or another German model (livekroko.go); nil if none
	check func([]float32) sherpa.Result
	vad   *sherpa.VAD
	tune  liveTuning
	gain  gainControl

	rest      []float32 // less than a VAD window, waiting for more
	base      int       // sample index where the VAD started counting (it is reset on Flush)
	hist      []float32 // the audio kept: what is not final yet, and context and lead before it
	histStart int       // sample index of hist[0]
	from      int       // sample index up to which words have been given out as finals
	open      int       // where the preview starts: from, or liveLead before speech while silent
	shown     int       // end of hist at the last preview
	spoke     int       // sample index where the speech going on began; -1 while silent
	covered   int       // end of the audio the last event was decoded from
	held      int       // end of an utterance that stopped mid-sentence and waits; 0 if none
	heldFrom  int       // where it began
	last      model.Word
	voice     voice        // the user's words and spoken punctuation
	said      []model.Word // the last words given out, as given out
	after     string       // the mark a command set at the end of the last final
	closed    bool
	gave      func(stream.Event, []model.Word) // if set, gets every final with its words (recordings as a whole)
}

func (s *liveStream) end() int { return s.histStart + len(s.hist) }

func (s *liveStream) Accept(pcm []float32) []stream.Event {
	if s.tune.gain {
		pcm = s.gain.apply(pcm, s.vad.Speaking())
	}
	s.hist = append(s.hist, pcm...)
	s.rest = append(s.rest, pcm...)
	var out []stream.Event
	for len(s.rest) >= liveWin {
		s.vad.Accept(s.rest[:liveWin])
		s.rest = s.rest[liveWin:]
		out = append(out, s.finals()...)
		pos := s.end() - len(s.rest)
		switch on := s.vad.Speaking(); {
		case on && s.spoke < 0:
			s.spoke = max(s.from, pos-liveOnset)
		case !on:
			s.spoke = -1
		}
	}
	s.rest = append([]float32(nil), s.rest...)
	if ev, ok := s.split(); ok {
		out = append(out, ev)
	}
	if s.held > 0 && !s.vad.Speaking() && s.end()-s.held >= int(s.tune.hold*stream.SR) {
		if ev, ok := s.final(s.heldFrom, s.held, true); ok { // the speaker did not go on
			out = append(out, ev)
		}
	}
	if !s.vad.Speaking() && s.held == 0 {
		s.open = max(s.from, s.end()-liveLead)
	}
	s.trim()
	return out
}

// split ends speech that goes on too long, so that a line does not grow without end and the
// preview stays quick: after the last sentence finished in it, else at its quietest moment in the
// last seconds.
func (s *liveStream) split() (stream.Event, bool) {
	a := max(s.open, s.from)
	if s.tune.split == 0 || !s.vad.Speaking() || s.end()-a < int(s.tune.split*stream.SR) {
		return stream.Event{}, false
	}
	if s.held > 0 {
		a = s.heldFrom
	}
	x0 := max(s.histStart, s.from-int((s.tune.pad+s.tune.context)*stream.SR))
	ws := s.words(s.rec.Decode, x0, s.end(), s.from, s.end())
	// after the last sentence in the last seconds, else after the last comma, if the next word
	// has begun already
	for _, ends := range []string{".?!", ",;:"} {
		for i := len(ws) - 2; i >= 0; i-- {
			w := ws[i].W
			cut := int(ws[i+1].Start * stream.SR)
			if cut < s.end()-liveSplitBack {
				break
			}
			if strings.ContainsAny(w[len(w)-1:], ends) && cut < s.end()-stream.SR/2 {
				lo := s.from
				s.from, s.open, s.covered, s.held = cut, cut, max(s.covered, cut), 0
				return s.event(max(a, lo), cut, ws[:i+1])
			}
		}
	}
	// between 3.5 s and 0.5 s back, enough left for the pad after it
	t := float64(s.end()-s.histStart)/stream.SR - 2
	cut := s.histStart + int(SnapToPause(s.hist, t, 1.5)*stream.SR)
	cut = min(cut, s.end()-int(s.tune.pad*stream.SR))
	ev, ok := s.final(a, cut, true)
	s.open = s.from
	return ev, ok
}

func endsSentence(w string) bool { return strings.ContainsAny(w[len(w)-1:], ".?!") }

// trim drops the audio no decoding needs any more.
func (s *liveStream) trim() {
	keep := liveLead + int((s.tune.pad+s.tune.context)*stream.SR)
	if n := min(s.open-keep, s.end()) - s.histStart; n > 0 {
		s.hist = append([]float32(nil), s.hist[n:]...)
		s.histStart += n
	}
}

// finals decodes the utterances the VAD has closed.
func (s *liveStream) finals() []stream.Event {
	var out []stream.Event
	for _, sp := range s.vad.Pop() {
		a := s.base + sp.Start
		if ev, ok := s.final(a, a+len(sp.Samples), false); ok {
			out = append(out, ev)
		}
	}
	return out
}

// final decodes the utterance from sample a to b, with pad and context around it, and gives the
// words in it not given out before. Unless forced, an utterance that stops mid-sentence waits
// for more (hold).
func (s *liveStream) final(a, b int, force bool) (stream.Event, bool) {
	if s.held > 0 {
		a = s.heldFrom
	}
	pad := int(s.tune.pad * stream.SR)
	lo, hi := s.from, b+min(pad, stream.SR/10)
	x0 := a
	if s.tune.seamless {
		x0 = s.from
	}
	x0 = max(s.histStart, x0-pad-int(s.tune.context*stream.SR))
	x1 := min(s.end(), b+pad)
	if b-a < stream.SR/10 || x1 <= x0 {
		s.from, s.open, s.covered, s.held = max(s.from, hi), max(s.open, hi), max(s.covered, b), 0
		return stream.Event{}, false
	}
	ws := s.words(s.rec.Decode, x0, x1, lo, hi)
	if s.tune.hold > 0 && !force && len(ws) > 0 && !endsSentence(ws[len(ws)-1].W) && !(s.voice.commands && endsWithCommand(ws)) {
		if s.held == 0 {
			s.heldFrom = a
		}
		s.held, s.open, s.covered = b, s.from, max(s.covered, b)
		return stream.Event{}, false
	}
	s.from, s.open, s.covered, s.held = max(s.from, hi), max(s.open, hi), max(s.covered, b), 0
	// a command said on its own („Komma“) is short and may sound doubtful, but is meant
	if _, n := matchCommand(ws); s.doubtful(ws) && !(s.voice.commands && n > 0 && n == len(ws)) {
		ws = s.recheck(ws, x0, x1, lo, hi)
	}
	return s.event(max(a, lo), b, ws)
}

// event gives the words ws of the utterance from sample a to b as its final; a is where it
// began, or where the line before ended if that was later.
func (s *liveStream) event(a, b int, ws []model.Word) (stream.Event, bool) {
	// the word at the seam, recognised in both
	if len(ws) > 0 && s.last.W != "" && normWord(ws[0].W) == normWord(s.last.W) && ws[0].Start-s.last.Start < 0.5 {
		ws = ws[1:]
	}
	if len(ws) == 0 {
		return stream.Event{}, false
	}
	s.last = ws[len(ws)-1]
	// what comes after the last word is decoded again with the next line: a word's time differs a
	// little from one decoding to the next, and one at the seam must not get lost (nor come twice)
	s.from = min(s.from, int((s.last.Start+0.05)*stream.SR))
	start := math.Min(float64(a)/stream.SR, ws[0].Start)
	ws = s.own(ws, false)
	ev := stream.Event{Final: true, Text: liveText(ws), Start: round(start, 2), End: round(float64(b)/stream.SR, 2)}
	if s.gave != nil {
		s.gave(ev, ws)
	}
	return ev, true
}

// own writes the user's words in ws as they are written and turns their spoken punctuation into
// marks (voice.go); for a final (not open), what it gave out is the context of the next.
func (s *liveStream) own(ws []model.Word, open bool) []model.Word {
	ws = fixWords(ws, s.voice.words)
	if !s.voice.commands {
		return ws
	}
	out, end := spokenMarks(ws, s.said, s.after, open)
	if !open {
		s.said = append(s.said, out...)
		s.said = s.said[max(0, len(s.said)-3):]
		s.after = end
	}
	return out
}

func liveText(ws []model.Word) string {
	p := make([]string, len(ws))
	for i, w := range ws {
		p[i] = w.W
	}
	return stream.JoinWords(p)
}

// words decodes hist from sample x0 to x1 with decode and gives the words starting in [lo, hi).
func (s *liveStream) words(decode func([]float32) sherpa.Result, x0, x1, lo, hi int) []model.Word {
	res := decode(s.hist[x0-s.histStart : x1-s.histStart])
	a, b := float64(x0)/stream.SR, float64(x1)/stream.SR
	var out []model.Word
	for _, w := range tokensToWords(res.Tokens, a, b) {
		if t := int(w.Start * stream.SR); t >= lo && t < hi {
			out = append(out, w)
		}
	}
	return out
}

// doubtful: a cough or a door the model turned into a word or two; with recheck also what sounds
// English or is unsure, as deFallback finds it.
func (s *liveStream) doubtful(ws []model.Word) bool { return doubtful(ws, s.tune.recheck) }

func doubtful(ws []model.Word, recheck bool) bool {
	n := len(ws)
	conf, _ := meanP(ws)
	if !recheck {
		return n > 0 && n < 3 && conf < 0.4
	}
	return n >= 3 && (englishRatio(JoinWords(ws)) > 0.15 || conf < 0.7) || n > 0 && n < 3 && conf < 0.4
}

// recheck decodes a doubtful utterance with the German model as deFallback does: its words when
// it is surer, none when it hears nothing either. Without recheck a doubtful utterance is dropped;
// when the German model is the one that heard it, it stays.
func (s *liveStream) recheck(ws []model.Word, x0, x1, lo, hi int) []model.Word {
	if !s.tune.recheck {
		return nil
	}
	if s.check == nil {
		return ws
	}
	words, drop := germanCheck(JoinWords(ws), ws, s.words(s.check, x0, x1, lo, hi))
	switch {
	case drop:
		return nil
	case words != nil:
		return words
	}
	return ws
}

func (s *liveStream) Preview() (stream.Event, bool) {
	if !s.vad.Speaking() || s.end()-s.shown < liveStep {
		return stream.Event{}, false
	}
	s.shown = s.end()
	a := max(s.open, s.from)
	if s.end()-a < stream.SR/10 {
		return stream.Event{}, false
	}
	res := s.rec.Decode(s.hist[a-s.histStart:])
	text := strings.TrimSpace(res.Text)
	if text != "" && (len(s.voice.words) > 0 || s.voice.commands) {
		text = liveText(s.own(tokensToWords(res.Tokens, float64(a)/stream.SR, float64(s.end())/stream.SR), true))
	}
	if text == "" {
		return stream.Event{}, false
	}
	s.covered = s.end()
	return stream.Event{Text: text, Start: round(float64(a)/stream.SR, 2), End: round(float64(s.end())/stream.SR, 2)}, true
}

// Pending implements stream.Hearer: the speech going on that no event has covered yet.
func (s *liveStream) Pending() (from, to float64, ok bool) {
	if s.spoke < 0 {
		return 0, 0, false
	}
	return float64(max(s.spoke, s.covered)) / stream.SR, float64(s.end()) / stream.SR, true
}

// Flush decodes what is open as one utterance and starts afresh: the VAD's own Flush breaks the
// stream for audio that comes after it.
func (s *liveStream) Flush() []stream.Event {
	out := s.finals()
	b := s.held
	if s.vad.Speaking() {
		b = s.end()
	}
	if b > 0 {
		if ev, ok := s.final(max(s.open, s.from), b, true); ok {
			out = append(out, ev)
		}
	}
	s.vad.Reset()
	s.base = s.end()
	s.from, s.open, s.covered, s.spoke, s.held = s.base, s.base, s.base, -1, 0
	s.rest = nil
	s.trim()
	return out
}

func (s *liveStream) Close() {
	if s.closed {
		return
	}
	s.closed = true
	s.vad.Close()
	releaseLiveRecognizer(s.key)
	if s.de != nil {
		releaseLiveRecognizer(s.deKey)
	}
}

// gainControl raises quiet input towards a level for speech: slowly, so that it does not pump,
// and never so far that a sample clips.
type gainControl struct {
	gain  float64 // applied at the end of the last piece; 0 before the first
	level float64 // RMS of speech, followed over a few seconds; 0 until speech was heard
}

const (
	gainTarget = 0.1 // -20 dBFS
	gainMax    = 10  // +20 dB
	gainRise   = 3.0 // dB per second at most
	gainPeak   = 0.9
)

func (g *gainControl) apply(pcm []float32, speaking bool) []float32 {
	out := make([]float32, len(pcm))
	for i := 0; i < len(pcm); i += stream.SR / 10 {
		j := min(len(pcm), i+stream.SR/10)
		g.piece(pcm[i:j], out[i:j], speaking)
	}
	return out
}

func (g *gainControl) piece(x, y []float32, speaking bool) {
	if g.gain == 0 {
		g.gain = 1
	}
	var peak float64
	for _, v := range x {
		peak = math.Max(peak, math.Abs(float64(v)))
	}
	if r := rms(x); speaking && r > 1e-5 {
		if g.level == 0 {
			g.level = r
		} else { // in dB, a time constant of about two seconds
			g.level = math.Exp(0.95*math.Log(g.level) + 0.05*math.Log(r))
		}
	}
	want := 1.0
	if g.level > 0 {
		want = math.Max(1, math.Min(gainMax, gainTarget/g.level))
	}
	start := g.gain
	if peak > 0 {
		want = math.Max(1, math.Min(want, gainPeak/peak))
		start = math.Max(1, math.Min(start, gainPeak/peak)) // down at once
	}
	next := math.Min(want, start*math.Pow(10, gainRise/20*float64(len(x))/stream.SR))
	for i, v := range x {
		k := start + (next-start)*float64(i)/float64(len(x))
		y[i] = float32(float64(v) * k)
	}
	g.gain = next
}

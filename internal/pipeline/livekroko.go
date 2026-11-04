// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"log"
	"math"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/internal/model"
	"github.com/moliri-app/transcribe/internal/models"
	"github.com/moliri-app/transcribe/internal/sherpa"
)

// Live transcription of German: Kroko's streaming Zipformer hears the speech as it comes, and its
// running result is the preview, without decoding the utterance again and again: a word shows
// about 0.9 s after it began, however long the utterance (the model decodes in chunks). The final
// text is Parakeet's: the live stream other languages use (liveStream, without its preview) runs
// beside it on the same audio, in a goroutine of its own, so that the preview never waits for it;
// it cuts the utterances, decodes each once it has ended, with the seconds before it as context,
// and checks doubtful ones with Kroko. Until a final comes, the preview shows Kroko's words of
// its utterance. Without Parakeet, each stretch of speech Kroko heard is a final of its own.
//
// A first version cut the utterances itself, by the VAD and Kroko's words, and gave each to
// Parakeet: Kroko trails the speech, so where the VAD ends an utterance it cannot tell yet whether
// a sentence ended, and its lines broke mid-sentence; TestLiveBench (kroko variants) and
// docs/live-models.md.

// krokoThreads: Kroko decodes a chunk of about a second at a time; on a busy machine that took
// about 0.2 s on one thread, half that on two (the preview came more evenly)
const krokoThreads = 2

func newKroko(m models.Set) (*sherpa.OnlineRecognizer, error) {
	t := m.Kroko()
	return sherpa.NewOnlineRecognizer(sherpa.OnlineConfig{Encoder: t.Encoder, Decoder: t.Decoder, Joiner: t.Joiner, Tokens: t.Tokens, Threads: krokoThreads})
}

// openKroko opens live transcription of German. It needs Kroko and, for the final text, Parakeet,
// and downloads both on demand; without Parakeet it goes on with Kroko alone.
func (r *Runner) openKroko(ctx context.Context, v voice) (live.Stream, error) {
	m := r.models()
	on, err := r.liveOnline(ctx, "kroko")
	if err != nil {
		return nil, err
	}
	finKey := recKey("parakeet", v.words)
	fin, err := r.liveRecognizer(ctx, finKey)
	if err != nil {
		if ctx.Err() != nil {
			releaseLiveRecognizer("kroko")
			return nil, ctx.Err()
		}
		log.Println("live: Parakeet for the final text:", err)
		fin, finKey = nil, ""
	}
	s, err := newKrokoStream(on, fin, "kroko", finKey, m.SileroVAD(), liveTune, v)
	if err != nil {
		releaseLiveRecognizer("kroko")
		if fin != nil {
			releaseLiveRecognizer(finKey)
		}
		return nil, err
	}
	return s, nil
}

// krokoTail is the silence given to Kroko after speech, so that it decodes the last chunk.
const krokoTail = 0.3

// krokoRun is a stretch of speech Kroko heard, from the VAD's start of speech to its end.
type krokoRun struct {
	a, b  int          // sample indexes; b is 0 while it goes on
	from  int          // its words start here (a run the VAD cut while the speaker went on)
	words []model.Word // once it has ended
}

type krokoStream struct {
	on    *sherpa.OnlineRecognizer
	onKey string // the recognizer's key (recKey), released by Close
	vad   *sherpa.VAD
	tune  liveTuning
	gain  gainControl
	voice voice

	rest      []float32 // less than a VAD window, waiting for more
	base      int       // sample index where the VAD started counting (it is reset on Flush)
	hist      []float32 // the audio Kroko has not heard yet, and the lead before speech
	histStart int       // sample index of hist[0]
	spoke     int       // sample index where the speech going on began; -1 while silent
	covered   int       // end of the audio the last preview showed

	st     *sherpa.OnlineStream // Kroko on the speech going on; nil while silent
	stFrom int                  // sample index of the first sample st was given
	stFed  int                  // end of what it was given
	stEnd  int                  // the next one starts here at the earliest: no sound is heard twice
	runs   []*krokoRun          // what Kroko heard that no final has covered yet; the last may go on
	until  float64              // seconds: the finals have covered Kroko's words that begin before
	shown  string               // the text of the last preview
	said   []model.Word         // the last words of the finals, as given out (voice.go)
	ready  []live.Event         // Kroko's finals, when there is no Parakeet

	fin    *liveStream // Parakeet's, fed by work; nil: Kroko's text is the final
	finKey string
	audio  chan finMsg
	mu     sync.Mutex
	made   []finEvent // the finals work made, not handed out yet
	heard  int        // how far fin has heard (samples), and whether it is silent there with
	idle   bool       // nothing held: what Kroko heard before and no final covered will get none
	quit   atomic.Bool
	closed bool
}

// finMsg is audio for the finals, or a Flush (whose finals are handed out once flushed closes).
type finMsg struct {
	pcm     []float32
	flushed chan struct{}
}

type finEvent struct {
	ev live.Event
	ws []model.Word
}

// newKrokoStream opens a stream on Kroko (on) and Parakeet (fin, or nil); Close releases the
// recognizers of the keys onKey and finKey ("" for none).
func newKrokoStream(on *sherpa.OnlineRecognizer, fin *sherpa.Recognizer, onKey, finKey, vadModel string, tune liveTuning, v voice) (*krokoStream, error) {
	vad, err := sherpa.NewVAD(sherpa.VADConfig{Model: vadModel, Threshold: tune.threshold, MinSilence: tune.minSilence, MinSpeech: 0.25, MaxSpeech: 15, Threads: 1}, 60)
	if err != nil {
		return nil, err
	}
	s := &krokoStream{on: on, onKey: onKey, finKey: finKey, vad: vad, tune: tune, voice: v, spoke: -1}
	if fin == nil {
		return s, nil
	}
	if s.fin, err = newLiveStream(fin, nil, finKey, vadModel, tune, v); err != nil {
		vad.Close()
		return nil, err
	}
	s.fin.check = on.Decode // doubtful utterances are checked with Kroko, which knows only German
	s.audio = make(chan finMsg, 600)
	go s.work()
	return s, nil
}

// work runs Parakeet's live stream on the audio, beside the stream.
func (s *krokoStream) work() {
	defer func() { // Close is done with the stream: what it used is let go once this is done too
		s.fin.vad.Close()
		if s.finKey != "" {
			releaseLiveRecognizer(s.finKey)
		}
		if s.onKey != "" {
			releaseLiveRecognizer(s.onKey) // it checks doubtful utterances
		}
	}()
	var made []finEvent
	s.fin.gave = func(ev live.Event, ws []model.Word) { made = append(made, finEvent{ev, ws}) }
	// in the pieces the recording gives, also when it is behind: given 2 s at once when it was, it
	// made a third more errors on the headset recording (TestLiveBench)
	for m := range s.audio {
		if s.quit.Load() {
			if m.flushed != nil {
				close(m.flushed)
			}
			continue
		}
		if m.flushed != nil {
			s.fin.Flush()
		} else {
			s.fin.Accept(m.pcm)
		}
		s.mu.Lock()
		s.made = append(s.made, made...)
		s.heard = s.fin.end()
		s.idle = !s.fin.vad.Speaking() && s.fin.held == 0
		s.mu.Unlock()
		made = nil
		if m.flushed != nil {
			close(m.flushed)
		}
	}
}

func (s *krokoStream) end() int { return s.histStart + len(s.hist) }

func (s *krokoStream) Accept(pcm []float32) []live.Event {
	if s.fin != nil {
		s.audio <- finMsg{pcm: append([]float32(nil), pcm...)} // it raises quiet input itself
	}
	if s.tune.gain {
		pcm = s.gain.apply(pcm, s.vad.Speaking())
	}
	s.hist = append(s.hist, pcm...)
	s.rest = append(s.rest, pcm...)
	for len(s.rest) >= liveWin {
		s.vad.Accept(s.rest[:liveWin])
		s.rest = s.rest[liveWin:]
		pos := s.end() - len(s.rest)
		for _, sp := range s.vad.Pop() {
			s.ended(s.base+sp.Start+len(sp.Samples), pos, s.vad.Speaking())
		}
		switch on := s.vad.Speaking(); {
		case on && s.spoke < 0:
			s.spoke = max(s.base, s.stEnd, pos-liveOnset)
		case !on:
			s.spoke = -1
		}
		if s.spoke >= 0 && s.st == nil {
			s.begin(pos)
		}
	}
	s.rest = append([]float32(nil), s.rest...)
	if s.st != nil {
		s.feed(s.end())
	}
	s.trim()
	return s.collect()
}

// begin lets Kroko hear the speech going on; pos is how far the VAD has heard.
func (s *krokoStream) begin(pos int) {
	// some audio before, so that the model hears the first sound whole
	s.stFrom = max(s.stEnd, s.histStart, s.spoke-liveLead)
	s.stFed = s.stFrom
	s.st = s.on.NewStream()
	s.runs = append(s.runs, &krokoRun{a: s.spoke, from: s.stFrom})
	s.feed(pos)
}

// feed gives Kroko the audio up to sample b.
func (s *krokoStream) feed(b int) {
	if b > s.stFed {
		s.st.Accept(s.hist[s.stFed-s.histStart : b-s.histStart])
		s.stFed = b
	}
}

// heardWords are Kroko's words of the speech going on, as far as they go.
func (s *krokoStream) heardWords() []model.Word {
	if s.st == nil {
		return nil
	}
	res := s.st.Result()
	from := s.runs[len(s.runs)-1].from
	var out []model.Word
	for _, w := range tokensToWords(res.Tokens, float64(s.stFrom)/live.SR, float64(s.stFed)/live.SR) {
		if int(w.Start*live.SR) >= from {
			out = append(out, w)
		}
	}
	return out
}

// ended is the VAD's end of speech at sample b; pos is how far it has heard. With goOn the VAD cut
// speech that goes on, and Kroko hears on.
func (s *krokoStream) ended(b, pos int, goOn bool) {
	if s.st == nil {
		return
	}
	r := s.runs[len(s.runs)-1]
	if !goOn {
		s.feed(pos)
		s.st.Finish(krokoTail)
	}
	ws := s.heardWords()
	next := b
	for i, w := range ws {
		t := int(w.Start * live.SR)
		if goOn && (t >= b || i == len(ws)-1) { // the last word heard may still grow
			next = min(next, t)
			break
		}
		r.words = append(r.words, w)
	}
	r.b = b
	if goOn {
		// with the words Kroko has not given yet, which may begin before b
		s.runs = append(s.runs, &krokoRun{a: b, from: next})
	} else {
		s.st.Close()
		s.st, s.stEnd = nil, s.stFed
	}
	if s.fin == nil {
		s.ready = append(s.ready, s.krokoFinal(r)...)
	}
}

// krokoFinal is the final of a run Kroko heard, when there is no Parakeet.
func (s *krokoStream) krokoFinal(r *krokoRun) []live.Event {
	s.runs = slices.DeleteFunc(s.runs, func(x *krokoRun) bool { return x == r })
	ws := r.words
	// a command said on its own („Komma“) is short and may sound doubtful, but is meant
	if _, n := matchCommand(ws); doubtful(ws, false) && !(s.voice.commands && n > 0 && n == len(ws)) {
		ws = nil // a cough or a door the model turned into a word or two
	}
	if len(ws) == 0 {
		return nil
	}
	ws = s.own(ws, false)
	text := liveText(ws)
	if text == "" {
		return nil
	}
	start := math.Min(float64(r.a)/live.SR, ws[0].Start)
	return []live.Event{{Final: true, Text: text, Start: round(start, 2), End: round(float64(r.b)/live.SR, 2)}}
}

// collect hands out the finals made since the last call, and forgets what of Kroko's they cover.
func (s *krokoStream) collect() []live.Event {
	if s.fin == nil {
		out := s.ready
		s.ready = nil
		if len(out) > 0 {
			s.shown = ""
		}
		return out
	}
	s.mu.Lock()
	made, heard, idle := s.made, s.heard, s.idle
	s.made = nil
	s.mu.Unlock()
	var out []live.Event
	for _, f := range made {
		out = append(out, f.ev)
		s.said = append(s.said, f.ws...)
		s.said = s.said[max(0, len(s.said)-3):]
		// a word Kroko heard at the end of the line may begin a little after it
		s.until = max(s.until, f.ev.End+0.1)
		s.shown = "" // the preview shows what is left
	}
	// what ended before the finals' stream went silent without a final for it was a cough
	for len(s.runs) > 0 {
		r := s.runs[0]
		if r.b == 0 || !(float64(r.b)/live.SR < s.until || idle && r.b < heard-live.SR/2) {
			break
		}
		s.runs = s.runs[1:]
	}
	return out
}

// own is liveStream.own for the preview, which comes after the finals given out.
func (s *krokoStream) own(ws []model.Word, open bool) []model.Word {
	ws = fixWords(ws, s.voice.words)
	if !s.voice.commands {
		return ws
	}
	out, _ := spokenMarks(ws, s.said, "", open)
	if !open {
		s.said = append(s.said, out...)
		s.said = s.said[max(0, len(s.said)-3):]
	}
	return out
}

// Preview is what Kroko heard that no final has covered yet.
func (s *krokoStream) Preview() (live.Event, bool) {
	var ws []model.Word
	a := -1
	for i, r := range s.runs {
		src := r.words
		if i == len(s.runs)-1 && r.b == 0 {
			src = s.heardWords()
		}
		for _, w := range src {
			if w.Start >= s.until {
				if a < 0 {
					a = r.a
				}
				ws = append(ws, w)
			}
		}
	}
	text := ""
	if len(ws) > 0 {
		text = liveText(s.own(ws, true))
	}
	if text == s.shown {
		return live.Event{}, false
	}
	t := round(float64(s.end())/live.SR, 2)
	if text == "" { // what it showed came to no final: a cough
		s.shown = ""
		return live.Event{Start: t, End: t}, true
	}
	s.shown, s.covered = text, s.end()
	return live.Event{Text: text, Start: round(float64(max(a, 0))/live.SR, 2), End: t}, true
}

// Pending implements live.Hearer: the speech going on that no event has covered yet.
func (s *krokoStream) Pending() (from, to float64, ok bool) {
	if s.spoke < 0 {
		return 0, 0, false
	}
	return float64(max(s.spoke, s.covered)) / live.SR, float64(s.end()) / live.SR, true
}

// trim drops the audio Kroko has heard, but for the lead before speech.
func (s *krokoStream) trim() {
	keep := s.end() - liveLead
	if s.st != nil {
		keep = min(keep, s.stFed)
	}
	if n := keep - s.histStart; n > 0 {
		s.hist = append([]float32(nil), s.hist[n:]...)
		s.histStart += n
	}
}

// Flush ends what is open as one utterance, waits for all finals and starts afresh.
func (s *krokoStream) Flush() []live.Event {
	pos := s.end()
	for _, sp := range s.vad.Pop() {
		s.ended(s.base+sp.Start+len(sp.Samples), pos, s.vad.Speaking())
	}
	s.ended(pos, pos, false)
	if s.fin != nil {
		flushed := make(chan struct{})
		s.audio <- finMsg{flushed: flushed}
		<-flushed
	}
	out := s.collect()
	s.vad.Reset()
	s.base = s.end()
	s.spoke, s.covered, s.stEnd = -1, s.base, s.base
	s.rest, s.runs, s.shown = nil, nil, ""
	s.until = float64(s.base) / live.SR
	s.trim()
	return out
}

func (s *krokoStream) Close() {
	if s.closed {
		return
	}
	s.closed = true
	if s.st != nil {
		s.st.Close()
	}
	s.vad.Close()
	if s.fin != nil {
		s.quit.Store(true)
		close(s.audio) // work lets the recognizers go when it ends
	} else if s.onKey != "" {
		releaseLiveRecognizer(s.onKey)
	}
}

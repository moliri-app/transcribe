// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/audio"
	"github.com/moliri-app/transcribe/models"
	"github.com/moliri-app/transcribe/sherpa"
	"github.com/moliri-app/transcribe/stream"
)

// TestLiveBench plays recordings through the live stream as a recording does, in pieces of 100 ms,
// and measures its finals against a reference transcript: the word error rate, the words missing,
// how many lines, and how long after its last word began a final comes. For de it measures live
// transcription as it is for German (krokoStream: Kroko's preview, Parakeet's finals, and Kroko
// alone; benchKroko). Kroko is taken from TRANSCRIBE_MODELS like the others; without it those lines
// are left out.
//
// The bench is a directory (TRANSCRIBE_LIVE_BENCH) holding excerpts of a German podcast, each a
// reference text and two recordings of it: reference.txt with clean.wav and headset.wav,
// reference2.txt with clean2.wav and headset2.wav. The headset recording is the clean one
// band-limited to 8 kHz and 15 dB quieter, as a Bluetooth headset in hands-free mode sends it:
//
//	ffmpeg -ss 297 -t 166.8 -i podcast.mp3 -ac 1 -ar 16000 clean.wav
//	ffmpeg -ss 1698.9 -t 158.8 -i podcast.mp3 -ac 1 -ar 16000 clean2.wav
//	ffmpeg -i clean.wav -af "aresample=8000,highpass=f=200,lowpass=f=3400,volume=-15dB,aresample=16000" headset.wav
//
// A reference is what was said, word for word with repetitions: where the full transcription
// (Parakeet), Whisper turbo and the German model disagreed, by the majority and the sense.
//
// SHERPA_LIB and TRANSCRIBE_MODELS name the runtime and the models (the app's data/lib and
// data/models). Without them the test skips. go test -run LiveBench -v ./pipeline
// TRANSCRIBE_LIVE_LANGS (de auto), TRANSCRIBE_LIVE_VARIANTS (see benchVariants) and
// TRANSCRIBE_LIVE_TEXT=1 (the text recognised) narrow or widen it. TRANSCRIBE_LIVE_HOTWORDS
// (comma-separated) and TRANSCRIBE_LIVE_HOTWORDS_SCORE decode with hotwords, to see what they
// cost and whether they turn words that were said into one of them.
func TestLiveBench(t *testing.T) {
	m, excerpts := loadBench(t)
	langs := strings.Fields(os.Getenv("TRANSCRIBE_LIVE_LANGS"))
	if len(langs) == 0 {
		langs = []string{"de", "auto"}
	}
	variants := benchVariants()
	if v := os.Getenv("TRANSCRIBE_LIVE_VARIANTS"); v != "" {
		var pick []benchVariant
		for _, name := range strings.Split(v, ",") {
			for _, bv := range variants {
				if bv.name == name {
					pick = append(pick, bv)
				}
			}
		}
		variants = pick
	}
	recs := map[string]*sherpa.Recognizer{}
	recognizer := func(key string) *sherpa.Recognizer {
		if recs[key] == nil {
			recs[key] = benchRecognizer(t, m, key)
		}
		return recs[key]
	}
	var summary []string
	for _, lang := range langs {
		key := liveModel(lang)
		var de *sherpa.Recognizer
		if key == "parakeet" && germanMayBeSpoken(lang) {
			de = recognizer("german")
		}
		for _, kind := range []string{"clean", "headset"} {
			if key == "kroko" {
				summary = append(summary, benchKroko(t, m, excerpts, kind, recognizer("parakeet"))...)
			}
			for _, v := range variants {
				if key == "kroko" {
					break
				}
				var sum werResult
				var lines, whole int
				var delay, compute float64
				for _, e := range excerpts {
					s, err := newLiveStream(recognizer(key), de, key, m.SileroVAD(), v.tune, voice{})
					if err != nil {
						t.Fatal(err)
					}
					r := playLive(s, e.pcm[kind])
					s.vad.Close()
					w := wordErrors(e.ref, benchWords(r.text))
					t.Logf("%-4s %-8s %-21s WER %5.1f %% (%3d of %d words: %2d missing, %2d wrong, %2d extra), %2d lines (%2d end a sentence), final %.2f s after its last word began (+%.2f s computing)",
						lang, kind+e.name, v.name, 100*w.rate(), w.sub+w.del+w.ins, w.n, w.del, w.sub, w.ins, r.lines, r.whole, r.delay, r.compute)
					if os.Getenv("TRANSCRIBE_LIVE_TEXT") != "" {
						t.Logf("  missing: %s", strings.Join(w.missing, " "))
						t.Logf("  text:\n%s", r.all)
					}
					sum.n, sum.sub, sum.del, sum.ins = sum.n+w.n, sum.sub+w.sub, sum.del+w.del, sum.ins+w.ins
					lines += r.lines
					whole += r.whole
					delay += r.delay * float64(r.lines)
					compute += r.compute * float64(r.lines)
				}
				summary = append(summary, fmt.Sprintf("%-4s %-8s %-21s WER %5.1f %% (%2d missing, %2d wrong, %2d extra), %2d lines (%2d end a sentence), final %.2f s after its last word began (+%.2f s computing)",
					lang, kind, v.name, 100*sum.rate(), sum.del, sum.sub, sum.ins, lines, whole, delay/float64(max(1, lines)), compute/float64(max(1, lines))))
			}
		}
	}
	t.Log("all excerpts:\n" + strings.Join(summary, "\n"))
}

// benchKroko measures live transcription of German (krokoStream) on the kind of recording: with
// Parakeet's finals (fin) and with Kroko's alone. Besides the error rate it tells how far the
// preview trails (from the start of a word to the first preview that shows it, in pieces of
// 100 ms), the longest call of the stream (what Kroko holds up the recording for; the finals are
// made beside it), and the computing per second of audio, Parakeet's included.
func benchKroko(t *testing.T, m models.Set, excerpts []benchExcerpt, kind string, fin *sherpa.Recognizer) []string {
	if _, err := os.Stat(m.Kroko().Encoder); err != nil {
		t.Log("no Kroko:", err)
		return nil
	}
	on, err := newKroko(m)
	if err != nil {
		t.Fatal(err)
	}
	defer on.Close()
	var summary []string
	for _, v := range []struct {
		name string
		fin  *sherpa.Recognizer
	}{{"kroko+parakeet", fin}, {"kroko alone", nil}} {
		if pick := os.Getenv("TRANSCRIBE_LIVE_VARIANTS"); pick != "" && !slices.Contains(strings.Split(pick, ","), v.name) {
			continue
		}
		var sum werResult
		var lines, whole int
		var lags []float64
		var longest time.Duration
		var took, secs float64
		for _, e := range excerpts {
			s, err := newKrokoStream(on, v.fin, "", "", m.SileroVAD(), liveTune, voice{})
			if err != nil {
				t.Fatal(err)
			}
			pcm := e.pcm[kind]
			var texts []string
			var all string
			add := func(evs []stream.Event) {
				for _, ev := range evs {
					if ev.Final {
						lines++
						if strings.ContainsAny(ev.Text[len(ev.Text)-1:], ".?!…") {
							whole++
						}
						texts = append(texts, ev.Text)
						all += fmt.Sprintf("  %6.1f-%6.1f  %s\n", ev.Start, ev.End, ev.Text)
					}
				}
			}
			seen := -1.0
			t0 := time.Now()
			const piece = stream.SR / 10
			for i := 0; i < len(pcm); i += piece {
				j := min(len(pcm), i+piece)
				c0 := time.Now()
				add(s.Accept(pcm[i:j]))
				a := time.Since(c0)
				_, shown := s.Preview()
				if d := time.Since(c0); d > time.Second {
					t.Logf("  at %.1f s a call took %.2f s (Accept %.2f s)", float64(j)/stream.SR, d.Seconds(), a.Seconds())
				}
				longest = max(longest, time.Since(c0))
				if shown {
					for _, w := range s.heardWords() {
						if w.Start > seen {
							lags = append(lags, float64(j)/stream.SR-w.Start)
							seen = w.Start
						}
					}
				}
			}
			add(s.Flush())
			took += time.Since(t0).Seconds()
			secs += float64(len(pcm)) / stream.SR
			s.Close()
			w := wordErrors(e.ref, benchWords(strings.Join(texts, " ")))
			t.Logf("de   %-8s %-21s WER %5.1f %% (%3d of %d words: %2d missing, %2d wrong, %2d extra)", kind+e.name, v.name, 100*w.rate(), w.sub+w.del+w.ins, w.n, w.del, w.sub, w.ins)
			if os.Getenv("TRANSCRIBE_LIVE_TEXT") != "" {
				t.Logf("  missing: %s", strings.Join(w.missing, " "))
				t.Logf("  text:\n%s", all)
			}
			sum.n, sum.sub, sum.del, sum.ins = sum.n+w.n, sum.sub+w.sub, sum.del+w.del, sum.ins+w.ins
		}
		slices.Sort(lags)
		var mean float64
		for _, l := range lags {
			mean += l / float64(len(lags))
		}
		p90 := 0.0
		if len(lags) > 0 {
			p90 = lags[len(lags)*9/10]
		}
		summary = append(summary, fmt.Sprintf("de   %-8s %-21s WER %5.1f %% (%2d missing, %2d wrong, %2d extra), %2d lines (%2d end a sentence), preview %.2f s after a word began (90 %%: %.2f s), longest call %.2f s, %.2f s computing per second of audio",
			kind, v.name, 100*sum.rate(), sum.del, sum.sub, sum.ins, lines, whole, mean, p90, longest.Seconds(), took/secs))
	}
	return summary
}

type benchExcerpt struct {
	ref  []string
	pcm  map[string][]float32 // clean, headset
	name string
}

// loadBench opens the models and the excerpts of the bench, or skips the test.
func loadBench(t *testing.T) (models.Set, []benchExcerpt) {
	lib := sherpa.FindLib(os.Getenv("SHERPA_LIB"))
	dir, bench := os.Getenv("TRANSCRIBE_MODELS"), os.Getenv("TRANSCRIBE_LIVE_BENCH")
	if lib == "" || dir == "" || bench == "" {
		t.Skip("set SHERPA_LIB, TRANSCRIBE_MODELS, TRANSCRIBE_LIVE_BENCH")
	}
	sherpa.SetLibDir(lib)
	m := models.Set{Dir: dir}
	for _, p := range []string{m.SileroVAD(), m.Parakeet().Encoder, m.GermanFastConformer().Encoder} {
		if _, err := os.Stat(p); err != nil {
			t.Skip(err)
		}
	}
	var excerpts []benchExcerpt
	for _, n := range []string{"", "2"} {
		ref, err := os.ReadFile(filepath.Join(bench, "reference"+n+".txt"))
		if err != nil {
			continue
		}
		e := benchExcerpt{ref: benchWords(string(ref)), pcm: map[string][]float32{}, name: n}
		for _, kind := range []string{"clean", "headset"} {
			pcm, err := audio.Decode(context.Background(), filepath.Join(bench, kind+n+".wav"))
			if err != nil {
				t.Skip(err)
			}
			e.pcm[kind] = pcm
		}
		excerpts = append(excerpts, e)
	}
	if len(excerpts) == 0 {
		t.Skip("no reference.txt in " + bench)
	}
	return m, excerpts
}

type benchVariant struct {
	name string
	tune liveTuning
}

// benchVariants are the changes to the live stream, alone and together, and what live
// transcription uses now.
func benchVariants() []benchVariant {
	before := liveTuning{threshold: 0.5, minSilence: 0.5}
	var out []benchVariant
	add := func(name string, changes ...func(*liveTuning)) {
		t := before
		for _, f := range changes {
			f(&t)
		}
		out = append(out, benchVariant{name, t})
	}
	seamless := func(t *liveTuning) { t.seamless = true }
	pad := func(t *liveTuning) { t.pad = 0.3 }
	split := func(t *liveTuning) { t.split = 12 }
	vad := func(t *liveTuning) { t.threshold = 0.35 }
	recheck := func(t *liveTuning) { t.recheck = true }
	gain := func(t *liveTuning) { t.gain = true }
	hold := func(sec float64) func(*liveTuning) { return func(t *liveTuning) { t.hold = sec } }
	context := func(t *liveTuning) { t.context = 4 }
	add("before")
	add("seamless", seamless)
	add("s+pad", seamless, pad)
	add("s+p+split", seamless, pad, split)
	add("s+p+split+vad", seamless, pad, split, vad)
	add("s+p+split+vad+ctx", seamless, pad, split, vad, context)
	add("+hold", seamless, pad, split, vad, context, hold(1))
	add("+silence", seamless, pad, split, vad, context, hold(1), func(t *liveTuning) { t.minSilence = 1 })
	add("+recheck+gain", seamless, pad, split, vad, context, hold(1), recheck, gain)
	return append(out, benchVariant{"now", liveTune})
}

func benchRecognizer(t *testing.T, m models.Set, key string) *sherpa.Recognizer {
	tr := m.Parakeet()
	if key == "german" {
		tr = m.GermanFastConformer()
	}
	var hot []string
	if h := os.Getenv("TRANSCRIBE_LIVE_HOTWORDS"); h != "" && key == "parakeet" { // as recKey
		hot = strings.Split(h, ",")
	}
	var score float32
	fmt.Sscan(os.Getenv("TRANSCRIBE_LIVE_HOTWORDS_SCORE"), &score)
	rec, err := sherpa.NewRecognizer(sherpa.RecognizerConfig{Kind: "transducer", Encoder: tr.Encoder, Decoder: tr.Decoder, Joiner: tr.Joiner, Tokens: tr.Tokens, Threads: 4, Hotwords: hot, HotwordsScore: score})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rec.Close)
	return rec
}

type liveRun struct {
	text, all string
	lines     int
	whole     int     // lines that end a sentence
	delay     float64 // mean seconds of audio between the end of an utterance and its final
	compute   float64 // mean seconds the call that gave a final took
}

// playLive feeds pcm in pieces of 100 ms and collects the finals. The delay of a final counts from
// the start of its last word (the end of the last word is not known).
func playLive(s *liveStream, pcm []float32) liveRun {
	var r liveRun
	var texts []string
	var delay, compute float64
	add := func(evs []stream.Event, pos int, took time.Duration) {
		for _, ev := range evs {
			if !ev.Final {
				continue
			}
			r.lines++
			if strings.ContainsAny(ev.Text[len(ev.Text)-1:], ".?!…") {
				r.whole++
			}
			texts = append(texts, ev.Text)
			r.all += fmt.Sprintf("  %6.1f-%6.1f  %s\n", ev.Start, ev.End, ev.Text)
			delay += float64(pos)/stream.SR - s.last.Start
			compute += took.Seconds()
		}
	}
	const piece = stream.SR / 10
	for i := 0; i < len(pcm); i += piece {
		j := min(len(pcm), i+piece)
		t0 := time.Now()
		evs := s.Accept(pcm[i:j])
		add(evs, j, time.Since(t0))
	}
	t0 := time.Now()
	add(s.Flush(), len(pcm), time.Since(t0))
	r.text = strings.Join(texts, " ")
	if r.lines > 0 {
		r.delay, r.compute = delay/float64(r.lines), compute/float64(r.lines)
	}
	return r
}

var benchFillers = map[string]bool{"ähm": true, "äh": true, "öhm": true, "hm": true, "mhm": true, "hmm": true}

// benchWords are the words of text as compared: lower case, without punctuation and fillers,
// compounds with hyphens apart, numbers as digits.
func benchWords(text string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(text, func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' || r == '-' }) {
		w := toDigits(normWord(f))
		if w != "" && !benchFillers[w] {
			out = append(out, w)
		}
	}
	return out
}

type werResult struct {
	n, sub, del, ins int
	missing          []string
}

func (w werResult) rate() float64 { return float64(w.sub+w.del+w.ins) / float64(max(1, w.n)) }

// wordErrors aligns hyp to ref (Levenshtein over words).
func wordErrors(ref, hyp []string) werResult {
	n, m := len(ref), len(hyp)
	d := make([][]int, n+1)
	for i := range d {
		d[i] = make([]int, m+1)
		d[i][0] = i
	}
	for j := 0; j <= m; j++ {
		d[0][j] = j
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			c := 1
			if ref[i-1] == hyp[j-1] {
				c = 0
			}
			d[i][j] = min(d[i-1][j-1]+c, d[i-1][j]+1, d[i][j-1]+1)
		}
	}
	r := werResult{n: n}
	for i, j := n, m; i > 0 || j > 0; {
		switch {
		case i > 0 && j > 0 && d[i][j] == d[i-1][j-1]+map[bool]int{true: 0, false: 1}[ref[i-1] == hyp[j-1]]:
			if ref[i-1] != hyp[j-1] {
				r.sub++
			}
			i, j = i-1, j-1
		case i > 0 && d[i][j] == d[i-1][j]+1:
			r.del++
			r.missing = append([]string{ref[i-1]}, r.missing...)
			i--
		default:
			r.ins++
			j--
		}
	}
	return r
}

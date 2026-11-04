// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/moliri-app/transcribe/internal/audio"
	"github.com/moliri-app/transcribe/internal/model"
	"github.com/moliri-app/transcribe/internal/sherpa"
)

func hasAlnum(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// tokensToWords merges SentencePiece tokens ('▁' or a leading space marks a word start) into timed words.
// A word's confidence p is the probability of its least likely token.
func tokensToWords(tokens []sherpa.Token, offset, segEnd float64) []model.Word {
	type wd struct {
		w          string
		start, end float64
		lp         float64
	}
	var words []*wd
	pending := true
	for _, tk := range tokens {
		if strings.HasPrefix(tk.Text, "▁") || strings.HasPrefix(tk.Text, " ") {
			pending = true
		}
		piece := strings.TrimLeft(tk.Text, "▁ ")
		if piece == "" {
			continue
		}
		t := offset + tk.TS
		if pending && len(words) > 0 && !hasAlnum(piece) {
			pending = false
		}
		if pending || len(words) == 0 {
			if n := len(words); n > 0 {
				words[n-1].end = t
			}
			words = append(words, &wd{piece, t, -1, tk.LogP})
			pending = false
		} else {
			w := words[len(words)-1]
			w.w += piece
			w.lp = math.Min(w.lp, tk.LogP)
		}
	}
	if n := len(words); n > 0 {
		words[n-1].end = segEnd
	}
	out := make([]model.Word, 0, len(words))
	for i, w := range words {
		if w.end <= w.start {
			if i+1 < len(words) {
				w.end = words[i+1].start
			} else {
				w.end = segEnd
			}
		}
		out = append(out, model.Word{W: w.w, Start: round(w.start, 3), End: round(w.end, 3), P: round(math.Exp(w.lp), 3)})
	}
	return out
}

// speakerTurns merges neighbouring same-speaker segments and pads them slightly.
func speakerTurns(segs []SpeakerSeg, duration float64) [][2]float64 {
	type turn struct {
		a, b float64
		spk  int
	}
	var turns []turn
	for _, s := range segs {
		if n := len(turns); n > 0 && turns[n-1].spk == s.Speaker && s.Start-turns[n-1].b < 1.0 {
			turns[n-1].b = s.End
		} else {
			turns = append(turns, turn{s.Start, s.End, s.Speaker})
		}
	}
	out := make([][2]float64, len(turns))
	for i, t := range turns {
		out[i] = [2]float64{math.Max(0, t.a-0.2), math.Min(duration, t.b+0.2)}
	}
	return out
}

// speakerCuts partitions the whole timeline at speaker changes; turns shorter than minTurn are merged
// into a neighbour first.
func speakerCuts(segs []SpeakerSeg, duration, minTurn float64, pcm []float32, snap float64) [][2]float64 {
	turns := speakerTurns(segs, duration)
	for changed := true; changed && len(turns) > 1; {
		changed = false
		for i, t := range turns {
			if t[1]-t[0] >= minTurn {
				continue
			}
			var j int
			switch {
			case i == 0:
				j = 1
			case i == len(turns)-1:
				j = i - 1
			case t[0]-turns[i-1][1] <= turns[i+1][0]-t[1]:
				j = i - 1
			default:
				j = i + 1
			}
			lo, hi := min(i, j), max(i, j)
			turns[lo] = [2]float64{math.Min(turns[lo][0], turns[hi][0]), math.Max(turns[lo][1], turns[hi][1])}
			turns = append(turns[:hi], turns[hi+1:]...)
			changed = true
			break
		}
	}
	cuts := []float64{0}
	for i := 0; i+1 < len(turns); i++ {
		a, b := turns[i], turns[i+1]
		c := b[0]
		if b[0] > a[1] {
			c = (a[1] + b[0]) / 2
		}
		cuts = append(cuts, math.Max(cuts[len(cuts)-1], c))
	}
	if snap > 0 && pcm != nil {
		rest := make([]float64, 0, len(cuts))
		for _, c := range cuts[1:] {
			rest = append(rest, snapToPause(pcm, c, snap))
		}
		sort.Float64s(rest)
		cuts = append([]float64{0}, rest...)
	}
	cuts = append(cuts, duration)
	var out [][2]float64
	for i := 0; i+1 < len(cuts); i++ {
		if cuts[i+1]-cuts[i] > 0.05 {
			out = append(out, [2]float64{cuts[i], cuts[i+1]})
		}
	}
	return out
}

// splitLong cuts a speech piece into parts of at most maxLen seconds at quiet points.
func splitLong(sp sherpa.Speech, maxLen float64) []sherpa.Speech {
	n := len(sp.Samples)
	if float64(n)/audio.SR <= maxLen+1 {
		return []sherpa.Speech{sp}
	}
	var out []sherpa.Speech
	from := 0
	for float64(n-from)/audio.SR > maxLen+1 {
		target := float64(from)/audio.SR + maxLen - 2
		cut := int(snapToPause(sp.Samples, target, 2) * audio.SR)
		cut = max(from+audio.SR, min(cut, n))
		out = append(out, sherpa.Speech{Start: sp.Start + from, Samples: sp.Samples[from:cut]})
		from = cut
	}
	return append(out, sherpa.Speech{Start: sp.Start + from, Samples: sp.Samples[from:]})
}

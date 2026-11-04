// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package annotate

import (
	"maps"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ResolveMerges follows merge chains: {"3": merge into 1, "1": merge into 0} -> {3: 0, 1: 0}.
func ResolveMerges(speakers map[string]SpeakerAnn) map[int]int {
	m := map[int]int{}
	for k, v := range speakers {
		from, ok1 := Flex(k).Int()
		to, ok2 := v.MergeInto.Int()
		if v.MergeInto != "" && ok1 && ok2 {
			m[from] = to
		}
	}
	out := map[int]int{}
	for k := range m {
		t, seen := k, map[int]bool{}
		for {
			next, ok := m[t]
			if !ok || seen[t] {
				break
			}
			seen[t] = true
			t = next
		}
		out[k] = t
	}
	return out
}

// round3 rounds like Python's round(x, 3).
func round3(x float64) float64 {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 3, 64), 64)
	return r
}

// majoritySpeaker is the speaker with the most words (-1 = none), the first one seen on a tie.
func majoritySpeaker(words []Word) int {
	count := map[int]int{}
	var order []int
	for _, w := range words {
		s := Speaker(w)
		if _, ok := count[s]; !ok {
			order = append(order, s)
		}
		count[s]++
	}
	best := -1
	bestN := 0
	for _, s := range order {
		if count[s] > bestN {
			best, bestN = s, count[s]
		}
	}
	return best
}

func sortByStart(words []Word) {
	sort.SliceStable(words, func(i, j int) bool { return Num(words[i], "start") < Num(words[j], "start") })
}

// ApplyEdit mode "replace": the words in [a, b] are replaced; the new words share the time of the old words
// and of the marked range (a range marked over a pause gives inserted words their real time).
// mode "insert": nothing is removed; a word reaching into [a, b] is shortened to end at a.
func ApplyEdit(words []Word, a, b float64, text, mode string) []Word {
	var old, keep []Word
	if mode == "insert" {
		for _, w := range words {
			w = maps.Clone(w)
			if Num(w, "start") < a && a < Num(w, "end") {
				w["end"] = round3(a)
			}
			keep = append(keep, w)
		}
	} else {
		for _, w := range words {
			if m := Mid(w); a <= m && m <= b {
				old = append(old, w)
			} else {
				keep = append(keep, w)
			}
		}
	}
	toks := strings.Fields(text)
	if len(toks) == 0 {
		return keep
	}
	t0, t1 := a, b
	if len(old) > 0 {
		t0, t1 = min(a, Num(old[0], "start")), max(b, Num(old[len(old)-1], "end"))
	}
	t1 = max(t1, t0+0.05*float64(len(toks)))
	spk := majoritySpeaker(old)
	if spk < 0 { // inserted into a gap: take the speaker of the nearest word
		best := math.Inf(1)
		for _, w := range keep {
			if d := math.Min(math.Abs(Num(w, "end")-a), math.Abs(Num(w, "start")-b)); d < best {
				best, spk = d, Speaker(w)
			}
		}
	}
	total := 0
	for _, t := range toks {
		total += utf8.RuneCountInString(t)
	}
	t := t0
	out := keep
	for _, tok := range toks {
		d := (t1 - t0) * float64(utf8.RuneCountInString(tok)) / float64(total)
		w := Word{"w": tok, "start": round3(t), "end": round3(t + d), "p": 1.0, "edited": true}
		SetSpeaker(w, spk)
		out = append(out, w)
		t += d
	}
	sortByStart(out)
	return out
}

type turn struct{ a, b float64 }

func speakerTurns(words []Word, spk int) []turn {
	var turns []turn
	var cur *turn
	for _, w := range words {
		switch s := Speaker(w); {
		case s == spk:
			if cur == nil {
				cur = &turn{Num(w, "start"), Num(w, "end")}
			} else {
				cur.b = Num(w, "end")
			}
		case cur != nil && s >= 0:
			turns = append(turns, *cur)
			cur = nil
		}
	}
	if cur != nil {
		turns = append(turns, *cur)
	}
	return turns
}

// smoothSentences gives every sentence the speaker who talks longest in it.
func smoothSentences(words []Word) {
	for i := 0; i < len(words); {
		j := i
		for j < len(words)-1 && !SentenceEnd(Text(words[j])) &&
			Num(words[j+1], "start")-Num(words[j], "end") < 1.0 && Fill(words[j+1]) == Fill(words[i]) {
			j++
		}
		dur := map[int]float64{}
		var order []int
		for _, w := range words[i : j+1] {
			if s := Speaker(w); s >= 0 {
				if _, ok := dur[s]; !ok {
					order = append(order, s)
				}
				dur[s] += Num(w, "end") - Num(w, "start")
			}
		}
		if len(dur) > 1 {
			top := order[0]
			for _, s := range order {
				if dur[s] > dur[top] {
					top = s
				}
			}
			for _, w := range words[i : j+1] {
				if !Truthy(w["speaker_locked"]) && !(Fill(w) && Speaker(w) < 0) {
					SetSpeaker(w, top)
				}
			}
		}
		i = j + 1
	}
}

// ToSegments groups words into segments: a new one at every speaker change or sentence end after a pause.
func ToSegments(words []Word) []Word {
	var segs []Word
	for _, w := range words {
		var last Word
		if len(segs) > 0 {
			last = segs[len(segs)-1]
		}
		if last == nil || Speaker(last) != Speaker(w) || lastEndsSentence(last) && Num(w, "start")-Num(last, "end") > 1.5 {
			s := Word{"start": Num(w, "start"), "end": Num(w, "end"), "words": []Word{}}
			SetSpeaker(s, Speaker(w))
			segs = append(segs, s)
			last = s
		}
		last["words"] = append(last["words"].([]Word), w)
		last["end"] = Num(w, "end")
	}
	for _, s := range segs {
		ws := s["words"].([]Word)
		texts := make([]string, len(ws))
		allFill := true
		for i, w := range ws {
			texts[i] = Text(w)
			allFill = allFill && Fill(w)
		}
		s["text"] = strings.Join(texts, " ")
		if allFill {
			s["fill"] = true
		}
	}
	return segs
}

func lastEndsSentence(seg Word) bool {
	ws := seg["words"].([]Word)
	return SentenceEnd(Text(ws[len(ws)-1]))
}

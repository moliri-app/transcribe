// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package annotate

import (
	"maps"
	"math"
)

// Time anchors are the word starts a person can grab: the first word, a word after a pause of at least
// AnchorGap seconds, a word after a sentence end, and every word a person placed an anchor on ("pin").
// A word a person took an anchor away from carries "nopin". The editor (frontend/src/lib/editor/model.js, isAnchor) has the same rule.
const AnchorGap = 0.35

// IsAnchor reports whether word i of the start-sorted words carries a time anchor.
func IsAnchor(words []Word, i int) bool {
	w := words[i]
	if Truthy(w["pin"]) {
		return true
	}
	if Truthy(w["nopin"]) {
		return false
	}
	if i == 0 {
		return true
	}
	p := words[i-1]
	return Num(w, "start")-Num(p, "end") >= AnchorGap || SentenceEnd(Text(p))
}

// nearest is the index of the word starting closest to t (within 30 ms), or -1.
func nearest(words []Word, t float64) int {
	best, bi := 0.03, -1
	for i, w := range words {
		if d := math.Abs(Num(w, "start") - t); d <= best {
			best, bi = d, i
		}
	}
	return bi
}

// MoveAnchor puts the anchor of the word starting at from on time to: that word then starts at to and the
// words up to the neighbouring anchors are stretched linearly, so the text between anchors keeps its rhythm.
// With drop >= 0 the anchor was moved in the text: the word starting at drop gives its anchor up (its time to
// is handed to the word at from).
func MoveAnchor(words []Word, from, to, drop float64) []Word {
	out := make([]Word, len(words))
	for i, w := range words {
		out[i] = maps.Clone(w)
	}
	sortByStart(out)
	i := nearest(out, from)
	if i < 0 {
		return out
	}
	j := -1
	if drop >= 0 {
		if j = nearest(out, drop); j == i {
			j = -1
		}
	}
	if j >= 0 {
		delete(out[j], "pin")
		out[j]["nopin"] = true
	}
	prev, next := -1, len(out)
	for k := i - 1; k >= 0; k-- {
		if k != j && IsAnchor(out, k) {
			prev = k
			break
		}
	}
	for k := i + 1; k < len(out); k++ {
		if k != j && IsAnchor(out, k) {
			next = k
			break
		}
	}
	// source and target times of the three fixed points
	sPrev, tPrev := 0.0, 0.0
	if prev >= 0 {
		sPrev, tPrev = Num(out[prev], "start"), Num(out[prev], "start")
	} else if i > 0 {
		sPrev, tPrev = Num(out[0], "start"), Num(out[0], "start")
	}
	sNext := Num(out[len(out)-1], "end")
	if next < len(out) {
		sNext = Num(out[next], "start")
	}
	tNext := sNext
	sI := Num(out[i], "start")
	to = math.Max(tPrev+0.02*float64(i-max(prev, 0)), math.Min(to, tNext-0.02*float64(next-i)))
	f := func(x float64) float64 {
		switch {
		case x <= sPrev:
			return x
		case x < sI:
			if sI-sPrev < 1e-6 {
				return tPrev
			}
			return tPrev + (x-sPrev)*(to-tPrev)/(sI-sPrev)
		case x < sNext:
			if sNext-sI < 1e-6 {
				return to
			}
			return to + (x-sI)*(tNext-to)/(sNext-sI)
		default:
			return x
		}
	}
	lo := max(prev, 0)
	if prev < 0 && i == 0 {
		lo = 0
	}
	for k := lo; k < next; k++ {
		s, e := Num(out[k], "start"), Num(out[k], "end")
		ns := f(s)
		ne := math.Max(ns+0.01, f(math.Min(e, sNext)))
		out[k]["start"], out[k]["end"] = round3(ns), round3(ne)
	}
	out[i]["start"] = round3(to)
	delete(out[i], "nopin")
	out[i]["pin"] = true
	sortByStart(out)
	return out
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"sort"

	"github.com/moliri-app/transcribe/audio"
	"github.com/moliri-app/transcribe/jobs"
	"github.com/moliri-app/transcribe/model"
	"github.com/moliri-app/transcribe/sherpa"
)

type eventRule struct {
	Label  string
	MinP   float64
	MinDur float64
}

// AudioSet label -> shown label, min probability, min duration (s)
var eventMap = map[string]eventRule{
	"Music":                  {"Musik", 0.35, 2.0},
	"Musical instrument":     {"Musik", 0.45, 2.0},
	"Singing":                {"Gesang", 0.35, 1.5},
	"Applause":               {"Applaus", 0.3, 1.0},
	"Clapping":               {"Applaus", 0.4, 1.0},
	"Laughter":               {"Lachen", 0.3, 0.8},
	"Giggle":                 {"Lachen", 0.4, 0.8},
	"Chuckle, chortle":       {"Lachen", 0.4, 0.8},
	"Cheering":               {"Jubel", 0.3, 1.0},
	"Crowd":                  {"Menschenmenge", 0.4, 2.0},
	"Chatter":                {"Stimmengewirr", 0.4, 2.0},
	"Shout":                  {"Ruf", 0.25, 0.5},
	"Yell":                   {"Ruf", 0.25, 0.5},
	"Children shouting":      {"Ruf", 0.25, 0.5},
	"Screaming":              {"Schrei", 0.3, 0.5},
	"Telephone bell ringing": {"Telefon", 0.4, 0.5},
	"Ringtone":               {"Telefon", 0.4, 0.5},
	"Dog":                    {"Hund", 0.4, 0.5},
	"Bell":                   {"Glocke", 0.4, 0.5},
	"Whistling":              {"Pfeifen", 0.4, 0.8},
}

type hit struct {
	t     float64
	label string
	p     float64
}

// TagEvents ports analyze.py tag_events with bounds "mid".
func TagEvents(ctx context.Context, pcm []float32, t *sherpa.Tagger, win, hop, gap float64) ([]model.Event, error) {
	W, H := int(win*audio.SR), int(hop*audio.SR)
	var hits []hit
	limit := max(1, len(pcm)-W+H)
	for start := 0; start < limit; start += H {
		if err := jobs.Hold(ctx); err != nil {
			return nil, err
		}
		end := min(start+W, len(pcm))
		if end <= start {
			break
		}
		best := map[string]float64{}
		for _, ev := range t.Compute(pcm[start:end], 10) {
			if r, ok := eventMap[ev.Name]; ok && ev.Prob >= r.MinP && ev.Prob > best[r.Label] {
				best[r.Label] = ev.Prob
			}
		}
		for l, p := range best {
			hits = append(hits, hit{float64(start) / audio.SR, l, p})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].t != hits[j].t {
			return hits[i].t < hits[j].t
		}
		if hits[i].label != hits[j].label {
			return hits[i].label < hits[j].label
		}
		return hits[i].p < hits[j].p
	})

	lo, hi := win/2-hop/2, win/2+hop/2
	slack := win - hop
	minDur := map[string]float64{}
	for _, r := range eventMap {
		if d, ok := minDur[r.Label]; !ok || r.MinDur < d {
			minDur[r.Label] = r.MinDur
		}
	}
	var events []*model.Event
	open := map[string]*model.Event{}
	for _, h := range hits {
		if ev := open[h.label]; ev != nil && h.t+lo <= ev.End+max(hop, gap) {
			ev.End = h.t + hi
			ev.Prob = max(ev.Prob, h.p)
		} else {
			ev = &model.Event{Label: h.label, Start: h.t + lo, End: h.t + hi, Prob: h.p}
			open[h.label] = ev
			events = append(events, ev)
		}
	}
	dur := float64(len(pcm)) / audio.SR
	out := []model.Event{}
	for _, ev := range events {
		ev.End = min(ev.End, dur)
		if ev.End-ev.Start+slack >= minDur[ev.Label] {
			out = append(out, model.Event{Label: ev.Label, Start: round(ev.Start, 2), End: round(ev.End, 2), Prob: round(ev.Prob, 2)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out, nil
}

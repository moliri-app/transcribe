// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package annotate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strconv"

	"github.com/moliri-app/transcribe/model"
)

// Retranscriber recognises [a, b] seconds of an audio file again with the language forced to lang.
type Retranscriber interface {
	Retranscribe(ctx context.Context, audioPath string, a, b float64, lang string) ([]model.Word, error)
}

type region struct {
	a, b float64
	lang string
}

func isLang(f Flex) bool { return f != "" && f != "auto" }

func inRange(w Word, a, b float64) bool {
	m := Mid(w)
	return a <= m && m <= b
}

// Input is what one Apply run works on.
type Input struct {
	Machine       json.RawMessage            // the machine result (_machine.json)
	Annotations   *Annotations               // the human corrections; nil = none
	Retranscribed map[string]json.RawMessage // language re-recognitions of earlier runs; nil = none
	Audio         string                     // path of the recording; "" = no re-recognition
}

// Apply returns the final transcript (_final.json): machine result + language re-recognition + text edits +
// speaker spans + merges + one speaker per sentence, and the updated re-recognition cache. Language regions
// are recognised again only with rt and Audio set; otherwise only regions already in the cache are applied
// and final["languages_pending"] counts the ones left out (the caller queues a run with the audio for them).
func Apply(ctx context.Context, in Input, rt Retranscriber) (map[string]any, map[string]json.RawMessage, error) {
	if len(in.Machine) == 0 {
		return nil, nil, errors.New("no machine result (run the recognition first)")
	}
	var machine map[string]json.RawMessage
	if err := json.Unmarshal(in.Machine, &machine); err != nil {
		return nil, nil, fmt.Errorf("machine: %w", err)
	}
	var segs []struct {
		Words []Word `json:"words"`
	}
	if err := json.Unmarshal(machine["segments"], &segs); err != nil {
		return nil, nil, fmt.Errorf("machine segments: %w", err)
	}
	ann := in.Annotations
	if ann == nil {
		ann = &Annotations{}
	}
	if ann.Speakers == nil {
		ann.Speakers = map[string]SpeakerAnn{}
	}
	cache := map[string]json.RawMessage{}
	for k, v := range in.Retranscribed {
		cache[k] = v
	}
	if in.Audio == "" {
		rt = nil
	}
	var words []Word
	for _, s := range segs {
		words = append(words, s.Words...)
	}
	sortByStart(words)

	// 1. language: only what Whisper knows is recognised again; other languages (dialects) stay a note
	var regions []region
	for _, s := range ann.Spans {
		if isLang(s.Language) && Recognizable(string(s.Language)) {
			regions = append(regions, region{s.Start, s.End, string(s.Language)})
		}
	}
	for k, v := range ann.Speakers {
		if id, ok := Flex(k).Int(); ok && isLang(v.Language) && Recognizable(string(v.Language)) {
			for _, t := range speakerTurns(words, id) {
				regions = append(regions, region{t.a, t.b, string(v.Language)})
			}
		}
	}
	sort.SliceStable(regions, func(i, j int) bool {
		x, y := regions[i], regions[j]
		if x.a != y.a {
			return x.a < y.a
		}
		if x.b != y.b {
			return x.b < y.b
		}
		return x.lang < y.lang
	})
	pending := 0
	for _, r := range regions {
		// The speaker of the passage as a reader saw it: one speaker per sentence (smoothSentences),
		// not the raw speaker of each machine word, which may belong to another turn.
		shown := make([]Word, len(words))
		for i, w := range words {
			shown[i] = maps.Clone(w)
		}
		smoothSentences(shown)
		var old []Word
		for _, w := range shown {
			if inRange(w, r.a, r.b) {
				old = append(old, w)
			}
		}
		key := fmt.Sprintf("v2:%.2f-%.2f-%s", r.a, r.b, r.lang)
		if _, ok := cache[key]; !ok {
			if rt == nil {
				pending++
				continue
			}
			got, err := rt.Retranscribe(ctx, in.Audio, r.a, r.b, r.lang)
			if err != nil {
				return nil, nil, fmt.Errorf("re-transcribing %.1f-%.1f s as %s: %w", r.a, r.b, r.lang, err)
			}
			if got == nil {
				got = []model.Word{}
			}
			b, err := json.Marshal(got)
			if err != nil {
				return nil, nil, err
			}
			cache[key] = b
		}
		var fresh []Word
		if err := json.Unmarshal(cache[key], &fresh); err != nil {
			return nil, nil, fmt.Errorf("retranscribed %s: %w", key, err)
		}
		spk := majoritySpeaker(old)
		next := make([]Word, 0, len(words)+len(fresh))
		for _, w := range words {
			if !inRange(w, r.a, r.b) {
				next = append(next, w)
			}
		}
		for _, w := range fresh {
			if _, has := w["speaker"]; !(Truthy(w["kept"]) && has) {
				SetSpeaker(w, spk)
			}
			next = append(next, w)
		}
		words = next
		sortByStart(words)
	}
	// 2. text edits (in the order they were made; a later edit of the same words wins)
	for _, e := range ann.Edits {
		if e.Mode == "anchor" {
			drop := -1.0
			if e.Drop != nil {
				drop = *e.Drop
			}
			words = MoveAnchor(words, e.Start, e.End, drop)
			continue
		}
		words = ApplyEdit(words, e.Start, e.End, e.Text, e.Mode)
	}

	// 3. speakers
	for _, s := range ann.Spans {
		id, ok := s.Speaker.Int()
		if s.Speaker == "" || !ok {
			continue
		}
		for _, w := range words {
			if inRange(w, s.Start, s.End) {
				SetSpeaker(w, id)
				w["speaker_locked"] = true
			}
		}
	}
	merges := ResolveMerges(ann.Speakers)
	for _, w := range words {
		if to, ok := merges[Speaker(w)]; ok && Speaker(w) >= 0 {
			SetSpeaker(w, to)
		}
	}

	// 4. one speaker per sentence
	smoothSentences(words)
	for _, w := range words {
		delete(w, "speaker_locked")
	}

	present := map[int]bool{}
	edited := 0
	for _, w := range words {
		if s := Speaker(w); s >= 0 {
			present[s] = true
		}
		if Truthy(w["edited"]) {
			edited++
		}
	}
	speakers := map[string]map[string]string{}
	for k := range present {
		a := ann.Speakers[strconv.Itoa(k)]
		name, lang := a.Name, string(a.Language)
		if name == "" {
			name = "S" + strconv.Itoa(k+1)
		}
		if lang == "" {
			lang = "auto"
		}
		speakers[strconv.Itoa(k)] = map[string]string{"name": name, "language": lang}
	}
	final := map[string]any{}
	for k, v := range machine {
		if k != "segments" {
			final[k] = v
		}
	}
	final["engine"] = "★ Ergebnis"
	final["speakers"] = speakers
	final["annotated"] = len(ann.Speakers) > 0 || len(ann.Spans) > 0 || len(ann.Edits) > 0
	final["edited_words"] = edited
	final["languages_pending"] = pending
	// the editor puts only the edits made after this run on top of _final.json (an insert must not count twice)
	final["edits_applied"] = ann.Edits
	final["segments"] = ToSegments(words)
	return final, cache, nil
}

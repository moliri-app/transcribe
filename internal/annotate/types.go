// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package annotate puts the human corrections (_annotations.json) on the machine result and writes _final.json.
package annotate

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// Word is a transcript word as a free-form object, so fields this code does not know survive a rewrite.
type Word = map[string]any

var sentEnd = regexp.MustCompile(`[.!?…]["»«“”)]?$`)

// SentenceEnd reports whether a word text ends a sentence.
func SentenceEnd(w string) bool { return sentEnd.MatchString(w) }

func Num(w Word, key string) float64 {
	v, _ := w[key].(float64)
	return v
}

func Text(w Word) string {
	s, _ := w["w"].(string)
	return s
}

func Mid(w Word) float64 { return (Num(w, "start") + Num(w, "end")) / 2 }

// Speaker is the word's speaker, or -1 if it has none.
func Speaker(w Word) int {
	switch v := w["speaker"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return -1
}

func SetSpeaker(w Word, spk int) {
	if spk < 0 {
		w["speaker"] = nil
	} else {
		w["speaker"] = spk
	}
}

func Truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	}
	return false
}

func Fill(w Word) bool { return Truthy(w["fill"]) }

// Flex accepts a JSON string or number (the UI sends speaker ids as strings, hand-written files may use numbers).
type Flex string

func (f *Flex) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case string(b) == "null":
		*f = ""
	case len(b) > 0 && b[0] == '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = Flex(s)
	default:
		*f = Flex(b)
	}
	return nil
}

// Int parses the value as an integer ("2", 2, "2.0").
func (f Flex) Int() (int, bool) {
	if n, err := strconv.Atoi(string(f)); err == nil {
		return n, true
	}
	if x, err := strconv.ParseFloat(string(f), 64); err == nil {
		return int(x), true
	}
	return 0, false
}

type SpeakerAnn struct {
	Name      string `json:"name"`
	Language  Flex   `json:"language"`
	MergeInto Flex   `json:"merge_into"`
}

type SpanAnn struct {
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Language Flex    `json:"language"`
	Speaker  Flex    `json:"speaker"`
}

type EditAnn struct {
	Start float64  `json:"start"`
	End   float64  `json:"end"`
	Text  string   `json:"text"`
	Mode  string   `json:"mode"`           // "replace" (or empty), "insert", "anchor" (start = from, end = to)
	Drop  *float64 `json:"drop,omitempty"` // mode "anchor" moved in the text: the word that gives its anchor up
}

// Clip is a passage a person marked to play and export on its own; it changes nothing in the transcript.
type Clip struct {
	ID    string   `json:"id"`
	Start float64  `json:"start"`
	End   float64  `json:"end"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

// Annotations is the parsed results/<stem>/_annotations.json.
type Annotations struct {
	Speakers map[string]SpeakerAnn `json:"speakers"`
	Spans    []SpanAnn             `json:"spans"`
	Edits    []EditAnn             `json:"edits"`
	Clips    []Clip                `json:"clips"`
}

func ResultDir(dataDir, stem string) string { return filepath.Join(dataDir, "results", stem) }

// LoadAnnotations reads the annotations of a stem; a missing file means none.
func LoadAnnotations(dataDir, stem string) (*Annotations, error) {
	ann := &Annotations{}
	b, err := os.ReadFile(filepath.Join(ResultDir(dataDir, stem), "_annotations.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(b, ann); err != nil {
			return nil, err
		}
	}
	if ann.Speakers == nil {
		ann.Speakers = map[string]SpeakerAnn{}
	}
	return ann, nil
}

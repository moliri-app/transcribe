// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package model holds the JSON shapes shared by every part of the app. They are byte-compatible with the
// files the Python version writes (results/<stem>/_machine.json, _final.json, _annotations.json ...),
// so an existing results/ folder keeps working.
package model

import "encoding/json"

type Word struct {
	W       string  `json:"w"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	P       float64 `json:"p"`
	Speaker *int    `json:"speaker,omitempty"`
	Fill    bool    `json:"fill,omitempty"`
	Edited  bool    `json:"edited,omitempty"`
	Redone  bool    `json:"redone,omitempty"`
	Lid     string  `json:"lid,omitempty"`
	Lang    string  `json:"lang,omitempty"` // forced language of a re-transcribed word
	Kept    bool    `json:"kept,omitempty"` // re-transcription fell short: this is the machine's original word
}

type Segment struct {
	Start   float64         `json:"start"`
	End     float64         `json:"end"`
	Speaker *int            `json:"speaker,omitempty"`
	Text    string          `json:"text"`
	Words   []Word          `json:"words"`
	Lid     json.RawMessage `json:"lid,omitempty"`
}

type Event struct {
	Label string  `json:"label"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Prob  float64 `json:"prob,omitempty"`
}

type SpeakerInfo struct {
	Name      string `json:"name,omitempty"`
	Language  string `json:"language,omitempty"`
	MergeInto string `json:"merge_into,omitempty"`
}

// Machine is what the recognition pipeline writes (_machine.json).
type Machine struct {
	Audio           string    `json:"audio"`
	Preset          string    `json:"preset"`
	SpeakersGiven   *int      `json:"speakers_given"`
	Quality         string    `json:"quality"`
	Language        string    `json:"language"`
	EngineUsed      string    `json:"engine_used"`
	SpeakersVariant *string   `json:"speakers_variant"`
	EventsVariant   string    `json:"events_variant"`
	Duration        float64   `json:"duration"`
	Segments        []Segment `json:"segments"`
	Events          []Event   `json:"events"`
}

// Final is machine result + human annotations (_final.json), what the editor and the exports read.
type Final struct {
	Machine
	Speakers map[string]SpeakerInfo `json:"speakers,omitempty"`
}

type Span struct {
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Language string  `json:"language,omitempty"`
	Speaker  string  `json:"speaker,omitempty"`
}

type Edit struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
	Mode  string  `json:"mode,omitempty"` // "" = replace, "insert"
}

// Annotations is results/<stem>/_annotations.json.
type Annotations struct {
	Speakers map[string]SpeakerInfo `json:"speakers"`
	Spans    []Span                 `json:"spans"`
	Edits    []Edit                 `json:"edits"`
	Clips    []Clip                 `json:"clips,omitempty"`
}

// Clip is a marked passage with a title and tags, played and exported on its own.
type Clip struct {
	ID    string   `json:"id"`
	Start float64  `json:"start"`
	End   float64  `json:"end"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

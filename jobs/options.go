// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Options of one recognition job.
type Options struct {
	Preset   string `json:"preset"`
	Speakers int    `json:"speakers"` // 0 = auto
	Quality  string `json:"quality"`  // "", "fast", "best"; "" = what the preset says (the desktop app always asks for best)
	Language string `json:"language"` // "auto", a code like fr, or several like de,en
	Threads  int    `json:"threads"`
	Engine   string `json:"engine"` // what recognises in the desktop app: "local" (also "") or "api"
}

// wire is the shape persisted in _queue.json and sent to the UI: speakers and quality are null when unset.
type wire struct {
	Preset   string  `json:"preset"`
	Speakers *int    `json:"speakers"`
	Quality  *string `json:"quality"`
	Language string  `json:"language"`
	Threads  int     `json:"threads"`
	Engine   string  `json:"engine,omitempty"`
}

func (o Options) MarshalJSON() ([]byte, error) {
	w := wire{Preset: o.Preset, Language: o.Language, Threads: o.Threads, Engine: o.Engine}
	if o.Speakers != 0 {
		w.Speakers = &o.Speakers
	}
	if o.Quality != "" {
		w.Quality = &o.Quality
	}
	return json.Marshal(w)
}

func (o *Options) UnmarshalJSON(b []byte) error {
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*o = Options{Preset: w.Preset, Language: w.Language, Threads: w.Threads, Engine: w.Engine}
	if w.Speakers != nil {
		o.Speakers = *w.Speakers
	}
	if w.Quality != nil {
		o.Quality = *w.Quality
	}
	return nil
}

// JobError is a client error: the request is wrong, not the server.
type JobError string

func (e JobError) Error() string { return string(e) }

var Presets = []string{"auto", "interview", "gespraech", "vortrag", "musik"}

var langRe = regexp.MustCompile(`^[a-z]{2,3}$`)

// NormLanguage reads a language choice: "auto" (or nothing), one code like fr, or several codes
// separated by commas or spaces ("de, en"), which it gives back as "de,en" without repeats.
func NormLanguage(s string) (string, error) {
	var codes []string
	for _, c := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ',' || r == ' ' }) {
		if c == "auto" {
			continue
		}
		if !langRe.MatchString(c) {
			return "", JobError("language must be auto or ISO codes like fr or de,en")
		}
		if !slices.Contains(codes, c) {
			codes = append(codes, c)
		}
	}
	if len(codes) == 0 {
		return "auto", nil
	}
	return strings.Join(codes, ","), nil
}

// Languages are the codes of a language choice; nil for auto.
func Languages(language string) []string {
	if language == "" || language == "auto" {
		return nil
	}
	return strings.Split(language, ",")
}

func isEmpty(v any) bool {
	s, ok := v.(string)
	return v == nil || ok && s == ""
}

// toInt reads a JSON number or numeric string the way Python's int() would.
func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, false
		}
		return int(x), true
	case json.Number:
		n, err := strconv.Atoi(x.String())
		return n, err == nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		return n, err == nil
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// CleanOptions lets only values the recognition understands through; the rest is a JobError.
// defaultThreads (0 = 4) applies when threads is missing or empty; an explicit 0 is rejected.
func CleanOptions(in map[string]any, defaultThreads int) (Options, error) {
	var o Options
	if p, _ := in["preset"].(string); p != "" {
		o.Preset = p
	} else if isEmpty(in["preset"]) {
		o.Preset = "auto"
	} else {
		return o, JobError(fmt.Sprintf("unknown preset: %v", in["preset"]))
	}
	known := false
	for _, p := range Presets {
		known = known || p == o.Preset
	}
	if !known {
		return o, JobError("unknown preset: " + o.Preset)
	}

	if sp := in["speakers"]; !isEmpty(sp) && sp != "auto" {
		n, ok := toInt(sp)
		if !ok {
			return o, JobError("speakers must be a number")
		}
		if n < 1 || n > 20 {
			return o, JobError("speakers must be between 1 and 20")
		}
		o.Speakers = n
	}

	if !isEmpty(in["quality"]) {
		q, _ := in["quality"].(string)
		if q != "fast" && q != "best" {
			return o, JobError("quality must be fast or best")
		}
		o.Quality = q
	}

	o.Language = "auto"
	if !isEmpty(in["language"]) {
		l, ok := in["language"].(string)
		if !ok {
			return o, JobError("language must be auto or ISO codes like fr or de,en")
		}
		var err error
		if o.Language, err = NormLanguage(l); err != nil {
			return o, err
		}
	}

	if !isEmpty(in["engine"]) {
		e, _ := in["engine"].(string)
		if e != "local" && e != "api" {
			return o, JobError("engine must be local or api")
		}
		o.Engine = e
	}

	if defaultThreads <= 0 {
		defaultThreads = 4
	}
	o.Threads = defaultThreads
	if !isEmpty(in["threads"]) {
		n, ok := toInt(in["threads"])
		if !ok {
			return o, JobError("threads must be a number")
		}
		if n < 1 || n > 64 {
			return o, JobError("threads must be between 1 and 64")
		}
		o.Threads = n
	}
	return o, nil
}

// DecodeOptions parses a JSON request body into the loose map CleanOptions takes.
func DecodeOptions(body []byte) (map[string]any, error) {
	m := map[string]any{}
	if len(bytes.TrimSpace(body)) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, JobError("invalid JSON: " + err.Error())
	}
	return m, nil
}

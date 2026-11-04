// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package jobs

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLanguages(t *testing.T) {
	for in, want := range map[string]string{"": "auto", "auto": "auto", "fr": "fr", "De, en": "de,en", "de en de": "de,en", "auto,en": "en"} {
		if o, err := CleanOptions(map[string]any{"language": in}, 0); err != nil || o.Language != want {
			t.Errorf("%q: %q %v, want %q", in, o.Language, err, want)
		}
	}
	if l := Languages("de,en"); len(l) != 2 || l[0] != "de" || l[1] != "en" || Languages("auto") != nil {
		t.Errorf("Languages: %v", l)
	}
}

func TestDefaultsAndValidation(t *testing.T) {
	o, err := CleanOptions(map[string]any{}, 0)
	if err != nil || o.Preset != "auto" || o.Threads != 4 || o.Language != "auto" || o.Speakers != 0 {
		t.Fatalf("defaults: %+v %v", o, err)
	}
	if o, _ = CleanOptions(map[string]any{"speakers": "2"}, 0); o.Speakers != 2 {
		t.Errorf("speakers: %+v", o)
	}
	if o, _ = CleanOptions(map[string]any{"threads": "", "quality": "best"}, 6); o.Threads != 6 || o.Quality != "best" {
		t.Errorf("threads: %+v", o)
	}
	bad := []map[string]any{
		{"preset": "nope"}, {"speakers": 99}, {"speakers": 0}, {"language": "../x"},
		{"threads": 0}, {"threads": 65}, {"quality": "x"}, {"speakers": "abc"},
		{"language": "de,x"}, {"language": []any{"de"}},
	}
	for _, b := range bad {
		if _, err := CleanOptions(b, 0); err == nil {
			t.Errorf("accepted %v", b)
		} else if _, ok := err.(JobError); !ok {
			t.Errorf("%v: not a JobError", b)
		}
	}
}

func TestOptionsJSONShape(t *testing.T) {
	b, _ := json.Marshal(Options{Preset: "auto", Language: "auto", Threads: 4})
	if !strings.Contains(string(b), `"speakers":null`) || !strings.Contains(string(b), `"quality":null`) {
		t.Errorf("auto shape: %s", b)
	}
	var o Options
	if err := json.Unmarshal([]byte(`{"preset":"musik","speakers":3,"quality":"fast","language":"fr","threads":2}`), &o); err != nil ||
		o != (Options{"musik", 3, "fast", "fr", 2, ""}) {
		t.Errorf("unmarshal: %+v %v", o, err)
	}
}

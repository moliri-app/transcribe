// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package msg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestNewFillsTheGermanTextAndKeepsTheArguments(t *testing.T) {
	inner := New("inner", "innen")
	m := New("outer", "Fehler {code}: {why} ({seconds} s, {cause})", "code", 3, "why", inner, "seconds", 0.4, "cause", errors.New("eof"))
	if got, want := m.Error(), "Fehler 3: innen (0,4 s, eof)"; got != want {
		t.Errorf("German text %q, want %q", got, want)
	}
	b, _ := json.Marshal(m)
	if got, want := string(b), `{"key":"outer","args":{"cause":"eof","code":3,"seconds":0.4,"why":{"key":"inner"}}}`; got != want {
		t.Errorf("JSON %s, want %s", got, want)
	}
	if !errors.Is(m, inner) {
		t.Error("the error among the arguments is not unwrapped")
	}
}

type carried struct{}

func (carried) Error() string { return "getragen" }
func (carried) Msg() *M       { return New("carried", "getragen") }

func TestOf(t *testing.T) {
	m := New("k", "Text")
	for _, c := range []struct {
		err  error
		want string
	}{
		{nil, ""},
		{errors.New("plain"), ""},
		{m, "k"},
		{fmt.Errorf("%w", m), "k"},           // wrapped without text of its own
		{fmt.Errorf("more: %w", m), ""},      // the interface would lose "more"
		{carried{}, "carried"},               // an error type that says itself as a message
		{fmt.Errorf("x: %w", carried{}), ""}, // ditto
	} {
		got := ""
		if o := Of(c.err); o != nil {
			got = o.Key
		}
		if got != c.want {
			t.Errorf("Of(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

// Every message the Go side makes needs its text in the frontend, else the interface shows the
// German text in every language.
func TestFrontendKnowsEveryMessage(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	frontend := read("frontend/src/lib/meldungen.js") + read("frontend/src/lib/live/stand.svelte.js")

	patterns := []struct {
		re     *regexp.Regexp
		prefix string
	}{
		{regexp.MustCompile(`\bNew\("([A-Za-z]+\.[A-Za-z.]+)"`), ""},
		{regexp.MustCompile(`\bNewFact\("([A-Za-z]+)"`), "fact."},
		{regexp.MustCompile(`\bsay\("([A-Za-z]+)"`), "ocrApi."}, // internal/ocrapi: ServiceError.Msg
	}
	keys := map[string]string{}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "frontend" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, pat := range patterns {
			for _, m := range pat.re.FindAllStringSubmatch(string(b), -1) {
				keys[pat.prefix+m[1]] = p
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) < 50 {
		t.Fatalf("found only %d messages; does the search still match how they are made?", len(keys))
	}
	for key, file := range keys {
		if !strings.Contains(frontend, "'"+key+"'") {
			t.Errorf("%s: message %q has no text in frontend/src/lib/meldungen.js", file, key)
		}
	}
}

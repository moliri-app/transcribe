// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

// fake is an engine for the tests: lines without word boxes, as an API would give them.
type fake struct {
	usable bool
	result *Result
}

func (f *fake) Info() Info {
	return Info{ID: "fake", Name: "Fake", Usable: f.usable, Ready: f.usable, Languages: []string{"deu"}, Layouts: []string{"auto"}, Boxes: "lines"}
}

func (f *fake) Clean(o Options) (Options, error) { return Clean(o, []string{"deu"}) }

func (f *fake) Read(context.Context, string, Options, int) (*Result, error) {
	r := *f.result
	return &r, nil
}

func conf(p float64) *float64 { return &p }

func lineOnly() *Result {
	return &Result{Engine: "fake-1", Pages: []Page{{Width: 100, Height: 50, Lines: []Line{
		{BBox: Box{1, 2, 30, 10}, Text: "Zinnteller", P: conf(0.8)},
		{Text: "um 1800", Words: []Word{{W: "um", P: conf(0.9)}, {W: "1800", P: conf(0.4)}}},
		{BBox: Box{1, 30, 20, 10}, Para: 1, Text: "Zinn", Words: []Word{{W: "Zinn", BBox: Box{1, 30, 20, 10}}}},
	}}}}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry("desktop", &fake{usable: true, result: lineOnly()})
	e, o, err := r.Prepare("", Options{Layout: "Lines"})
	if err != nil || o.Languages != "deu" || o.Layout != "lines" {
		t.Fatalf("prepare: %v %+v", err, o)
	}
	res, err := r.Read(context.Background(), e, "x.jpg", o, 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != Format || res.Engine != "desktop:fake-1" {
		t.Errorf("stamp: %d %q", res.Format, res.Engine)
	}
	if _, _, err := r.Prepare("onnx", Options{}); !errors.Is(err, ErrUnknownEngine) || !strings.Contains(err.Error(), "engines: fake") {
		t.Errorf("unknown engine: %v", err)
	}
	if _, _, err := r.Prepare("FAKE", Options{Languages: "lat"}); !errors.Is(err, ErrOptions) {
		t.Errorf("bad language: %v", err)
	}
	off := NewRegistry("desktop", &fake{result: lineOnly()})
	if _, _, err := off.Prepare("fake", Options{}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("unusable engine: %v", err)
	}
	if _, _, err := off.Prepare("", Options{}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("no usable engine: %v", err)
	}
}

// paid is an engine read only by choice, like the AI service.
type paid struct{ fake }

func (p *paid) Info() Info {
	i := p.fake.Info()
	i.ID = "api"
	return i
}

func (p *paid) ByChoice() bool { return true }

// The default of a request without engine: the one of the settings when usable, else the first
// usable one that is not read by choice only. The list names the default first.
func TestRegistryDefault(t *testing.T) {
	api := &paid{fake{usable: true, result: lineOnly()}}
	local := &fake{usable: true, result: lineOnly()}
	r := NewRegistry("desktop", api, local)
	ids := func() string {
		var s []string
		for _, i := range r.Engines() {
			s = append(s, i.ID)
		}
		return strings.Join(s, " ")
	}
	if e, _ := r.Get(""); e != Engine(local) || r.DefaultEngine() != "fake" || ids() != "api fake" {
		t.Errorf("without a setting: %v %q %q", e, r.DefaultEngine(), ids())
	}
	def := "API"
	r.Default = func() string { return def }
	if e, _ := r.Get(""); e != Engine(api) || r.DefaultEngine() != "api" || ids() != "api fake" {
		t.Errorf("api chosen: %v %q", e, ids())
	}
	def = "fake"
	if e, _ := r.Get(""); e != Engine(local) || ids() != "fake api" {
		t.Errorf("fake chosen: %v %q", e, ids())
	}
	api.usable, def = false, "api"
	if e, _ := r.Get(""); e != Engine(local) {
		t.Errorf("chosen but not usable: %v", e)
	}
	local.usable, api.usable, def = false, true, ""
	if _, err := r.Get(""); !errors.Is(err, ErrUnavailable) || r.DefaultEngine() != "" {
		t.Errorf("only the paid one usable: %v", err)
	}
	if e, _ := r.Get("api"); e != Engine(api) {
		t.Error("by name")
	}
}

func TestRegistryRefusesBrokenResult(t *testing.T) {
	bad := lineOnly()
	bad.Pages[0].Lines[1].Words[0].P = conf(93) // a percentage instead of 0–1
	r := NewRegistry("server", &fake{usable: true, result: bad})
	e, o, _ := r.Prepare("fake", Options{})
	if _, err := r.Read(context.Background(), e, "x", o, 1); err == nil || !strings.Contains(err.Error(), "p 93") {
		t.Errorf("want the broken word named, got %v", err)
	}
}

// TestFormatJSON pins the JSON of format 1: boxes left out where an engine has none, the line's
// confidence only where given, a word's confidence left out where the engine has none.
func TestFormatJSON(t *testing.T) {
	res := lineOnly()
	res.Format = Format
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"format":1,"engine":"fake-1","pages":[{"width":100,"height":50,"lines":[` +
		`{"bbox":[1,2,30,10],"para":0,"text":"Zinnteller","p":0.8},` +
		`{"para":0,"text":"um 1800","words":[{"w":"um","p":0.9},{"w":"1800","p":0.4}]},` +
		`{"bbox":[1,30,20,10],"para":1,"text":"Zinn","words":[{"w":"Zinn","bbox":[1,30,20,10]}]}]}]}`
	if string(b) != want {
		t.Errorf("json:\n%s\nwant\n%s", b, want)
	}
	var back Result
	if err := json.Unmarshal(b, &back); err != nil || back.Check() != nil || back.Pages[0].Lines[1].BBox != (Box{}) {
		t.Errorf("round trip: %v %v", err, back.Check())
	}
}

func TestCheck(t *testing.T) {
	for name, broken := range map[string]func(*Result){
		"format":     func(r *Result) { r.Format = 2 },
		"engine":     func(r *Result) { r.Engine = "" },
		"pages":      func(r *Result) { r.Pages = nil },
		"page size":  func(r *Result) { r.Pages[0].Width = 0 },
		"para":       func(r *Result) { r.Pages[0].Lines[0].Para = -1 },
		"line p":     func(r *Result) { p := 1.5; r.Pages[0].Lines[0].P = &p },
		"line box":   func(r *Result) { r.Pages[0].Lines[0].BBox = Box{-1, 0, 5, 5} },
		"empty word": func(r *Result) { r.Pages[0].Lines[1].Words[0].W = "" },
		"word p":     func(r *Result) { r.Pages[0].Lines[1].Words[0].P = conf(-0.1) },
	} {
		r := lineOnly()
		r.Format = Format
		broken(r)
		if r.Check() == nil {
			t.Errorf("%s: broken result passed", name)
		}
	}
	r := lineOnly()
	r.Format = Format
	if err := r.Check(); err != nil {
		t.Errorf("good result: %v", err)
	}
}

// TestSchema checks that the schema says what the Go types write: the required keys and format 1.
func TestSchema(t *testing.T) {
	var s struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
		Defs       map[string]struct {
			Required []string `json:"required"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(Schema, &s); err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.Required, ",") != "format,engine,pages" || string(s.Properties["format"]) != `{ "const": 1 }` {
		t.Errorf("top: %v %s", s.Required, s.Properties["format"])
	}
	for def, want := range map[string]string{"page": "width,height,lines", "line": "text,para", "word": "w"} {
		if got := strings.Join(s.Defs[def].Required, ","); got != want {
			t.Errorf("%s requires %s, want %s", def, got, want)
		}
	}
}

func TestScore(t *testing.T) {
	cer, found := Score("Die Brüder Grimm-\nsammelten Mär-\nchen", "Die Brüder Grimmsammelten Märchen")
	if cer != 0 || found != 1 {
		t.Errorf("hyphens: %v %v", cer, found)
	}
	cer, found = Score("ab cd", "ab cx")
	if math.Abs(cer-0.2) > 1e-9 || found != 0.5 {
		t.Errorf("one error: %v %v", cer, found)
	}
	if cer, _ := Score("daß „es“", `das "es"`); math.Abs(cer-1.0/8) > 1e-9 {
		t.Errorf("quotes and ſ: %v", cer)
	}
}

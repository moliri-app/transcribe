// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package osocr

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/ocr"
)

type fakeSystem struct {
	tags []string
	page *page
	read string // the tag read with
}

func (f *fakeSystem) Name() string                 { return "Fake-OCR" }
func (f *fakeSystem) Version() string              { return "10.0.1" }
func (f *fakeSystem) Languages() ([]string, error) { return f.tags, nil }
func (f *fakeSystem) Read(_ context.Context, _, tag string) (*page, error) {
	f.read = tag
	return f.page, nil
}

func TestCode(t *testing.T) {
	for tag, want := range map[string]string{
		"de-DE": "deu", "de": "deu", "en-GB": "eng", "zh-Hans-CN": "chi_sim", "zh-Hant": "chi_tra",
		"sr-Latn-RS": "srp_latn", "sr-Cyrl": "srp", "nb-NO": "nor", "xx-YY": "xx-YY",
	} {
		if got := code(tag); got != want {
			t.Errorf("%s: %s, want %s", tag, got, want)
		}
	}
}

func TestClean(t *testing.T) {
	e := &Engine{sys: &fakeSystem{tags: []string{"en-US", "de-DE", "en-GB", "xx-YY"}}}
	info := e.Info()
	if strings.Join(info.Languages, ",") != "deu,eng,xx-YY" || !info.Usable || !info.Ready || info.Version != "10.0.1" {
		t.Errorf("info: %+v", info)
	}
	for asked, want := range map[[2]string]ocr.Options{
		{"", ""}:                  {Languages: "deu", Layout: "auto"},
		{"deu+frak2021", "auto"}:  {Languages: "deu", Layout: "auto"},
		{"frak2021+ENG", "lines"}: {Languages: "eng", Layout: "lines"},
		{"de-DE", "block"}:        {Languages: "deu", Layout: "lines"},
		{"en", "sparse"}:          {Languages: "eng", Layout: "auto"},
		{"xx-yy", ""}:             {Languages: "xx-YY", Layout: "auto"},
	} {
		got, err := e.Clean(ocr.Options{Languages: asked[0], Layout: asked[1]})
		if err != nil || got != want {
			t.Errorf("%v: %+v %v, want %+v", asked, got, err, want)
		}
	}
	for _, bad := range []ocr.Options{{Languages: "frak2021"}, {Languages: "fra"}, {Layout: "columns"}} {
		if _, err := e.Clean(bad); !errors.Is(err, ocr.ErrOptions) {
			t.Errorf("%+v: %v", bad, err)
		}
	}

	none := &Engine{sys: &fakeSystem{}}
	if info := none.Info(); info.Usable || info.Error == "" {
		t.Errorf("no languages: %+v", info)
	}
	reg := ocr.NewRegistry("desktop", none)
	if _, _, err := reg.Prepare("os", ocr.Options{}); !errors.Is(err, ocr.ErrUnavailable) {
		t.Errorf("no languages: %v", err)
	}
}

// card is an index card as Windows reads it: the labels as one column, then the values.
func card() *page {
	return &page{Width: 1000, Height: 600, Scale: 1, Lines: []rawLine{
		{Text: "INVENTARKARTE", Words: []rawWord{{"INVENTARKARTE", 100, 40, 300, 40}}},
		{Text: "Inv.-Nr.", Words: []rawWord{{"Inv.-Nr.", 100, 150, 80, 20}}},
		{Text: "Gegenstand", Words: []rawWord{{"Gegenstand", 100, 200, 110, 20}}},
		{Text: "1987/214", Words: []rawWord{{"1987/214", 300, 148, 140, 28}}},
		{Text: "Zinnteller mit", Words: []rawWord{{"Zinnteller", 300, 198, 180, 28}, {"mit", 490, 198, 50, 28}}},
		{Text: "Leichte Dellen", Words: []rawWord{{"Leichte", 300, 420, 120, 28}, {"Dellen", 430, 420, 100, 28}}},
		{Text: "", Words: []rawWord{{" ", 0, 0, 0, 0}}},
	}}
}

func TestReadRows(t *testing.T) {
	sys := &fakeSystem{tags: []string{"de-DE"}, page: card()}
	reg := ocr.NewRegistry("desktop", &Engine{sys: sys})
	e, o, err := reg.Prepare("os", ocr.Options{Languages: "deu", Layout: "lines"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := reg.Read(context.Background(), e, "karte.jpg", o, 4)
	if err != nil {
		t.Fatal(err)
	}
	if sys.read != "de-DE" || res.Engine != "desktop:windows-ocr-10.0.1" || res.Language != "deu" || res.Layout != "lines" {
		t.Errorf("result: %q %q %q %q", sys.read, res.Engine, res.Language, res.Layout)
	}
	if got := res.Text(); got != "INVENTARKARTE\nInv.-Nr. 1987/214\nGegenstand Zinnteller mit\nLeichte Dellen\n" {
		t.Errorf("text: %q", got)
	}
	lines := res.Pages[0].Lines
	if lines[1].BBox != (ocr.Box{100, 148, 340, 28}) || len(lines[2].Words) != 3 || lines[2].Words[2].BBox != (ocr.Box{490, 198, 50, 28}) {
		t.Errorf("joined row: %+v / %+v", lines[1], lines[2])
	}
	if lines[0].Para != 0 || lines[1].Para != 1 || lines[2].Para != 1 || lines[3].Para != 2 {
		t.Errorf("paragraphs: %d %d %d %d", lines[0].Para, lines[1].Para, lines[2].Para, lines[3].Para)
	}
	if w := lines[1].Words[0]; w.P != nil {
		t.Errorf("Windows gives no confidence: %+v", w)
	}

	o.Layout = "auto"
	res, _ = reg.Read(context.Background(), e, "karte.jpg", o, 4)
	if n := len(res.Pages[0].Lines); n != 6 || res.Pages[0].Lines[1].Text != "Inv.-Nr." {
		t.Errorf("auto keeps Windows' order: %d lines", n)
	}
}

// TestBoxes turns boxes of a shrunk, skewed reading back into pixels of the image.
func TestBoxes(t *testing.T) {
	// read at half size: boxes double
	pg := result(&page{Width: 2000, Height: 1000, Scale: 0.5, Lines: []rawLine{
		{Text: "a", Words: []rawWord{{"a", 10, 20, 30, 40}}},
	}}, false)
	if b := pg.Lines[0].Words[0].BBox; b != (ocr.Box{20, 40, 60, 80}) {
		t.Errorf("unshrunk: %v", b)
	}
	// text turned 90° clockwise around the centre (50, 50): the deskewed box at the top left
	// stands at the top right of the image
	pg = result(&page{Width: 100, Height: 100, Scale: 1, Angle: 90, Lines: []rawLine{
		{Text: "a", Words: []rawWord{{"a", 10, 10, 20, 10}}},
	}}, false)
	if b := pg.Lines[0].Words[0].BBox; b != (ocr.Box{80, 10, 10, 20}) {
		t.Errorf("turned back: %v", b)
	}
	// boxes stay within the image
	pg = result(&page{Width: 100, Height: 100, Scale: 1, Lines: []rawLine{
		{Text: "a", Words: []rawWord{{"a", -5, 90, 20, 20}}},
	}}, false)
	if b := pg.Lines[0].Words[0].BBox; b != (ocr.Box{0, 90, 15, 10}) {
		t.Errorf("clamped: %v", b)
	}
}

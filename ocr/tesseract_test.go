// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// tsv is Tesseract 5.3 output for a museum label (shortened), with an empty word and a line without words.
const tsv = "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
	"1\t1\t0\t0\t0\t0\t0\t0\t1300\t780\t-1\t\n" +
	"2\t1\t1\t0\t0\t0\t227\t206\t475\t303\t-1\t\n" +
	"3\t1\t1\t1\t0\t0\t227\t206\t286\t61\t-1\t\n" +
	"4\t1\t1\t1\t1\t0\t227\t206\t286\t61\t-1\t\n" +
	"5\t1\t1\t1\t1\t1\t227\t206\t286\t61\t92.217575\tZinnteller\n" +
	"3\t1\t1\t2\t0\t0\t229\t290\t194\t39\t-1\t\n" +
	"4\t1\t1\t2\t1\t0\t229\t290\t194\t39\t-1\t\n" +
	"5\t1\t1\t2\t1\t1\t229\t302\t108\t27\t92.591248\tPewter\n" +
	"5\t1\t1\t2\t1\t2\t346\t290\t77\t35\t96.98494\tplate\n" +
	"5\t1\t1\t2\t1\t3\t430\t290\t5\t35\t95\t \n" +
	"4\t1\t1\t2\t2\t0\t229\t329\t285\t49\t-1\t\n" +
	"5\t1\t1\t2\t2\t1\t229\t350\t143\t28\t95.918945\tNürnberg,\n" +
	"5\t1\t1\t2\t2\t2\t386\t343\t43\t20\t-3\tum\n" +
	"3\t1\t2\t1\t0\t0\t10\t700\t50\t20\t-1\t\n" +
	"4\t1\t2\t1\t1\t0\t10\t700\t50\t20\t-1\t\n" +
	"5\t1\t2\t1\t1\t1\t10\t700\t50\t20\t0\t\n"

func TestParseTSV(t *testing.T) {
	pages, err := ParseTSV([]byte(strings.ReplaceAll(tsv, "\n", "\r\n")))
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || pages[0].Width != 1300 || pages[0].Height != 780 {
		t.Fatalf("pages: %+v", pages)
	}
	lines := pages[0].Lines
	if len(lines) != 3 {
		t.Fatalf("want 3 lines (the one without words left out), got %+v", lines)
	}
	if lines[0].Text != "Zinnteller" || lines[1].Text != "Pewter plate" || lines[2].Text != "Nürnberg, um" {
		t.Errorf("texts: %q %q %q", lines[0].Text, lines[1].Text, lines[2].Text)
	}
	if lines[0].Para != 0 || lines[1].Para != 1 || lines[2].Para != 1 {
		t.Errorf("paragraphs: %d %d %d", lines[0].Para, lines[1].Para, lines[2].Para)
	}
	if lines[1].BBox != (Box{229, 290, 194, 39}) {
		t.Errorf("line box: %v", lines[1].BBox)
	}
	w := lines[1].Words[1]
	if w.W != "plate" || w.BBox != (Box{346, 290, 77, 35}) || w.P == nil || *w.P != 0.97 {
		t.Errorf("word: %+v", w)
	}
	if p := lines[2].Words[1].P; p == nil || *p != 0 {
		t.Errorf("negative confidence must clamp to 0, got %v", p)
	}
	b, _ := json.Marshal(Result{Engine: "tesseract-5.3.0", Language: "deu", Layout: "auto", Pages: pages})
	if !strings.Contains(string(b), `"words":[{"w":"Zinnteller","bbox":[227,206,286,61],"p":0.922}]`) {
		t.Errorf("json: %s", b)
	}
}

func TestParseTSVErrors(t *testing.T) {
	if _, err := ParseTSV([]byte("level\n1\t2\n")); err == nil {
		t.Error("short row must fail")
	}
	if pages, err := ParseTSV(nil); err != nil || len(pages) != 0 {
		t.Errorf("empty: %v %v", pages, err)
	}
}

func TestParseLanguages(t *testing.T) {
	got := parseLanguages("List of available languages in \"/usr/share/tesseract-ocr/5/tessdata/\" (5):\nFraktur\ndeu\neng\nfrk\nosd\n")
	if strings.Join(got, ",") != "Fraktur,deu,eng,frk" {
		t.Errorf("got %v", got)
	}
}

func TestClean(t *testing.T) {
	installed := []string{"Fraktur", "deu", "eng", "frk"}
	o, err := Clean(Options{}, installed)
	if err != nil || o.Languages != "deu" || o.Layout != "auto" {
		t.Errorf("defaults: %+v %v", o, err)
	}
	o, err = Clean(Options{Languages: "DEU+fraktur+deu", Layout: "Lines"}, installed)
	if err != nil || o.Languages != "deu+Fraktur" || o.Layout != "lines" {
		t.Errorf("case and duplicates: %+v %v", o, err)
	}
	if _, err = Clean(Options{Languages: "deu+lat"}, installed); !errors.Is(err, ErrOptions) || !strings.Contains(err.Error(), "installed: Fraktur, deu, eng, frk") {
		t.Errorf("unknown language: %v", err)
	}
	if _, err = Clean(Options{Languages: "deu+"}, installed); err == nil {
		t.Error("empty part must fail")
	}
	if _, err = Clean(Options{Layout: "spalten"}, installed); !errors.Is(err, ErrOptions) {
		t.Error("unknown layout must fail")
	}
}

// TestTesseract runs the real program on OCR_IMAGE (e.g. testdata/etikett.jpg) when tesseract is installed.
func TestTesseract(t *testing.T) {
	img := os.Getenv("OCR_IMAGE")
	if img == "" {
		t.Skip("OCR_IMAGE not set")
	}
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract not installed")
	}
	e := &Tesseract{}
	o, err := e.Clean(Options{Languages: os.Getenv("OCR_LANGUAGES"), Layout: os.Getenv("OCR_LAYOUT")})
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Read(context.Background(), img, o, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.Engine, "tesseract-5") || res.Words() == 0 {
		t.Errorf("result: %+v", res)
	}
}

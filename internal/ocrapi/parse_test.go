// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocrapi

import (
	"reflect"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/internal/ocr"
)

// The expectations of Moliri's tests/php/ai_ocr.test.php, so both places read a model's answer alike.

var page = Size{Width: 2000, Height: 1000, SentWidth: 1600, SentHeight: 800}

func parse(content string) []ocr.Line {
	return Parse(content, page, "api:m", "deu", "auto").Pages[0].Lines
}

func boxes(lines []ocr.Line) []ocr.Box {
	out := []ocr.Box{}
	for _, l := range lines {
		out = append(out, l.BBox)
	}
	return out
}

func texts(lines []ocr.Line) []string {
	out := []string{}
	for _, l := range lines {
		out = append(out, l.Text)
	}
	return out
}

func paras(lines []ocr.Line) []int {
	out := []int{}
	for _, l := range lines {
		out = append(out, l.Para)
	}
	return out
}

func same(t *testing.T, name string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s:\n  got  %v\n  want %v", name, got, want)
	}
}

func TestPrompt(t *testing.T) {
	p := Prompt("deu+frak2021", "lines")
	same(t, "prompt names the languages once", []any{strings.Contains(p, "German or German (Fraktur)"), strings.Count(p, "German (Fraktur)")}, []any{true, 1})
	same(t, "index cards keep label and value together", strings.Contains(p, "label and its value"), true)
	same(t, "without known language", strings.Contains(Prompt("xyz", "auto"), "probably"), false)
	same(t, "auto without the index card sentence", strings.Contains(Prompt("deu", "auto"), "index card"), false)
	if !strings.Contains(p, "\nAnswer as JSON: {\"lines\": [{\"text\": \"…\", \"box\": [x0, y0, x1, y1], \"para\": 0}]}") ||
		!strings.HasPrefix(p, "Transcribe all text on this scanned page exactly as written") || !strings.Contains(p, "Sütterlin") {
		t.Errorf("prompt: %s", p)
	}
}

func TestParse(t *testing.T) {
	res := Parse(`{"lines":[{"text":"Sehr  geehrter Herr","box":[100,100,600,150],"para":0},{"text":"Pfarrer","box":[100,200,300,250],"para":1}]}`,
		page, "api:gemini-2.5-pro", "deu", "auto")
	pg := res.Pages[0]
	same(t, "format 1 with engine and page size", []any{res.Format, res.Engine, pg.Width, pg.Height}, []any{1, "api:gemini-2.5-pro", 2000, 1000})
	same(t, "thousandths → pixels of the image", pg.Lines[0].BBox, ocr.Box{200, 100, 1000, 50})
	same(t, "text with whitespace folded, words without boxes", []any{pg.Lines[0].Text, pg.Lines[0].Words},
		[]any{"Sehr geehrter Herr", []ocr.Word{{W: "Sehr"}, {W: "geehrter"}, {W: "Herr"}}})
	same(t, "paragraphs kept", paras(pg.Lines), []int{0, 1})

	same(t, "pixels of the image sent (a value above 1000), in a code fence",
		parse("```json\n{\"lines\":[{\"text\":\"a\",\"box\":[100,80,1500,160]}]}\n```")[0].BBox, ocr.Box{125, 100, 1750, 100})

	same(t, "a bare list, strings as lines", texts(parse(`[{"text":"eins","box_2d":[0,0,100,100]},"zwei"]`)), []string{"eins", "zwei"})

	l := parse("Erste Zeile\n\nZweite Zeile\n")
	same(t, "plain text: one line per line, bands over the page", []any{texts(l), l[0].BBox, l[1].BBox},
		[]any{[]string{"Erste Zeile", "Zweite Zeile"}, ocr.Box{100, 0, 1800, 500}, ocr.Box{100, 500, 1800, 500}})
	same(t, "plain text: a paragraph per line", paras(l), []int{0, 1})

	l = parse(`{"lines":[{"text":"a","box":[100,100,500,150]},{"text":"b"},{"text":"c"},{"text":"d","box":[100,400,600,450]}]}`)
	same(t, "missing boxes share the gap between their neighbours", []ocr.Box{l[1].BBox, l[2].BBox}, []ocr.Box{{200, 150, 1000, 125}, {200, 275, 1000, 125}})

	l = parse(`{"lines":[{"text":"a","box":[100,100,500,150]},{"text":"b"}]}`)
	same(t, "a missing box after the last one: one line further down", l[1].BBox, ocr.Box{200, 150, 800, 50})

	l = parse(`{"lines":[{"text":"x","box":[500,500,500,600]},{"text":"  "}]}`)
	same(t, "empty boxes and lines dropped or filled", []any{len(l), l[0].BBox != ocr.Box{}}, []any{1, true})

	same(t, "no text: a page without lines", parse(`{"lines":[]}`), []ocr.Line{})

	bs := string(rune(92))
	same(t, "unicode escaped twice by the model", parse(`{"lines":[{"text":"da` + bs + bs + `u00df er"}]}`)[0].Text, "da"+string(rune(0xDF))+" er")
}

// More than the PHP test holds: what the port must read alike too.
func TestParseMore(t *testing.T) {
	l := parse("Hier die Zeilen:\n```json\n{\"lines\":[{\"text\":\"eins\",\"box\":[0,0,500,100],\"para\":\"2\"}]}\n```\nFertig.")
	same(t, "JSON with prose around it", []any{texts(l), l[0].Para}, []any{[]string{"eins"}, 2})

	l = parse(`{"lines":[{"text":"a","para":3},{"text":"b"},{"text":"c","para":-4}]}`)
	same(t, "paragraphs of the model kept, the others counted, none below 0", paras(l), []int{3, 0, 0})

	l = parse(`{"lines":[{"text":"a","box":["100","100","500","150"]},{"text":"b","box":[1,2,3]}]}`)
	same(t, "numbers as strings; a box of three is none", boxes(l), []ocr.Box{{200, 100, 800, 50}, {200, 150, 800, 50}})

	l = parse(`{"lines":[{"text":"b"},{"text":"a","box":[100,500,500,600]}]}`)
	same(t, "a missing box before the first one: one line further up", l[0].BBox, ocr.Box{200, 400, 800, 100})

	l = parse(`{"lines":[{"text":"weit","box":[900,900,1200,1100]}]}`)
	same(t, "a pixel box below the page is none: the line gets a band", l[0].BBox, ocr.Box{100, 0, 1800, 1000})

	same(t, "plain text with a fence of its own", texts(parse("```\nZeile eins\n```")), []string{"Zeile eins"})
	same(t, "an object without lines is plain text", texts(parse(`{"text":"x"}`)), []string{`{"text":"x"}`})
	same(t, "words split on any space", parse(`["a` + "\u00a0" + `b  c"]`)[0].Words, []ocr.Word{{W: "a"}, {W: "b"}, {W: "c"}})

	res := Parse(`{"lines":[{"text":"x"}]}`, page, "api:m", "deu", "auto")
	res.Engine = "desktop:" + res.Engine
	if err := res.Check(); err != nil {
		t.Errorf("format: %v", err)
	}
}

func TestEngineName(t *testing.T) {
	same(t, "engine name", []string{EngineName("gemini 2.5"), EngineName(""), EngineName(" qwen2.5vl:7b ")},
		[]string{"api:gemini-2.5", "api:model", "api:qwen2.5vl:7b"})
	long := EngineName(strings.Repeat("ä", 80))
	if len("desktop:"+long) > 100 || !strings.HasPrefix(long, "api:ää") || strings.ContainsRune(long, '\uFFFD') {
		t.Errorf("long name: %q (%d)", long, len(long))
	}
}

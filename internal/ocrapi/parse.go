// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocrapi

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/moliri-app/transcribe/ocr"
)

// The prompt and the reading of the answer, ported from Moliri's static/api/_func/ai_ocr.php
// (aiOcrPrompt, aiOcrParse, aiOcrDecodeLines, aiOcrUnescape, aiOcrFillBoxes); its test
// tests/php/ai_ocr.test.php and parse_test.go here hold the same expectations. Change both
// together.

// languageNames are the language hints of the prompt, by the Tesseract model names the places share.
var languageNames = map[string]string{
	"deu": "German", "eng": "English", "fra": "French", "ita": "Italian", "nld": "Dutch",
	"spa": "Spanish", "lat": "Latin", "frk": "German (Fraktur)", "frak2021": "German (Fraktur)",
	"fraktur": "German (Fraktur)",
}

// Prompt is the request for one page: languages ("deu+frak2021") and layout as the places share them.
func Prompt(languages, layout string) string {
	var names []string
	for _, code := range strings.Split(strings.ToLower(languages), "+") {
		if name, ok := languageNames[code]; ok && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	p := "Transcribe all text on this scanned page exactly as written: same spelling, umlauts and punctuation. " +
		"Old German scripts (Fraktur, Kurrent, Sütterlin) are written out in modern Latin letters, long s as s. " +
		"Do not correct, complete or translate anything; an illegible word is written as far as it can be read."
	if len(names) > 0 {
		p += " The text is probably " + strings.Join(names, " or ") + "."
	}
	if layout == "lines" {
		p += " It is an index card or a form: a label and its value stay on one line."
	}
	p += "\nAnswer as JSON: {\"lines\": [{\"text\": \"…\", \"box\": [x0, y0, x1, y1], \"para\": 0}]} with one entry per written line of the page, in reading order. " +
		"box is the line's bounding box in thousandths of the image's width and height (0–1000); para counts the paragraphs or blocks from 0. Nothing besides the JSON."
	return p
}

type entry struct {
	text string
	box  []float64 // x0, y0, x1, y1 as the model gave them; nil: none
	para int       // -1: none
}

type line struct {
	text  string
	box   ocr.Box
	boxed bool
	para  int // -1: none
}

var (
	codeFence  = regexp.MustCompile("^```(?:json)?\\s*|\\s*```$")
	fenceLine  = regexp.MustCompile("(?m)^```\\w*\\s*$")
	lineBreak  = regexp.MustCompile("\r\n|[\n\r\v\f\u0085\u2028\u2029]")
	twiceEsc   = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)
	numberLike = regexp.MustCompile(`^\s*[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?\s*$`)
)

// Parse turns the model's answer into a result in format 1. It takes the JSON asked for (also in
// a code fence or with prose around it, or a bare list of lines), else the answer as plain text,
// one line per line. Boxes: thousandths as asked; a model that answers in pixels of the image it
// saw (a value above 1000 somewhere) is taken in those. Lines without a box get one between their
// neighbours; words carry no boxes.
func Parse(content string, size Size, engine, languages, layout string) *ocr.Result {
	width, height := max(1, size.Width), max(1, size.Height)
	sentWidth, sentHeight := size.SentWidth, size.SentHeight
	if sentWidth <= 0 {
		sentWidth = width
	}
	if sentHeight <= 0 {
		sentHeight = height
	}

	var entries []entry
	if decoded, ok := decodeLines(content); ok {
		for _, item := range decoded {
			var fields map[string]any
			switch v := item.(type) {
			case string:
				fields = map[string]any{"text": v}
			case map[string]any:
				fields = v
			default:
				continue
			}
			text := strings.Join(strings.Fields(unescape(phpString(fields["text"]))), " ")
			if text == "" {
				continue
			}
			e := entry{text: text, para: -1}
			if b := firstSet(fields, "box", "bbox", "box_2d"); b != nil {
				e.box = numbers4(b)
			}
			if f, ok := number(fields["para"]); ok {
				e.para = max(0, int(math.Trunc(f)))
			}
			entries = append(entries, e)
		}
	} else {
		plain := fenceLine.ReplaceAllString(content, "")
		for _, l := range lineBreak.Split(plain, -1) {
			if text := strings.Join(strings.Fields(l), " "); text != "" {
				entries = append(entries, entry{text: text, para: -1})
			}
		}
	}

	// thousandths, or pixels of the image sent when a value goes beyond 1000
	top := 0.0
	for _, e := range entries {
		for _, v := range e.box {
			top = max(top, v)
		}
	}
	pixels := top > 1000.5
	fx, fy := float64(width)/1000, float64(height)/1000
	if pixels {
		fx, fy = float64(width)/float64(sentWidth), float64(height)/float64(sentHeight)
	}
	lines := make([]line, 0, len(entries))
	for _, e := range entries {
		l := line{text: e.text, para: e.para}
		if b := e.box; b != nil {
			left := max(0, min(b[0], b[2])*fx)
			up := max(0, min(b[1], b[3])*fy)
			right := min(float64(width), max(b[0], b[2])*fx)
			down := min(float64(height), max(b[1], b[3])*fy)
			if right-left >= 1 && down-up >= 1 {
				l.box = ocr.Box{round(left), round(up), round(right - left), round(down - up)}
				l.boxed = true
			}
		}
		lines = append(lines, l)
	}
	fillBoxes(lines, width, height)

	// without paragraphs from the model every line is one: Moliri's import joins lines that
	// follow closely itself
	out := make([]ocr.Line, 0, len(lines))
	para := 0
	for _, l := range lines {
		p := l.para
		if p < 0 {
			p = para
			para++
		}
		words := []ocr.Word{}
		for _, w := range strings.Fields(l.text) {
			words = append(words, ocr.Word{W: w})
		}
		out = append(out, ocr.Line{BBox: l.box, Para: p, Text: l.text, Words: words})
	}
	return &ocr.Result{Format: ocr.Format, Engine: engine, Language: languages, Layout: layout,
		Pages: []ocr.Page{{Width: width, Height: height, Lines: out}}}
}

// unescape turns Unicode escapes a model wrote twice (the text then reads "da\u00df" after
// decoding the JSON), as some small models do in JSON mode, into the characters.
func unescape(text string) string {
	return twiceEsc.ReplaceAllStringFunc(text, func(m string) string {
		n, err := strconv.ParseUint(m[2:], 16, 32)
		if err != nil || !utf8.ValidRune(rune(n)) {
			return m // a surrogate half stays as it was
		}
		return string(rune(n))
	})
}

// decodeLines is the list of lines in an answer: {"lines": […]}, a bare list; false when it is no
// JSON of that kind.
func decodeLines(content string) ([]any, bool) {
	content = strings.TrimSpace(content)
	content = codeFence.ReplaceAllString(content, "")
	decoded, ok := decodeJSON(content)
	if !ok {
		start := strings.IndexAny(content, "[{")
		end := max(strings.LastIndex(content, "}"), strings.LastIndex(content, "]"))
		if start >= 0 && end > start {
			decoded, ok = decodeJSON(content[start : end+1])
		}
	}
	if !ok {
		return nil, false
	}
	switch v := decoded.(type) {
	case []any:
		return v, true
	case map[string]any:
		if lines, ok := v["lines"].([]any); ok {
			return lines, true
		}
		if len(v) == 0 {
			return []any{}, true
		}
	}
	return nil, false
}

// decodeJSON decodes one JSON array or object with nothing after it; numbers stay as written.
func decodeJSON(s string) (any, bool) {
	d := json.NewDecoder(strings.NewReader(s))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil {
		return nil, false
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return nil, false
	}
	switch v.(type) {
	case []any, map[string]any:
		return v, true
	}
	return nil, false
}

// phpString is a JSON value as text, as PHP's (string) cast gives it.
func phpString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "1"
		}
	}
	return ""
}

// firstSet is the first of the keys present and not null.
func firstSet(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

// number reads a JSON number or a numeric string (PHP's is_numeric).
func number(v any) (float64, bool) {
	var s string
	switch x := v.(type) {
	case json.Number:
		s = x.String()
	case string:
		s = x
	default:
		return 0, false
	}
	if !numberLike.MatchString(s) {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f, err == nil && !math.IsInf(f, 0)
}

// numbers4 is a box of four numbers, or nil.
func numbers4(v any) []float64 {
	list, ok := v.([]any)
	if !ok || len(list) != 4 {
		return nil
	}
	out := make([]float64, 4)
	for i, x := range list {
		f, ok := number(x)
		if !ok {
			return nil
		}
		out[i] = f
	}
	return out
}

func round(f float64) int { return int(math.Round(f)) }

// fillBoxes gives lines without a box one: between the boxed lines above and below, as wide as
// they are; on a page without any box the lines share its height in equal bands.
func fillBoxes(lines []line, width, height int) {
	var boxed []int
	for i, l := range lines {
		if l.boxed {
			boxed = append(boxed, i)
		}
	}
	w, h := float64(width), float64(height)
	if len(boxed) == 0 {
		band := h / float64(max(1, len(lines)))
		for i := range lines {
			lines[i].box = ocr.Box{round(w * 0.05), round(float64(i) * band), max(1, round(w*0.9)), max(1, round(band))}
			lines[i].boxed = true
		}
		return
	}
	for i := range lines {
		if lines[i].boxed {
			continue
		}
		before, after := -1, -1
		for _, j := range boxed {
			if j < i {
				before = j
			}
			if j > i && after < 0 {
				after = j
			}
		}
		var x, y, bw, lineHeight float64
		switch {
		case before >= 0 && after >= 0:
			// the gap between the two, shared by the lines without a box in it
			a, b := lines[before].box, lines[after].box
			missing := float64(after - before - 1)
			top := float64(a[1] + a[3])
			gap := max(0, float64(b[1])-top)
			lineHeight = float64(a[3])
			if gap > 0 {
				lineHeight = gap / missing
			}
			y = top + float64(i-before-1)*lineHeight
			x = float64(min(a[0], b[0]))
			bw = float64(max(a[0]+a[2], b[0]+b[2])) - x
		case before >= 0:
			a := lines[before].box
			lineHeight = float64(a[3])
			y = float64(a[1] + a[3]*(i-before))
			x, bw = float64(a[0]), float64(a[2])
		default:
			b := lines[after].box
			lineHeight = float64(b[3])
			y = float64(b[1] - b[3]*(after-i))
			x, bw = float64(b[0]), float64(b[2])
		}
		y = max(0, min(h-1, y))
		lines[i].box = ocr.Box{round(x), round(y), max(1, round(bw)), max(1, round(min(lineHeight, h-y)))}
	}
}

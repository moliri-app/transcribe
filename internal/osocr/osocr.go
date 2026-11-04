// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package osocr reads text with the operating system's own text recognition, the engine "os":
// Windows.Media.Ocr on Windows, with the languages installed in Windows. It is there at once and
// very fast, but weaker than Tesseract (no Fraktur, no handwriting) and gives no confidence. New
// returns nil where the system has none, so the engine is listed only on Windows (macOS Vision
// may follow).
package osocr

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/moliri-app/transcribe/ocr"
)

// system is the text recognition of one operating system.
type system interface {
	Name() string    // for people: "Windows-Texterkennung"
	Version() string // of the system, e.g. "10.0.26300"
	// Languages are the BCP-47 tags of the installed recognition languages, e.g. "de-DE", "en-US".
	Languages() ([]string, error)
	// Read recognises the image at path in the language tag. Unreadable images wrap ocr.ErrImage.
	Read(ctx context.Context, path, tag string) (*page, error)
}

// page is what the system found on one image, before it becomes format 1.
type page struct {
	Width, Height int     // of the image read
	Scale         float64 // the image was read shrunk by this factor (1 = full size)
	Angle         float64 // clockwise skew of the text in degrees; word boxes are in the deskewed image
	Lines         []rawLine
}

type rawLine struct {
	Text  string
	Words []rawWord
}

type rawWord struct {
	Text       string
	X, Y, W, H float64
}

// Engine is the engine "os".
type Engine struct {
	sys system
}

func (e *Engine) Info() ocr.Info {
	info := ocr.Info{ID: "os", Name: e.sys.Name(), Version: e.sys.Version(), Languages: []string{},
		Layouts: []string{"auto", "lines"}, Boxes: "words"}
	codes, _, err := e.languages()
	info.Languages = append(info.Languages, codes...)
	switch {
	case err != nil:
		info.Error = err.Error()
	case len(codes) == 0:
		info.Error = "In Windows ist keine Sprache für die Texterkennung installiert (Einstellungen › Zeit und Sprache › Sprache)"
	default:
		info.Ready, info.Usable = true, true
	}
	return info
}

// languages gives the installed languages as the codes the engine lists (Tesseract's, e.g. "deu",
// so a caller can ask every engine alike; a tag without one as itself) and the tag of each code.
func (e *Engine) languages() ([]string, map[string]string, error) {
	tags, err := e.sys.Languages()
	if err != nil {
		return nil, nil, err
	}
	var codes []string
	tagOf := map[string]string{}
	for _, tag := range tags {
		c := code(tag)
		if _, ok := tagOf[c]; !ok {
			tagOf[c] = tag
			codes = append(codes, c)
		}
	}
	sort.Strings(codes)
	return codes, tagOf, nil
}

// Clean takes the first of the languages asked for that Windows reads ("deu+frak2021" reads German:
// Windows has no Fraktur, and reads one language at a time); a language is a code of Info
// ("deu") or a tag ("de-DE", "de"). Windows finds the lines itself and reads column by column;
// the layouts that read across (lines, block) become lines, which joins the pieces of a row into
// one line afterwards (the label and value of an index card); auto and sparse keep Windows' order.
func (e *Engine) Clean(o ocr.Options) (ocr.Options, error) {
	codes, tagOf, err := e.languages()
	if err != nil {
		return o, fmt.Errorf("%w: %v", ocr.ErrUnavailable, err)
	}
	langs := strings.TrimSpace(o.Languages)
	if langs == "" {
		langs = "deu"
		if !slices.Contains(codes, langs) && len(codes) > 0 {
			langs = codes[0]
		}
	}
	found := ""
	for _, p := range strings.Split(langs, "+") {
		if found = match(strings.TrimSpace(p), codes, tagOf); found != "" {
			break
		}
	}
	if found == "" {
		return o, fmt.Errorf("%w: Windows reads none of %q (installed: %s)", ocr.ErrOptions, langs, strings.Join(codes, ", "))
	}
	layout := strings.ToLower(strings.TrimSpace(o.Layout))
	if _, ok := ocr.Layouts[layout]; !ok && layout != "" {
		return o, fmt.Errorf("%w: layout must be auto, lines, block or sparse, not %q", ocr.ErrOptions, o.Layout)
	}
	if layout == "lines" || layout == "block" {
		layout = "lines"
	} else {
		layout = "auto"
	}
	return ocr.Options{Languages: found, Layout: layout}, nil
}

// match finds the installed language code for p: a code ("deu", any case), a tag ("de-DE") or a
// language without region ("de").
func match(p string, codes []string, tagOf map[string]string) string {
	if p == "" {
		return ""
	}
	for _, c := range codes {
		if strings.EqualFold(c, p) || strings.EqualFold(tagOf[c], p) {
			return c
		}
	}
	if c := code(p); slices.Contains(codes, c) {
		return c
	}
	return ""
}

func (e *Engine) Read(ctx context.Context, path string, o ocr.Options, _ int) (*ocr.Result, error) {
	_, tagOf, err := e.languages()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ocr.ErrUnavailable, err)
	}
	tag, ok := tagOf[o.Languages]
	if !ok {
		return nil, fmt.Errorf("%w: language %q is not installed", ocr.ErrOptions, o.Languages)
	}
	pg, err := e.sys.Read(ctx, path, tag)
	if err != nil {
		return nil, err
	}
	engine := "windows-ocr"
	if v := e.sys.Version(); v != "" {
		engine += "-" + v
	}
	return &ocr.Result{Engine: engine, Language: o.Languages, Layout: o.Layout, Pages: []ocr.Page{result(pg, o.Layout == "lines")}}, nil
}

// result turns what the system found into a page of format 1: word boxes back in pixels of the
// image read (unshrunk, the skew turned back), line boxes around their words, with rows the lines
// side by side joined into one, paragraphs where the gap between lines is wider than a line or
// the text goes back up (the next column).
func result(pg *page, rows bool) ocr.Page {
	out := ocr.Page{Width: pg.Width, Height: pg.Height, Lines: []ocr.Line{}}
	scale := pg.Scale
	if scale <= 0 {
		scale = 1
	}
	// the image the system read, shrunk; its centre is the centre of the skew
	cx, cy := float64(pg.Width)*scale/2, float64(pg.Height)*scale/2
	sin, cos := math.Sincos(pg.Angle * math.Pi / 180)
	toBox := func(w rawWord) ocr.Box {
		x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, c := range [4][2]float64{{w.X, w.Y}, {w.X + w.W, w.Y}, {w.X, w.Y + w.H}, {w.X + w.W, w.Y + w.H}} {
			// turn the corner clockwise by the skew around the centre, back into the image read
			x := cx + (c[0]-cx)*cos - (c[1]-cy)*sin
			y := cy + (c[0]-cx)*sin + (c[1]-cy)*cos
			x0, y0, x1, y1 = math.Min(x0, x), math.Min(y0, y), math.Max(x1, x), math.Max(y1, y)
		}
		clamp := func(v float64, max int) int {
			return int(math.Max(0, math.Min(float64(max), math.Round(v/scale))))
		}
		l, t, r, b := clamp(x0, pg.Width), clamp(y0, pg.Height), clamp(x1, pg.Width), clamp(y1, pg.Height)
		return ocr.Box{l, t, r - l, b - t}
	}

	var heights []int
	for _, rl := range pg.Lines {
		line := ocr.Line{Text: strings.TrimSpace(rl.Text), Words: []ocr.Word{}}
		first := true
		var x0, y0, x1, y1 int
		for _, rw := range rl.Words {
			text := strings.TrimSpace(rw.Text)
			if text == "" {
				continue
			}
			b := toBox(rw)
			line.Words = append(line.Words, ocr.Word{W: text, BBox: b})
			if first {
				x0, y0, x1, y1 = b[0], b[1], b[0]+b[2], b[1]+b[3]
				first = false
			} else {
				x0, y0, x1, y1 = min(x0, b[0]), min(y0, b[1]), max(x1, b[0]+b[2]), max(y1, b[1]+b[3])
			}
		}
		if line.Text == "" {
			texts := make([]string, len(line.Words))
			for i, w := range line.Words {
				texts[i] = w.W
			}
			line.Text = strings.Join(texts, " ")
		}
		if line.Text == "" {
			continue
		}
		if !first {
			line.BBox = ocr.Box{x0, y0, x1 - x0, y1 - y0}
			heights = append(heights, y1-y0)
		}
		out.Lines = append(out.Lines, line)
	}

	if rows {
		out.Lines = joinRows(out.Lines)
	}
	if len(heights) > 0 {
		sort.Ints(heights)
		h := heights[len(heights)/2]
		para := 0
		for i := 1; i < len(out.Lines); i++ {
			prev, cur := out.Lines[i-1].BBox, out.Lines[i].BBox
			if prev != (ocr.Box{}) && cur != (ocr.Box{}) {
				if gap := cur[1] - (prev[1] + prev[3]); gap > h || cur[1]+cur[3] <= prev[1] {
					para++
				}
			}
			out.Lines[i].Para = para
		}
	}
	return out
}

// codes maps the tags of Windows' recognition languages (their language and script) to
// Tesseract's model names.
var codes = map[string]string{
	"ar": "ara", "bg": "bul", "cs": "ces", "da": "dan", "de": "deu", "el": "ell", "en": "eng",
	"es": "spa", "et": "est", "fi": "fin", "fr": "fra", "hr": "hrv", "hu": "hun", "it": "ita",
	"ja": "jpn", "ko": "kor", "lt": "lit", "lv": "lav", "nb": "nor", "nl": "nld", "no": "nor",
	"pl": "pol", "pt": "por", "ro": "ron", "ru": "rus", "sk": "slk", "sl": "slv", "sv": "swe",
	"tr": "tur", "uk": "ukr",
	"zh-hans": "chi_sim", "zh-cn": "chi_sim", "zh-hant": "chi_tra", "zh-tw": "chi_tra",
	"sr-cyrl": "srp", "sr-latn": "srp_latn", "sr": "srp_latn",
}

// code is Tesseract's name for a language tag ("de-DE" -> "deu", "zh-Hans-CN" -> "chi_sim"); a tag
// without one stays as it is.
func code(tag string) string {
	t := strings.ToLower(tag)
	for t != "" {
		if c, ok := codes[t]; ok {
			return c
		}
		i := strings.LastIndex(t, "-")
		if i < 0 {
			break
		}
		t = t[:i]
	}
	return tag
}

// joinRows joins lines that stand side by side (their vertical middles within each other's
// height) into one line read from left to right, the rows from top to bottom. Lines without a
// box stay where they are, after the rows.
func joinRows(lines []ocr.Line) []ocr.Line {
	var boxed, rest []ocr.Line
	for _, l := range lines {
		if l.BBox == (ocr.Box{}) {
			rest = append(rest, l)
		} else {
			boxed = append(boxed, l)
		}
	}
	mid := func(b ocr.Box) int { return b[1] + b[3]/2 }
	sort.SliceStable(boxed, func(i, j int) bool { return mid(boxed[i].BBox) < mid(boxed[j].BBox) })
	var rows [][]ocr.Line
	for _, l := range boxed {
		if n := len(rows); n > 0 {
			last := rows[n-1][len(rows[n-1])-1].BBox
			if m := mid(l.BBox); (m >= last[1] && m <= last[1]+last[3]) || (mid(last) >= l.BBox[1] && mid(last) <= l.BBox[1]+l.BBox[3]) {
				rows[n-1] = append(rows[n-1], l)
				continue
			}
		}
		rows = append(rows, []ocr.Line{l})
	}
	out := make([]ocr.Line, 0, len(rows)+len(rest))
	for _, row := range rows {
		sort.SliceStable(row, func(i, j int) bool { return row[i].BBox[0] < row[j].BBox[0] })
		joined := row[0]
		joined.Words = slices.Clone(joined.Words)
		for _, l := range row[1:] {
			b, c := joined.BBox, l.BBox
			x0, y0 := min(b[0], c[0]), min(b[1], c[1])
			x1, y1 := max(b[0]+b[2], c[0]+c[2]), max(b[1]+b[3], c[1]+c[3])
			joined.BBox = ocr.Box{x0, y0, x1 - x0, y1 - y0}
			joined.Text += " " + l.Text
			joined.Words = append(joined.Words, l.Words...)
		}
		out = append(out, joined)
	}
	return append(out, rest...)
}

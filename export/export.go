// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package export renders a transcript as HTML (noScribe anchors), plain text or WebVTT, corrections included.
package export

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/moliri-app/transcribe/annotate"
)

const (
	paraGap, paraWords  = 2.0, 80  // same paragraph rules as the editor
	cueMaxS, cueMaxChar = 8.0, 120 // subtitle cues stay readable
)

var (
	sentEnd    = regexp.MustCompile(`[.!?…]["»«“”)]?$`)
	skipEvents = map[string]bool{"Ruf": true, "Schrei": true} // shown in the editor only as marks on the timeline
)

// Final is the part of _final.json an export needs.
type Final struct {
	Audio  string `json:"audio"`
	Events []struct {
		Label string  `json:"label"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
	} `json:"events"`
	Speakers map[string]struct {
		Name string `json:"name"`
	} `json:"speakers"`
	Segments []struct {
		Words []annotate.Word `json:"words"`
	} `json:"segments"`
}

// Format is how to render and how to serve one export format.
type Format struct {
	Render      func(f *Final, ann *annotate.Annotations) string
	ContentType string
}

var Formats = map[string]Format{
	"html": {func(f *Final, a *annotate.Annotations) string { return ToHTML(f, a, "") }, "text/html; charset=utf-8"},
	"txt":  {ToTXT, "text/plain; charset=utf-8"},
	"vtt":  {ToVTT, "text/vtt; charset=utf-8"},
}

type sentence struct {
	start, end float64
	text       string
}

type block struct {
	cue        bool
	start, end float64
	speaker    int // -1 = none
	name       string
	sentences  []sentence
	text       string // cue label
}

// Render returns a transcript in the given format and that format's content type. media overrides the
// audio_source of the HTML export ("" = the file name the transcript was made from).
func Render(format string, f *Final, ann *annotate.Annotations, media string) ([]byte, string, error) {
	fm, ok := Formats[format]
	if !ok {
		return nil, "", fmt.Errorf("unknown format %q", format)
	}
	if ann == nil {
		ann = &annotate.Annotations{Speakers: map[string]annotate.SpeakerAnn{}}
	}
	if format == "html" {
		return []byte(ToHTML(f, ann, media)), fm.ContentType, nil
	}
	return []byte(fm.Render(f, ann)), fm.ContentType, nil
}

func currentWords(f *Final, ann *annotate.Annotations) []annotate.Word {
	var words []annotate.Word
	for _, s := range f.Segments {
		for _, w := range s.Words {
			c := make(annotate.Word, len(w))
			for k, v := range w {
				c[k] = v
			}
			words = append(words, c)
		}
	}
	for _, e := range ann.Edits {
		words = annotate.ApplyEdit(words, e.Start, e.End, e.Text, e.Mode)
	}
	for _, s := range ann.Spans {
		id, ok := s.Speaker.Int()
		if s.Speaker == "" || !ok {
			continue
		}
		for _, w := range words {
			if annotate.Mid(w) >= s.Start && annotate.Mid(w) <= s.End {
				annotate.SetSpeaker(w, id)
			}
		}
	}
	return words
}

func speakerName(f *Final, ann *annotate.Annotations, k int) string {
	key := strconv.Itoa(k)
	if n := ann.Speakers[key].Name; n != "" {
		return n
	}
	if n := f.Speakers[key].Name; n != "" {
		return n
	}
	return fmt.Sprintf("Sprecher %d", k+1)
}

// wordText shows background voices and calls in brackets, like the editor.
func wordText(words []annotate.Word, i int) string {
	w := words[i]
	if !annotate.Fill(w) {
		return annotate.Text(w)
	}
	s := annotate.Text(w)
	if i == 0 || !annotate.Fill(words[i-1]) {
		s = "(" + s
	}
	if i+1 == len(words) || !annotate.Fill(words[i+1]) {
		s += ")"
	}
	return s
}

func sentences(words []annotate.Word) []sentence {
	var out []sentence
	var cur []string
	for i, w := range words {
		cur = append(cur, wordText(words, i))
		if sentEnd.MatchString(annotate.Text(w)) || i+1 == len(words) {
			first := i + 1 - len(cur)
			out = append(out, sentence{annotate.Num(words[first], "start"), annotate.Num(w, "end"), strings.Join(cur, " ")})
			cur = nil
		}
	}
	return out
}

// blocks are the paragraphs in reading order, speech and event cues.
func blocks(f *Final, ann *annotate.Annotations) []block {
	return blocksIn(f, ann, math.Inf(-1), math.Inf(1))
}

// blocksIn are the blocks of the words whose middle lies in [a, b] and of the events inside it;
// speakers are named as in the whole transcript.
func blocksIn(f *Final, ann *annotate.Annotations, a, b float64) []block {
	words := currentWords(f, ann)
	sort.SliceStable(words, func(i, j int) bool { return annotate.Num(words[i], "start") < annotate.Num(words[j], "start") })
	speakers := map[int]bool{}
	for _, w := range words {
		if s := annotate.Speaker(w); s >= 0 {
			speakers[s] = true
		}
	}
	named := len(speakers) > 1
	for k := range speakers {
		if ann.Speakers[strconv.Itoa(k)].Name != "" {
			named = true
		}
	}
	in := words[:0]
	for _, w := range words {
		if m := annotate.Mid(w); a <= m && m <= b {
			in = append(in, w)
		}
	}
	words = in

	var cues []block
	for _, e := range f.Events {
		if skipEvents[e.Label] || (e.Start+e.End)/2 < a || (e.Start+e.End)/2 > b {
			continue
		}
		spoken := 0.0
		for _, w := range words {
			ws, we := annotate.Num(w, "start"), annotate.Num(w, "end")
			if ws < e.End && we > e.Start {
				spoken += math.Max(0, math.Min(we, e.End)-math.Max(ws, e.Start))
			}
		}
		if spoken <= 0.5*(e.End-e.Start) { // under speech it would only be noise in the text
			cues = append(cues, block{cue: true, start: e.Start, end: e.End, text: e.Label, speaker: -1})
		}
	}

	var paras []block
	var groups [][]annotate.Word
	for _, w := range words {
		spk := annotate.Speaker(w)
		isNew := len(groups) == 0
		if !isNew {
			cur := groups[len(groups)-1]
			prev := cur[len(cur)-1]
			isNew = spk != annotate.Speaker(cur[0]) ||
				sentEnd.MatchString(annotate.Text(prev)) && (annotate.Num(w, "start")-annotate.Num(prev, "end") > paraGap || len(cur) >= paraWords)
		}
		if isNew {
			groups = append(groups, nil)
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], w)
	}
	for _, ws := range groups {
		p := block{speaker: annotate.Speaker(ws[0]), start: annotate.Num(ws[0], "start"), end: annotate.Num(ws[len(ws)-1], "end"), sentences: sentences(ws)}
		if named && p.speaker >= 0 {
			p.name = speakerName(f, ann, p.speaker)
		}
		paras = append(paras, p)
	}
	all := append(paras, cues...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].start < all[j].start })
	return all
}

func hms(t float64) string {
	s := int(t)
	return fmt.Sprintf("%02d:%02d:%02d", s/3600, s%3600/60, s%60)
}

func vttTime(t float64) string {
	ms := int64(math.RoundToEven(t * 1000))
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

// titleOf is the audio file name without folder and extension.
func titleOf(f *Final) string {
	name := f.Audio
	if name == "" {
		name = "Transkript"
	}
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func ToTXT(f *Final, ann *annotate.Annotations) string {
	lines := txtLines([]string{titleOf(f), ""}, blocks(f, ann))
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n\v\f") + "\n"
}

// ClipsTXT is the text of the given clips in their order, each under its title, time and tags.
func ClipsTXT(f *Final, ann *annotate.Annotations, clips []annotate.Clip) string {
	lines := []string{titleOf(f), ""}
	for i, c := range clips {
		head := fmt.Sprintf("%d. %s", i+1, c.Title)
		info := "[" + hms(c.Start) + " – " + hms(c.End) + "]"
		if len(c.Tags) > 0 {
			info += " Schlagworte: " + strings.Join(c.Tags, ", ")
		}
		lines = append(lines, head, info, "")
		lines = txtLines(lines, blocksIn(f, ann, c.Start, c.End))
		lines = append(lines, "")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n\v\f") + "\n"
}

func txtLines(lines []string, bs []block) []string {
	for _, b := range bs {
		if b.cue {
			lines = append(lines, "["+b.text+"]", "")
			continue
		}
		label := ""
		if b.name != "" {
			label = b.name + ": "
		}
		texts := make([]string, len(b.sentences))
		for i, s := range b.sentences {
			texts[i] = s.text
		}
		lines = append(lines, label+"["+hms(b.start)+"] "+strings.Join(texts, " "), "")
	}
	return lines
}

// escape is Python's html.escape: & < > and, with quote, " and '.
func escape(s string, quote bool) string {
	pairs := []string{"&", "&amp;", "<", "&lt;", ">", "&gt;"}
	if quote {
		pairs = append(pairs, `"`, "&quot;", "'", "&#x27;")
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

func ToVTT(f *Final, ann *annotate.Annotations) string {
	out := []string{"WEBVTT " + titleOf(f), ""}
	n := 0
	cue := func(start, end float64, text, name string) {
		n++
		voice := ""
		if name != "" {
			voice = "<v " + strings.ReplaceAll(strings.ReplaceAll(name, ">", ""), "&", "&amp;") + ">"
		}
		out = append(out, strconv.Itoa(n), vttTime(start)+" --> "+vttTime(math.Max(end, start+0.05)), voice+escape(text, false), "")
	}
	for _, b := range blocks(f, ann) {
		if b.cue {
			cue(b.start, b.end, "["+b.text+"]", "")
			continue
		}
		var group []sentence
		join := func() string {
			t := make([]string, len(group))
			for i, g := range group {
				t[i] = g.text
			}
			return strings.Join(t, " ")
		}
		for i := 0; i <= len(b.sentences); i++ {
			last := i == len(b.sentences)
			if len(group) > 0 && (last || b.sentences[i].end-group[0].start > cueMaxS ||
				utf8.RuneCountInString(join())+utf8.RuneCountInString(b.sentences[i].text) > cueMaxChar) {
				cue(group[0].start, group[len(group)-1].end, join(), b.name)
				group = nil
			}
			if !last {
				group = append(group, b.sentences[i])
			}
		}
	}
	return strings.Join(out, "\n")
}

// ToHTML uses the anchor format of noScribe (<a name="ts_<start ms>_<end ms>_S01">). media overrides the audio_source.
func ToHTML(f *Final, ann *annotate.Annotations, media string) string {
	title := titleOf(f)
	if media == "" {
		media = f.Audio
	}
	bs := blocks(f, ann)
	codes := map[int]string{}
	for _, b := range bs {
		if !b.cue && b.speaker >= 0 {
			codes[b.speaker] = fmt.Sprintf("S%02d", b.speaker+1)
		}
	}
	ms := func(t float64) int64 { return int64(math.RoundToEven(t * 1000)) }
	var parts []string
	for _, b := range bs {
		if b.cue {
			parts = append(parts, fmt.Sprintf(`<p><a name="ts_%d_%d_">[%s]</a></p>`, ms(b.start), ms(b.end), escape(b.text, true)))
			continue
		}
		var sb strings.Builder
		sb.WriteString("<p>")
		for i, s := range b.sentences {
			text := escape(s.text, true)
			if i == 0 {
				label := ""
				if b.name != "" {
					label = escape(b.name, true) + ": "
				}
				text = label + `<span style="color: #78909C">[` + hms(b.start) + `]</span> ` + text
			} else {
				text = " " + text
			}
			fmt.Fprintf(&sb, `<a name="ts_%d_%d_%s">%s</a>`, ms(s.start), ms(s.end), codes[b.speaker], text)
		}
		sb.WriteString("</p>")
		parts = append(parts, sb.String())
	}
	meta := ""
	if media != "" {
		meta = `<meta name="audio_source" content="./` + escape(media, true) + `" />`
	}
	return "<!DOCTYPE HTML>\n<html>\n<head>\n<meta charset=\"UTF-8\" />\n<title>" + escape(title, true) + "</title>\n" +
		"<style type=\"text/css\">p { font-size: 0.9em; font-family: Arial, sans-serif; }</style>\n" +
		meta + "\n</head>\n<body>\n<div class=\"WordSection1\">\n<p style=\"font-weight: 600\">" + escape(title, true) + "</p>\n" +
		strings.Join(parts, "\n") + "\n</div>\n</body>\n</html>\n"
}

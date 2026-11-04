// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/internal/model"
	"github.com/moliri-app/transcribe/internal/prefs"
)

// „Meine Stimme“ on the Live page: the user's own words, which live transcription should write as
// they are written, and punctuation they say aloud. Both apply to live text and its preview, not to
// transcribing a recording in full.
//
// The words are hotwords of Parakeet (liveRecognizer, recKey): modified beam search favours them,
// which made it write „Obeya“ where it wrote „Ubaya“ greedily, for about 20 % more time per
// decoding and no more errors on the podcast of TestLiveBench. The German model got markedly
// worse with beam search, with hotwords more so (docs/live-models.md); it decodes greedily, and
// its live text is Parakeet's after the recording anyway. What comes out a little off is put
// right afterwards when it sounds nearly the same (fixWords).

// voice is what the user set for their voice; the zero value changes nothing.
type voice struct {
	words    []string // their own words, as they are written
	commands bool     // spoken punctuation is turned into marks
}

func voiceOf(p prefs.Prefs) voice { return voice{words: p.LiveWords, commands: !p.LiveNoCommands} }

// hotwordScore is how much a hotword's tokens are favoured: sherpa-onnx's default. With 2.5 and 4
// the podcast of TestLiveBench came out with more errors, and with words of it turned into
// hotwords („Juri“, „Silero“); with 1 and 1.5 it did not.
const hotwordScore = 1.5

// hotwords are the lines of the recognizer's hotwords for words: each, and with spaces for
// hyphens, as a model may write „sherpa-onnx“ as two words.
func hotwords(words []string) []string {
	var out []string
	for _, w := range words {
		out = append(out, w)
		if s := strings.Join(strings.FieldsFunc(w, func(r rune) bool { return r == '-' || r == ' ' }), " "); s != w {
			out = append(out, s)
		}
	}
	return out
}

// ---- the user's words ----

// fixWords writes what sounds nearly like one of words, in one to three words recognised, as that
// one is written: „Molly Ri“ as „Moliri“, „sherpa on X“ as „sherpa-onnx“. Cautious: a word must
// sound the same but for one sound in five, begin with the same sound and have five sounds at
// least; shorter ones only get their spelling (case, hyphens).
func fixWords(ws []model.Word, words []string) []model.Word {
	if len(words) == 0 || len(ws) == 0 {
		return ws
	}
	keys := make([]string, len(words))
	for i, w := range words {
		keys[i] = soundKey(w)
	}
	var out []model.Word
	for i := 0; i < len(ws); {
		n, w := matchOwn(ws[i:], words, keys)
		if n == 0 {
			out = append(out, ws[i])
			i++
			continue
		}
		last := ws[i+n-1]
		p := ws[i].P
		for _, x := range ws[i : i+n] {
			p = min(p, x.P)
		}
		trail := last.W[len(strings.TrimRight(last.W, live.Marks)):]
		out = append(out, model.Word{W: w + trail, Start: ws[i].Start, End: last.End, P: p})
		i += n
	}
	return out
}

// matchOwn finds the user's word the words at the start of ws sound like, the most words first:
// how many words it takes and the user's word, 0 if none.
func matchOwn(ws []model.Word, words, keys []string) (int, string) {
	for n := min(3, len(ws)); n >= 1; n-- {
		var b strings.Builder
		ok := true
		for j, w := range ws[:n] {
			if j < n-1 && strings.ContainsAny(w.W, live.Marks) {
				ok = false // a sentence or clause ends inside
				break
			}
			b.WriteString(w.W)
		}
		if !ok {
			continue
		}
		heard := soundKey(b.String())
		if heard == "" {
			continue
		}
		for k, key := range keys {
			if heard == key {
				if n == 1 && strings.TrimRight(ws[0].W, live.Marks) == words[k] {
					return 0, "" // written so already
				}
				return n, words[k]
			}
			if closeSound(heard, key) {
				return n, words[k]
			}
		}
	}
	return 0, ""
}

// closeSound: heard sounds like key but for one sound in five (at most three), with the same
// first sound; key has five sounds at least.
func closeSound(heard, key string) bool {
	h, k := []rune(heard), []rune(key)
	if len(k) < 5 || h[0] != k[0] {
		return false
	}
	allowed := min(3, len(k)/5)
	if d := len(h) - len(k); d > allowed || -d > allowed {
		return false
	}
	return editDistanceRunes(h, k) <= allowed
}

// soundKey is roughly how a word sounds, in German or English spelling: lower case, letters that
// sound alike as one, doubled ones once, no h after a vowel, nothing but letters and digits.
func soundKey(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		switch r {
		case 'ä':
			return 'e'
		case 'ö':
			return 'o'
		case 'ü':
			return 'u'
		case 'ß':
			return 's'
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s)
	s = strings.NewReplacer("sch", "s", "sh", "s", "ch", "k", "ck", "k", "ph", "f", "th", "t", "qu", "kv", "tz", "ts", "z", "ts", "x", "ks", "c", "k",
		"ey", "ai", "ay", "ai", "ei", "ai", "eu", "oi", "ie", "i", "y", "i", "w", "v", "v", "f", "dt", "t").Replace(s)
	var b []rune
	for _, r := range s {
		if n := len(b); n > 0 && (b[n-1] == r || r == 'h' && strings.ContainsRune("aeiou", b[n-1])) {
			continue
		}
		b = append(b, r)
	}
	return string(b)
}

func editDistanceRunes(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			c := 1
			if a[i-1] == b[j-1] {
				c = 0
			}
			cur[j] = min(prev[j-1]+c, prev[j]+1, cur[j-1]+1)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// ---- spoken punctuation ----

type command struct {
	words []string // as normWord gives them
	mark  string
}

// commands are what may be said for a mark, in German and English; the longer ones first.
var commands = []command{
	{[]string{"neuer", "absatz"}, "\n\n"}, {[]string{"neuen", "absatz"}, "\n\n"}, {[]string{"new", "paragraph"}, "\n\n"},
	{[]string{"neue", "zeile"}, "\n"}, {[]string{"new", "line"}, "\n"},
	{[]string{"question", "mark"}, "?"}, {[]string{"exclamation", "mark"}, "!"}, {[]string{"exclamation", "point"}, "!"},
	{[]string{"full", "stop"}, "."},
	{[]string{"fragezeichen"}, "?"}, {[]string{"ausrufezeichen"}, "!"}, {[]string{"doppelpunkt"}, ":"},
	{[]string{"komma"}, ","}, {[]string{"koma"}, ","}, {[]string{"punkt"}, "."}, // the German model writes „Koma“ at times
	{[]string{"comma"}, ","}, {[]string{"period"}, "."}, {[]string{"colon"}, ":"},
}

// matchCommand is the command the words at the start of ws say, and how many words it takes.
func matchCommand(ws []model.Word) (command, int) {
	for _, c := range commands {
		if len(c.words) > len(ws) {
			continue
		}
		ok := true
		for j, w := range c.words {
			if normWord(ws[j].W) != w {
				ok = false
				break
			}
		}
		if ok {
			return c, len(c.words)
		}
	}
	return command{}, 0
}

// A command counts only where it is said as one: at the end of an utterance, or before a pause
// (from the start of its first word to the next word, the time it takes to say plus commandPause),
// and not after an article („der springende Punkt“, „a period of“, „eine neue Zeile“). Better
// to miss one than to turn a word that was meant into a mark.
const (
	commandPause = 0.25 // seconds
	perLetter    = 0.07 // seconds it takes to say a letter, roughly
)

// spokenMarks turns the commands in ws, an utterance (ended, unless open), into marks: on the word
// before (the model's mark there goes), or, at its start, as a word of its own for the line
// before; before is what was given out before it, after the mark at its end if it ended in one.
// The word after a mark gets a capital after the end of a sentence or a line, and loses one after
// a comma or colon if it is a little word that only had it for starting a sentence. end is the
// mark ws ends in, if a command set it.
func spokenMarks(ws, before []model.Word, after string, open bool) (out []model.Word, end string) {
	ws, joined := splitCommands(ws)
	mark := after // the last mark set, for the case of the word after it
	for i := 0; i < len(ws); {
		c, n := matchCommand(ws[i:])
		if n == 0 || !commandSaid(ws, i, n, open) || articleBefore(append(slices.Clone(before), out...)) || len(before)+len(out) == 0 {
			w := ws[i]
			if k := len(out) - 1; joined[i] && k >= 0 { // no command: the word stays as it was
				out[k].W += "-" + w.W
				out[k].End = w.End
				i++
				continue
			}
			if mark != "" {
				w.W = caseAfter(mark, w.W)
				mark = ""
			}
			out = append(out, w)
			i++
			continue
		}
		if k := len(out) - 1; k >= 0 {
			out[k].W = live.AddMark(out[k].W, c.mark)
		} else {
			out = append(out, model.Word{W: c.mark, Start: ws[i].Start, End: ws[i+n-1].End, P: ws[i].P})
		}
		mark = c.mark
		i += n
	}
	return out, mark
}

// splitCommands parts a command the model joined to the word before with a hyphen („Obeya-
// Fragezeichen.“) from it; the parts share the word's time. joined marks the parts split off.
func splitCommands(ws []model.Word) (out []model.Word, joined []bool) {
	for _, w := range ws {
		i := strings.LastIndex(w.W, "-")
		tail := w
		tail.W = w.W[max(0, i)+1:]
		if _, n := matchCommand([]model.Word{tail}); i <= 0 || n != 1 {
			out, joined = append(out, w), append(joined, false)
			continue
		}
		head := w
		head.W = w.W[:i]
		head.End = (w.Start + w.End) / 2
		tail.Start = head.End
		out, joined = append(out, head, tail), append(joined, false, true)
	}
	return out, joined
}

// commandSaid: the command of n words at ws[i] ends the utterance or comes before a pause, or
// before another one that does („Punkt, neuer Absatz“).
func commandSaid(ws []model.Word, i, n int, open bool) bool {
	next := i + n
	if next == len(ws) {
		return !open
	}
	if _, m := matchCommand(ws[next:]); m > 0 && commandSaid(ws, next, m, open) {
		return true
	}
	letters := 0
	for _, w := range ws[i:next] {
		letters += utf8.RuneCountInString(normWord(w.W))
	}
	return ws[next].Start-ws[i].Start >= float64(letters)*perLetter+commandPause
}

var articles = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`der die das den dem des ein eine einen einem einer eines kein keine keinen keinem keiner keines
		mein meine meinen meinem meiner meines dein deine deinen deinem deiner deines sein seine seinen seinem seiner seines
		ihr ihre ihren ihrem ihrer ihres unser unsere unseren unserem unserer unseres euer eure euren eurem eurer eures
		dieser diese diesen diesem dieses jener jene jenen jenem jenes jeder jede jeden jedem jedes welcher welche welchen welchem welches
		zum zur am im beim vom ins ans aufs
		a an the this that these those my your his her its our their no every each any some another`) {
		articles[w] = true
	}
	for _, w := range strings.Fields("a an the this that my your his her its our their no every each any some another") {
		englishArticles[w] = true
	}
}

var englishArticles = map[string]bool{}

// articleBefore: the words before end in an article, perhaps with an adjective or two after it in
// German („der springende“) or a word in English („a long“): the command is a noun there.
func articleBefore(before []model.Word) bool {
	ended := func(w string) bool { return strings.ContainsAny(w, live.Marks+"\n") } // a clause ended
	for k, back := len(before)-1, 0; k >= 0 && back < 3; k, back = k-1, back+1 {
		w := before[k].W
		switch {
		case ended(w):
			return false
		case articles[normWord(w)]:
			return true
		case adjectiveLike(w):
			continue
		}
		r, _ := utf8.DecodeRuneInString(w)
		return back == 0 && k > 0 && unicode.IsLower(r) && !ended(before[k-1].W) && englishArticles[normWord(before[k-1].W)]
	}
	return false
}

// adjectiveLike: lower case (a German noun has a capital) and ending as an inflected adjective.
func adjectiveLike(w string) bool {
	r, _ := utf8.DecodeRuneInString(w)
	if !unicode.IsLower(r) {
		return false
	}
	for _, end := range []string{"e", "en", "er", "es", "em"} {
		if strings.HasSuffix(w, end) {
			return true
		}
	}
	return false
}

// little are words that are never nouns: after a spoken comma or colon they are written small.
var little = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`und oder aber denn sondern doch wenn weil dass ob als wie wo was wer warum da dann so also auch noch nur schon nicht
		ich du er es wir man mich dich sich uns euch mir dir ihm ihn
		der die das den dem des ein eine einen einem einer eines kein keine mit von zu für auf in an bei nach aus um über unter vor seit bis ohne gegen durch
		damit obwohl während bevor nachdem sobald falls jedoch sonst hier dort jetzt ja nein bitte vielleicht etwa zum zur am im
		and or but because if when how that which who what where why then also too not you he she it we they me him us them
		the a an of to in on at for with from by as is are was were be been this these those there here now just only so`) {
		little[w] = true
	}
}

// caseAfter is w as it is written after mark.
func caseAfter(mark, w string) string {
	r, size := utf8.DecodeRuneInString(w)
	switch {
	case strings.ContainsAny(mark, ".?!\n"):
		return string(unicode.ToUpper(r)) + w[size:]
	case unicode.IsUpper(r) && little[normWord(w)]:
		return string(unicode.ToLower(r)) + w[size:]
	}
	return w
}

// endsWithCommand: the last words of ws are a command that ends a sentence or a line, so the
// utterance need not wait for more (liveTuning.hold).
func endsWithCommand(ws []model.Word) bool {
	for n := 1; n <= 2 && n <= len(ws); n++ {
		if c, k := matchCommand(ws[len(ws)-n:]); k == n && strings.ContainsAny(c.mark, ".?!\n") {
			return true
		}
	}
	return false
}

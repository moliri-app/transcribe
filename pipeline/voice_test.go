// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/model"
)

// said makes words of text, one every 0.4 s; a "|" after a word is a pause of a second after it.
func said(text string) []model.Word {
	var out []model.Word
	t := 0.0
	for _, f := range strings.Fields(text) {
		if f == "|" {
			t += 1
			continue
		}
		out = append(out, model.Word{W: f, Start: t, End: t + 0.4, P: 0.9})
		t += 0.4
	}
	return out
}

func text(ws []model.Word) string { return liveText(ws) }

func TestFixWords(t *testing.T) {
	words := []string{"Moliri", "Obeya", "sherpa-onnx", "Juri"}
	for _, c := range []struct{ in, want string }{
		{"Ich arbeite an Molly Ri.", "Ich arbeite an Moliri."},
		{"danach an Obeia.", "danach an Obeya."},
		{"Das läuft mit sherpa on X auf dem Rechner.", "Das läuft mit sherpa-onnx auf dem Rechner."},
		{"Sherpa onnx, oder?", "sherpa-onnx, oder?"},
		{"die neue Version von Mollire an das Team", "die neue Version von Moliri an das Team"},
		{"dass moliri schnell ist", "dass Moliri schnell ist"},
		{"Frag juri.", "Frag Juri."},
		// what only sounds a little alike stays
		{"Wir gehen in die Oper.", "Wir gehen in die Oper."},
		{"Obaier zeigt die Karten an.", "Obeya zeigt die Karten an."}, // no word, as the TTS said it
		{"Das ist Yuri.", "Das ist Yuri."},
		{"Molly. Ri", "Molly. Ri"},
	} {
		if got := text(fixWords(said(c.in), words)); got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSpokenMarks(t *testing.T) {
	before := said("Also")
	for _, c := range []struct {
		in, want string
		after    string // the mark the line before ended in
		open     bool
	}{
		{in: "Ich komme morgen Punkt", want: "Ich komme morgen."},
		{in: "Ich komme morgen. Punkt.", want: "Ich komme morgen."},
		{in: "Kommst du morgen Fragezeichen", want: "Kommst du morgen?"},
		{in: "Ich komme morgen Komma | Wenn es geht.", want: "Ich komme morgen, wenn es geht."},
		{in: "Ich komme morgen, Komma. | Hund und Katze.", want: "Ich komme morgen, Hund und Katze."},
		{in: "Liebe Grüße Doppelpunkt | das war's", want: "Liebe Grüße: das war's"},
		{in: "Ich komme morgen Punkt | dann gehen wir", want: "Ich komme morgen. Dann gehen wir"},
		{in: "Mein erster Punkt neuer Absatz | zweitens", want: "Mein erster Punkt\n\nZweitens"},
		{in: "Hallo neue Zeile", want: "Hallo\n"},
		{in: "Morgen zeige ich dir Obeya-Fragezeichen.", want: "Morgen zeige ich dir Obeya?"},
		{in: "Ein Grenz-Punkt ist das.", want: "Ein Grenz-Punkt ist das."},
		{in: "Hallo Punkt neuer Absatz", want: "Hallo.\n\n"},
		{in: "I will come tomorrow full stop", want: "I will come tomorrow."},
		{in: "Are you there question mark", want: "Are you there?"},
		{in: "Dear Anna comma | How are you", want: "Dear Anna, how are you"},
		// at the start: a mark of its own, for the line before
		{in: "Punkt. | Dann gehen wir.", want: ". Dann gehen wir."},
		{in: "Komma | Wenn es geht.", want: ", wenn es geht."},
		{in: "Neuer Absatz.", want: "\n\n"},
		// after a spoken comma at the end of the line before
		{in: "Wenn es geht.", after: ",", want: "wenn es geht."},
		{in: "Hunde mag ich.", after: ",", want: "Hunde mag ich."},
		{in: "dann gehen wir.", after: ".", want: "Dann gehen wir."},
		// meant as words
		{in: "Der springende Punkt ist, dass es geht.", want: "Der springende Punkt ist, dass es geht."},
		{in: "Das ist der springende Punkt.", want: "Das ist der springende Punkt."},
		{in: "Das ist ein wichtiger Punkt.", want: "Das ist ein wichtiger Punkt."},
		{in: "Da fehlt ein Komma.", want: "Da fehlt ein Komma."},
		{in: "Das ist ein großes Fragezeichen.", want: "Das ist ein großes Fragezeichen."},
		{in: "Mach eine neue Zeile.", want: "Mach eine neue Zeile."},
		{in: "It was a long period.", want: "It was a long period."},
		{in: "Wir treffen uns Punkt acht Uhr.", want: "Wir treffen uns Punkt acht Uhr."},
		// still being said: the end may go on
		{in: "Ich komme morgen Punkt", open: true, want: "Ich komme morgen Punkt"},
	} {
		got, _ := spokenMarks(said(c.in), before, c.after, c.open)
		if text(got) != c.want {
			t.Errorf("%q: got %q, want %q", c.in, text(got), c.want)
		}
	}
	// the first words of all: nothing before them to take the mark
	if got, _ := spokenMarks(said("Punkt."), nil, "", false); text(got) != "Punkt." {
		t.Errorf("a command with nothing before it: got %q", text(got))
	}
	if _, end := spokenMarks(said("Ich komme morgen Komma"), before, "", false); end != "," {
		t.Errorf("the mark at the end: got %q, want \",\"", end)
	}
}

func TestEndsWithCommand(t *testing.T) {
	for in, want := range map[string]bool{"Ich komme morgen Punkt": true, "und dann neuer Absatz": true, "Ich komme morgen Komma": false, "Ich komme": false} {
		if got := endsWithCommand(said(in)); got != want {
			t.Errorf("%q: got %v", in, got)
		}
	}
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package stream

import "strings"

// Spoken punctuation: who says „Komma“, „Punkt“ or „neuer Absatz“ gets the mark or the break
// instead of the word (pipeline/voice.go). Said at the start of an utterance, it belongs
// to the end of the one before: the final then begins with the mark (". Dann gehen wir", or just
// "."), and whoever puts the finals together attaches it there with Attach.

// Marks are the punctuation marks a model sets at the end of a word, which a spoken one replaces.
const Marks = ".,;:?!…"

// AddMark puts mark at the end of text: a punctuation mark replaces the one there, a line break
// ("\n", or "\n\n" for a paragraph) comes after it.
func AddMark(text, mark string) string {
	if strings.HasPrefix(mark, "\n") {
		return strings.TrimRight(text, " ") + mark
	}
	body := strings.TrimRight(text, " \n")
	breaks := text[len(body):]
	return strings.TrimRight(body, Marks) + mark + strings.TrimLeft(breaks, " ")
}

// Attach takes the marks text begins with (spoken at the start of an utterance) onto prev, the
// text before it: it gives prev with them and what is left of text. Text that begins with none
// comes back as it is.
func Attach(prev, text string) (string, string) {
	for {
		rest := strings.TrimLeft(text, " ")
		switch {
		case strings.HasPrefix(rest, "\n\n"):
			prev, text = AddMark(prev, "\n\n"), rest[2:]
		case strings.HasPrefix(rest, "\n"):
			prev, text = AddMark(prev, "\n"), rest[1:]
		case rest != "" && strings.ContainsRune(".,:?!", rune(rest[0])):
			prev, text = AddMark(prev, rest[:1]), rest[1:]
		default:
			return prev, text
		}
	}
}

// StartsWithMark reports whether text begins with a spoken mark that belongs to the text before.
func StartsWithMark(text string) bool {
	rest := strings.TrimLeft(text, " ")
	return rest != "" && strings.ContainsRune(".,:?!\n", rune(rest[0]))
}

// JoinWords puts words together with spaces, but none after a line break.
func JoinWords(words []string) string {
	var b strings.Builder
	for i, w := range words {
		if i > 0 && !strings.HasSuffix(words[i-1], "\n") {
			b.WriteByte(' ')
		}
		b.WriteString(w)
	}
	return b.String()
}

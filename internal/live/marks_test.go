// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import "testing"

func TestAttach(t *testing.T) {
	for _, c := range []struct{ prev, text, wantPrev, wantRest string }{
		{"Ich komme morgen", ". Dann gehen wir.", "Ich komme morgen.", " Dann gehen wir."},
		{"Ich komme morgen.", "?", "Ich komme morgen?", ""},
		{"Ich komme morgen,", ". Gut.", "Ich komme morgen.", " Gut."},
		{"Liebe Grüße", "\n\nAnna", "Liebe Grüße\n\n", "Anna"},
		{"Liebe Grüße.", ".\n\nAnna", "Liebe Grüße.\n\n", "Anna"},
		{"Erstens\n", ".", "Erstens.\n", ""},
		{"Ich komme", "morgen.", "Ich komme", "morgen."},
	} {
		p, r := Attach(c.prev, c.text)
		if p != c.wantPrev || r != c.wantRest {
			t.Errorf("Attach(%q, %q) = %q, %q; want %q, %q", c.prev, c.text, p, r, c.wantPrev, c.wantRest)
		}
	}
	if got := JoinWords([]string{"Hallo.\n\n", "Anna", "und", "Ben"}); got != "Hallo.\n\nAnna und Ben" {
		t.Errorf("JoinWords: %q", got)
	}
}

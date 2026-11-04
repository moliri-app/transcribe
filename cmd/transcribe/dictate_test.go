// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import "testing"

func TestJoinCapitalisesSentences(t *testing.T) {
	got := join([]string{"guten Tag, Herr Berger.", "wie hat das angefangen?", "", "im Sommer", "neunzehnhundertsechzig"})
	want := "Guten Tag, Herr Berger. Wie hat das angefangen? Im Sommer neunzehnhundertsechzig"
	if got != want {
		t.Errorf("join: %q, want %q", got, want)
	}
	if got := join([]string{"äpfel."}); got != "Äpfel." {
		t.Errorf("join: %q", got)
	}
}

func TestTailStartsWithAWord(t *testing.T) {
	text := "Guten Tag, Herr Berger, Sie haben vierzig Jahre in der Papierfabrik gearbeitet. Wie hat das angefangen? Angefangen hat es im Sommer neunzehnhundertsechzig."
	got := tail(text)
	if got != "…haben vierzig Jahre in der Papierfabrik gearbeitet. Wie hat das angefangen? Angefangen hat es im Sommer neunzehnhundertsechzig." {
		t.Errorf("tail: %q", got)
	}
	if tail("kurz") != "kurz" {
		t.Error("a short text is shown whole")
	}
}

func TestJoinTakesSpokenMarks(t *testing.T) {
	got := join([]string{"Ich komme morgen", ". Dann gehen wir", ",", "wenn es geht.\n\n", "Gruß Anna"})
	want := "Ich komme morgen. Dann gehen wir, wenn es geht.\n\nGruß Anna"
	if got != want {
		t.Errorf("join: %q, want %q", got, want)
	}
}

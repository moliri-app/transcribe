// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package annotate

import (
	"reflect"
	"testing"
)

func w(s string, a, b float64, spk int) Word {
	x := Word{"w": s, "start": a, "end": b}
	SetSpeaker(x, spk)
	return x
}

func texts(ws []Word) []string {
	var out []string
	for _, x := range ws {
		out = append(out, Text(x))
	}
	return out
}

func TestResolveMerges(t *testing.T) {
	got := ResolveMerges(map[string]SpeakerAnn{
		"3": {MergeInto: "1"}, "1": {MergeInto: "0"}, "5": {MergeInto: "6"}, "6": {MergeInto: "5"},
	})
	if got[3] != 0 || got[1] != 0 {
		t.Errorf("chain: %v", got)
	}
	if _, ok := got[5]; !ok { // cycles must terminate
		t.Errorf("cycle: %v", got)
	}
}

func TestApplyEditReplace(t *testing.T) {
	words := []Word{w("a", 0, 1, 0), w("b", 1, 2, 1), w("c", 2, 3, 1), w("d", 3, 4, 0)}
	got := ApplyEdit(words, 1, 3, "x y z", "replace")
	if want := []string{"a", "x", "y", "z", "d"}; !reflect.DeepEqual(texts(got), want) {
		t.Fatalf("got %v", texts(got))
	}
	if Speaker(got[1]) != 1 || !Truthy(got[1]["edited"]) {
		t.Errorf("speaker/edited: %v", got[1])
	}
	if s := Num(got[1], "start"); s != 1 {
		t.Errorf("start %v", s)
	}
	if e := Num(got[3], "end"); e != 3 {
		t.Errorf("end %v", e)
	}
	if got := ApplyEdit(words, 1, 3, "", "replace"); len(got) != 2 {
		t.Errorf("deletion: %v", texts(got))
	}
}

func TestApplyEditInsert(t *testing.T) {
	words := []Word{w("a", 0, 2, 0), w("d", 3, 4, 0)}
	got := ApplyEdit(words, 1, 2.5, "neu", "insert")
	if want := []string{"a", "neu", "d"}; !reflect.DeepEqual(texts(got), want) {
		t.Fatalf("got %v", texts(got))
	}
	if Num(got[0], "end") != 1 {
		t.Errorf("not shortened: %v", got[0])
	}
	if Num(words[0], "end") != 2 {
		t.Error("input mutated")
	}
}

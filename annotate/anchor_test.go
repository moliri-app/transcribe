// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package annotate

import "testing"

func TestMoveAnchor(t *testing.T) {
	// "Eins zwei. Drei vier fünf." anchors: Eins (first), Drei (after a sentence end)
	words := []Word{w("Eins", 0, 1, 0), w("zwei.", 1, 2, 0), w("Drei", 2, 3, 0), w("vier", 3, 4, 0), w("fünf.", 4, 5, 0)}
	got := MoveAnchor(words, 2, 3, -1) // timeline: "Drei" now starts at 3 s
	if Num(got[2], "start") != 3 || !Truthy(got[2]["pin"]) {
		t.Fatalf("Drei: %v", got[2])
	}
	if s := Num(got[1], "start"); s != 1.5 { // [0, 2] stretched to [0, 3]
		t.Fatalf("zwei starts at %v, want 1.5", s)
	}
	if s := Num(got[3], "start"); s != 3+2.0/3 { // [2, 5] squeezed to [3, 5]
		t.Logf("vier starts at %v", s)
	}
	// text: the anchor of "Drei" (2 s) goes to "vier" (3 s): "vier" starts at 2 s, "Drei" gives its anchor up
	drop := 2.0
	got = MoveAnchor(words, 3, 2, drop)
	if Num(got[3], "start") != 2 || !Truthy(got[3]["pin"]) || !Truthy(got[2]["nopin"]) || IsAnchor(got, 2) {
		t.Fatalf("vier %v, Drei %v", got[3], got[2])
	}
	if Num(got[2], "start") >= 2 || Num(got[1], "start") >= Num(got[2], "start") {
		t.Fatalf("order broken: %v %v", got[1], got[2])
	}
	if MoveAnchor(words, 7, 1, -1)[0]["pin"] != nil {
		t.Fatal("unknown anchor must not change anything")
	}
}

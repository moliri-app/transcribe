// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/internal/audio"
	"github.com/moliri-app/transcribe/internal/model"
)

// An empty live recording made the speaker analysis panic, a very short one crashed the app.
func TestTooShortRecordingsAreExplained(t *testing.T) {
	if err := tooShort(nil); err == nil || !strings.Contains(err.Error(), "leer") {
		t.Errorf("empty: %v", err)
	}
	if err := tooShort(make([]float32, audio.SR/10)); err == nil || !strings.Contains(err.Error(), "0,1 s") {
		t.Errorf("0.1 s: %v", err)
	}
	if err := tooShort(make([]float32, audio.SR)); err != nil {
		t.Errorf("1 s: %v", err)
	}
}

// The text step decodes a recording as a whole and splits its lines at the speaker turns.
func TestSplitSegmentsAtSpeakerTurns(t *testing.T) {
	w := func(s string, a float64) model.Word { return model.Word{W: s, Start: a, End: a + 0.3} }
	segs := []tseg{
		{Segment: model.Segment{Start: 0, End: 3, Text: "a b c", Words: []model.Word{w("a", 0.1), w("b", 1.1), w("c", 2.1)}}},
		{Segment: model.Segment{Start: 4, End: 5, Text: "d", Words: []model.Word{w("d", 4.2)}}},
	}
	got := splitSegments(segs, []float64{1, 2, 4.5})
	var texts []string
	for _, sg := range got {
		texts = append(texts, fmt.Sprintf("%g-%g %s", sg.Start, sg.End, sg.Text))
	}
	if want := "0-1 a|1-2 b|2-3 c|4-5 d"; strings.Join(texts, "|") != want {
		t.Errorf("got %q, want %q", strings.Join(texts, "|"), want)
	}
}

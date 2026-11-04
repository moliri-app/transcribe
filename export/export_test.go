// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package export

import (
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/annotate"
)

func fixture() *Final {
	w := func(s string, a, b float64, spk int) annotate.Word {
		return annotate.Word{"w": s, "start": a, "end": b, "speaker": float64(spk)}
	}
	f := &Final{Speakers: map[string]struct {
		Name string `json:"name"`
	}{"0": {"Anna"}, "1": {"Ben"}}}
	f.Segments = []struct {
		Words []annotate.Word `json:"words"`
	}{
		{Words: []annotate.Word{w("Hallo", 0, 1, 0), w("Welt.", 1, 2, 0)}},
		{Words: []annotate.Word{w("Guten", 4, 5, 1), w("Tag.", 5, 6, 1)}},
	}
	return f
}

func TestFormats(t *testing.T) {
	ann := &annotate.Annotations{Speakers: map[string]annotate.SpeakerAnn{}}
	txt := ToTXT(fixture(), ann)
	if !strings.Contains(txt, "Hallo Welt.") || !strings.Contains(txt, "Anna") {
		t.Errorf("txt: %q", txt)
	}
	vtt := ToVTT(fixture(), ann)
	if !strings.HasPrefix(vtt, "WEBVTT") || !strings.Contains(vtt, "<v Anna>") {
		t.Errorf("vtt: %q", vtt)
	}
	html := ToHTML(fixture(), ann, "x.mp4")
	if !strings.Contains(html, `name="audio_source"`) || !strings.Contains(html, "ts_0_") {
		t.Errorf("html missing anchors")
	}
}

func TestAnnotationOverlay(t *testing.T) {
	ann := &annotate.Annotations{
		Speakers: map[string]annotate.SpeakerAnn{"1": {Name: "Berta"}},
		Edits:    []annotate.EditAnn{{Start: 4, End: 6, Text: "Servus", Mode: "replace"}},
	}
	txt := ToTXT(fixture(), ann)
	if !strings.Contains(txt, "Berta") || !strings.Contains(txt, "Servus") || strings.Contains(txt, "Guten") {
		t.Errorf("overlay not applied: %q", txt)
	}
}

func TestClipsTXT(t *testing.T) {
	ann := &annotate.Annotations{Speakers: map[string]annotate.SpeakerAnn{}}
	txt := ClipsTXT(fixture(), ann, []annotate.Clip{{Start: 3.5, End: 6, Title: "Begrüßung", Tags: []string{"Anfang", "Ben"}}})
	if !strings.Contains(txt, "1. Begrüßung") || !strings.Contains(txt, "Schlagworte: Anfang, Ben") ||
		!strings.Contains(txt, "Ben: [00:00:04] Guten Tag.") || strings.Contains(txt, "Hallo") {
		t.Errorf("clip text: %q", txt)
	}
}

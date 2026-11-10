// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"os"
	"testing"

	"github.com/moliri-app/transcribe/audio"
	"github.com/moliri-app/transcribe/models"
	"github.com/moliri-app/transcribe/sherpa"
)

// Passages in two languages one after the other go through one Whisper turbo, switched to the
// language of each: forced to English, German speech comes out as from a Whisper turbo made for
// English, and back in German as it did the first time.
func TestTurboLoadedOnceForAllLanguages(t *testing.T) {
	lib := sherpa.FindLib(os.Getenv("SHERPA_LIB"))
	dir, file := os.Getenv("TRANSCRIBE_MODELS"), os.Getenv("TRANSCRIBE_AUDIO")
	if lib == "" || dir == "" || file == "" {
		t.Skip("set SHERPA_LIB, TRANSCRIBE_MODELS, TRANSCRIBE_AUDIO (German speech)")
	}
	sherpa.SetLibDir(lib)
	m := models.Set{Dir: dir, Offline: true}
	if err := m.Ensure(context.Background(), nil, "whisper-turbo", "silero"); err != nil {
		t.Skip(err)
	}
	pcm, err := audio.Decode(context.Background(), file)
	if err != nil {
		t.Skip(err)
	}
	text := func(e *Engine, lang string) string {
		ws, err := e.whisperSnippet(context.Background(), pcm, 10, 20, lang)
		if err != nil {
			t.Fatal(err)
		}
		return JoinWords(ws)
	}
	e := NewEngine(m, 4, nil)
	de, en, again := text(e, "de"), text(e, "en"), text(e, "de")
	t.Logf("de: %s\nen: %s", de, en)
	if len(e.rec) != 1 || e.rec["turbo"] == nil {
		t.Fatalf("recognizers %v, want only turbo", e.rec)
	}
	if again != de {
		t.Errorf("German again %q, first %q", again, de)
	}
	e.Close()

	// a Whisper turbo made for English from the start
	w := m.Whisper("turbo")
	r, err := sherpa.NewRecognizer(sherpa.RecognizerConfig{Kind: "whisper", Encoder: w.Encoder, Decoder: w.Decoder, Tokens: w.Tokens, Language: "en", Threads: 4, UseGPU: true})
	if err != nil {
		t.Fatal(err)
	}
	en1 := NewEngine(m, 4, nil)
	en1.rec["turbo"] = r
	defer en1.Close()
	if want := text(en1, "en"); de == "" || en == de || en != want {
		t.Errorf("switched to English %q, made for English %q, German %q", en, want, de)
	}
}

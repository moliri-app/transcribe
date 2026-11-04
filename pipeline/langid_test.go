// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/moliri-app/transcribe/audio"
	"github.com/moliri-app/transcribe/models"
	"github.com/moliri-app/transcribe/sherpa"
)

// With the languages of a recording given, language identification only chooses among them: German
// speech stays German among German and English, and becomes one of the two among English and French.
func TestLanguageIDAmongGivenLanguages(t *testing.T) {
	lib := sherpa.FindLib(os.Getenv("SHERPA_LIB"))
	src, file := os.Getenv("TRANSCRIBE_MODELS"), os.Getenv("TRANSCRIBE_AUDIO")
	if lib == "" || src == "" || file == "" {
		t.Skip("set SHERPA_LIB, TRANSCRIBE_MODELS, TRANSCRIBE_AUDIO (German speech)")
	}
	sherpa.SetLibDir(lib)
	// the restricted encoder is written next to the models: copy them into a scratch directory
	m := models.Set{Dir: t.TempDir()}
	for _, f := range []string{m.Whisper("base").Encoder, m.Whisper("base").Decoder} {
		rel, _ := filepath.Rel(m.Dir, f)
		b, err := os.ReadFile(filepath.Join(src, rel))
		if err != nil {
			t.Skip(err)
		}
		_ = os.MkdirAll(filepath.Dir(f), 0o755)
		if err := os.WriteFile(f, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pcm, err := audio.Decode(context.Background(), file)
	if err != nil {
		t.Skip(err)
	}
	detect := func(langs ...string) string {
		e := NewEngine(m, 2, nil)
		e.Langs = langs
		defer e.Close()
		return e.spokenLanguage(context.Background(), pcm, 10, 20, 0)
	}
	if l := detect(); l != "de" {
		t.Fatalf("unrestricted: %q, want de", l)
	}
	if l := detect("de", "en"); l != "de" {
		t.Errorf("among de, en: %q", l)
	}
	if l := detect("en", "fr"); !slices.Contains([]string{"en", "fr"}, l) {
		t.Errorf("among en, fr: %q", l)
	}
}

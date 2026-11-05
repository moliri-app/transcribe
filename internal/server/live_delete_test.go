// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"os"
	"path/filepath"
	"testing"
)

// A deleted Live note takes its recording along unless it was transcribed in full.
func TestDropRecording(t *testing.T) {
	s, d := newServer(t)
	audio := func(name string) string { return filepath.Join(d, "audio", name) }
	_ = os.WriteFile(audio("Live 1.wav"), []byte("RIFF"), 0o644)
	_ = os.WriteFile(audio("talk.wav"), []byte("RIFF"), 0o644) // results/talk/_final.json exists

	s.dropRecording("Live 1.wav")
	if _, err := os.Stat(audio("Live 1.wav")); !os.IsNotExist(err) {
		t.Error("the untranscribed recording stayed")
	}
	s.dropRecording("talk.wav")
	if _, err := os.Stat(audio("talk.wav")); err != nil {
		t.Error("the transcribed recording went:", err)
	}
	s.dropRecording(`..\talk.wav`) // never outside audio/
	s.dropRecording("missing.wav")
}

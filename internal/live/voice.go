// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import "github.com/moliri-app/transcribe/internal/prefs"

// „Meine Stimme“ on the Live page: the user's own words and spoken punctuation
// (pipeline/voice.go). Both count from the next recording on.

func voiceMsg(p prefs.Prefs) map[string]any {
	words := p.LiveWords
	if words == nil {
		words = []string{}
	}
	return map[string]any{"type": "voice", "words": words, "commands": !p.LiveNoCommands}
}

// SetWords keeps the user's own words and has recognition loaded with them.
func (h *Hub) SetWords(words []string) error {
	p, err := prefs.Update(h.dataDir, func(p *prefs.Prefs) { p.LiveWords = words })
	if err != nil {
		return err
	}
	h.broadcast(voiceMsg(p))
	h.Warm() // the recognizers favour them
	return nil
}

// SetCommands switches spoken punctuation on or off.
func (h *Hub) SetCommands(on bool) error {
	p, err := prefs.Update(h.dataDir, func(p *prefs.Prefs) { p.LiveNoCommands = !on })
	if err != nil {
		return err
	}
	h.broadcast(voiceMsg(p))
	return nil
}

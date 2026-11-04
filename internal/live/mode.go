// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"cmp"
	"errors"
	"strings"

	"github.com/moliri-app/transcribe/internal/prefs"
)

// The background mode ("Im Hintergrund bereit", once the live mode) is switched on and off on the
// Live page and in the settings. It only decides whether programs may have the app record the
// microphone (or what else the Live page records) for them. On (the default), they may, and live
// recognition stays loaded, so that a recording starts at once; off, they are turned away and
// nothing is kept loaded. What a program sends itself (audio to transcribe, images to read) is
// always taken, and recording on the Live page and dictating work either way: off, they load
// recognition when they start.

// Warmer keeps live recognition loaded.
type Warmer interface {
	// KeepWarm loads what live transcription of lang ("auto" or a code like de) needs and keeps it
	// until the next call; "" lets it go. It reports whether it is loaded now: false when its
	// models have not been downloaded yet (it downloads nothing) or loading failed.
	KeepWarm(lang string) bool
}

// What the Live page shows about keeping recognition loaded.
const (
	WarmLoading = "loading" // it is being loaded
	WarmReady   = "ready"   // a recording starts at once
)

var errOff = errors.New("Transcribe nimmt für andere Programme gerade nicht über das Mikrofon auf: „Im Hintergrund bereit“ ist aus. Geschickte Audiodaten und Bilder verarbeitet es weiterhin. Zum Aufnehmen in Transcribe unter Einstellungen, „Andere Programme“, einschalten.")

// Enabled reports whether the background mode is on.
func (h *Hub) Enabled() bool { return !prefs.Load(h.dataDir).LiveOff }

// SetEnabled switches the background mode on or off. Recordings that run go on until they end.
func (h *Hub) SetEnabled(on bool) error {
	if _, err := prefs.Update(h.dataDir, func(p *prefs.Prefs) { p.LiveOff = !on }); err != nil {
		return err
	}
	h.broadcast(map[string]any{"type": "enabled", "enabled": on})
	h.Warm()
	return nil
}

// Warm has recognition loaded as the settings want it now: for the language of the recording
// while the live mode is on and recognises on this computer, else nothing. Call it when one of
// them changes. It returns at once; loading takes a few seconds.
func (h *Hub) Warm() {
	if h.Warmer == nil {
		return
	}
	p := prefs.Load(h.dataDir)
	want := ""
	if !p.LiveOff && p.Engine("", p.LiveEngine) == prefs.Local {
		// the language, and the user's words the recognizers favour (KeepWarm reads them)
		want = strings.Join(append([]string{cmp.Or(p.LiveLanguage, "auto")}, p.LiveWords...), "\n")
	}
	h.warmMu.Lock()
	h.warmWant = want
	start := !h.warmRunning
	h.warmRunning = true
	h.warmMu.Unlock()
	if start {
		go h.warmLoop()
	}
}

// warmLoop loads what Warm asked for, one after the other, until it has the latest. What could
// not be loaded is tried again on the next Warm (after a recording downloaded the models).
func (h *Hub) warmLoop() {
	tried := ""
	failed := false
	for {
		h.warmMu.Lock()
		want := h.warmWant
		if (h.warmHave != nil && *h.warmHave == want) || (failed && tried == want) {
			h.warmRunning = false
			h.warmMu.Unlock()
			return
		}
		h.warmMu.Unlock()
		if want != "" {
			h.setWarm(WarmLoading)
		}
		lang, _, _ := strings.Cut(want, "\n")
		ok := h.Warmer.KeepWarm(lang) || want == ""
		h.warmMu.Lock()
		h.warmHave, tried, failed = nil, want, !ok
		if ok {
			h.warmHave = &want
		}
		h.warmMu.Unlock()
		if ok && want != "" {
			h.setWarm(WarmReady)
		} else {
			h.setWarm("")
		}
	}
}

func (h *Hub) setWarm(state string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.warm == state {
		return
	}
	h.warm = state
	h.broadcastLocked(map[string]any{"type": "warm", "warm": state})
}

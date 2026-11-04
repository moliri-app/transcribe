// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/moliri-app/transcribe/internal/prefs"
)

// TRANSCRIBE_LIVE_DEVICES names a file that, once it exists, stands in for the microphones
// connected (for tests and demos): a name per line, the system's default marked with a leading
// "* ". Rewriting it plugs them in and out.

// testDevices reads that file; ok is false if there is none.
func testDevices() (names []string, def, stamp string, ok bool) {
	f := os.Getenv("TRANSCRIBE_LIVE_DEVICES")
	if f == "" {
		return nil, "", "", false
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, "", "", false
	}
	names = []string{}
	for l := range strings.Lines(string(b)) {
		name, isDef := strings.CutPrefix(strings.TrimSpace(l), "* ")
		if name = strings.TrimSpace(name); name == "" {
			continue
		}
		names = append(names, name)
		if isDef {
			def = name
		}
	}
	return names, def, string(b), true
}

// DeviceState tells the microphones apart cheaply, without ffmpeg: stamp changes when one is
// plugged in or out or another becomes the system's default. def is the name of the default (""
// if there is none or it is not known); ok is false if the system could not say.
func DeviceState() (stamp, def string, ok bool) {
	if _, def, stamp, ok := testDevices(); ok {
		return stamp, def, true
	}
	return deviceState()
}

// Microphones is what the Live page offers to record from.
type Microphones struct {
	Devices []string `json:"devices"` // the microphones connected, as ffmpeg names them
	Default string   `json:"default"` // the name of the system's default one, "" if unknown
	Device  string   `json:"device"`  // the one chosen, "" for the system's default
}

// ListMicrophones is what the Live page offers now.
func ListMicrophones(ctx context.Context, dataDir string) Microphones {
	m := Microphones{Devices: Devices(ctx), Device: prefs.Load(dataDir).LiveDevice}
	if m.Devices == nil {
		m.Devices = []string{}
	}
	_, m.Default, _ = DeviceState()
	return m
}

// How often the microphones are looked at while the Live page is open.
const deviceTick = time.Second

// watchDevices tells the Live page when a microphone is plugged in or out or another becomes the
// default, for as long as a page is subscribed. Only one runs at a time (h.watching).
func (h *Hub) watchDevices() {
	last, _, ok := DeviceState()
	tick := time.NewTicker(deviceTick)
	defer tick.Stop()
	for ok {
		<-tick.C
		h.mu.Lock()
		open := len(h.subs) > 0
		h.mu.Unlock()
		if !open {
			break
		}
		var now string
		if now, _, ok = DeviceState(); ok && now != last {
			last = now
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			m := ListMicrophones(ctx, h.dataDir)
			cancel()
			h.broadcast(map[string]any{"type": "devices", "microphones": m})
		}
	}
	h.mu.Lock()
	h.watching = false
	h.mu.Unlock()
}

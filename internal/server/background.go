// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/moliri-app/transcribe/internal/autostart"
	"github.com/moliri-app/transcribe/internal/prefs"
)

// WindowSeen reports whether a window of the app has ever sent a heartbeat.
func (s *Server) WindowSeen() bool { return s.lastPing.Load() != 0 }

// instance names the data dir this app serves, so a second start can find it (cmd/transcribe/instance.go).
func (s *Server) instance(w http.ResponseWriter, r *http.Request) {
	abs, _ := filepath.Abs(s.cfg.DataDir)
	writeJSON(w, 200, map[string]any{"app": "transcribe", "dataDir": abs, "window": s.WindowOpen()})
}

// background reads (GET) or changes (POST) whether the app starts with Windows, whether it keeps
// running in the notification area once the window is closed, and whether it is ready for other
// programs (the background mode, live.Hub.Enabled; "ready" is absent without live transcription).
func (s *Server) background(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if !allowedPost(r) {
			http.Error(w, "forbidden", 403)
			return
		}
		var in struct {
			Autostart  *bool `json:"autostart"`
			Background *bool `json:"background"`
			Ready      *bool `json:"ready"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil {
			http.Error(w, "bad request", 400)
			return
		}
		if in.Autostart != nil {
			if err := autostart.Set(*in.Autostart, s.cfg.DataDir); err != nil {
				fail(w, 500, "Autostart ließ sich nicht ändern: "+err.Error())
				return
			}
		}
		if in.Background != nil {
			if _, err := prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) { p.Background = *in.Background }); err != nil {
				fail(w, 500, err.Error())
				return
			}
		}
		if in.Ready != nil && s.Live != nil {
			if err := s.Live.SetEnabled(*in.Ready); err != nil {
				fail(w, 500, err.Error())
				return
			}
		}
	} else if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	out := map[string]any{
		"supported":  autostart.Supported(),
		"autostart":  autostart.Enabled(),
		"background": prefs.Load(s.cfg.DataDir).Background,
	}
	if s.Live != nil {
		out["ready"] = s.Live.Enabled()
	}
	writeJSON(w, 200, out)
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"encoding/json"
	"net/http"

	"github.com/moliri-app/transcribe/internal/prefs"
)

// Dictation is the global shortcut that dictates into any program (cmd/transcribe).
type Dictation interface {
	// Apply takes the shortcut from the preferences anew; the error says why it does not work.
	Apply() error
	// Err is why the shortcut does not work, nil while it works or is switched off.
	Err() error
}

// dictate reads (GET) or changes (POST) the shortcut for dictating and where its text goes: to
// the cursor, to the clipboard, both, or neither (then it is only shown, and kept on the Live
// page).
func (s *Server) dictate(w http.ResponseWriter, r *http.Request) {
	var err error
	if r.Method == http.MethodPost {
		if !allowedPost(r) {
			http.Error(w, "forbidden", 403)
			return
		}
		var in struct {
			Key       *string `json:"key"`
			Insert    *bool   `json:"insert"`
			Clipboard *bool   `json:"clipboard"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil {
			http.Error(w, "bad request", 400)
			return
		}
		if _, uerr := prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) {
			if in.Key != nil {
				p.DictateKey = *in.Key
			}
			if in.Insert != nil {
				p.DictateNoInsert = !*in.Insert
			}
			if in.Clipboard != nil {
				p.DictateNoClipboard = !*in.Clipboard
			}
		}); uerr != nil {
			fail(w, 500, uerr.Error())
			return
		}
		if s.Dictation != nil {
			err = s.Dictation.Apply()
		}
	} else if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	} else if s.Dictation != nil {
		err = s.Dictation.Err()
	}
	p := prefs.Load(s.cfg.DataDir)
	out := map[string]any{"supported": s.Dictation != nil, "key": p.DictateKey, "insert": !p.DictateNoInsert, "clipboard": !p.DictateNoClipboard, "keys": prefs.DictateKeys}
	if err != nil {
		out["error"] = err.Error()
	}
	writeJSON(w, 200, out)
}

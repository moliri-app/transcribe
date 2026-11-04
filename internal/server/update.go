// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"encoding/json"
	"net/http"

	"github.com/moliri-app/transcribe/internal/update"
)

// updateAPI reads (GET) the app's version and whether a newer one is out, or (POST) acts on it:
// {"action":"check"} looks now, {"action":"install"} downloads the newer one, puts it in place and
// restarts the app once no job or recording runs.
func (s *Server) updateAPI(w http.ResponseWriter, r *http.Request) {
	u := s.cfg.Updater
	if u == nil {
		u = &update.Updater{} // tests and builds without updates: a development build that never looks
	}
	if r.Method == http.MethodPost {
		if !allowedPost(r) {
			http.Error(w, "forbidden", 403)
			return
		}
		var in struct{ Action string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil {
			http.Error(w, "bad request", 400)
			return
		}
		switch in.Action {
		case "check":
			if !u.Status().Checks {
				fail(w, 409, "Eine Entwicklerversion sucht nicht nach Updates.")
				return
			}
			if err := u.Check(r.Context()); err != nil {
				fail(w, 502, "GitHub war nicht zu erreichen. Versuch es später noch einmal.")
				return
			}
		case "install":
			if err := u.Install(); err != nil {
				fail(w, 409, err.Error())
				return
			}
		default:
			http.Error(w, "bad request", 400)
			return
		}
	} else if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	writeJSON(w, 200, u.Status())
}

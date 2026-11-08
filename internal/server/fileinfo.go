// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/moliri-app/transcribe/internal/media"
)

// fileInfo handles GET /api/fileinfo?name=<name>: what the library's details show of a recording,
// in audio/ or in a connected folder. The file system's part (where, how big, when) comes always;
// what ffprobe reads from the file (length, format, when it was recorded) only where it can.
func (s *Server) fileInfo(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	path := media.Path(s.cfg.DataDir, name)
	if path == "" {
		fail(w, 404, "unknown file")
		return
	}
	st, err := os.Stat(path)
	if err != nil {
		fail(w, 404, "unknown file")
		return
	}
	abs, _ := filepath.Abs(path)
	out := map[string]any{
		"file": filepath.Base(path), "path": abs, "size": st.Size(), "modified": st.ModTime().Unix(),
	}
	if b := birth(st); !b.IsZero() {
		out["created"] = b.Unix()
	}
	if info, err := media.Probe(path); err == nil {
		out["media"] = info
	}
	writeJSON(w, 200, out)
}

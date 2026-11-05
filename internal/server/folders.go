// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/pick"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/internal/queue"
	"github.com/moliri-app/transcribe/internal/sidecar"
	"github.com/moliri-app/transcribe/jobs"
)

// Connected folders: their recordings stand in the library where they are (internal/media), each
// transcript also next to its recording (internal/sidecar). Removing a folder only takes it out
// of the library; the app never deletes anything in it.

// withSidecar writes the sidecar of a linked recording once the queue transcribed it.
func withSidecar(run queue.Runner) queue.Runner {
	if run == nil {
		return nil
	}
	return func(ctx context.Context, dataDir, audioFile string, o jobs.Options, p jobs.Progress) error {
		err := run(ctx, dataDir, audioFile, o, p)
		if err == nil && media.IsLinked(audioFile) && !prefs.Load(dataDir).SidecarOff {
			err = sidecar.Write(dataDir, media.Stem(audioFile))
		}
		return err
	}
}

// exeDir is the folder of the EXE; tests set it.
var exeDir = func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

// folderHint is the folder of the EXE when recordings lie in it (or in folders below it) and it is
// neither connected nor declined; else "".
func (s *Server) folderHint(p prefs.Prefs) (string, int) {
	dir := exeDir()
	if dir == "" || prefs.SamePath(dir, p.FolderHint) || slices.ContainsFunc(p.Folders, func(f string) bool { return prefs.SamePath(f, dir) }) {
		return "", 0
	}
	n := media.CountMedia(dir, s.cfg.DataDir, 2, 1000)
	if n == 0 {
		return "", 0
	}
	return dir, n
}

type folderView struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
}

// folders lists the connected folders (GET), or changes them (POST, action "add", "remove" or
// "decline" with path; "sidecar" with on).
func (s *Server) folders(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if !allowedPost(r) {
			http.Error(w, "forbidden", 403)
			return
		}
		var body struct {
			Action string `json:"action"`
			Path   string `json:"path"`
			On     bool   `json:"on"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body) != nil {
			fail(w, 400, "bad request")
			return
		}
		if err := s.changeFolders(body.Action, body.Path, body.On); err != nil {
			fail(w, 400, err.Error())
			return
		}
		media.Forget(s.cfg.DataDir)
	}
	p := prefs.Load(s.cfg.DataDir)
	list := []folderView{}
	for _, f := range p.Folders {
		st, err := os.Stat(f)
		list = append(list, folderView{f, filepath.Base(f), err == nil && st.IsDir()})
	}
	out := map[string]any{"ok": true, "folders": list, "sidecar": !p.SidecarOff, "pick": pick.Supported()}
	if dir, n := s.folderHint(p); dir != "" {
		out["hint"] = map[string]any{"path": dir, "name": filepath.Base(dir), "count": n}
	}
	writeJSON(w, 200, out)
}

func (s *Server) changeFolders(action, path string, on bool) error {
	path = strings.Trim(strings.TrimSpace(path), `"`)
	switch action {
	case "add":
		if !filepath.IsAbs(path) {
			return errors.New("Bitte den ganzen Pfad des Ordners angeben, etwa C:\\Nextcloud\\Interviews.")
		}
		path = filepath.Clean(path)
		if st, err := os.Stat(path); err != nil || !st.IsDir() {
			return errors.New("Diesen Ordner gibt es nicht: " + path)
		}
		_, err := prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) { p.Folders = append(p.Folders, path) })
		return err
	case "remove":
		_, err := prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) {
			p.Folders = slices.DeleteFunc(p.Folders, func(f string) bool { return prefs.SamePath(f, filepath.Clean(path)) })
		})
		return err
	case "decline":
		_, err := prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) { p.FolderHint = filepath.Clean(path) })
		return err
	case "sidecar":
		_, err := prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) { p.SidecarOff = !on })
		return err
	}
	return errors.New("unknown action")
}

// folderPick lets the user choose a folder in the system's dialog; path is "" if they cancelled.
func (s *Server) folderPick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	path, err := pick.Folder("Ordner mit Aufnahmen verbinden")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "path": path})
}

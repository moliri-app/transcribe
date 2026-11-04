// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/models"
	"github.com/moliri-app/transcribe/internal/proc"
)

// dataFolders are the folders the settings page may open in the file manager.
var dataFolders = []struct{ key, label string }{
	{"data", ""}, {"audio", "audio"}, {"results", "results"}, {"models", "models"}, {"lib", "lib"}, {"bin", "bin"},
}

func (s *Server) folderPath(key string) (string, bool) {
	for _, f := range dataFolders {
		if f.key == key {
			return filepath.Join(s.cfg.DataDir, f.label), true
		}
	}
	return "", false
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	abs, _ := filepath.Abs(s.cfg.DataDir)
	exe, _ := os.Executable()
	folders := []map[string]any{}
	for _, f := range dataFolders {
		p, _ := s.folderPath(f.key)
		folders = append(folders, map[string]any{"key": f.key, "path": p, "size": models.DirSize(p)})
	}
	items, ready := models.Snapshot()
	downloading := false
	for _, it := range items {
		if it.State == "downloading" {
			downloading = true
		}
	}
	writeJSON(w, 200, map[string]any{
		"dataDir":     abs,
		"exe":         exe,
		"log":         filepath.Join(abs, "transcribe.log"),
		"os":          runtime.GOOS + "/" + runtime.GOARCH,
		"sherpa":      models.SherpaVersion,
		"ready":       ready,
		"downloading": downloading,
		"folders":     folders,
		"components":  models.Info(abs, media.Tool("ffmpeg")),
	})
}

// settingsDownload starts the download of one missing component in the background.
func (s *Server) settingsDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	var body struct{ Name string }
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body) != nil || body.Name == "" {
		http.Error(w, "bad request", 400)
		return
	}
	known := false
	for _, c := range models.Info(s.cfg.DataDir, "") {
		known = known || c.Name == body.Name
	}
	if !known {
		http.Error(w, "unknown component", 400)
		return
	}
	go func() {
		if models.Download(context.Background(), s.cfg.DataDir, body.Name) == nil && body.Name == "nvidia" {
			cachedGPU(s.cfg.DataDir, true) // the libraries are there now
		}
	}()
	writeJSON(w, 202, map[string]any{"ok": true})
}

// settingsOpen shows one of the data folders in the system file manager.
func (s *Server) settingsOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	var body struct{ Folder string }
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body)
	p, ok := s.folderPath(body.Folder)
	if !ok {
		http.Error(w, "unknown folder", 400)
		return
	}
	_ = os.MkdirAll(p, 0o755)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", p)
	case "darwin":
		cmd = exec.Command("open", p)
	default:
		cmd = exec.Command("xdg-open", p)
	}
	_ = proc.Hide(cmd).Start() // explorer exits with code 1 even on success
	writeJSON(w, 200, map[string]any{"ok": true})
}

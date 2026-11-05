// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/moliri-app/transcribe/internal/datadir"
	"github.com/moliri-app/transcribe/internal/folderpick"
	"github.com/moliri-app/transcribe/internal/setup"
	"github.com/moliri-app/transcribe/models"
)

// Moving the data dir: the settings page names a folder (chosen in the system's dialog, or typed),
// the app copies the data there while it runs, remembers the new place (internal/datadir) and
// starts anew, which removes the old folder. Nothing may run meanwhile that writes to the data.

// moving is the move going on, or the last one that failed.
type moving struct {
	mu     sync.Mutex
	target string
	done   int64
	total  int64
	state  string // "", "copying", "restarting", "error"
	err    string
}

func (m *moving) snapshot() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == "" {
		return nil
	}
	return map[string]any{"target": m.target, "done": m.done, "total": m.total, "state": m.state, "error": m.err}
}

// location is what the settings page shows of the data dir's place.
func (s *Server) location() map[string]any {
	abs, _ := filepath.Abs(s.cfg.DataDir)
	return map[string]any{
		"dataDir": abs,
		"default": datadir.Default(),
		"fixed":   s.Restart == nil,
		"pick":    folderpick.Supported() || os.Getenv("TRANSCRIBE_PICK_FOLDER") != "",
		"move":    s.move.snapshot(),
	}
}

// busy says why the data cannot move now, or "".
func (s *Server) busy() string {
	if s.Queue.Busy() {
		return "Gerade läuft ein Auftrag. Der Datenordner lässt sich verschieben, wenn alle Aufträge fertig sind."
	}
	if s.Live != nil && s.Live.Busy() {
		return "Gerade läuft eine Aufnahme. Der Datenordner lässt sich verschieben, wenn sie beendet ist."
	}
	items, _ := models.Snapshot()
	for _, it := range items {
		if it.State == "downloading" {
			return "Gerade lädt die App Modelle. Der Datenordner lässt sich verschieben, wenn der Download fertig ist."
		}
	}
	for _, c := range setup.Info(s.cfg.DataDir, "") {
		if c.State == "downloading" {
			return "Gerade lädt die App Komponenten. Der Datenordner lässt sich verschieben, wenn der Download fertig ist."
		}
	}
	return ""
}

// locationPick shows the system's folder dialog and returns the folder chosen ("" when cancelled).
func (s *Server) locationPick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	// TRANSCRIBE_PICK_FOLDER stands in for the user's choice in the dialog (e2e tests)
	dir, err := os.Getenv("TRANSCRIBE_PICK_FOLDER"), error(nil)
	if dir == "" {
		dir, err = folderpick.Pick("Wohin sollen die Daten von Transcribe?", filepath.Dir(s.cfg.DataDir))
	}
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	target := ""
	if dir != "" {
		if target, err = datadir.Target(s.cfg.DataDir, dir); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	writeJSON(w, 200, map[string]any{"path": dir, "target": target, "size": datadir.Size(s.cfg.DataDir)})
}

// locationMove checks the folder named (POST {path, check: true}: where the data would go and how
// much it is), or moves the data there (POST {path}); an empty path is the default folder.
func (s *Server) locationMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	var in struct {
		Path  string `json:"path"`
		Check bool   `json:"check"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if s.Restart == nil {
		fail(w, 409, "Der Datenordner ist beim Start festgelegt (--data oder TRANSCRIBE_DATA) und lässt sich hier nicht ändern.")
		return
	}
	target, err := datadir.Target(s.cfg.DataDir, in.Path)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if in.Check {
		writeJSON(w, 200, map[string]any{"target": target, "size": datadir.Size(s.cfg.DataDir)})
		return
	}
	if why := s.busy(); why != "" {
		fail(w, 409, why)
		return
	}
	m := &s.move
	m.mu.Lock()
	if m.state == "copying" || m.state == "restarting" {
		m.mu.Unlock()
		fail(w, 409, "Der Datenordner wird schon verschoben.")
		return
	}
	m.target, m.done, m.total, m.state, m.err = target, 0, datadir.Size(s.cfg.DataDir), "copying", ""
	m.mu.Unlock()
	go s.relocate(target)
	writeJSON(w, 202, map[string]any{"target": target})
}

func (s *Server) relocate(target string) {
	from, _ := filepath.Abs(s.cfg.DataDir)
	m := &s.move
	log.Printf("moving the data from %s to %s", from, target)
	err := datadir.Copy(context.Background(), from, target, func(done int64) {
		m.mu.Lock()
		m.done = done
		m.mu.Unlock()
	})
	if err == nil {
		err = datadir.Moved(from, target)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		log.Println("moving the data:", err)
		m.state, m.err = "error", "Das Verschieben hat nicht geklappt, die Daten liegen weiter am alten Ort. ("+err.Error()+")"
		return
	}
	m.state = "restarting"
	log.Println("data moved, starting anew")
	go s.Restart()
}

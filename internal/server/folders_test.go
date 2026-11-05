// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/moliri-app/transcribe/internal/media"
)

type folderState struct {
	Folders []folderView `json:"folders"`
	Sidecar bool         `json:"sidecar"`
	Hint    *struct {
		Path  string `json:"path"`
		Count int    `json:"count"`
	} `json:"hint"`
}

func folderList(t *testing.T, s *Server, method, body string) folderState {
	t.Helper()
	w := do(s, method, "/api/folders", body, nil)
	var st folderState
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &st) != nil {
		t.Fatalf("%s /api/folders: %d %s", method, w.Code, w.Body)
	}
	return st
}

func TestConnectedFolder(t *testing.T) {
	s, d := newServer(t)
	exeDir = func() string { return "" }
	folder := t.TempDir()
	_ = os.MkdirAll(filepath.Join(folder, "Rundgänge"), 0o755)
	_ = os.WriteFile(filepath.Join(folder, "Rundgänge", "Halle 1.wav"), []byte("0123456789"), 0o644)

	if w := do(s, "POST", "/api/folders", `{"action":"add","path":"relativ"}`, nil); w.Code != 400 {
		t.Errorf("relative path: %d", w.Code)
	}
	b, _ := json.Marshal(map[string]string{"action": "add", "path": folder})
	if st := folderList(t, s, "POST", string(b)); len(st.Folders) != 1 || st.Folders[0].Path != folder || !st.Sidecar {
		t.Fatalf("after add: %+v", st)
	}

	var linked map[string]any
	for _, it := range s.buildIndex() {
		if it["source"] == "folder" {
			linked = it
		}
	}
	if linked == nil || linked["title"] != "Halle 1" || linked["dir"] != "Rundgänge" {
		t.Fatalf("index: %+v", linked)
	}
	name := linked["name"].(string)
	if w := do(s, "GET", "/audio/"+pyQuote(name), "", nil); w.Code != 200 || w.Body.String() != "0123456789" {
		t.Errorf("audio of a linked recording: %d", w.Code)
	}
	if n, err := s.Queue.Add([]string{name}, nil); err != nil || n != 1 {
		t.Errorf("queue: %d %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(d, "audio", "Halle 1.wav")); !os.IsNotExist(err) {
		t.Error("the recording was copied into audio/")
	}

	b, _ = json.Marshal(map[string]string{"action": "remove", "path": folder})
	if st := folderList(t, s, "POST", string(b)); len(st.Folders) != 0 {
		t.Fatalf("after remove: %+v", st)
	}
	if _, err := os.Stat(filepath.Join(folder, "Rundgänge", "Halle 1.wav")); err != nil {
		t.Error("removing the folder touched its files")
	}
	if media.Path(d, name) != "" {
		t.Error("a removed folder still gives its recordings")
	}
}

func TestFolderNextToTheExe(t *testing.T) {
	s, d := newServer(t)
	exe := t.TempDir()
	exeDir = func() string { return exe }
	t.Cleanup(func() { exeDir = func() string { return "" } })

	if st := folderList(t, s, "GET", ""); st.Hint != nil {
		t.Fatalf("hint without recordings: %+v", st.Hint)
	}
	_ = os.MkdirAll(filepath.Join(exe, "Interviews", "2024"), 0o755)
	_ = os.WriteFile(filepath.Join(exe, "Interviews", "2024", "a.mp3"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(exe, "b.m4a"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(exe, "notiz.txt"), []byte("x"), 0o644)
	st := folderList(t, s, "GET", "")
	if st.Hint == nil || st.Hint.Path != exe || st.Hint.Count != 2 {
		t.Fatalf("hint: %+v", st.Hint)
	}
	b, _ := json.Marshal(map[string]string{"action": "decline", "path": exe})
	if st := folderList(t, s, "POST", string(b)); st.Hint != nil {
		t.Errorf("declined, still offered: %+v", st.Hint)
	}
	_ = d
}

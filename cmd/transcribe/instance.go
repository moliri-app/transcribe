// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// One app serves a data dir: it holds a lock for it from its start on (lockInstance), and
// <data>/instance.json names its address, so a second start (a double click while it runs in the
// background) shows that app's window instead of serving the same data twice.

func instanceFile(dataDir string) string { return filepath.Join(dataDir, "instance.json") }

// instanceKey names dataDir in the lock, the same for every way of writing its path.
func instanceKey(dataDir string) string {
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		abs = dataDir
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(abs))))
	return hex.EncodeToString(sum[:8])
}

// claimInstance waits until this app may serve dataDir: ok once it holds the lock (release lets
// go of it), false when another app serves it; with show, that app's window is shown then. An app
// that is still starting gets a while to answer, as does one that is ending (an update starts the
// new version just before the old one is gone).
func claimInstance(dataDir string, show bool) (release func(), ok bool) {
	deadline := time.Now().Add(20 * time.Second)
	for {
		release, ok := lockInstance(dataDir)
		url, window := runningInstance(dataDir) // even with the lock: a version without it may run
		if url != "" {
			if ok {
				release()
			}
			fmt.Println("Transcribe läuft schon:", url)
			if show {
				showAppWindow(url, dataDir, window)
			}
			return nil, false
		}
		if ok {
			return release, true
		}
		if time.Now().After(deadline) {
			fmt.Println("Transcribe läuft schon.")
			return nil, false
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// runningInstance returns the address of the app already serving dataDir, or "", and whether a
// window of it is open.
func runningInstance(dataDir string) (url string, window bool) {
	var inst struct{ URL string }
	b, err := os.ReadFile(instanceFile(dataDir))
	if err != nil || json.Unmarshal(b, &inst) != nil || inst.URL == "" {
		return "", false
	}
	// the file may be left over from a crash, or the port may now belong to another program
	c := http.Client{Timeout: 3 * time.Second}
	r, err := c.Get(inst.URL + "/api/instance")
	if err != nil {
		return "", false
	}
	defer r.Body.Close()
	var got struct {
		DataDir string
		Window  bool
	}
	if r.StatusCode != 200 || json.NewDecoder(r.Body).Decode(&got) != nil {
		return "", false
	}
	abs, _ := filepath.Abs(dataDir)
	if !strings.EqualFold(got.DataDir, abs) {
		return "", false
	}
	return inst.URL, got.Window
}

// showAppWindow brings the app's open window to the front, or opens one when none is open.
func showAppWindow(url, dataDir string, open bool) {
	if !open || !focusAppWindow() {
		openAppWindow(url, dataDir)
	}
}

func writeInstance(dataDir, url string) {
	b, _ := json.Marshal(map[string]any{"url": url, "pid": os.Getpid()})
	_ = os.WriteFile(instanceFile(dataDir), b, 0o644)
}

func removeInstance(dataDir string) { _ = os.Remove(instanceFile(dataDir)) }

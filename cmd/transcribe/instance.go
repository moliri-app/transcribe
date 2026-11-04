// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// <data>/instance.json names the address of the app serving this data dir, so a
// second start (a double click while it runs in the background) opens that
// app's window instead of serving the same data twice.

func instanceFile(dataDir string) string { return filepath.Join(dataDir, "instance.json") }

// runningInstance returns the address of the app already serving dataDir, or "".
func runningInstance(dataDir string) string {
	var inst struct{ URL string }
	b, err := os.ReadFile(instanceFile(dataDir))
	if err != nil || json.Unmarshal(b, &inst) != nil || inst.URL == "" {
		return ""
	}
	// the file may be left over from a crash, or the port may now belong to another program
	c := http.Client{Timeout: 3 * time.Second}
	r, err := c.Get(inst.URL + "/api/instance")
	if err != nil {
		return ""
	}
	defer r.Body.Close()
	var got struct{ DataDir string }
	if r.StatusCode != 200 || json.NewDecoder(r.Body).Decode(&got) != nil {
		return ""
	}
	abs, _ := filepath.Abs(dataDir)
	if !strings.EqualFold(got.DataDir, abs) {
		return ""
	}
	return inst.URL
}

func writeInstance(dataDir, url string) {
	b, _ := json.Marshal(map[string]any{"url": url, "pid": os.Getpid()})
	_ = os.WriteFile(instanceFile(dataDir), b, 0o644)
}

func removeInstance(dataDir string) { _ = os.Remove(instanceFile(dataDir)) }

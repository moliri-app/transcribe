// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "v1.2.3", 0},
		{"1.2.3", "v1.2.3", 0},
		{"v1.2.3", "v1.2.4", -1},
		{"v1.10.0", "v1.9.9", 1},
		{"v2.0.0", "v1.99.99", 1},
		{"v0.0.10", "v0.0.9", 1},
		{"v1.0.0-rc.1", "v1.0.0", -1},
		{"v1.0.0-alpha", "v1.0.0-alpha.1", -1},
		{"v1.0.0-alpha.1", "v1.0.0-alpha.beta", -1},
		{"v1.0.0-beta.2", "v1.0.0-beta.11", -1},
		{"v1.0.0-rc.1", "v1.0.0-beta.11", 1},
		{"v1.0.0+build.5", "v1.0.0", 0},
		{"dev-3c6302f", "v0.0.1", -1},
		{"v0.0.1", "", 1},
		{"v1.2", "v0.0.1", -1},
		{"v01.2.3", "v0.0.1", -1},
	} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := Compare(c.b, c.a); got != -c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.b, c.a, got, -c.want)
		}
	}
}

func TestIsRelease(t *testing.T) {
	for v, want := range map[string]bool{
		"v1.2.3": true, "0.1.0": true, "": false, "dev": false, "dev-v1.2.3-4-gabc": false,
		"v1.2.3-rc.1": false, "v1.2": false, "v1.2.x": false,
	} {
		if IsRelease(v) != want {
			t.Errorf("IsRelease(%q) = %v", v, !want)
		}
	}
}

func TestSwap(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "transcribe.exe")
	write(t, exe, "old")
	write(t, filepath.Join(dir, "transcribe.old.exe"), "older") // left over from the update before
	write(t, newPath(exe), "new")
	write(t, filepath.Join(dir, "data", "settings.json"), "{}")

	if err := Swap(exe, newPath(exe)); err != nil {
		t.Fatal(err)
	}
	if got := read(t, exe); got != "new" {
		t.Errorf("exe = %q, want the new one", got)
	}
	if got := read(t, OldPath(exe)); got != "old" {
		t.Errorf("transcribe.old.exe = %q, want the old one", got)
	}
	if _, err := os.Stat(newPath(exe)); !os.IsNotExist(err) {
		t.Errorf("the download is still there: %v", err)
	}
	if got := read(t, filepath.Join(dir, "data", "settings.json")); got != "{}" {
		t.Errorf("data/ changed: %q", got)
	}

	Cleanup(exe)
	if _, err := os.Stat(OldPath(exe)); !os.IsNotExist(err) {
		t.Errorf("transcribe.old.exe is still there: %v", err)
	}
	if got := read(t, exe); got != "new" {
		t.Errorf("exe after Cleanup = %q", got)
	}
}

func TestSwapPutsBack(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "transcribe.exe")
	write(t, exe, "old")
	// the download is missing: the second rename fails, the old EXE must be back in place
	if err := Swap(exe, newPath(exe)); err == nil {
		t.Fatal("Swap without a download succeeded")
	}
	if got := read(t, exe); got != "old" {
		t.Errorf("exe = %q, want the old one back", got)
	}
}

func TestOldPath(t *testing.T) {
	if got := OldPath(filepath.Join("x", "transcribe.exe")); got != filepath.Join("x", "transcribe.old.exe") {
		t.Errorf("OldPath = %q", got)
	}
}

// fakeGitHub serves a latest release with the given tag whose transcribe.exe is body.
func fakeGitHub(t *testing.T, tag string, body []byte, extra map[string]any) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			rel := map[string]any{
				"tag_name": tag, "html_url": srv.URL + "/page", "draft": false, "prerelease": false,
				"assets": []map[string]any{
					{"name": "notes.txt", "browser_download_url": srv.URL + "/notes", "size": 3},
					{"name": "transcribe.exe", "browser_download_url": srv.URL + "/exe", "size": len(body)},
				},
			}
			for k, v := range extra {
				rel[k] = v
			}
			_ = json.NewEncoder(w).Encode(rel)
		case "/exe":
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheck(t *testing.T) {
	ctx := context.Background()
	srv := fakeGitHub(t, "v1.1.0", []byte("MZ"), nil)
	found := []string{}
	u := &Updater{Current: "v1.0.0", API: srv.URL + "/latest", Found: func(v string) { found = append(found, v) }}
	if err := u.Check(ctx); err != nil {
		t.Fatal(err)
	}
	st := u.Status()
	if st.State != Available || st.Latest != "v1.1.0" || st.Page != srv.URL+"/page" || !st.Checks {
		t.Errorf("status = %+v", st)
	}
	_ = u.Check(ctx)
	if len(found) != 1 {
		t.Errorf("Found called %d times, want once per version", len(found))
	}

	u = &Updater{Current: "v1.1.0", API: srv.URL + "/latest"}
	_ = u.Check(ctx)
	if st := u.Status(); st.State != Current {
		t.Errorf("same version: status = %+v", st)
	}
	u = &Updater{Current: "v2.0.0", API: srv.URL + "/latest"}
	_ = u.Check(ctx)
	if st := u.Status(); st.State != Current {
		t.Errorf("newer than the release: status = %+v", st)
	}

	pre := fakeGitHub(t, "v9.0.0", []byte("MZ"), map[string]any{"prerelease": true})
	u = &Updater{Current: "v1.0.0", API: pre.URL + "/latest"}
	_ = u.Check(ctx)
	if st := u.Status(); st.State != Current {
		t.Errorf("pre-release offered: %+v", st)
	}

	// a development build never asks
	u = &Updater{Current: "dev-abc", API: srv.URL + "/latest"}
	if err := u.Check(ctx); err == nil || u.Status().Checks {
		t.Errorf("a development build looked: %+v", u.Status())
	}

	// offline: the state stays as it was, without an error to show
	u = &Updater{Current: "v1.0.0", API: "http://127.0.0.1:9/latest"}
	if err := u.Check(ctx); err == nil {
		t.Error("no error offline")
	}
	if st := u.Status(); st.State != Unknown || st.Error != "" {
		t.Errorf("offline: status = %+v", st)
	}
}

func TestInstall(t *testing.T) {
	body := []byte("MZ new program")
	srv := fakeGitHub(t, "v1.1.0", body, nil)
	dir := t.TempDir()
	exe := filepath.Join(dir, "transcribe.exe")
	write(t, exe, "MZ old program")

	idle := make(chan bool, 1)
	idle <- false // a job still runs at first
	restarted := make(chan struct{})
	u := &Updater{
		Current: "v1.0.0", API: srv.URL + "/latest", Exe: exe,
		Idle: func() bool {
			select {
			case v := <-idle:
				return v
			default:
				return true
			}
		},
		Restart: func() { close(restarted) },
		Verify: func(path, version string) error {
			if version != "v1.1.0" || read(t, path) != string(body) {
				t.Errorf("Verify(%q, %q)", path, version)
			}
			return nil
		},
	}
	if err := u.Install(); err == nil {
		t.Error("Install before a check found something")
	}
	if err := u.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := u.Install(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-restarted:
	case <-time.After(30 * time.Second):
		t.Fatalf("no restart; status = %+v", u.Status())
	}
	if got := read(t, exe); got != string(body) {
		t.Errorf("exe = %q", got)
	}
	if got := read(t, OldPath(exe)); got != "MZ old program" {
		t.Errorf("old exe = %q", got)
	}
	if st := u.Status(); st.State != Restarting {
		t.Errorf("status = %+v", st)
	}
}

func TestInstallRejectsNonProgram(t *testing.T) {
	srv := fakeGitHub(t, "v1.1.0", []byte("<html>"), nil)
	dir := t.TempDir()
	exe := filepath.Join(dir, "transcribe.exe")
	write(t, exe, "MZ old")
	u := &Updater{Current: "v1.0.0", API: srv.URL + "/latest", Exe: exe, Restart: func() { t.Error("restarted") }}
	_ = u.Check(context.Background())
	if err := u.Install(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for u.Status().State == Downloading && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if st := u.Status(); st.State != Failed || st.Error == "" {
		t.Errorf("status = %+v", st)
	}
	if got := read(t, exe); got != "MZ old" {
		t.Errorf("exe = %q", got)
	}
	if _, err := os.Stat(newPath(exe)); !os.IsNotExist(err) {
		t.Errorf("the download was left: %v", err)
	}
}

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

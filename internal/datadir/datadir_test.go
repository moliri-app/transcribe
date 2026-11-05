// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package datadir

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// home points Default into a temporary folder and returns a folder for an EXE.
func home(t *testing.T) (exeDir string) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("LOCALAPPDATA", filepath.Join(tmp, "local"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "local"))
	t.Setenv("HOME", filepath.Join(tmp, "home"))
	exeDir = filepath.Join(tmp, "app")
	_ = os.MkdirAll(exeDir, 0o755)
	return exeDir
}

func put(t *testing.T, path, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolve(t *testing.T) {
	exeDir := home(t)
	exe := filepath.Join(exeDir, "transcribe.exe")
	if got, _ := Resolve(exe); got != Default() {
		t.Fatalf("fresh: got %s, want %s", got, Default())
	}

	// an earlier version's data next to the EXE stays where it is, and is remembered
	legacy := filepath.Join(exeDir, "data")
	put(t, filepath.Join(legacy, "settings.json"), "{}")
	if got, _ := Resolve(exe); got != legacy {
		t.Fatalf("legacy: got %s, want %s", got, legacy)
	}
	// the EXE moves elsewhere: the data is still found
	if got, _ := Resolve(filepath.Join(t.TempDir(), "transcribe.exe")); got != legacy {
		t.Fatalf("moved EXE: got %s, want %s", got, legacy)
	}

	// the chosen folder is gone (a drive not connected), and nothing lies next to the EXE
	other := filepath.Join(t.TempDir(), "transcribe.exe")
	if err := os.RemoveAll(legacy); err != nil {
		t.Fatal(err)
	}
	if got, missing := Resolve(other); got != "" || missing != legacy {
		t.Fatalf("missing: got %q, %q", got, missing)
	}
	if err := Forget(); err != nil {
		t.Fatal(err)
	}
	if got, _ := Resolve(other); got != Default() {
		t.Fatalf("forgotten: got %s, want %s", got, Default())
	}
}

func TestTarget(t *testing.T) {
	home(t)
	current := filepath.Join(t.TempDir(), "data")
	put(t, filepath.Join(current, "settings.json"), "{}")

	if got, err := Target(current, ""); err != nil || got != Default() {
		t.Errorf("default: got %s, %v", got, err)
	}
	fresh := filepath.Join(t.TempDir(), "neu")
	if got, err := Target(current, fresh); err != nil || got != fresh {
		t.Errorf("new folder: got %s, %v", got, err)
	}
	full := t.TempDir()
	put(t, filepath.Join(full, "notes.txt"), "x")
	if got, err := Target(current, full); err != nil || got != filepath.Join(full, Name) {
		t.Errorf("folder with other files: got %s, %v", got, err)
	}
	put(t, filepath.Join(full, Name, "x.txt"), "x")
	if _, err := Target(current, full); err != ErrNotEmpty {
		t.Errorf("taken: got %v", err)
	}
	if _, err := Target(current, current); err != ErrSame {
		t.Errorf("same: got %v", err)
	}
	if _, err := Target(current, filepath.Join(current, "sub")); err != ErrInside {
		t.Errorf("inside: got %v", err)
	}
	if _, err := Target(current, "relativ"); err != ErrRelative {
		t.Errorf("relative: got %v", err)
	}
}

func TestMove(t *testing.T) {
	exeDir := home(t)
	from := filepath.Join(exeDir, "data")
	put(t, filepath.Join(from, "settings.json"), `{"a":1}`)
	put(t, filepath.Join(from, "models", "m", "model.onnx"), "0123456789")
	put(t, filepath.Join(from, "instance.json"), "{}")
	put(t, filepath.Join(from, "transcribe.log"), "log")

	to := filepath.Join(t.TempDir(), "neu")
	var done int64
	if err := Copy(context.Background(), from, to, func(n int64) { done = n }); err != nil {
		t.Fatal(err)
	}
	if want := Size(from); done != want || want != int64(len(`{"a":1}`)+10) {
		t.Errorf("progress %d, size %d", done, want)
	}
	if b, _ := os.ReadFile(filepath.Join(to, "models", "m", "model.onnx")); string(b) != "0123456789" {
		t.Errorf("model not copied: %q", b)
	}
	if _, err := os.Stat(filepath.Join(to, "instance.json")); err == nil {
		t.Error("instance.json copied")
	}

	if err := Moved(from, to); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(exeDir, "transcribe.exe")
	if got, _ := Resolve(exe); got != to {
		t.Fatalf("after the move: got %s, want %s", got, to)
	}
	Cleanup(to)
	if _, err := os.Stat(from); err == nil {
		t.Error("the old folder is still there")
	}

	// and back to the default folder, which keeps location.json
	if err := Copy(context.Background(), to, Default(), func(int64) {}); err != nil {
		t.Fatal(err)
	}
	if err := Moved(to, Default()); err != nil {
		t.Fatal(err)
	}
	if got, _ := Resolve(exe); got != Default() {
		t.Fatalf("back: got %s, want %s", got, Default())
	}
	Cleanup(Default())
	if _, err := os.Stat(to); err == nil {
		t.Error("the chosen folder is still there")
	}
	if b, _ := os.ReadFile(filepath.Join(Default(), "settings.json")); string(b) != `{"a":1}` {
		t.Errorf("settings: %q", b)
	}
}

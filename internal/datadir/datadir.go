// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package datadir finds the folder that holds the app's data (recordings, results, models,
// settings), and moves it to another one. It lies in the user's own app data
// (%LOCALAPPDATA%\Transcribe by Moliri on Windows), not next to the EXE, so the EXE can be moved
// or replaced without losing anything. The user may choose another folder in the settings;
// location.json in the default folder remembers it. Versions before kept the data in a "data"
// folder next to the EXE: such a folder is used where it is, and remembered, until the user moves
// it.
package datadir

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Name is the folder the data lies in, and the one made in a folder the user chose that holds
// other things.
const Name = "Transcribe by Moliri"

// Default is the folder that holds the data unless the user chose another.
func Default() string {
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, Name)
		}
	case "darwin":
		if d, err := os.UserConfigDir(); err == nil { // ~/Library/Application Support
			return filepath.Join(d, Name)
		}
	default:
		if d := os.Getenv("XDG_DATA_HOME"); d != "" {
			return filepath.Join(d, "transcribe")
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
			return filepath.Join(h, Name)
		}
		return filepath.Join(h, ".local", "share", "transcribe")
	}
	return "data"
}

// location is what location.json holds.
type location struct {
	DataDir string `json:"dataDir,omitempty"` // the folder chosen; empty: Default
	Remove  string `json:"remove,omitempty"`  // the folder the data was moved from, removed at the next start
}

const locationName = "location.json"

func locationFile() string { return filepath.Join(Default(), locationName) }

func read() (location, bool) {
	var l location
	b, err := os.ReadFile(locationFile())
	if err != nil || json.Unmarshal(b, &l) != nil {
		return l, false
	}
	return l, true
}

func write(l location) error {
	if err := os.MkdirAll(Default(), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(l, "", "  ")
	return os.WriteFile(locationFile(), b, 0o644)
}

// Resolve returns the data dir for the app at exe: the folder chosen, or the "data" folder next to
// exe of an earlier version, or Default. When the folder chosen is not there (a drive that is not
// connected) and no data lies next to exe, it returns "" and that folder as missing.
func Resolve(exe string) (dir, missing string) {
	legacy := filepath.Join(filepath.Dir(exe), "data")
	l, ok := read()
	switch {
	case ok && l.DataDir == "":
		return Default(), ""
	case ok && isDir(l.DataDir):
		return l.DataDir, ""
	case IsData(legacy):
		// an earlier version's data, or the EXE was moved together with the folder beside it
		l.DataDir = legacy
		_ = write(l)
		return legacy, ""
	case ok:
		return "", l.DataDir
	}
	return Default(), ""
}

// Forget drops the folder chosen, so the data lies in Default again.
func Forget() error {
	l, _ := read()
	l.DataDir = ""
	return write(l)
}

// IsData reports whether dir holds the app's data.
func IsData(dir string) bool {
	for _, n := range []string{"settings.json", "audio", "results", "models"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			return true
		}
	}
	return false
}

func isDir(dir string) bool {
	st, err := os.Stat(dir)
	return err == nil && st.IsDir()
}

// empty reports whether dir is missing or holds nothing (location.json aside).
func empty(dir string) bool {
	es, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	if err != nil {
		return false
	}
	for _, e := range es {
		if e.Name() != locationName {
			return false
		}
	}
	return true
}

// same reports whether a and b name the same folder.
func same(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// inside reports whether dir lies in parent (or is it).
func inside(dir, parent string) bool {
	rel, err := filepath.Rel(parent, dir)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		if r2, err := filepath.Rel(strings.ToLower(parent), strings.ToLower(dir)); err == nil {
			rel = r2
		}
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// Errors of Target, said to the user.
var (
	ErrRelative = errors.New("Bitte einen vollständigen Pfad angeben, etwa D:\\Transcribe.")
	ErrSame     = errors.New("Die Daten liegen schon dort.")
	ErrInside   = errors.New("Der neue Ort darf nicht im jetzigen Datenordner liegen.")
	ErrNotEmpty = errors.New("Dort liegen schon andere Dateien. Bitte einen leeren Ordner wählen.")
	ErrNotDir   = errors.New("Dort liegt eine Datei, kein Ordner.")
)

// Target is the folder the data in current moves to when the user chose dir: dir itself when it
// is new or empty, else a folder Name in it.
func Target(current, dir string) (string, error) {
	if dir == "" {
		dir = Default()
	}
	if !filepath.IsAbs(dir) {
		return "", ErrRelative
	}
	dir = filepath.Clean(dir)
	if st, err := os.Stat(dir); err == nil && !st.IsDir() {
		return "", ErrNotDir
	}
	if !empty(dir) && !same(dir, current) {
		dir = filepath.Join(dir, Name)
	}
	switch {
	case same(dir, current):
		return "", ErrSame
	case inside(dir, current):
		return "", ErrInside
	case !empty(dir):
		return "", ErrNotEmpty
	}
	return dir, nil
}

// Size is the size of what Copy copies from dir.
func Size(dir string) int64 {
	var n int64
	_ = walk(dir, func(_ string, d fs.DirEntry) {
		if info, err := d.Info(); err == nil {
			n += info.Size()
		}
	})
	return n
}

// skip are the files in the data dir that belong to the running app only.
var skip = map[string]bool{"instance.json": true, "transcribe.log": true, locationName: true}

// lenient are the folders whose files may be in use by another program (the app window's
// browser profile); what cannot be read there is left out.
const lenient = "browser-profile"

func walk(dir string, f func(rel string, d fs.DirEntry)) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		rel, _ := filepath.Rel(dir, p)
		if err != nil {
			if strings.HasPrefix(rel, lenient) {
				return nil
			}
			return err
		}
		if rel == "." {
			return nil
		}
		if !strings.Contains(rel, string(filepath.Separator)) && skip[rel] {
			return nil
		}
		if !d.IsDir() {
			f(rel, d)
		}
		return nil
	})
}

// Copy copies the data in from to the folder to, reporting the bytes copied so far. On an error
// or when ctx ends, it removes what it copied.
func Copy(ctx context.Context, from, to string, progress func(done int64)) error {
	var done int64
	var files []string
	err := walk(from, func(rel string, _ fs.DirEntry) { files = append(files, rel) })
	if err == nil {
		err = os.MkdirAll(to, 0o755)
	}
	for _, rel := range files {
		if err != nil {
			break
		}
		if err = ctx.Err(); err != nil {
			break
		}
		err = copyFile(filepath.Join(from, rel), filepath.Join(to, rel), func(n int64) {
			done += n
			progress(done)
		})
		if err != nil && strings.HasPrefix(rel, lenient) {
			err = nil
		}
	}
	if err != nil {
		removeContents(to)
		_ = os.Remove(to) // only when empty, and never Default, which holds location.json
	}
	return err
}

func copyFile(src, dst string, progress func(n int64)) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, err = io.CopyBuffer(&progressWriter{out, progress}, in, make([]byte, 1<<20))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		if st, serr := in.Stat(); serr == nil {
			_ = os.Chtimes(dst, st.ModTime(), st.ModTime())
		}
	}
	return err
}

type progressWriter struct {
	w io.Writer
	f func(n int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.f(int64(n))
	return n, err
}

// Moved remembers that the data now lies in to, and that from is to be removed at the next start.
func Moved(from, to string) error {
	l := location{Remove: from}
	if !same(to, Default()) {
		l.DataDir = to
	}
	return write(l)
}

// Cleanup removes the folder the data was moved from, unless it is current. What is still in use
// (the browser profile of a window still open) stays, and is tried again at the next start.
func Cleanup(current string) {
	l, ok := read()
	if !ok || l.Remove == "" || same(l.Remove, current) || inside(current, l.Remove) {
		return
	}
	if removeContents(l.Remove) == nil {
		if !same(l.Remove, Default()) {
			_ = os.Remove(l.Remove)
		}
		l.Remove = ""
		_ = write(l)
	}
}

// removeContents removes what dir holds, location.json aside.
func removeContents(dir string) error {
	es, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var first error
	for _, e := range es {
		if e.Name() == locationName {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil && first == nil {
			first = err
		}
	}
	return first
}

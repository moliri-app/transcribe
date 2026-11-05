// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package media

import (
	"crypto/sha1"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/moliri-app/transcribe/internal/prefs"
)

// Recordings in a connected folder (prefs.Folders) stay where they are. The app knows each by a
// name of its own, "~<hash of its path>~<file name>", which stands for a file in audio/ everywhere
// else: in the queue, under results/ and in the addresses of the pages.

// Linked is a recording in a connected folder.
type Linked struct {
	Name   string // the app's name for it (LinkedName)
	Path   string // where it is
	Folder string // the connected folder it is in
	Dir    string // the subfolder of Folder it is in, with "/" between folders; "" at the top
}

const hashLen = 10

// LinkedName is the app's name for the recording at path.
func LinkedName(path string) string {
	key := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	sum := sha1.Sum([]byte(key))
	return "~" + hex.EncodeToString(sum[:])[:hashLen] + "~" + filepath.Base(path)
}

// IsLinked reports whether name (or a stem) is the name of a recording in a connected folder.
func IsLinked(name string) bool {
	if len(name) < hashLen+3 || name[0] != '~' || name[hashLen+1] != '~' {
		return false
	}
	_, err := hex.DecodeString(name[1 : hashLen+1])
	return err == nil
}

// Title is how the pages name the recording of a stem: a linked one by its file name.
func Title(stem string) string {
	if IsLinked(stem) {
		return stem[hashLen+2:]
	}
	return stem
}

// skipDir reports whether a folder below a connected one is not searched for recordings: hidden
// ones, and the data folder of the app (a folder next to the EXE holds it).
func skipDir(path, name, dataDir string) bool {
	return strings.HasPrefix(name, ".") || prefs.SamePath(path, dataDir)
}

// maxLinked is how many recordings the connected folders give at most, so that connecting a whole
// drive by mistake does not stall the library.
const maxLinked = 5000

type scan struct {
	at    time.Time
	key   string
	items []Linked
}

var (
	scanMu sync.Mutex
	scans  = map[string]scan{}
)

// LinkedIn lists the recordings in the connected folders, also in their subfolders. A listing is
// kept for two seconds: the library and the queue ask often.
func LinkedIn(dataDir string) []Linked {
	folders := prefs.Load(dataDir).Folders
	key := strings.Join(folders, "\x00")
	scanMu.Lock()
	defer scanMu.Unlock()
	if s, ok := scans[dataDir]; ok && s.key == key && time.Since(s.at) < 2*time.Second {
		return s.items
	}
	abs, _ := filepath.Abs(dataDir)
	var items []Linked
	seen := map[string]bool{}
	for _, folder := range folders {
		_ = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
			if len(items) >= maxLinked {
				return fs.SkipAll
			}
			if err != nil || d == nil {
				return nil // a folder that cannot be read (or is gone) gives nothing
			}
			if d.IsDir() {
				if path != folder && skipDir(path, d.Name(), abs) {
					return filepath.SkipDir
				}
				return nil
			}
			if !IsMedia(d.Name()) || strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			dir, _ := filepath.Rel(folder, filepath.Dir(path))
			if dir == "." {
				dir = ""
			}
			name := LinkedName(path)
			if seen[name] { // in a folder connected twice, as itself and below another
				return nil
			}
			seen[name] = true
			items = append(items, Linked{Name: name, Path: path, Folder: folder, Dir: filepath.ToSlash(dir)})
			return nil
		})
	}
	scans[dataDir] = scan{time.Now(), key, items}
	return items
}

// Forget drops the kept listing of the connected folders, after they changed.
func Forget(dataDir string) {
	scanMu.Lock()
	delete(scans, dataDir)
	scanMu.Unlock()
}

// findLinked is the recording in a connected folder whose name or stem is s; looking again once
// if it is new.
func findLinked(dataDir, s string, stem bool) (Linked, bool) {
	for try := range 2 {
		if try == 1 {
			Forget(dataDir)
		}
		for _, l := range LinkedIn(dataDir) {
			if l.Name == s || stem && Stem(l.Name) == s {
				return l, true
			}
		}
	}
	return Linked{}, false
}

// Path is where the recording called name is: in audio/ or in a connected folder; "" if there is none.
func Path(dataDir, name string) string {
	if name == "" || strings.ContainsAny(name, `/\`) || !IsMedia(name) {
		return ""
	}
	if IsLinked(name) {
		l, _ := findLinked(dataDir, name, false)
		return l.Path
	}
	p := filepath.Join(AudioDir(dataDir), name)
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return ""
	}
	return p
}

// CountMedia counts the recordings in dir and the folders below it, down to depth levels, up to
// max; skipping hidden folders and the data folder.
func CountMedia(dir, dataDir string, depth, max int) int {
	abs, _ := filepath.Abs(dataDir)
	n := 0
	var walk func(d string, level int)
	walk = func(d string, level int) {
		entries, err := os.ReadDir(d)
		if err != nil {
			return
		}
		for _, e := range entries {
			if n >= max {
				return
			}
			p := filepath.Join(d, e.Name())
			switch {
			case e.IsDir():
				if level < depth && !skipDir(p, e.Name(), abs) {
					walk(p, level+1)
				}
			case IsMedia(e.Name()) && !strings.HasPrefix(e.Name(), "."):
				n++
			}
		}
	}
	walk(dir, 0)
	return n
}

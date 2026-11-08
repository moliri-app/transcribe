// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package shelf keeps what people say about their recordings in the library: a name of their own,
// tags, and whether the recording is hidden. None of it touches the recording itself, so it works
// alike for files in audio/ and in connected folders, where the app never renames or deletes
// anything. It is one file in the data dir, library.json, keyed by stem (the folder under results/).
package shelf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/moliri-app/transcribe/fileutil"
	"github.com/moliri-app/transcribe/internal/media"
)

// Entry is what the library knows of a recording beyond its file.
type Entry struct {
	Title  string   `json:"title,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Hidden bool     `json:"hidden,omitempty"`
}

func (e Entry) empty() bool { return e.Title == "" && len(e.Tags) == 0 && !e.Hidden }

type file struct {
	Recordings map[string]Entry `json:"recordings"`
}

const (
	maxTitle = 200 // bytes
	maxTag   = 60
	maxTags  = 30
)

var mu sync.Mutex

func path(dataDir string) string { return filepath.Join(dataDir, "library.json") }

func load(dataDir string) map[string]Entry {
	var f file
	if b, err := os.ReadFile(path(dataDir)); err == nil {
		_ = json.Unmarshal(b, &f)
	}
	if f.Recordings == nil {
		f.Recordings = map[string]Entry{}
	}
	return f.Recordings
}

func save(dataDir string, all map[string]Entry) error {
	b, err := fileutil.JSON(file{all}, "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteAtomic(path(dataDir), b)
}

// All returns every entry, by stem.
func All(dataDir string) map[string]Entry {
	mu.Lock()
	defer mu.Unlock()
	return load(dataDir)
}

// Get returns the entry of a stem (empty if there is none).
func Get(dataDir, stem string) Entry {
	return All(dataDir)[stem]
}

// Title is how the pages name the recording of a stem: by the name people gave it, else by its file.
func Title(dataDir, stem string) string {
	if t := Get(dataDir, stem).Title; t != "" {
		return t
	}
	return media.Title(stem)
}

// Update lets f change the entry of a stem, cleans it up and saves it; an entry left empty goes.
func Update(dataDir, stem string, f func(*Entry)) (Entry, error) {
	mu.Lock()
	defer mu.Unlock()
	all := load(dataDir)
	e := all[stem]
	f(&e)
	e = clean(e, stem)
	if e.empty() {
		delete(all, stem)
	} else {
		all[stem] = e
	}
	return e, save(dataDir, all)
}

// Forget drops the entry of a stem, when its recording is deleted.
func Forget(dataDir, stem string) error {
	mu.Lock()
	defer mu.Unlock()
	all := load(dataDir)
	if _, ok := all[stem]; !ok {
		return nil
	}
	delete(all, stem)
	return save(dataDir, all)
}

// clean trims the title and the tags; a title that only repeats the file's name is no title of its
// own, and a tag is there once (whatever its case), in the order given.
func clean(e Entry, stem string) Entry {
	e.Title = cut(oneLine(e.Title), maxTitle)
	if e.Title == media.Title(stem) {
		e.Title = ""
	}
	var tags []string
	for _, t := range e.Tags {
		t = cut(oneLine(t), maxTag)
		if t == "" || slices.ContainsFunc(tags, func(o string) bool { return strings.EqualFold(o, t) }) {
			continue
		}
		tags = append(tags, t)
		if len(tags) == maxTags {
			break
		}
	}
	e.Tags = tags
	return e
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return strings.TrimSpace(s[:n])
}

// FileName is Title made fit to name a file (an export): without the characters Windows forbids.
func FileName(dataDir, stem string) string {
	name := strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, Title(dataDir, stem))
	if name = strings.Trim(name, ". "); name == "" {
		return media.Title(stem)
	}
	return name
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package memo keeps what took long to find out (what ffprobe says of a recording, the summary of
// a transcript, the listing of the connected folders) in a file under <data>/cache, so that the
// library shows it at once on the next start too. Each value has a key and a stamp, mostly the
// size and time of the file it was read from: a value whose stamp no longer matches is not given.
package memo

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/moliri-app/transcribe/fileutil"
)

type entry struct {
	Stamp string          `json:"s"`
	Value json.RawMessage `json:"v"`
	Used  int64           `json:"u"` // unix day it was last given or kept
}

// Store is one such file. The zero path keeps the values in memory only.
type Store struct {
	path   string
	once   sync.Once
	mu     sync.Mutex
	m      map[string]entry
	timer  *time.Timer
	writes sync.Mutex // one write of the file at a time
}

// delay gathers the values found one after the other into one write of the file.
var delay = 2 * time.Second

// keep is how long a value nobody asked for stays: a recording gone from the library for that long
// takes its values with it.
const keep = 60 // days

var (
	storesMu sync.Mutex
	stores   = map[string]*Store{}
)

// In is the store called name in dataDir's cache folder, the same one for every caller.
func In(dataDir, name string) *Store {
	if dataDir == "" {
		return &Store{}
	}
	p := filepath.Join(dataDir, "cache", name+".json")
	storesMu.Lock()
	defer storesMu.Unlock()
	if s, ok := stores[p]; ok {
		return s
	}
	s := &Store{path: p}
	stores[p] = s
	return s
}

func (s *Store) load() {
	s.once.Do(func() {
		s.m = map[string]entry{}
		if s.path != "" {
			_ = fileutil.ReadJSON(s.path, &s.m) // none yet, or unreadable: found out anew
		}
		if s.m == nil {
			s.m = map[string]entry{}
		}
	})
}

func today() int64 { return time.Now().Unix() / 86400 }

// Get puts the value kept for key into v, if its stamp is stamp.
func (s *Store) Get(key, stamp string, v any) bool {
	s.load()
	s.mu.Lock()
	e, ok := s.m[key]
	if ok && e.Stamp == stamp && e.Used != today() {
		e.Used = today()
		s.m[key] = e
	}
	s.mu.Unlock()
	return ok && e.Stamp == stamp && json.Unmarshal(e.Value, v) == nil
}

// Put keeps v for key with stamp; the file is written a moment later.
func (s *Store) Put(key, stamp string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.load()
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.m[key]; ok && e.Stamp == stamp && string(e.Value) == string(b) {
		return
	}
	s.m[key] = entry{stamp, b, today()}
	if s.path != "" && s.timer == nil {
		s.timer = time.AfterFunc(delay, s.write)
	}
}

// Delete forgets the value kept for key.
func (s *Store) Delete(key string) {
	s.load()
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; !ok {
		return
	}
	delete(s.m, key)
	if s.path != "" && s.timer == nil {
		s.timer = time.AfterFunc(delay, s.write)
	}
}

func (s *Store) write() {
	s.writes.Lock()
	defer s.writes.Unlock()
	s.mu.Lock()
	s.timer = nil
	for k, e := range s.m {
		if today()-e.Used > keep {
			delete(s.m, k)
		}
	}
	b, err := json.Marshal(s.m)
	s.mu.Unlock()
	if err == nil {
		err = fileutil.WriteAtomic(s.path, b)
	}
	if err != nil {
		log.Println("cache:", err)
	}
}

// Flush writes what waits to be written, before the app ends.
func Flush() {
	storesMu.Lock()
	var due []*Store
	for _, s := range stores {
		s.mu.Lock()
		if s.timer != nil && s.timer.Stop() {
			due = append(due, s)
		}
		s.mu.Unlock()
	}
	storesMu.Unlock()
	for _, s := range due {
		s.write()
	}
}

// FileStamp is the stamp of the file at path: its size and when it was last changed; "" if there
// is none.
func FileStamp(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return Stamp(st)
}

// Stamp is FileStamp of a file already looked at.
func Stamp(st os.FileInfo) string {
	return fmt.Sprintf("%d/%d", st.Size(), st.ModTime().UnixNano())
}

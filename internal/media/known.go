// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package media

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/moliri-app/transcribe/internal/memo"
)

// What ffprobe says of a recording is kept (memo, <data>/cache/media.json) until the file changes:
// the library lists how long each recording is, its format and when it was recorded without
// waiting for ffprobe, which takes a moment per file and much longer on a slow drive.

// probes keeps what Probe found; in memory only until SetDataDir.
var probes atomic.Pointer[memo.Store]

func probeStore() *memo.Store {
	if s := probes.Load(); s != nil {
		return s
	}
	probes.CompareAndSwap(nil, &memo.Store{})
	return probes.Load()
}

var (
	failedMu sync.Mutex
	// failed are the files (path and stamp) ffprobe could not read in this run; not kept, since
	// ffprobe may only be missing until the first start downloaded it
	failed = map[string]string{}
)

// Probe reads what there is to know about the recording at path, from the cache when it has not
// changed since.
func Probe(path string) (*Info, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	key, stamp := probeKey(path), memo.Stamp(st)
	var info Info
	if probeStore().Get(key, stamp, &info) {
		return &info, nil
	}
	got, err := probe(path)
	failedMu.Lock()
	if err != nil {
		failed[key] = stamp
	} else {
		delete(failed, key)
	}
	failedMu.Unlock()
	if err != nil {
		return nil, err
	}
	probeStore().Put(key, stamp, got)
	return got, nil
}

// Known is what Probe found of the recording at path, if it did already and the file has not
// changed since (st is the file as just looked at). Otherwise info is nil and the file is probed in
// the background, for the next time: pending is false only if ffprobe could not read it.
func Known(path string, st os.FileInfo) (info *Info, pending bool) {
	key, stamp := probeKey(path), memo.Stamp(st)
	var i Info
	if probeStore().Get(key, stamp, &i) {
		return &i, false
	}
	failedMu.Lock()
	gaveUp := failed[key] == stamp
	failedMu.Unlock()
	if gaveUp {
		return nil, false
	}
	probeLater(path)
	return nil, true
}

// LooksLikeVideo is whether the file's name says it may hold a video; ffprobe decides (HasVideo).
func LooksLikeVideo(name string) bool { return videoExt[strings.ToLower(filepath.Ext(name))] }

func probeKey(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return path
}

// probers is how many files are probed at a time in the background.
const probers = 2

var (
	laterMu sync.Mutex
	later   []string
	waiting = map[string]bool{}
	running int
)

// probeLater probes the file at path in the background, once.
func probeLater(path string) {
	laterMu.Lock()
	defer laterMu.Unlock()
	if waiting[path] {
		return
	}
	waiting[path] = true
	later = append(later, path)
	if running < probers {
		running++
		go probeLoop()
	}
}

func probeLoop() {
	for {
		laterMu.Lock()
		if len(later) == 0 {
			running--
			laterMu.Unlock()
			return
		}
		path := later[0]
		later = later[1:]
		laterMu.Unlock()
		_, _ = Probe(path)
		laterMu.Lock()
		delete(waiting, path)
		laterMu.Unlock()
	}
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package sidecar keeps the transcript of a recording in a connected folder in a file next to it,
// "<recording>.transcribe.json" (as photo programs keep their edits in an .xmp next to the
// picture), so whoever else connects the folder (a Nextcloud shared by a team, say) finds the
// transcript, the corrections and the clips there. The app works on its copy in
// <data>/results/<stem>/ as on any other; Write puts it next to the recording, Read takes a
// sidecar that changed since (someone else worked on it) into the copy.
//
// No standard format holds what the app keeps apart: the machine's result with every word's time
// and certainty, the corrections by hand, and clips. WebVTT and the like keep only the finished
// text. The sidecar holds the app's own JSON, with the finished transcript under "final" for
// anyone who only wants to read it.
package sidecar

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/moliri-app/transcribe/fileutil"
	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/internal/results"
)

// Ext follows the full file name of the recording: "Interview.mp3.transcribe.json".
const Ext = ".transcribe.json"

// Format names the content of a sidecar.
const Format = "transcribe-by-moliri"

// File is the content of a sidecar. Final, Machine, Annotations and Retranscribed are
// _final.json, _machine.json, _annotations.json and _retranscribed.json of results/<stem>/.
type File struct {
	Format        string          `json:"format"`
	Version       int             `json:"version"`
	Audio         string          `json:"audio"` // the file name of the recording
	Saved         time.Time       `json:"saved"`
	Final         json.RawMessage `json:"final"`
	Machine       json.RawMessage `json:"machine"`
	Annotations   json.RawMessage `json:"annotations,omitempty"`
	Retranscribed json.RawMessage `json:"retranscribed,omitempty"`
}

// Path is the sidecar of the recording at audio.
func Path(audio string) string { return audio + Ext }

// parts are the files of results/<stem>/ a sidecar holds, the first one required.
var parts = []string{"_final.json", "_machine.json", "_annotations.json", "_retranscribed.json"}

// synced records the sidecar as the copy last wrote or read it, in results/<stem>/_sidecar.json:
// a sidecar that differs from it was written by someone else.
type synced struct {
	Size int64 `json:"size"`
	Mod  int64 `json:"mod"` // unix nanoseconds
}

func stamp(path string) (synced, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return synced{}, false
	}
	return synced{st.Size(), st.ModTime().UnixNano()}, true
}

func syncedPath(dataDir, stem string) string {
	return filepath.Join(results.Dir(dataDir, stem), "_sidecar.json")
}

// mu keeps a Write and a Read of the same recording from running at once.
var mu sync.Mutex

// Write puts the transcript of the linked recording stem next to it. Without a transcript it
// does nothing.
func Write(dataDir, stem string) error {
	mu.Lock()
	defer mu.Unlock()
	return write(dataDir, stem)
}

func write(dataDir, stem string) error {
	audio := media.FindMedia(dataDir, stem)
	if !media.IsLinked(stem) || audio == "" {
		return nil
	}
	d := results.Dir(dataDir, stem)
	f := File{Format: Format, Version: 1, Audio: filepath.Base(audio), Saved: time.Now().UTC()}
	fields := []*json.RawMessage{&f.Final, &f.Machine, &f.Annotations, &f.Retranscribed}
	for i, name := range parts {
		b, err := os.ReadFile(filepath.Join(d, name))
		if errors.Is(err, os.ErrNotExist) {
			if i == 0 {
				return nil
			}
			continue
		} else if err != nil {
			return err
		}
		*fields[i] = compact(b)
	}
	b, err := fileutil.JSON(f, "")
	if err != nil {
		return err
	}
	side := Path(audio)
	if err := fileutil.WriteAtomic(side, b); err != nil {
		return err
	}
	if s, ok := stamp(side); ok {
		return fileutil.WriteJSON(syncedPath(dataDir, stem), s)
	}
	return nil
}

func compact(b []byte) json.RawMessage {
	var buf bytes.Buffer
	if json.Compact(&buf, b) != nil {
		return b
	}
	return buf.Bytes()
}

// Read takes the sidecar of the linked recording stem into the app's copy if it changed since the
// copy last wrote or read it. Corrections the copy had are kept in _annotations_history/ first.
// It reports whether it took one.
func Read(dataDir, stem string) (bool, error) {
	audio := media.FindMedia(dataDir, stem)
	if !media.IsLinked(stem) || audio == "" {
		return false, nil
	}
	return read(dataDir, stem, audio)
}

// unreadable are the sidecars (path and stamp) that held no transcript of the app, so that each
// is reported once and not at every look.
var unreadable sync.Map

func read(dataDir, stem, audio string) (bool, error) {
	mu.Lock()
	defer mu.Unlock()
	side := Path(audio)
	now, ok := stamp(side)
	if !ok {
		return false, nil
	}
	var last synced
	if fileutil.ReadJSON(syncedPath(dataDir, stem), &last) == nil && last == now {
		return false, nil
	}
	if bad, ok := unreadable.Load(side); ok && bad.(synced) == now {
		return false, nil
	}
	var f File
	err := fileutil.ReadJSON(side, &f)
	if err == nil && (f.Format != Format || len(f.Final) == 0 || len(f.Machine) == 0) {
		err = errors.New("not a transcript of Transcribe")
	}
	if err != nil {
		unreadable.Store(side, now)
		return false, fmt.Errorf("%s: %w", side, err)
	}
	d := results.Dir(dataDir, stem)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return false, err
	}
	keepAnnotations(d, f.Annotations)
	for i, raw := range []json.RawMessage{f.Final, f.Machine, f.Annotations, f.Retranscribed} {
		p := filepath.Join(d, parts[i])
		if len(raw) == 0 {
			_ = os.Remove(p)
			continue
		}
		if err := fileutil.WriteAtomic(p, raw); err != nil {
			return false, err
		}
	}
	return true, fileutil.WriteJSON(syncedPath(dataDir, stem), now)
}

// keepAnnotations saves the corrections in d to _annotations_history/ before the sidecar's
// replace them, unless they are the same.
func keepAnnotations(d string, next json.RawMessage) {
	old, err := os.ReadFile(filepath.Join(d, "_annotations.json"))
	if err != nil || bytes.Equal(compact(old), compact(next)) {
		return
	}
	name := time.Now().Format("20060102-150405") + "-vor-abgleich.json"
	_ = fileutil.WriteAtomic(filepath.Join(d, "_annotations_history", name), old)
}

// Sync brings the copy and the sidecar of the linked recording stem at audio together: a sidecar
// someone else changed is read; with write (sidecars are written), a transcript without one gets one.
func Sync(dataDir, stem, audio string, write bool) {
	took, err := read(dataDir, stem, audio)
	if err != nil {
		log.Println("sidecar:", err)
		return
	}
	if took || !write {
		return
	}
	if _, err := os.Stat(Path(audio)); errors.Is(err, os.ErrNotExist) {
		if _, err := os.Stat(filepath.Join(results.Dir(dataDir, stem), parts[0])); err == nil {
			Schedule(dataDir, stem)
		}
	}
}

var (
	timersMu sync.Mutex
	timers   = map[string]*time.Timer{}
)

// delay gathers the saves of the editor (corrections, then the finished transcript) into one
// write of the sidecar.
var delay = 1500 * time.Millisecond

// Schedule writes the sidecar of a linked recording a moment from now, if sidecars are written.
func Schedule(dataDir, stem string) {
	if !media.IsLinked(stem) || prefs.Load(dataDir).SidecarOff {
		return
	}
	key := dataDir + "\x00" + stem
	timersMu.Lock()
	defer timersMu.Unlock()
	if t, ok := timers[key]; ok {
		t.Stop()
	}
	timers[key] = time.AfterFunc(delay, func() {
		timersMu.Lock()
		delete(timers, key)
		timersMu.Unlock()
		if err := Write(dataDir, stem); err != nil {
			log.Println("sidecar:", err)
		}
	})
}

// Flush writes the sidecars still waiting for their moment, before the app ends.
func Flush() {
	timersMu.Lock()
	pending := timers
	timers = map[string]*time.Timer{}
	timersMu.Unlock()
	for key, t := range pending {
		if t.Stop() {
			dataDir, stem, _ := strings.Cut(key, "\x00")
			if err := Write(dataDir, stem); err != nil {
				log.Println("sidecar:", err)
			}
		}
	}
}

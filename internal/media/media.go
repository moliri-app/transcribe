// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package media finds recordings in <data>/audio and the connected folders and derives waveform peaks, video thumbnails and PCM via ffmpeg.
package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moliri-app/transcribe/audio"
	"github.com/moliri-app/transcribe/proc"
)

var (
	audioExt = map[string]bool{".mp3": true, ".wav": true, ".m4a": true, ".ogg": true, ".opus": true, ".flac": true, ".aac": true, ".wma": true}
	// .webm/.mp4 may hold audio only: ffprobe decides
	videoExt = map[string]bool{".mp4": true, ".m4v": true, ".mov": true, ".webm": true, ".mkv": true, ".avi": true}
)

// SetDataDir makes <dir>/bin a place where ffmpeg and ffprobe are looked up (after PATH and the executable's folder).
func SetDataDir(dir string) { audio.SetToolDir(filepath.Join(dir, "bin")) }

// IsMedia reports whether the file name has an extension we accept as input.
func IsMedia(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return audioExt[ext] || videoExt[ext]
}

// Stem is the file name without its last extension.
func Stem(name string) string { return strings.TrimSuffix(name, filepath.Ext(name)) }

func AudioDir(dataDir string) string { return filepath.Join(dataDir, "audio") }

func ResultsDir(dataDir string) string { return filepath.Join(dataDir, "results") }

// FindMedia returns the path of the recording whose stem is `stem`, or "".
func FindMedia(dataDir, stem string) string {
	if IsLinked(stem) {
		l, _ := findLinked(dataDir, stem, true)
		return l.Path
	}
	entries, err := os.ReadDir(AudioDir(dataDir))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && IsMedia(e.Name()) && Stem(e.Name()) == stem {
			return filepath.Join(AudioDir(dataDir), e.Name())
		}
	}
	return ""
}

// Tool returns the path of ffmpeg/ffprobe (PATH, next to the executable, or <data>/bin).
func Tool(name string) string { return audio.Tool(name) }

func tool(name string) string { return audio.Tool(name) }

type hasVideoKey struct {
	path  string
	mtime time.Time
}

var (
	videoMu    sync.Mutex
	videoCache = map[hasVideoKey]bool{}
)

// HasVideo reports whether the file has a real video stream (cover art in an mp3 does not count).
func HasVideo(path string) bool {
	if !videoExt[strings.ToLower(filepath.Ext(path))] {
		return false
	}
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	key := hasVideoKey{path, st.ModTime()}
	videoMu.Lock()
	v, ok := videoCache[key]
	videoMu.Unlock()
	if ok {
		return v
	}
	v = probeVideo(path)
	videoMu.Lock()
	videoCache[key] = v
	videoMu.Unlock()
	return v
}

func probeVideo(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := proc.Hide(exec.CommandContext(ctx, tool("ffprobe"), "-v", "error", "-select_streams", "v", "-show_entries",
		"stream=index:stream_disposition=attached_pic", "-of", "json", path)).Output()
	if err != nil {
		return false
	}
	var res struct {
		Streams []struct {
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if json.Unmarshal(out, &res) != nil {
		return false
	}
	for _, s := range res.Streams {
		if s.Disposition.AttachedPic == 0 {
			return true
		}
	}
	return false
}

// Duration of the file in seconds.
func Duration(path string) (float64, error) {
	out, err := run(exec.Command(tool("ffprobe"), "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path))
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

// run executes cmd and puts the tail of stderr into the error.
func run(cmd *exec.Cmd) ([]byte, error) {
	proc.Hide(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 500 {
			msg = msg[len(msg)-500:]
		}
		return nil, fmt.Errorf("%s: %w: %s", filepath.Base(cmd.Path), err, msg)
	}
	return out, nil
}

var (
	lockMu sync.Mutex
	locks  = map[string]*sync.Mutex{}
)

func stemLock(kind, stem string) *sync.Mutex {
	lockMu.Lock()
	defer lockMu.Unlock()
	k := kind + "\x00" + stem
	if locks[k] == nil {
		locks[k] = new(sync.Mutex)
	}
	return locks[k]
}

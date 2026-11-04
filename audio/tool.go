// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package audio

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
)

var toolDir atomic.Value

// SetToolDir makes dir a place where ffmpeg and ffprobe are looked up (after PATH and the
// executable's folder); the desktop app downloads them into <data>/bin.
func SetToolDir(dir string) { toolDir.Store(dir) }

// Tool returns the path of ffmpeg/ffprobe: PATH, next to the executable, or the SetToolDir folder;
// the bare name if none has it.
func Tool(name string) string {
	if p, ok := found.Load(name); ok {
		if st, err := os.Stat(p.(string)); err == nil && !st.IsDir() {
			return p.(string)
		}
		found.Delete(name)
	}
	p := lookTool(name)
	if p != name {
		found.Store(name, p)
	}
	return p
}

// found remembers where a tool was found: searching a long PATH takes seconds on some Windows
// machines, and the settings page asks every two seconds. Kept as long as the file is there.
var found sync.Map

func lookTool(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	file := name
	if runtime.GOOS == "windows" {
		file += ".exe"
	}
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	if d, ok := toolDir.Load().(string); ok && d != "" {
		dirs = append(dirs, d)
	}
	for _, d := range dirs {
		p := filepath.Join(d, file)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return name
}

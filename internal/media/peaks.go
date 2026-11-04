// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package media

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/moliri-app/transcribe/fileutil"
	"github.com/moliri-app/transcribe/proc"
)

const (
	sampleRate = 16000
	peakStep   = 0.025 // seconds per waveform bin
)

// ErrNoMedia means no recording with that stem exists.
var ErrNoMedia = errors.New("media not found")

// DecodePCM16k decodes anything ffmpeg understands to 16 kHz mono float32 in [-1, 1).
func DecodePCM16k(path string) ([]float32, error) {
	out, err := run(exec.Command(tool("ffmpeg"), "-nostdin", "-v", "error", "-i", path, "-f", "s16le", "-ac", "1", "-ar", "16000", "-"))
	if err != nil {
		return nil, err
	}
	x := make([]float32, len(out)/2)
	for i := range x {
		x[i] = float32(int16(binary.LittleEndian.Uint16(out[2*i:]))) / 32768
	}
	return x, nil
}

// Peaks returns the JSON of the waveform envelope for the editor timeline: the max |amplitude| per 25 ms as
// 0..255, computed once and cached in results/<stem>/_peaks.json. ErrNoMedia if there is nothing to compute from.
func Peaks(dataDir, stem string) ([]byte, error) {
	cache := filepath.Join(ResultsDir(dataDir), stem, "_peaks.json")
	lock := stemLock("peaks", stem)
	lock.Lock()
	defer lock.Unlock()
	if b, err := os.ReadFile(cache); err == nil {
		return b, nil
	}
	src := FindMedia(dataDir, stem)
	if src == "" {
		return nil, ErrNoMedia
	}
	env, total, err := envelope(src)
	if err != nil {
		return nil, err
	}
	var top float32
	for _, v := range env {
		top = max(top, v)
	}
	peaks := make([]int, len(env))
	for i, v := range env {
		// sqrt: quiet speech stays visible next to loud music
		peaks[i] = int(float32(math.Sqrt(float64(v/(top+1e-9)))) * 255)
	}
	b, err := fileutil.JSON(map[string]any{"step": peakStep, "duration": float64(total) / sampleRate, "peaks": peaks}, "")
	if err != nil {
		return nil, err
	}
	if err := fileutil.WriteAtomic(cache, b); err != nil {
		return nil, err
	}
	return b, nil
}

// envelope streams the decoded audio and returns the max |sample| per bin and the total sample count.
func envelope(src string) ([]float32, int, error) {
	cmd := proc.Hide(exec.Command(tool("ffmpeg"), "-nostdin", "-v", "error", "-i", src, "-f", "s16le", "-ac", "1", "-ar", "16000", "-"))
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, err
	}
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	const bin = int(sampleRate * peakStep)
	r := bufio.NewReaderSize(pipe, 1<<20)
	buf := make([]byte, bin*2)
	var env []float32
	total := 0
	for {
		n, err := io.ReadFull(r, buf)
		total += n / 2
		if n == len(buf) {
			var m float32
			for i := 0; i < bin; i++ {
				v := int(int16(binary.LittleEndian.Uint16(buf[2*i:])))
				if v < 0 {
					v = -v
				}
				m = max(m, float32(v)/32768)
			}
			env = append(env, m)
		}
		if err != nil {
			break
		}
	}
	io.Copy(io.Discard, pipe)
	if err := cmd.Wait(); err != nil {
		return nil, 0, err
	}
	return env, total, nil
}

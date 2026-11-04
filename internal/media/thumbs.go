// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package media

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/moliri-app/transcribe/internal/fileutil"
)

const (
	thumbW, thumbH = 160, 90
	thumbCols      = 20
	thumbMax       = 600 // frames per sprite: the step grows with the recording
)

// ThumbMeta describes the sprite results/<stem>/_thumbs.jpg.
type ThumbMeta struct {
	Step     int     `json:"step"`
	W        int     `json:"w"`
	H        int     `json:"h"`
	Cols     int     `json:"cols"`
	Rows     int     `json:"rows"`
	Count    int     `json:"count"`
	File     string  `json:"file"`
	Duration float64 `json:"duration"`
}

// Thumbs returns the sprite meta for a video, building the sprite once. nil, nil means there is no video.
func Thumbs(dataDir, stem string) (*ThumbMeta, error) {
	src := FindMedia(dataDir, stem)
	if src == "" || !HasVideo(src) {
		return nil, nil
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	rdir := filepath.Join(ResultsDir(dataDir), stem)
	metaFile, sprite := filepath.Join(rdir, "_thumbs.json"), filepath.Join(rdir, "_thumbs.jpg")

	lock := stemLock("thumbs", stem)
	lock.Lock()
	defer lock.Unlock()
	if st, err := os.Stat(sprite); err == nil && !st.ModTime().Before(srcInfo.ModTime()) {
		var meta ThumbMeta
		if fileutil.ReadJSON(metaFile, &meta) == nil {
			return &meta, nil
		}
	}
	if err := os.MkdirAll(rdir, 0o755); err != nil {
		return nil, err
	}
	duration, err := Duration(src)
	if err != nil {
		return nil, err
	}
	step := max(2, int(math.Ceil(duration/thumbMax)))
	count := int(duration/float64(step)) + 1
	rows := (count + thumbCols - 1) / thumbCols
	// keyframes only: seconds instead of minutes for a long recording
	vf := "fps=1/" + strconv.Itoa(step) + ",scale=" + strconv.Itoa(thumbW) + ":" + strconv.Itoa(thumbH) +
		":force_original_aspect_ratio=decrease,pad=" + strconv.Itoa(thumbW) + ":" + strconv.Itoa(thumbH) +
		":(ow-iw)/2:(oh-ih)/2,tile=" + strconv.Itoa(thumbCols) + "x" + strconv.Itoa(rows)
	tmp := filepath.Join(rdir, "_thumbs.tmp.jpg")
	_, err = run(exec.Command(tool("ffmpeg"), "-nostdin", "-y", "-v", "fatal", "-skip_frame", "nokey", "-i", src, "-an", "-sn",
		"-vf", vf, "-frames:v", "1", "-q:v", "6", tmp))
	if err != nil {
		os.Remove(tmp)
		return nil, err
	}
	if err := os.Rename(tmp, sprite); err != nil {
		return nil, err
	}
	meta := ThumbMeta{Step: step, W: thumbW, H: thumbH, Cols: thumbCols, Rows: rows, Count: count,
		File: "results/" + stem + "/_thumbs.jpg", Duration: duration}
	b, _ := json.Marshal(meta)
	if err := fileutil.WriteAtomic(metaFile, b); err != nil {
		return nil, err
	}
	return &meta, nil
}

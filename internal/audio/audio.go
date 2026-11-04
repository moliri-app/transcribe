// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package audio decodes any media file to 16 kHz mono float32 through ffmpeg.
package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os/exec"

	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/proc"
)

const SR = 16000

func Decode(ctx context.Context, path string) ([]float32, error) {
	var out, errb bytes.Buffer
	cmd := proc.Hide(exec.CommandContext(ctx, media.Tool("ffmpeg"), "-nostdin", "-v", "error", "-i", path, "-f", "s16le", "-ac", "1", "-ar", "16000", "-"))
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, errb.String())
	}
	b := out.Bytes()
	pcm := make([]float32, len(b)/2)
	for i := range pcm {
		pcm[i] = float32(int16(binary.LittleEndian.Uint16(b[2*i:]))) / 32768
	}
	return pcm, nil
}

// Slice returns pcm[a:b] in seconds, clamped.
func Slice(pcm []float32, a, b float64) []float32 {
	i, j := int(a*SR), int(b*SR)
	i = max(0, min(i, len(pcm)))
	j = max(i, min(j, len(pcm)))
	return pcm[i:j]
}

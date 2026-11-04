// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package media

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Range is a stretch of a recording in seconds.
type Range struct{ A, B float64 }

const clipFade = 0.02 // seconds faded in and out at every cut, so the joins do not click

// Cut writes the ranges of src one after the other into out: an MP3, or an MP4 (H.264/AAC) when video is set.
// Every range is its own input seeked by ffmpeg, so a clip late in a long recording costs no more than one at the start.
func Cut(ctx context.Context, src string, ranges []Range, video bool, out string) error {
	if len(ranges) == 0 {
		return errors.New("no ranges")
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	var chains, joins []string
	for i, r := range ranges {
		d := r.B - r.A
		if d <= 0 {
			return fmt.Errorf("empty range %.2f–%.2f", r.A, r.B)
		}
		args = append(args, "-ss", sec(r.A), "-t", sec(d), "-i", src)
		fade := clipFade
		if d < 4*fade {
			fade = d / 4
		}
		chains = append(chains, fmt.Sprintf("[%d:a]asetpts=PTS-STARTPTS,afade=t=in:d=%s,afade=t=out:st=%s:d=%s[a%d]",
			i, sec(fade), sec(d-fade), sec(fade), i))
		if video {
			// x264 with yuv420p needs even frame sizes
			chains = append(chains, fmt.Sprintf("[%d:v]setpts=PTS-STARTPTS,scale=trunc(iw/2)*2:trunc(ih/2)*2,format=yuv420p[v%d]", i, i))
			joins = append(joins, fmt.Sprintf("[v%d][a%d]", i, i))
		} else {
			joins = append(joins, fmt.Sprintf("[a%d]", i))
		}
	}
	v, outs, tail := 0, "[a]", []string{"-map", "[a]", "-c:a", "libmp3lame", "-q:a", "2"}
	if video {
		v, outs = 1, "[v][a]"
		tail = []string{"-map", "[v]", "-map", "[a]", "-c:v", "libx264", "-preset", "veryfast", "-crf", "20",
			"-c:a", "aac", "-b:a", "160k", "-movflags", "+faststart"}
	}
	graph := strings.Join(chains, ";") + ";" + strings.Join(joins, "") + fmt.Sprintf("concat=n=%d:v=%d:a=1%s", len(ranges), v, outs)
	args = append(append(args, "-filter_complex", graph), tail...)
	args = append(args, out)
	_, err := run(exec.CommandContext(ctx, tool("ffmpeg"), args...))
	return err
}

func sec(x float64) string { return strconv.FormatFloat(x, 'f', 3, 64) }

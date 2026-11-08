// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package media

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/moliri-app/transcribe/proc"
)

// Info is what ffprobe tells about a recording, for the library's details. Fields it does not know
// stay empty.
type Info struct {
	Format     string  `json:"format"`     // the container, in words ("QuickTime / MOV")
	Duration   float64 `json:"duration"`   // seconds
	Bitrate    int     `json:"bitrate"`    // bits per second, of the whole file
	Recorded   string  `json:"recorded"`   // when it was recorded, as the file says (creation_time), RFC 3339
	Title      string  `json:"title"`      // what the file's own tags call it
	Artist     string  `json:"artist"`     // and who made it
	Codec      string  `json:"codec"`      // of the (first) audio stream
	SampleRate int     `json:"sampleRate"` // Hz
	Channels   int     `json:"channels"`
	Video      *Video  `json:"video,omitempty"` // a real video stream (cover art does not count)
}

// Video is the picture of a recording that has one.
type Video struct {
	Codec  string  `json:"codec"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	FPS    float64 `json:"fps"`
}

// Probe reads what there is to know about the recording at path.
func Probe(path string) (*Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := proc.Hide(exec.CommandContext(ctx, tool("ffprobe"), "-v", "error",
		"-show_entries", "format=format_long_name,duration,bit_rate:format_tags=creation_time,title,artist"+
			":stream=codec_type,codec_long_name,codec_name,sample_rate,channels,width,height,avg_frame_rate:stream_disposition=attached_pic",
		"-of", "json", path)).Output()
	if err != nil {
		return nil, err
	}
	var res struct {
		Format struct {
			LongName string            `json:"format_long_name"`
			Duration string            `json:"duration"`
			BitRate  string            `json:"bit_rate"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			Type        string `json:"codec_type"`
			Name        string `json:"codec_name"`
			LongName    string `json:"codec_long_name"`
			SampleRate  string `json:"sample_rate"`
			Channels    int    `json:"channels"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			FrameRate   string `json:"avg_frame_rate"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, err
	}
	tag := func(k string) string {
		for key, v := range res.Format.Tags { // the case of the keys differs between containers
			if strings.EqualFold(key, k) {
				return strings.TrimSpace(v)
			}
		}
		return ""
	}
	info := &Info{Format: res.Format.LongName, Recorded: tag("creation_time"), Title: tag("title"), Artist: tag("artist")}
	info.Duration, _ = strconv.ParseFloat(res.Format.Duration, 64)
	info.Bitrate, _ = strconv.Atoi(res.Format.BitRate)
	for _, s := range res.Streams {
		switch {
		case s.Type == "audio" && info.Codec == "":
			info.Codec = codecName(s.Name, s.LongName)
			info.SampleRate, _ = strconv.Atoi(s.SampleRate)
			info.Channels = s.Channels
		case s.Type == "video" && s.Disposition.AttachedPic == 0 && info.Video == nil:
			info.Video = &Video{Codec: codecName(s.Name, s.LongName), Width: s.Width, Height: s.Height, FPS: rate(s.FrameRate)}
		}
	}
	return info, nil
}

// codecName is the short name people know ("AAC", "MP3", "H.264"), else ffprobe's long one.
func codecName(short, long string) string {
	known := map[string]string{
		"aac": "AAC", "mp3": "MP3", "opus": "Opus", "vorbis": "Vorbis", "flac": "FLAC", "alac": "ALAC",
		"pcm_s16le": "PCM 16 bit", "pcm_s24le": "PCM 24 bit", "pcm_s32le": "PCM 32 bit", "pcm_f32le": "PCM 32 bit float",
		"wmav2": "WMA", "amr_nb": "AMR", "h264": "H.264", "hevc": "H.265", "vp8": "VP8", "vp9": "VP9", "av1": "AV1",
	}
	if n, ok := known[short]; ok {
		return n
	}
	return long
}

// rate turns ffprobe's "30000/1001" into frames per second.
func rate(s string) float64 {
	a, b, ok := strings.Cut(s, "/")
	n, err1 := strconv.ParseFloat(a, 64)
	if !ok {
		return n
	}
	d, err2 := strconv.ParseFloat(b, 64)
	if err1 != nil || err2 != nil || d == 0 {
		return 0
	}
	return n / d
}

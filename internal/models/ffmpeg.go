// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package models

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/moliri-app/transcribe/internal/proc"
)

var ffmpegMu sync.Mutex

func ffmpegURL() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip", nil
	case "linux/amd64":
		return "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz", nil
	case "linux/arm64":
		return "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz", nil
	}
	return "", fmt.Errorf("no automatic ffmpeg download for %s/%s: install ffmpeg (e.g. brew install ffmpeg)", runtime.GOOS, runtime.GOARCH)
}

// EnsureFFmpeg puts ffmpeg and ffprobe into dir unless they are already there or on PATH.
func EnsureFFmpeg(ctx context.Context, dir string, progress func(string)) error {
	ffmpegMu.Lock()
	defer ffmpegMu.Unlock()
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		if _, err := exec.LookPath("ffprobe"); err == nil {
			return nil
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "ffmpeg"+ext)); err == nil {
		return nil
	}
	url, err := ffmpegURL()
	if err != nil {
		return err
	}
	if progress != nil {
		progress("downloading ffmpeg")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	archive := filepath.Join(dir, "ffmpeg-download.tmp")
	defer os.Remove(archive)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, counted("ffmpeg", resp.Body, resp.ContentLength)); err != nil {
		f.Close()
		return err
	}
	f.Close()
	if strings.HasSuffix(url, ".zip") {
		return unzipBinaries(archive, dir, ext)
	}
	cmd := proc.Hide(exec.CommandContext(ctx, "tar", "-xJf", archive, "-C", dir, "--strip-components=1", "--wildcards", "*/ffmpeg", "*/ffprobe"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar: %w: %s", err, out)
	}
	return nil
}

func unzipBinaries(archive, dir, ext string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	found := 0
	for _, zf := range zr.File {
		base := filepath.Base(zf.Name)
		if base != "ffmpeg"+ext && base != "ffprobe"+ext {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(filepath.Join(dir, base), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
		found++
	}
	if found < 2 {
		return fmt.Errorf("ffmpeg/ffprobe not found in archive")
	}
	return nil
}

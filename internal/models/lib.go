// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package models

import (
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

var libMu sync.Mutex

const SherpaVersion = "1.13.8"

func libName() string {
	switch runtime.GOOS {
	case "windows":
		return "sherpa-onnx-c-api.dll"
	case "darwin":
		return "libsherpa-onnx-c-api.dylib"
	}
	return "libsherpa-onnx-c-api.so"
}

func libArchive() (string, error) {
	v := "sherpa-onnx-v" + SherpaVersion + "-"
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return v + "win-x64-shared-MD-Release-no-tts", nil
	case "windows/arm64":
		return v + "win-arm64-shared-MD-Release-no-tts", nil
	case "linux/amd64":
		return v + "linux-x64-shared-no-tts", nil
	case "linux/arm64":
		return v + "linux-aarch64-shared-cpu", nil
	case "darwin/amd64", "darwin/arm64":
		return v + "osx-universal2-shared-no-tts", nil
	}
	return "", fmt.Errorf("no sherpa-onnx build for %s/%s", runtime.GOOS, runtime.GOARCH)
}

const onnxruntimeVersion = "1.28.2"

func cudaLibArchive(major int) (string, error) {
	if !cudaArchiveSupported() {
		return "", fmt.Errorf("no sherpa-onnx CUDA build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	suffix := "linux-x64-gpu"
	if runtime.GOOS == "windows" {
		suffix = "win-x64-cuda"
	}
	return fmt.Sprintf("sherpa-onnx-v%s-cuda-%d.x-cudnn-9.x-onnxruntime%s-%s", SherpaVersion, major, onnxruntimeVersion, suffix), nil
}

// CudaLibDir is where the CUDA build of the runtime lives.
func CudaLibDir(dataDir string) string { return filepath.Join(dataDir, "lib-cuda") }

// FindLib returns the first directory that contains the sherpa-onnx C API library.
func FindLib(dirs ...string) string {
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(d, libName())); err == nil {
			return d
		}
	}
	return ""
}

// EnsureLib downloads the sherpa-onnx shared libraries for this platform into dir.
func EnsureLib(ctx context.Context, dir string, progress func(string)) error {
	if FindLib(dir) != "" {
		return nil
	}
	name, err := libArchive()
	if err != nil {
		return err
	}
	return installLib(ctx, "runtime", name, dir)
}

// EnsureCudaLib downloads the CUDA build of the runtime (several hundred MB) into dataDir/lib-cuda.
func EnsureCudaLib(ctx context.Context, dataDir string, major int) error {
	dir := CudaLibDir(dataDir)
	if FindLib(dir) != "" {
		return nil
	}
	name, err := cudaLibArchive(major)
	if err != nil {
		return err
	}
	return installLib(ctx, "runtime-cuda", name, dir)
}

func installLib(ctx context.Context, track, name, dir string) error {
	libMu.Lock()
	defer libMu.Unlock()
	if FindLib(dir) != "" {
		return nil
	}
	url := fmt.Sprintf("%sv%s/%s.tar.bz2", base, SherpaVersion, name)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	tmp, err := os.MkdirTemp("", "sherpa-lib")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	cmd := proc.Hide(exec.CommandContext(ctx, "tar", "-xjf", "-", "-C", tmp))
	cmd.Stdin = counted(track, resp.Body, resp.ContentLength)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar: %w: %s", err, out)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	src := ""
	_ = filepath.WalkDir(tmp, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == libName() && src == "" {
			src = filepath.Dir(p)
		}
		return nil
	})
	if src == "" {
		return fmt.Errorf("%s not found in %s", libName(), name)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !(strings.HasSuffix(n, ".dll") || strings.Contains(n, ".so") || strings.HasSuffix(n, ".dylib")) {
			continue
		}
		if err := copyFile(filepath.Join(src, n), filepath.Join(dir, n)); err != nil {
			return err
		}
	}
	if FindLib(dir) == "" {
		return fmt.Errorf("%s not found in %s", libName(), name)
	}
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

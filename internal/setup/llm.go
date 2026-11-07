// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package setup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/moliri-app/transcribe/models"
	"github.com/moliri-app/transcribe/proc"
)

// The language model for Moliri's filter in plain words (docs/filter-api.md) is not shipped
// either: Qwen 2.5 7B Instruct (4-bit, Qwen's own GGUF, 4.7 GB) and llama.cpp's server program
// (25 MB) come when Moliri first asks or on the settings page, each pinned by its checksum. The
// server runs as a child process of the app (internal/llm).

// LLMDir holds the server program (llama/) and the model.
func LLMDir(dataDir string) string { return filepath.Join(dataDir, "llm") }

// LlamaDir holds llama.cpp's server and its libraries.
func LlamaDir(dataDir string) string { return filepath.Join(LLMDir(dataDir), "llama") }

// LLMModel is the model's first file; llama.cpp finds the second one beside it.
func LLMModel(dataDir string) string { return filepath.Join(LLMDir(dataDir), llmFiles[0].name) }

// LLMModelName names the model for callers that say which one answered.
const LLMModelName = "Qwen 2.5 7B Instruct (Q4_K_M)"

// llama.cpp b12110 is its release v0.9.0 (2026-11-04).
const llamaBase = "https://github.com/ggml-org/llama.cpp/releases/download/b12110/"

var llamaArchives = map[string]struct{ name, sum string }{
	"windows/amd64": {"llama-b12110-bin-win-cpu-x64.zip", "98901e327c252875accaf2d1bf364e10e868843b0b63468baad1aa945cdc5da2"},
	"windows/arm64": {"llama-b12110-bin-win-cpu-arm64.zip", "f8d20fb07d0b499988ced8963afcd5ded2d2cdd9983b757fbc7e51c4abd722e8"},
	"linux/amd64":   {"llama-b12110-bin-ubuntu-x64.tar.gz", "368227edcc229be5f9bed16a2223d79b5e41d0ac2e19bcf5825e82291c060e55"},
	"linux/arm64":   {"llama-b12110-bin-ubuntu-arm64.tar.gz", "4758b51335e37b4346251f2687d64841ff1c07ecb27fc46323945bb2ec9c44d2"},
	"darwin/arm64":  {"llama-b12110-bin-macos-arm64.tar.gz", "4f5b8de12e0b9516658ca6a5764c6ffb3d7c659c3dd538556b1413c7b4050662"},
}

const qwenBase = "https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/bb5d59e06d9551d752d08b292a50eb208b07ab1f/"

var llmFiles = []struct {
	name, sum string
	size      int64
}{
	{"qwen2.5-7b-instruct-q4_k_m-00001-of-00002.gguf", "dfce12e3862a5283ccfb88221b48480e58745165de856439950d0f22590580db", 3993201344},
	{"qwen2.5-7b-instruct-q4_k_m-00002-of-00002.gguf", "539cf93f78e887edea1c04e2d7d8cdaca9d01dae9c9025bcb8accbe29df3d72a", 689872288},
}

// LLMSizeMB is the whole download, for a page to say what a click costs.
const LLMSizeMB = 4490

var llmMu sync.Mutex

// LlamaServer is the path of llama.cpp's server program.
func LlamaServer(dataDir string) string {
	name := "llama-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(LlamaDir(dataDir), name)
}

// LLMSupported reports whether there is a server program for this system.
func LLMSupported() bool {
	_, ok := llamaArchives[runtime.GOOS+"/"+runtime.GOARCH]
	return ok
}

// LLMInstalled reports whether the server program and both model files are in place.
func LLMInstalled(dataDir string) bool {
	if _, err := os.Stat(LlamaServer(dataDir)); err != nil {
		return false
	}
	for _, f := range llmFiles {
		if st, err := os.Stat(filepath.Join(LLMDir(dataDir), f.name)); err != nil || st.Size() != f.size {
			return false
		}
	}
	return true
}

// FetchLLM downloads what is missing: the server program, then the model. A model download that
// broke off goes on where it stopped. One download runs at a time; a second caller waits for it
// and then finds the files there.
func FetchLLM(ctx context.Context, dataDir string) (err error) {
	llmMu.Lock()
	defer llmMu.Unlock()
	defer func() {
		if err != nil {
			models.Track("llm", "error", 0, 0, err)
		} else {
			models.Track("llm", "done", 0, 0, nil)
		}
	}()
	if LLMInstalled(dataDir) {
		return nil
	}
	models.Track("llm", "downloading", 0, 0, nil)
	if _, err := os.Stat(LlamaServer(dataDir)); err != nil {
		if err := fetchLlama(ctx, dataDir); err != nil {
			return err
		}
	}
	// One progress over both model files: the page shows a single bar for 4.5 GB.
	var total, before int64
	for _, f := range llmFiles {
		total += f.size
	}
	for _, f := range llmFiles {
		path := filepath.Join(LLMDir(dataDir), f.name)
		if st, err := os.Stat(path); err == nil && st.Size() == f.size {
			before += f.size
			continue
		}
		if err := resumeDownload(ctx, qwenBase+f.name, f.sum, path, before, total); err != nil {
			return err
		}
		before += f.size
	}
	return nil
}

// fetchLlama unpacks llama.cpp's server program and its libraries into LlamaDir.
func fetchLlama(ctx context.Context, dataDir string) error {
	a, ok := llamaArchives[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return fmt.Errorf("no language model server for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	dir := LlamaDir(dataDir)
	if err := os.MkdirAll(LLMDir(dataDir), 0o755); err != nil {
		return err
	}
	archive := filepath.Join(LLMDir(dataDir), a.name+".tmp")
	defer os.Remove(archive)
	if err := download(ctx, "llm", llamaBase+a.name, a.sum, archive); err != nil {
		return err
	}
	tmp := dir + ".tmp"
	os.RemoveAll(tmp)
	defer os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	if strings.HasSuffix(a.name, ".zip") {
		if err := unzipLlama(archive, tmp); err != nil {
			return err
		}
	} else {
		cmd := proc.Hide(exec.CommandContext(ctx, "tar", "-xzf", archive, "-C", tmp, "--strip-components=1"))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("tar: %w: %s", err, out)
		}
	}
	if _, err := os.Stat(filepath.Join(tmp, filepath.Base(LlamaServer(dataDir)))); err != nil {
		return fmt.Errorf("llama-server not found in %s", a.name)
	}
	os.RemoveAll(dir)
	return os.Rename(tmp, dir)
}

// unzipLlama takes the server program and the libraries; the archive's other tools stay out.
func unzipLlama(archive, dir string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		base := filepath.Base(zf.Name)
		if zf.FileInfo().IsDir() || !(base == "llama-server.exe" || strings.HasSuffix(base, ".dll") || strings.HasPrefix(base, "LICENSE")) {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeAtomically(filepath.Join(dir, base), rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// resumeDownload fetches url into path, going on from path+".part" where an earlier try stopped,
// and checks the SHA-256 of the whole. before and total place the file in the progress of all.
func resumeDownload(ctx context.Context, url, sum, path string, before, total int64) error {
	part := path + ".part"
	h := sha256.New()
	var have int64
	if f, err := os.Open(part); err == nil {
		have, _ = io.Copy(h, f)
		f.Close()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	switch {
	case resp.StatusCode == http.StatusPartialContent && have > 0:
	case resp.StatusCode == http.StatusOK:
		// The server sends it all again: start over.
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
		h.Reset()
		have = 0
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && have > 0:
		// The part is whole already (or longer than the file): check it below.
	default:
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	f, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		models.Track("llm", "downloading", before+have, total, nil)
		_, err = io.Copy(io.MultiWriter(f, h), &trackedReader{r: resp.Body, done: before + have, total: total})
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != sum {
		os.Remove(part) // a broken part would fail every next try
		return fmt.Errorf("%s: checksum %s, want %s", url, got, sum)
	}
	return os.Rename(part, path)
}

// trackedReader reports the progress of the model download as one over both files.
type trackedReader struct {
	r           io.Reader
	done, total int64
	last        int64
}

func (t *trackedReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	t.done += int64(n)
	if t.done-t.last >= 4<<20 || errors.Is(err, io.EOF) {
		t.last = t.done
		models.Track("llm", "downloading", t.done, t.total, nil)
	}
	return n, err
}

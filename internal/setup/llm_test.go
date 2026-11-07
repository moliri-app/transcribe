// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package setup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestResumeDownload: a model download that broke off goes on where it stopped, and a broken part
// is not kept.
func TestResumeDownload(t *testing.T) {
	data := bytes.Repeat([]byte("Qwen 2.5 "), 100000)
	sum := sha256.Sum256(data)
	var ranges []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("Range"))
		http.ServeContent(w, r, "model.gguf", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "model.gguf")
	os.WriteFile(path+".part", data[:300000], 0o644)
	if err := resumeDownload(context.Background(), srv.URL, hex.EncodeToString(sum[:]), path, 0, int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, data) || len(ranges) != 1 || ranges[0] != "bytes=300000-" {
		t.Fatalf("%d bytes, ranges %v", len(got), ranges)
	}
	if _, err := os.Stat(path + ".part"); !os.IsNotExist(err) {
		t.Error("part left behind")
	}

	// A part that does not belong to the file fails the checksum and is dropped.
	other := filepath.Join(t.TempDir(), "other.gguf")
	os.WriteFile(other+".part", []byte("not Qwen"), 0o644)
	err := resumeDownload(context.Background(), srv.URL, hex.EncodeToString(sum[:]), other, 0, int64(len(data)))
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("broken part: %v", err)
	}
	if _, err := os.Stat(other + ".part"); !os.IsNotExist(err) {
		t.Error("broken part kept")
	}
}

// TestVulkanDevice: the first graphics card in what llama-server --list-devices prints.
func TestVulkanDevice(t *testing.T) {
	out := "load_backend: loaded Vulkan backend from ggml-vulkan.dll\nAvailable devices:\n  Vulkan0: Intel(R) UHD Graphics (32631 MiB, 32631 MiB free)\n  Vulkan1: NVIDIA RTX A3000 Laptop GPU (5989 MiB, 5221 MiB free)\n"
	if m := vulkanLine.FindStringSubmatch(out); m == nil || m[1] != "Intel(R) UHD Graphics" {
		t.Errorf("got %q", m)
	}
	if vulkanLine.MatchString("Available devices:\n") {
		t.Error("a device where there is none")
	}
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package models

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// NVIDIA publishes its CUDA and cuDNN libraries as redistributable zip archives. Instead of asking the
// user to install the CUDA Toolkit and cuDNN, the app can fetch just the DLLs the runtime needs from
// them: HTTP range requests read the archive's directory and then only those entries, which skips the
// headers and static libraries that make up most of each archive.

const nvidiaRedist = "https://developer.download.nvidia.com/compute/"

// nvidiaPackage is one redistributable archive and the DLLs taken from it.
type nvidiaPackage struct {
	path string   // below nvidiaRedist
	dlls []string // file names; "*" stands for any run of characters
	size int64    // compressed size of those DLLs, for the progress bar and the size shown beforehand
}

// Pinned versions: a newer minor release may need a newer driver than the one installed.
var nvidiaPackages = map[int][]nvidiaPackage{
	13: {
		{"cuda/redist/cuda_cudart/windows-x86_64/cuda_cudart-windows-x86_64-13.0.96-archive.zip", []string{"cudart64_13.dll"}, 172439},
		{"cuda/redist/libcublas/windows-x86_64/libcublas-windows-x86_64-13.1.0.3-archive.zip", []string{"cublas64_13.dll", "cublasLt64_13.dll"}, 403332818},
		{"cuda/redist/libcufft/windows-x86_64/libcufft-windows-x86_64-12.0.0.61-archive.zip", []string{"cufft64_12.dll"}, 211889048},
		{"cudnn/redist/cudnn/windows-x86_64/cudnn-windows-x86_64-9.27.0.42_cuda13-archive.zip", []string{"cudnn*64_9.dll"}, 432954318},
	},
	12: {
		{"cuda/redist/cuda_cudart/windows-x86_64/cuda_cudart-windows-x86_64-12.9.79-archive.zip", []string{"cudart64_12.dll"}, 174759},
		{"cuda/redist/libcublas/windows-x86_64/libcublas-windows-x86_64-12.9.1.4-archive.zip", []string{"cublas64_12.dll", "cublasLt64_12.dll"}, 549364237},
		{"cuda/redist/libcufft/windows-x86_64/libcufft-windows-x86_64-11.4.1.4-archive.zip", []string{"cufft64_11.dll"}, 198245071},
		{"cudnn/redist/cudnn/windows-x86_64/cudnn-windows-x86_64-9.27.0.42_cuda12-archive.zip", []string{"cudnn*64_9.dll"}, 731031814},
	},
}

func (p nvidiaPackage) wants(name string) bool {
	for _, d := range p.dlls {
		if pre, rest, ok := strings.Cut(d, "*"); ok && strings.HasPrefix(name, pre) && strings.HasSuffix(name, rest) {
			return true
		}
		if strings.EqualFold(d, name) {
			return true
		}
	}
	return false
}

// NvidiaLibDir is where the app keeps the NVIDIA libraries it fetched itself.
func NvidiaLibDir(dataDir string) string { return filepath.Join(dataDir, "lib-nvidia") }

func nvidiaDir(dataDir string, major int) string {
	return filepath.Join(NvidiaLibDir(dataDir), fmt.Sprintf("cuda%d", major))
}

// nvidiaFetch returns the archives that hold the missing libraries (nil if the app cannot fetch them here).
func nvidiaFetch(major int, missing []string) []nvidiaPackage {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return nil
	}
	var out []nvidiaPackage
	for _, p := range nvidiaPackages[major] {
		for _, m := range missing {
			if p.wants(m) {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

var nvidiaMu sync.Mutex

// FetchNvidiaLibs downloads the CUDA and cuDNN libraries this GPU still lacks from NVIDIA into dataDir/lib-nvidia.
func FetchNvidiaLibs(ctx context.Context, dataDir string) error {
	nvidiaMu.Lock()
	defer nvidiaMu.Unlock()
	g := DetectGPU(dataDir)
	if !g.Present || !g.Supported {
		return fmt.Errorf("keine unterstützte NVIDIA-GPU gefunden")
	}
	if len(g.Missing) == 0 {
		return nil
	}
	pkgs := nvidiaFetch(g.CudaMajor, g.Missing)
	if len(pkgs) == 0 {
		return fmt.Errorf("die NVIDIA-Bibliotheken lassen sich auf diesem System nicht automatisch laden")
	}
	var total int64
	for _, p := range pkgs {
		total += p.size
	}
	prog := &nvidiaProgress{total: total}
	track("nvidia", "downloading", 0, total, nil)
	dir := nvidiaDir(dataDir, g.CudaMajor)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, p := range pkgs {
		if err := fetchFromZip(ctx, nvidiaRedist+p.path, p, dir, prog); err != nil {
			return err
		}
	}
	return nil
}

type nvidiaProgress struct {
	done  atomic.Int64
	total int64
}

func (p *nvidiaProgress) add(n int) {
	track("nvidia", "downloading", p.done.Add(int64(n)), p.total, nil)
}

// fetchFromZip copies the wanted DLLs of a remote zip archive into dir, reading only their entries.
func fetchFromZip(ctx context.Context, url string, p nvidiaPackage, dir string, prog *nvidiaProgress) error {
	ra, err := openRemote(ctx, url)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(ra, ra.size)
	if err != nil {
		return fmt.Errorf("%s: %w", url, err)
	}
	found := 0
	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if f.FileInfo().IsDir() || !p.wants(name) {
			continue
		}
		found++
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && st.Size() == int64(f.UncompressedSize64) {
			prog.add(int(f.CompressedSize64))
			continue
		}
		if err := fetchEntry(ctx, ra, f, filepath.Join(dir, name), prog); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	if found == 0 {
		return fmt.Errorf("%s: keine der gesuchten Bibliotheken im Archiv", url)
	}
	return nil
}

func fetchEntry(ctx context.Context, ra *remoteFile, f *zip.File, dst string, prog *nvidiaProgress) error {
	off, err := f.DataOffset()
	if err != nil {
		return err
	}
	body, err := ra.get(ctx, off, int64(f.CompressedSize64))
	if err != nil {
		return err
	}
	defer body.Close()
	var src io.Reader = &progressReader{r: body, prog: prog}
	switch f.Method {
	case zip.Store:
	case zip.Deflate:
		fr := flate.NewReader(src)
		defer fr.Close()
		src = fr
	default:
		return fmt.Errorf("unbekannte Kompression %d", f.Method)
	}
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := crc32.NewIEEE()
	n, err := io.Copy(io.MultiWriter(out, h), src)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && (uint64(n) != f.UncompressedSize64 || h.Sum32() != f.CRC32) {
		err = fmt.Errorf("Download beschädigt")
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

type progressReader struct {
	r    io.Reader
	prog *nvidiaProgress
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.prog.add(n)
	}
	return n, err
}

// remoteFile reads a file over HTTP range requests. The zip directory sits at the end, so the tail is
// fetched once and kept; the zip reader's many small reads there cost no further requests.
type remoteFile struct {
	url     string
	size    int64
	tail    []byte
	tailOff int64
	ctx     context.Context
}

const remoteTail = 1 << 20

func openRemote(ctx context.Context, url string) (*remoteFile, error) {
	r := &remoteFile{url: url, ctx: ctx}
	// a suffix range answers with the total size in Content-Range
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=-%d", remoteTail))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	cr := resp.Header.Get("Content-Range") // bytes a-b/size
	i := strings.LastIndex(cr, "/")
	if i < 0 {
		return nil, fmt.Errorf("%s: unerwartete Antwort (%q)", url, cr)
	}
	if r.size, err = strconv.ParseInt(cr[i+1:], 10, 64); err != nil {
		return nil, fmt.Errorf("%s: unerwartete Antwort (%q)", url, cr)
	}
	if r.tail, err = io.ReadAll(resp.Body); err != nil {
		return nil, err
	}
	r.tailOff = r.size - int64(len(r.tail))
	return r, nil
}

func (r *remoteFile) ReadAt(p []byte, off int64) (int, error) {
	if off >= r.tailOff {
		n := copy(p, r.tail[off-r.tailOff:])
		if n < len(p) {
			return n, io.EOF
		}
		return n, nil
	}
	body, err := r.get(r.ctx, off, int64(len(p)))
	if err != nil {
		return 0, err
	}
	defer body.Close()
	return io.ReadFull(body, p)
}

// get returns n bytes of the file from off on.
func (r *remoteFile) get(ctx context.Context, off, n int64) (io.ReadCloser, error) {
	if n == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", r.url, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+n-1))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", r.url, resp.Status)
	}
	return resp.Body, nil
}

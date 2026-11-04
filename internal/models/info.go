// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package models

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Component is one thing the app needs on disk, as shown on the settings page.
type Component struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Path      string `json:"path"`
	Installed bool   `json:"installed"`
	Size      int64  `json:"size"`
	State     string `json:"state"` // "", pending, downloading, done, error
	Done      int64  `json:"done"`
	Total     int64  `json:"total"`
	Error     string `json:"error,omitempty"`
	OnDemand  bool   `json:"onDemand"` // not fetched at startup
}

// DirSize sums the size of all files below dir.
func DirSize(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, e := d.Info(); e == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

func sizeOf(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if st.IsDir() {
		return DirSize(path)
	}
	return st.Size()
}

// modelRoot is the folder (archives) or file (single downloads) that holds a model.
func (s Set) modelRoot(it item) string {
	rel, err := filepath.Rel(s.Dir, it.check)
	if err != nil || strings.HasPrefix(rel, "..") {
		return it.check
	}
	return filepath.Join(s.Dir, strings.Split(rel, string(filepath.Separator))[0])
}

var modelOrder = []string{"parakeet", "german", "kroko", "whisper-base", "whisper-turbo", "ced", "segmentation", "campplus", "silero"}

// Info lists runtime, ffmpeg and all models with their locations, sizes and download state.
// ffmpegPath is whatever media.Tool("ffmpeg") resolved to.
func Info(dataDir, ffmpegPath string) []Component {
	status := map[string]Status{}
	items, _ := Snapshot()
	for _, it := range items {
		status[it.Name] = it
	}
	var out []Component
	add := func(c Component) {
		c.Label = labels[c.Name]
		if st, ok := status[c.Name]; ok {
			c.State, c.Done, c.Total, c.Error = st.State, st.Done, st.Total, st.Error
		}
		out = append(out, c)
	}

	libDir := FindLib(os.Getenv("SHERPA_LIB"), exeDir(), filepath.Join(dataDir, "lib"))
	if libDir == "" {
		libDir = filepath.Join(dataDir, "lib")
	}
	add(Component{Name: "runtime", Path: libDir, Installed: FindLib(libDir) != "", Size: sizeUnder(dataDir, libDir)})
	cudaDir := CudaLibDir(dataDir)
	add(Component{Name: "runtime-cuda", Path: cudaDir, Installed: FindLib(cudaDir) != "", Size: sizeOf(cudaDir), OnDemand: true})
	nvDir := NvidiaLibDir(dataDir)
	nvSize := sizeOf(nvDir)
	add(Component{Name: "nvidia", Path: nvDir, Installed: nvSize > 0, Size: nvSize, OnDemand: true})

	ff := Component{Name: "ffmpeg", Path: ffmpegPath}
	if p, err := exec.LookPath(ffmpegPath); err == nil {
		ff.Path, ff.Installed, ff.Size = p, true, sizeUnder(dataDir, p)
	}
	add(ff)

	if TesseractLibSupported() {
		dir := TesseractDir(dataDir)
		add(Component{Name: "tesseract", Path: dir, Installed: TesseractLibInstalled(dataDir), Size: sizeOf(dir), OnDemand: true})
	}
	ppocr := PPOCRDir(dataDir)
	add(Component{Name: "ppocr", Path: ppocr, Installed: PPOCRInstalled(dataDir), Size: sizeOf(ppocr), OnDemand: true})

	s := Set{Dir: filepath.Join(dataDir, "models")}
	cat := s.catalog()
	for _, n := range modelOrder {
		it := cat[n]
		root := s.modelRoot(it)
		_, err := os.Stat(it.check)
		add(Component{Name: n, Path: root, Installed: err == nil, Size: sizeOf(root), OnDemand: n == "whisper-turbo" || n == "kroko"})
	}
	return out
}

// sizeUnder measures path only if it lives inside dataDir (never the whole exe folder or system dirs).
func sizeUnder(dataDir, path string) int64 {
	rel, err := filepath.Rel(dataDir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return 0
	}
	return sizeOf(path)
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

// Download fetches one component that is missing (used by the settings page).
func Download(ctx context.Context, dataDir, name string) error {
	switch name {
	case "runtime":
		return EnsureLib(ctx, filepath.Join(dataDir, "lib"), nil)
	case "runtime-cuda":
		major := DetectGPU(dataDir).CudaMajor
		if major == 0 {
			major = 12
		}
		track("runtime-cuda", "downloading", 0, 0, nil)
		err := EnsureCudaLib(ctx, dataDir, major)
		if err != nil {
			track("runtime-cuda", "error", 0, 0, err)
		} else {
			track("runtime-cuda", "done", 0, 0, nil)
		}
		return err
	case "nvidia":
		err := FetchNvidiaLibs(ctx, dataDir)
		if err != nil {
			track("nvidia", "error", 0, 0, err)
		} else {
			track("nvidia", "done", 0, 0, nil)
		}
		return err
	case "ffmpeg":
		return EnsureFFmpeg(ctx, filepath.Join(dataDir, "bin"), nil)
	case "tesseract":
		if err := EnsureTesseractLib(ctx, dataDir); err != nil {
			return err
		}
		return FetchTesseractModel(ctx, dataDir, "deu")
	case "ppocr":
		for _, f := range PPOCRModels() {
			if err := FetchPPOCRModel(ctx, dataDir, f); err != nil {
				return err
			}
		}
		return nil
	}
	return Set{Dir: filepath.Join(dataDir, "models")}.Ensure(ctx, nil, name)
}

// SetUpGPU fetches the NVIDIA libraries the GPU still lacks (fetchLibs), then the CUDA build of the runtime.
func SetUpGPU(ctx context.Context, dataDir string, fetchLibs bool) error {
	if fetchLibs {
		track("runtime-cuda", "pending", 0, 0, nil) // so the page shows both steps as under way
		if err := Download(ctx, dataDir, "nvidia"); err != nil {
			track("runtime-cuda", "", 0, 0, nil)
			return err
		}
	}
	return Download(ctx, dataDir, "runtime-cuda")
}

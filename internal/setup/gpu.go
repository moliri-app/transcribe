// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package setup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/moliri-app/transcribe/proc"
)

// GPU describes an NVIDIA GPU and whether the CUDA runtime libraries it needs are installed.
type GPU struct {
	Present    bool     `json:"present"`
	Name       string   `json:"name,omitempty"`
	Driver     string   `json:"driver,omitempty"`
	MemoryMB   int      `json:"memoryMB,omitempty"`
	CudaMajor  int      `json:"cudaMajor,omitempty"`  // CUDA major version whose sherpa build would be used
	CudaDriver int      `json:"cudaDriver,omitempty"` // newest CUDA major version the driver runs
	Supported  bool     `json:"supported"`            // a sherpa CUDA build exists for this OS/arch
	Ready      bool     `json:"ready"`                // all CUDA/cuDNN libraries were found
	Missing    []string `json:"missing,omitempty"`
	Fetch      int64    `json:"fetch,omitempty"` // bytes the app would download from NVIDIA to fill in Missing (0: it cannot)
	Dirs       []string `json:"-"`               // folders holding the libraries (added to the library search path)
}

var cudaVer = regexp.MustCompile(`CUDA Version:\s*(\d+)\.(\d+)`)

func nvidiaSmi(args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, err := proc.Hide(exec.CommandContext(ctx, "nvidia-smi", args...)).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// DetectGPU looks for an NVIDIA GPU via nvidia-smi and checks the CUDA libraries,
// including those the app fetched itself into dataDir.
func DetectGPU(dataDir string) GPU {
	g := GPU{}
	q := strings.TrimSpace(nvidiaSmi("--query-gpu=name,driver_version,memory.total", "--format=csv,noheader,nounits"))
	if q == "" {
		return g
	}
	parts := strings.Split(strings.Split(q, "\n")[0], ",")
	if len(parts) < 3 {
		return g
	}
	g.Present = true
	g.Name = strings.TrimSpace(parts[0])
	g.Driver = strings.TrimSpace(parts[1])
	g.MemoryMB, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
	g.Supported = cudaArchiveSupported()

	driverMajor := 12
	if m := cudaVer.FindStringSubmatch(nvidiaSmi()); m != nil {
		driverMajor, _ = strconv.Atoi(m[1])
	}
	g.CudaDriver = driverMajor
	if driverMajor < 12 {
		g.Supported = false // sherpa only ships CUDA 12 and 13 builds
		return g
	}
	for _, major := range []int{13, 12} {
		if major > driverMajor {
			continue
		}
		missing, dirs := cudaLibsMissing(major, dataDir)
		if g.CudaMajor == 0 || len(missing) == 0 {
			g.CudaMajor, g.Missing, g.Dirs = major, missing, dirs
		}
		if len(missing) == 0 {
			break
		}
	}
	g.Ready = g.Supported && len(g.Missing) == 0
	if g.Supported {
		for _, p := range nvidiaFetch(g.CudaMajor, g.Missing) {
			g.Fetch += p.size
		}
	}
	return g
}

func cudaArchiveSupported() bool {
	return runtime.GOARCH == "amd64" && (runtime.GOOS == "windows" || runtime.GOOS == "linux")
}

// cudaLibsMissing reports which of the CUDA libraries are not found and in which folders the others are.
func cudaLibsMissing(major int, dataDir string) (missing, dirs []string) {
	var names []string
	var search []string
	if runtime.GOOS == "windows" {
		names = []string{fmt.Sprintf("cudart64_%d.dll", major), fmt.Sprintf("cublas64_%d.dll", major),
			fmt.Sprintf("cublasLt64_%d.dll", major), fmt.Sprintf("cufft64_%d.dll", major-1), "cudnn64_9.dll"}
		search = filepath.SplitList(os.Getenv("PATH"))
		for _, k := range []string{"CUDA_PATH", fmt.Sprintf("CUDA_PATH_V%d_0", major), "CUDNN_PATH"} {
			if p := os.Getenv(k); p != "" {
				search = append(search, filepath.Join(p, "bin"), filepath.Join(p, "bin", "x64"), p)
			}
		}
		pf := os.Getenv("ProgramFiles")
		if pf != "" {
			for _, pat := range []string{
				filepath.Join(pf, "NVIDIA GPU Computing Toolkit", "CUDA", fmt.Sprintf("v%d.*", major), "bin"),
				filepath.Join(pf, "NVIDIA GPU Computing Toolkit", "CUDA", fmt.Sprintf("v%d.*", major), "bin", "x64"),
				filepath.Join(pf, "NVIDIA", "CUDNN", "v9.*", "bin", "*"),
				filepath.Join(pf, "NVIDIA", "CUDNN", "v9.*", "bin"),
			} {
				m, _ := filepath.Glob(pat)
				search = append(search, m...)
			}
		}
		if dataDir != "" {
			search = append(search, nvidiaDir(dataDir, major))
		}
	} else {
		names = []string{fmt.Sprintf("libcudart.so.%d", major), fmt.Sprintf("libcublas.so.%d", major),
			fmt.Sprintf("libcublasLt.so.%d", major), fmt.Sprintf("libcufft.so.%d", major-1), "libcudnn.so.9"}
		search = append(filepath.SplitList(os.Getenv("LD_LIBRARY_PATH")), "/usr/lib/x86_64-linux-gnu", "/usr/lib64", "/usr/local/lib")
		m, _ := filepath.Glob("/usr/local/cuda*/lib64")
		search = append(search, m...)
		m, _ = filepath.Glob("/usr/local/cuda*/targets/x86_64-linux/lib")
		search = append(search, m...)
	}
	seen := map[string]bool{}
	for _, n := range names {
		found := false
		for _, d := range search {
			if d == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(d, n)); err == nil {
				found = true
				if !seen[d] {
					seen[d] = true
					dirs = append(dirs, d)
				}
				break
			}
		}
		if !found {
			missing = append(missing, n)
		}
	}
	return
}

// UseLibDirs makes the CUDA libraries visible to the dynamic loader of this process.
func UseLibDirs(dirs []string) {
	if len(dirs) == 0 {
		return
	}
	key := "PATH"
	if runtime.GOOS != "windows" {
		key = "LD_LIBRARY_PATH"
	}
	sep := string(os.PathListSeparator)
	os.Setenv(key, strings.Join(dirs, sep)+sep+os.Getenv(key))
}

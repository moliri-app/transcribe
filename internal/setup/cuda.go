// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package setup

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/moliri-app/transcribe/models"
)

const onnxruntimeVersion = "1.28.2"

func cudaLibArchive(major int) (string, error) {
	if !cudaArchiveSupported() {
		return "", fmt.Errorf("no sherpa-onnx CUDA build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	suffix := "linux-x64-gpu"
	if runtime.GOOS == "windows" {
		suffix = "win-x64-cuda"
	}
	return fmt.Sprintf("sherpa-onnx-v%s-cuda-%d.x-cudnn-9.x-onnxruntime%s-%s", models.SherpaVersion, major, onnxruntimeVersion, suffix), nil
}

// CudaLibDir is where the CUDA build of the runtime lives.
func CudaLibDir(dataDir string) string { return filepath.Join(dataDir, "lib-cuda") }

// EnsureCudaLib downloads the CUDA build of the runtime (several hundred MB) into dataDir/lib-cuda.
func EnsureCudaLib(ctx context.Context, dataDir string, major int) error {
	dir := CudaLibDir(dataDir)
	if models.FindLib(dir) != "" {
		return nil
	}
	name, err := cudaLibArchive(major)
	if err != nil {
		return err
	}
	return models.InstallLib(ctx, "runtime-cuda", name, dir)
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package models

import (
	"runtime"
	"testing"
)

func TestNvidiaFetch(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		if nvidiaFetch(13, []string{"cudnn64_9.dll"}) != nil {
			t.Error("offered a download where the app cannot fetch the libraries")
		}
		return
	}
	got := nvidiaFetch(13, []string{"cudnn64_9.dll"})
	if len(got) != 1 || got[0].path != nvidiaPackages[13][3].path {
		t.Fatalf("cudnn64_9.dll alone: got %v", got)
	}
	if got := nvidiaFetch(12, []string{"cudart64_12.dll", "cublasLt64_12.dll", "cufft64_11.dll"}); len(got) != 3 {
		t.Errorf("runtime, cuBLAS and cuFFT: got %d archives", len(got))
	}
	for _, major := range []int{12, 13} {
		for _, p := range nvidiaPackages[major] {
			if p.wants("nvblas64_13.dll") || p.wants("cufftw64_12.dll") {
				t.Errorf("%s takes DLLs the runtime does not need", p.path)
			}
		}
	}
	if !nvidiaPackages[13][3].wants("cudnn_engines_precompiled64_9.dll") {
		t.Error("cuDNN engine DLLs not taken")
	}
}

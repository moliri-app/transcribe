// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package sherpa

import (
	"path/filepath"
	"runtime"

	"github.com/ebitengine/purego"
)

func openLib(dir, name string) (uintptr, error) {
	ext := ".so"
	if runtime.GOOS == "darwin" {
		ext = ".dylib"
	}
	return purego.Dlopen(filepath.Join(dir, "lib"+name+ext), purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

// badFormat is Windows only; an empty placeholder is recognized on every system.
func badFormat(error) bool { return false }

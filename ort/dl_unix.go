// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package ort

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
)

func openLib(dir string) (uintptr, error) {
	path, err := libPath(dir)
	if err != nil {
		return 0, err
	}
	h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, fmt.Errorf("onnxruntime: %w", err)
	}
	return h, nil
}

func symbol(h uintptr, name string) (uintptr, error) {
	p, err := purego.Dlsym(h, name)
	if err != nil {
		return 0, fmt.Errorf("onnxruntime: %s: %w", name, err)
	}
	return p, nil
}

// nativePath is the model path as ORTCHAR_T*: a narrow string outside Windows.
func nativePath(path string) (unsafe.Pointer, error) {
	b := append([]byte(path), 0)
	return unsafe.Pointer(&b[0]), nil
}

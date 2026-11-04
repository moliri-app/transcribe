// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package ort

import (
	"fmt"
	"syscall"
	"unsafe"
)

func openLib(dir string) (uintptr, error) {
	path, err := libPath(dir)
	if err != nil {
		return 0, err
	}
	h, err := syscall.LoadLibrary(path)
	if err != nil {
		return 0, fmt.Errorf("onnxruntime: %w", err)
	}
	return uintptr(h), nil
}

func symbol(h uintptr, name string) (uintptr, error) {
	p, err := syscall.GetProcAddress(syscall.Handle(h), name)
	if err != nil {
		return 0, fmt.Errorf("onnxruntime: %s: %w", name, err)
	}
	return p, nil
}

// nativePath is the model path as ORTCHAR_T*: UTF-16 on Windows.
func nativePath(path string) (unsafe.Pointer, error) {
	p, err := syscall.UTF16PtrFromString(path)
	return unsafe.Pointer(p), err
}

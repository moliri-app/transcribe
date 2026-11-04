// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package sherpa

import (
	"errors"
	"path/filepath"
	"syscall"
	"unsafe"
)

func openLib(dir, name string) (uintptr, error) {
	// dependent DLLs (onnxruntime.dll) are looked up next to the library
	if k, err := syscall.LoadLibrary("kernel32.dll"); err == nil {
		if p, err := syscall.GetProcAddress(k, "SetDllDirectoryW"); err == nil {
			d, _ := syscall.UTF16PtrFromString(dir)
			syscall.SyscallN(p, uintptr(unsafe.Pointer(d)))
		}
	}
	h, err := syscall.LoadLibrary(filepath.Join(dir, name+".dll"))
	return uintptr(h), err
}

// badFormat reports ERROR_BAD_EXE_FORMAT ("%1 is not a valid Win32 application"): the file is no
// DLL this process can load.
func badFormat(err error) bool { return errors.Is(err, syscall.Errno(193)) }

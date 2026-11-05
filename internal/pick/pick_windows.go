// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pick

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// The dialog is the Explorer's own (IFileOpenDialog with FOS_PICKFOLDERS), driven over COM
// without cgo: each method is a slot in the object's table of functions.

type guid struct {
	d1     uint32
	d2, d3 uint16
	d4     [8]byte
}

var (
	clsidFileOpenDialog = guid{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidFileOpenDialog   = guid{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
)

const (
	coinitApartment  = 0x2
	coinitNoOLE1DDE  = 0x4
	clsctxInproc     = 0x1
	fosPickFolders   = 0x20
	fosForceFS       = 0x40
	fosPathMustExist = 0x800
	sigdnFileSysPath = 0x80058000
	errCancelled     = 0x800704C7 // HRESULT_FROM_WIN32(ERROR_CANCELLED)
	slotRelease      = 2
	slotShow         = 3
	slotSetOptions   = 9
	slotGetOptions   = 10
	slotSetTitle     = 17
	slotGetResult    = 20
	slotGetDisplayNm = 5 // IShellItem
	rpcChangedMode   = 0x80010106
	hresultSFalse    = 1
)

var (
	ole32                    = syscall.NewLazyDLL("ole32.dll")
	coInitializeEx           = ole32.NewProc("CoInitializeEx")
	coUninitialize           = ole32.NewProc("CoUninitialize")
	coCreateInstance         = ole32.NewProc("CoCreateInstance")
	coTaskMemFree            = ole32.NewProc("CoTaskMemFree")
	user32                   = syscall.NewLazyDLL("user32.dll")
	getForegroundWindow      = user32.NewProc("GetForegroundWindow")
	getWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	attachThreadInput        = user32.NewProc("AttachThreadInput")
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	getCurrentThreadId       = kernel32.NewProc("GetCurrentThreadId")
)

// Supported reports whether Folder shows a dialog.
func Supported() bool { return true }

var open sync.Mutex

// Folder shows the dialog to choose a folder, in front of the window the user works in, and
// returns the folder; "" if they cancelled.
func Folder(title string) (string, error) {
	if !open.TryLock() {
		return "", errors.New("Der Dialog zur Ordnerwahl ist schon offen.")
	}
	defer open.Unlock()
	type result struct {
		path string
		err  error
	}
	ch := make(chan result, 1)
	go func() { // COM wants a thread of its own for the dialog
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		p, err := folder(title)
		ch <- result{p, err}
	}()
	r := <-ch
	return r.path, r.err
}

func call(obj uintptr, slot int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}

func failed(hr uintptr) bool { return int32(hr) < 0 }

func folder(title string) (string, error) {
	hr, _, _ := coInitializeEx.Call(0, coinitApartment|coinitNoOLE1DDE)
	if hr == 0 || hr == hresultSFalse {
		defer coUninitialize.Call()
	} else if hr != rpcChangedMode {
		return "", fmt.Errorf("CoInitializeEx: %#x", hr)
	}
	var dlg uintptr
	hr, _, _ = coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0, clsctxInproc,
		uintptr(unsafe.Pointer(&iidFileOpenDialog)), uintptr(unsafe.Pointer(&dlg)))
	if failed(hr) || dlg == 0 {
		return "", fmt.Errorf("FileOpenDialog: %#x", hr)
	}
	defer call(dlg, slotRelease)

	var opts uint32
	call(dlg, slotGetOptions, uintptr(unsafe.Pointer(&opts)))
	call(dlg, slotSetOptions, uintptr(opts|fosPickFolders|fosForceFS|fosPathMustExist))
	if t, err := syscall.UTF16PtrFromString(title); err == nil {
		call(dlg, slotSetTitle, uintptr(unsafe.Pointer(t)))
	}

	// The user clicked in the app's window a moment ago: the dialog belongs to it, and joins its
	// input so Windows lets it come to the front.
	owner, _, _ := getForegroundWindow.Call()
	if owner != 0 {
		other, _, _ := getWindowThreadProcessId.Call(owner, 0)
		self, _, _ := getCurrentThreadId.Call()
		if other != 0 && other != self {
			if ok, _, _ := attachThreadInput.Call(self, other, 1); ok != 0 {
				defer attachThreadInput.Call(self, other, 0)
			}
		}
	}
	hr = call(dlg, slotShow, owner)
	if hr == errCancelled {
		return "", nil
	} else if failed(hr) {
		return "", fmt.Errorf("Show: %#x", hr)
	}
	var item uintptr
	if hr = call(dlg, slotGetResult, uintptr(unsafe.Pointer(&item))); failed(hr) || item == 0 {
		return "", fmt.Errorf("GetResult: %#x", hr)
	}
	defer call(item, slotRelease)
	var name *uint16
	if hr = call(item, slotGetDisplayNm, sigdnFileSysPath, uintptr(unsafe.Pointer(&name))); failed(hr) || name == nil {
		return "", fmt.Errorf("GetDisplayName: %#x", hr)
	}
	defer coTaskMemFree.Call(uintptr(unsafe.Pointer(name)))
	return utf16(name), nil
}

// utf16 reads the zero-terminated string at p.
func utf16(p *uint16) string {
	var s []uint16
	for ptr := unsafe.Pointer(p); ; ptr = unsafe.Add(ptr, 2) {
		c := *(*uint16)(ptr)
		if c == 0 {
			break
		}
		s = append(s, c)
	}
	return syscall.UTF16ToString(s)
}

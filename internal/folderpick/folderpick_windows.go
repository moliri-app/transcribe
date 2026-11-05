// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

// Package folderpick shows the system's dialog for choosing a folder. The browser cannot name a
// folder on disk, so the app shows it for the settings page.
package folderpick

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// The COM interfaces are called through their vtables, as in internal/live.

type guid struct {
	a    uint32
	b, c uint16
	d    [8]byte
}

var (
	clsidFileOpenDialog = guid{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = guid{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidIShellItem       = guid{0x43826D1E, 0xE718, 0x42EE, [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

var (
	ole32                       = syscall.NewLazyDLL("ole32.dll")
	coInitializeEx              = ole32.NewProc("CoInitializeEx")
	coUninitialize              = ole32.NewProc("CoUninitialize")
	coCreateInstance            = ole32.NewProc("CoCreateInstance")
	coTaskMemFree               = ole32.NewProc("CoTaskMemFree")
	shCreateItemFromParsingName = syscall.NewLazyDLL("shell32.dll").NewProc("SHCreateItemFromParsingName")
	getForegroundWindow         = syscall.NewLazyDLL("user32.dll").NewProc("GetForegroundWindow")
)

const (
	coinitApartmentThreaded = 0x2
	clsctxInprocServer      = 0x1
	fosPickFolders          = 0x20
	fosForceFileSystem      = 0x40
	fosPathMustExist        = 0x800
	sigdnFileSysPath        = 0x80058000
	errCancelled            = 0x800704C7
)

// vtable method numbers
const (
	mRelease        = 2
	mShow           = 3 // IModalWindow
	mSetOptions     = 9 // IFileDialog
	mGetOptions     = 10
	mSetFolder      = 12
	mSetTitle       = 17
	mSetOkLabel     = 18
	mGetResult      = 20
	mGetDisplayName = 5 // IShellItem
)

type comObj struct{ vtbl *[32]uintptr }

func (o *comObj) call(m int, args ...uintptr) error {
	hr, _, _ := syscall.SyscallN(o.vtbl[m], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	if int32(hr) < 0 {
		return fmt.Errorf("Ordnerauswahl: Fehler 0x%08X", uint32(hr))
	}
	return nil
}

func (o *comObj) release() { syscall.SyscallN(o.vtbl[mRelease], uintptr(unsafe.Pointer(o))) }

// Supported reports whether this system has the dialog.
func Supported() bool { return true }

// Pick shows the dialog, opened at start, in front of the window in front (the app's, from which
// the user asked for it). It returns the folder chosen, or "" when the user cancelled.
func Pick(title, start string) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if hr, _, _ := coInitializeEx.Call(0, coinitApartmentThreaded); int32(hr) >= 0 {
		defer coUninitialize.Call()
	}
	var d *comObj
	if hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIFileOpenDialog)), uintptr(unsafe.Pointer(&d))); int32(hr) < 0 {
		return "", fmt.Errorf("Ordnerauswahl: Fehler 0x%08X", uint32(hr))
	}
	defer d.release()
	var opts uint32
	_ = d.call(mGetOptions, uintptr(unsafe.Pointer(&opts)))
	if err := d.call(mSetOptions, uintptr(opts|fosPickFolders|fosForceFileSystem|fosPathMustExist)); err != nil {
		return "", err
	}
	t, _ := syscall.UTF16PtrFromString(title)
	_ = d.call(mSetTitle, uintptr(unsafe.Pointer(t)))
	ok, _ := syscall.UTF16PtrFromString("Ordner wählen")
	_ = d.call(mSetOkLabel, uintptr(unsafe.Pointer(ok)))
	if start != "" {
		p, _ := syscall.UTF16PtrFromString(start)
		var item *comObj
		if hr, _, _ := shCreateItemFromParsingName.Call(uintptr(unsafe.Pointer(p)), 0,
			uintptr(unsafe.Pointer(&iidIShellItem)), uintptr(unsafe.Pointer(&item))); int32(hr) >= 0 {
			_ = d.call(mSetFolder, uintptr(unsafe.Pointer(item)))
			item.release()
		}
	}
	owner, _, _ := getForegroundWindow.Call()
	hr, _, _ := syscall.SyscallN(d.vtbl[mShow], uintptr(unsafe.Pointer(d)), owner)
	if uint32(hr) == errCancelled {
		return "", nil
	}
	if int32(hr) < 0 {
		return "", fmt.Errorf("Ordnerauswahl: Fehler 0x%08X", uint32(hr))
	}
	var item *comObj
	if err := d.call(mGetResult, uintptr(unsafe.Pointer(&item))); err != nil {
		return "", err
	}
	defer item.release()
	var name *uint16
	if err := item.call(mGetDisplayName, sigdnFileSysPath, uintptr(unsafe.Pointer(&name))); err != nil {
		return "", err
	}
	defer coTaskMemFree.Call(uintptr(unsafe.Pointer(name)))
	return utf16PtrToString(name), nil
}

func utf16PtrToString(p *uint16) string {
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

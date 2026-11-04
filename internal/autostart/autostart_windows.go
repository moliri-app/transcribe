// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package autostart

import (
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	runKey      = `Software\Microsoft\Windows\CurrentVersion\Run`
	approvedKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
)

var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	regSetValueEx  = advapi32.NewProc("RegSetValueExW")
	regDeleteValue = advapi32.NewProc("RegDeleteValueW")
)

// Supported reports whether this system can start the app at sign-in (only Windows).
func Supported() bool { return true }

// Enabled reports whether this EXE starts at sign-in: the Run entry names it,
// and it was not switched off in the Task Manager's startup apps.
func Enabled() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	v, ok := readValue(runKey)
	if !ok || !strings.Contains(strings.ToLower(utf16String(v)), strings.ToLower(`"`+exe+`"`)) {
		return false
	}
	// StartupApproved holds 12 bytes; an odd first byte means "disabled"
	a, ok := readValue(approvedKey)
	return !ok || len(a) == 0 || a[0]&1 == 0
}

// Set turns starting at sign-in on or off.
func Set(on bool, dataDir string) error {
	k, err := openKey(runKey)
	if err != nil {
		return err
	}
	defer syscall.RegCloseKey(k)
	name, _ := syscall.UTF16PtrFromString(Name)
	if !on {
		if r, _, _ := regDeleteValue.Call(uintptr(k), uintptr(unsafe.Pointer(name))); r != 0 && syscall.Errno(r) != syscall.ERROR_FILE_NOT_FOUND {
			return syscall.Errno(r)
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	val, _ := syscall.UTF16FromString(command(exe, dataDir))
	if r, _, _ := regSetValueEx.Call(uintptr(k), uintptr(unsafe.Pointer(name)), 0, syscall.REG_SZ,
		uintptr(unsafe.Pointer(&val[0])), uintptr(len(val)*2)); r != 0 {
		return syscall.Errno(r)
	}
	// forget an earlier "disabled" from the Task Manager, so turning it on here really turns it on
	if a, err := openKey(approvedKey); err == nil {
		regDeleteValue.Call(uintptr(a), uintptr(unsafe.Pointer(name)))
		syscall.RegCloseKey(a)
	}
	return nil
}

func openKey(path string) (syscall.Handle, error) {
	p, _ := syscall.UTF16PtrFromString(path)
	var k syscall.Handle
	err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, p, 0, syscall.KEY_QUERY_VALUE|syscall.KEY_SET_VALUE, &k)
	return k, err
}

// readValue returns the raw data of our entry under path.
func readValue(path string) ([]byte, bool) {
	k, err := openKey(path)
	if err != nil {
		return nil, false
	}
	defer syscall.RegCloseKey(k)
	name, _ := syscall.UTF16PtrFromString(Name)
	buf := make([]byte, 2048)
	n := uint32(len(buf))
	if err := syscall.RegQueryValueEx(k, name, nil, nil, &buf[0], &n); err != nil {
		return nil, false
	}
	return buf[:n], true
}

// utf16String decodes REG_SZ data.
func utf16String(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return syscall.UTF16ToString(u)
}

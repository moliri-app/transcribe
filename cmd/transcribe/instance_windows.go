// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	procCreateMutex        = kernel32.NewProc("CreateMutexW")
	procCloseHandle        = kernel32.NewProc("CloseHandle")
	procEnumWindows        = user32.NewProc("EnumWindows")
	procGetClassName       = user32.NewProc("GetClassNameW")
	procGetWindowText      = user32.NewProc("GetWindowTextW")
	procIsWindowVisible    = user32.NewProc("IsWindowVisible")
	procIsIconic           = user32.NewProc("IsIconic")
	enumAppWindows         = syscall.NewCallback(enumAppWindow)
	foundAppWindow         uintptr
	foundAppWindowMu       sync.Mutex
)

// lockInstance claims dataDir for this app with a named mutex, which Windows drops when the app
// ends however it ends; ok is false while another app holds it. release lets go of it.
func lockInstance(dataDir string) (release func(), ok bool) {
	name := utf16(`Local\Transcribe-` + instanceKey(dataDir))
	h, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(&name[0])))
	if h == 0 {
		return func() {}, true // no telling: start as without
	}
	if err == syscall.ERROR_ALREADY_EXISTS {
		procCloseHandle.Call(h)
		return nil, false
	}
	var once sync.Once
	return func() { once.Do(func() { procCloseHandle.Call(h) }) }, true
}

// focusAppWindow brings the app's open window to the front; false if it found none.
func focusAppWindow() bool {
	foundAppWindowMu.Lock()
	defer foundAppWindowMu.Unlock()
	foundAppWindow = 0
	procEnumWindows.Call(enumAppWindows, 0)
	w := foundAppWindow
	if w == 0 {
		return false
	}
	const swRestore = 9
	if minimized, _, _ := procIsIconic.Call(w); minimized != 0 {
		procShowWindow.Call(w, swRestore)
	}
	procSetForegroundWindow.Call(w)
	return true
}

// enumAppWindow looks for a visible Chromium window showing one of the app's pages (their titles
// end in "Transcribe by Moliri").
func enumAppWindow(w, _ uintptr) uintptr {
	if visible, _, _ := procIsWindowVisible.Call(w); visible == 0 {
		return 1
	}
	buf := make([]uint16, 256)
	n, _, _ := procGetClassName.Call(w, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if syscall.UTF16ToString(buf[:n]) != "Chrome_WidgetWin_1" {
		return 1
	}
	n, _, _ = procGetWindowText.Call(w, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if !strings.HasSuffix(syscall.UTF16ToString(buf[:n]), "Transcribe by Moliri") {
		return 1
	}
	foundAppWindow = w
	return 0
}

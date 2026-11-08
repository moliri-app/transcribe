// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"encoding/binary"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	procCreateMutex     = kernel32.NewProc("CreateMutexW")
	procCloseHandle     = kernel32.NewProc("CloseHandle")
	procEnumWindows     = user32.NewProc("EnumWindows")
	procGetClassName    = user32.NewProc("GetClassNameW")
	procGetWindowText   = user32.NewProc("GetWindowTextW")
	procIsWindowVisible = user32.NewProc("IsWindowVisible")
	procIsIconic        = user32.NewProc("IsIconic")
	procGetWindowPID    = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess     = kernel32.NewProc("OpenProcess")
	procQueryProcess    = syscall.NewLazyDLL("ntdll.dll").NewProc("NtQueryInformationProcess")
	enumAppWindows      = syscall.NewCallback(enumAppWindow)
	foundAppWindows     []uintptr
	foundAppWindowsMu   sync.Mutex
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

// appWindows returns the open windows of the app serving dataDir: visible Chromium windows showing
// one of its pages (their titles end in "Transcribe by Moliri"; a browser's own windows add its
// name), of the browser that runs on its profile (openAppWindow), so not those of another app
// serving another data dir.
func appWindows(dataDir string) []uintptr {
	foundAppWindowsMu.Lock()
	foundAppWindows = nil
	procEnumWindows.Call(enumAppWindows, 0)
	ws := foundAppWindows
	foundAppWindowsMu.Unlock()
	flag := strings.ToLower("--user-data-dir=" + browserProfile(dataDir))
	var out []uintptr
	cmdLines := map[uint32]string{} // a browser's windows share its process
	for _, w := range ws {
		var pid uint32
		procGetWindowPID.Call(w, uintptr(unsafe.Pointer(&pid)))
		line, ok := cmdLines[pid]
		if !ok {
			line = strings.ToLower(commandLine(pid))
			cmdLines[pid] = line
		}
		if strings.Contains(line, flag) {
			out = append(out, w)
		}
	}
	return out
}

// hasAppWindow reports whether a window of the app serving dataDir is open, minimized or not.
func hasAppWindow(dataDir string) bool { return len(appWindows(dataDir)) > 0 }

// focusAppWindow brings the open window of the app serving dataDir to the front; false if it
// found none.
func focusAppWindow(dataDir string) bool {
	ws := appWindows(dataDir)
	if len(ws) == 0 {
		return false
	}
	w := ws[0] // the topmost
	const swRestore = 9
	if minimized, _, _ := procIsIconic.Call(w); minimized != 0 {
		procShowWindow.Call(w, swRestore)
	}
	procSetForegroundWindow.Call(w)
	return true
}

// closeAppWindows closes the open windows of the app serving dataDir, as their close button would.
func closeAppWindows(dataDir string) {
	for _, w := range appWindows(dataDir) {
		procPostMessage.Call(w, wmClose, 0, 0)
	}
}

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
	if strings.HasSuffix(syscall.UTF16ToString(buf[:n]), "Transcribe by Moliri") {
		foundAppWindows = append(foundAppWindows, w)
	}
	return 1
}

// commandLine returns the command line process pid was started with, "" when there is no telling.
func commandLine(pid uint32) string {
	const processQueryLimitedInformation = 0x1000
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)
	// ProcessCommandLineInformation (Windows 8.1 on): a UNICODE_STRING whose text follows it
	const processCommandLineInformation = 60
	buf := make([]byte, 1<<16+64)
	var n uint32
	if st, _, _ := procQueryProcess.Call(h, processCommandLineInformation, uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)), uintptr(unsafe.Pointer(&n))); st != 0 {
		return ""
	}
	size := int(binary.LittleEndian.Uint16(buf[0:]))
	start := int(uintptr(binary.LittleEndian.Uint64(buf[8:])) - uintptr(unsafe.Pointer(&buf[0])))
	if start < 16 || start+size > len(buf) {
		return ""
	}
	text := make([]uint16, size/2)
	for i := range text {
		text[i] = binary.LittleEndian.Uint16(buf[start+2*i:])
	}
	return syscall.UTF16ToString(text)
}

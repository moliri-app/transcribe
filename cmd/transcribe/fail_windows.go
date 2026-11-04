// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"syscall"
	"unsafe"
)

// showError displays a native message box; the EXE is built without a console.
func showError(msg string) {
	title, _ := syscall.UTF16PtrFromString("Transcribe by Moliri")
	text, _ := syscall.UTF16PtrFromString(msg)
	const mbOK, mbIconError = 0, 0x10
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0,
		uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), mbOK|mbIconError)
}

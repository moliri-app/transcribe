// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package prio

import "syscall"

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	setPriorityClass = kernel32.NewProc("SetPriorityClass")
)

const (
	normalPriorityClass      = 0x00000020
	belowNormalPriorityClass = 0x00004000
)

func background(on bool) {
	class := uintptr(normalPriorityClass)
	if on {
		class = belowNormalPriorityClass
	}
	h, err := syscall.GetCurrentProcess()
	if err != nil {
		return
	}
	setPriorityClass.Call(uintptr(h), class)
}

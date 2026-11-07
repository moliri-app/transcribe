// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package llm

import (
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// The server holds some gigabytes of memory: it must not outlive the app, also when the app
// crashes. A job object that ends its processes when its last handle closes (with the app) does
// that.

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	createJobObject   = kernel32.NewProc("CreateJobObjectW")
	setInformationJob = kernel32.NewProc("SetInformationJobObject")
	assignProcess     = kernel32.NewProc("AssignProcessToJobObject")

	jobOnce sync.Once
	job     uintptr
)

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x2000
	processSetQuota                   = 0x0100
	processTerminate                  = 0x0001
)

type ioCounters struct{ ReadOps, WriteOps, OtherOps, ReadBytes, WriteBytes, OtherBytes uint64 }

type basicLimit struct {
	PerProcessUserTimeLimit, PerJobUserTimeLimit int64
	LimitFlags                                   uint32
	MinimumWorkingSetSize, MaximumWorkingSetSize uintptr
	ActiveProcessLimit                           uint32
	Affinity                                     uintptr
	PriorityClass, SchedulingClass               uint32
}

type extendedLimit struct {
	Basic                                    basicLimit
	IO                                       ioCounters
	ProcessMemoryLimit, JobMemoryLimit       uintptr
	PeakProcessMemoryUsed, PeakJobMemoryUsed uintptr
}

func bindToApp(p *os.Process) {
	jobOnce.Do(func() {
		h, _, _ := createJobObject.Call(0, 0)
		if h == 0 {
			return
		}
		var info extendedLimit
		info.Basic.LimitFlags = jobObjectLimitKillOnJobClose
		if ok, _, _ := setInformationJob.Call(h, jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); ok == 0 {
			syscall.CloseHandle(syscall.Handle(h))
			return
		}
		job = h
	})
	if job == 0 || p == nil {
		return
	}
	h, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(p.Pid))
	if err != nil {
		return
	}
	defer syscall.CloseHandle(h)
	assignProcess.Call(job, uintptr(h))
}

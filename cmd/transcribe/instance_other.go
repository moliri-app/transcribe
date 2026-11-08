// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// lockInstance claims dataDir for this app with a lock on <data>/instance.lock, which the system
// drops when the app ends however it ends; ok is false while another app holds it. release lets
// go of it.
func lockInstance(dataDir string) (release func(), ok bool) {
	_ = os.MkdirAll(dataDir, 0o755)
	f, err := os.OpenFile(filepath.Join(dataDir, "instance.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return func() {}, true // no telling: start as without
	}
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		f.Close()
		return nil, false
	}
	var once sync.Once
	return func() { once.Do(func() { f.Close() }) }, true
}

// hasAppWindow would report whether a window of the app is open; there is no telling here, so the
// page's heartbeat alone decides.
func hasAppWindow(dataDir string) bool { return false }

// focusAppWindow brings the app's open window to the front; there is no telling which it is here.
func focusAppWindow(dataDir string) bool { return false }

// closeAppWindows would close the app's open windows; there is no telling which they are here.
func closeAppWindows(dataDir string) {}

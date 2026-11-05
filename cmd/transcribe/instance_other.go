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

// focusAppWindow brings the app's open window to the front; there is no telling which it is here.
func focusAppWindow() bool { return false }

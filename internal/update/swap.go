// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package update

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Windows lets a running EXE be renamed but not deleted or overwritten. So the update moves the
// running transcribe.exe aside to transcribe.old.exe, puts the new one in its place and starts
// it; the next start deletes the old one. Everything stays in the EXE's folder, data/ beside it
// is not touched.

// sibling is exe's name with tag before the extension: transcribe.exe -> transcribe.<tag>.exe.
func sibling(exe, tag string) string {
	ext := filepath.Ext(exe)
	return strings.TrimSuffix(exe, ext) + "." + tag + ext
}

// OldPath is where Swap moves the running EXE: transcribe.old.exe beside it.
func OldPath(exe string) string { return sibling(exe, "old") }

// newPath is where the download of the new EXE goes until it takes the place of the old one.
func newPath(exe string) string { return sibling(exe, "new") }

// Swap puts the file next in the place of exe and keeps exe as OldPath(exe). When the second step
// fails, exe is put back, so there is always an EXE to start.
func Swap(exe, next string) error {
	old := OldPath(exe)
	// a leftover of an earlier update that could not be deleted then
	if err := os.Remove(old); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(next, exe); err != nil {
		if back := os.Rename(old, exe); back != nil {
			return errors.Join(err, back)
		}
		return err
	}
	return nil
}

// Cleanup deletes what an update left beside exe: the old EXE, which the process that started
// this one may still be running for a moment, and a download that did not finish. It retries for
// a while and gives up quietly.
func Cleanup(exe string) {
	_ = os.Remove(newPath(exe))
	old := OldPath(exe)
	for range 30 {
		err := os.Remove(old)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(time.Second)
	}
}

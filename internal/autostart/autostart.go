// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package autostart starts the app in the background when the user signs in to
// Windows. It uses the user's Run key, so it needs no administrator rights; a
// Windows service would run in another session, without tray icon, window or
// microphone.
package autostart

import "path/filepath"

// Name is the entry shown in the Task Manager's startup apps.
const Name = "Transcribe by Moliri"

// command is what Windows runs at sign-in: the EXE in background mode, with --data when a start
// parameter fixed the data dir (dataDir is "" when the app finds it itself, internal/datadir).
func command(exe, dataDir string) string {
	q := func(s string) string { return `"` + s + `"` }
	cmd := q(exe) + " --background"
	if dataDir == "" {
		return cmd
	}
	if abs, err := filepath.Abs(dataDir); err == nil {
		dataDir = abs
	}
	return cmd + " --data " + q(dataDir)
}

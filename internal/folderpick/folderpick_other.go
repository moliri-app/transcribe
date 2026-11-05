// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

// Package folderpick shows the system's dialog for choosing a folder. The browser cannot name a
// folder on disk, so the app shows it for the settings page.
package folderpick

import "errors"

// Supported reports whether this system has the dialog; elsewhere the settings page asks for the
// path as text.
func Supported() bool { return false }

// Pick is not available on this system.
func Pick(title, start string) (string, error) {
	return "", errors.New("Die Ordnerauswahl gibt es nur unter Windows.")
}

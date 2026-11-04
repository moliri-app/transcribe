// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package autostart

import "errors"

// Supported reports whether this system can start the app at sign-in (only Windows).
func Supported() bool { return false }

// Enabled reports whether the app starts at sign-in.
func Enabled() bool { return false }

// Set turns starting at sign-in on or off.
func Set(on bool, dataDir string) error {
	return errors.New("Autostart gibt es nur unter Windows.")
}

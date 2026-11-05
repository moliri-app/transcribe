// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package pick

// Supported reports whether Folder shows a dialog.
func Supported() bool { return false }

// Folder is not available here.
func Folder(title string) (string, error) { return "", ErrUnsupported }

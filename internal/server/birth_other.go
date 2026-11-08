// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows && !darwin

package server

import (
	"os"
	"time"
)

// birth is when the file was created on this disk; os.Stat does not say on Linux.
func birth(os.FileInfo) time.Time { return time.Time{} }

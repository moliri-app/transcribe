// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"os"
	"syscall"
	"time"
)

// birth is when the file was created on this disk; zero where the system does not say.
func birth(st os.FileInfo) time.Time {
	if d, ok := st.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, d.CreationTime.Nanoseconds())
	}
	return time.Time{}
}

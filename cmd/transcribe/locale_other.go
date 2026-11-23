// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package main

import "os"

// systemLocale is the language the environment names (LC_ALL, LC_MESSAGES, LANG), as "de_DE.UTF-8".
func systemLocale() string {
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if l := os.Getenv(v); l != "" && l != "C" && l != "POSIX" {
			return l
		}
	}
	return ""
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import "syscall"

// systemLocale is the language of Windows' interface, as a code like "de"; "" for one the app
// does not speak.
func systemLocale() string {
	id, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage").Call()
	// the primary language of the LANGID
	switch id & 0x3ff {
	case 0x07:
		return "de"
	case 0x09:
		return "en"
	case 0x0a:
		return "es"
	case 0x0c:
		return "fr"
	case 0x11:
		return "ja"
	case 0x22:
		return "uk"
	case 0x08:
		return "el"
	}
	return ""
}

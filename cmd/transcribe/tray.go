// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

// trayActions are what the tray icon's menu does; each runs on its own goroutine.
type trayActions struct {
	open     func() // click on the icon, or "Transcribe öffnen"
	settings func()
	quit     func()
	// the shortcut for dictating (dictate.go), and Esc while dictating
	dictate, cancelDictate func()
	openNote               func(id string) // a click on the dictation's window
	// the menu's entry for an update: its text ("" none) and whether it can be clicked (updateMenuItem)
	updateItem func() (string, bool)
	update     func()
}

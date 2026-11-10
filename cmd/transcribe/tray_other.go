// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package main

import "time"

// tray is only implemented on Windows; elsewhere the app runs without an icon.
type tray struct{}

func startTray(trayActions) *tray       { return nil }
func (*tray) notify(title, text string) {}
func (*tray) notifyThen(string, string, func()) {}
func (*tray) close()                    {}

// dictating (dictate.go) is only implemented on Windows as well
func (*tray) hotkey(string) error         { return errNoTray }
func (*tray) escape(bool)                 {}
func (*tray) show(overlay, time.Duration) {}
func setClipboard(string) error           { return errNoTray }
func paste()                              {}
func typeText(string)                     {}

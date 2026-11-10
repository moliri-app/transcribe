// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package prio lowers the app's share of the processor while it works in the background, so the
// rest of the computer stays responsive: a transcription still uses whatever the processor has
// spare, but the programs one works with come first.
package prio

// Background lowers the priority of the whole process (and of the programs it starts meanwhile,
// such as ffmpeg) when on is true, and gives it back when on is false. It does nothing where the
// operating system offers no such class (only Windows for now).
func Background(on bool) { background(on) }

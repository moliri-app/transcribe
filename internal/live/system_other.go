// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package live

import "context"

// loopback is Windows only; elsewhere ffmpeg records the system sound (or cannot).
func loopback(context.Context, func([]float32)) error { return errNoSystem }

// defaultMic is Windows only; elsewhere ffmpeg records the system's default microphone.
func defaultMic(context.Context, func([]float32)) error { return errNoDevice }

// namedMic is Windows only; elsewhere ffmpeg records the microphone chosen.
func namedMic(context.Context, string, func([]float32)) (bool, error) { return false, nil }

// deviceState is Windows only: elsewhere the microphones are not listed.
func deviceState() (stamp, def string, ok bool) { return "", "", false }

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import "github.com/moliri-app/transcribe/stream"

// The stream a recognizer gives and its punctuation marks are the engine's (package stream).

const (
	SR    = stream.SR
	Marks = stream.Marks
)

type (
	Event  = stream.Event
	Stream = stream.Stream
	Hearer = stream.Hearer
	Opener = stream.Opener
)

var (
	AddMark        = stream.AddMark
	Attach         = stream.Attach
	StartsWithMark = stream.StartsWithMark
	JoinWords      = stream.JoinWords
)

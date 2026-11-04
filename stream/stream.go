// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package stream holds what a live transcription stream gives and takes: its events, the sample
// rate, and the spoken punctuation marks. The engine (package pipeline) opens streams; the desktop
// app (internal/live) feeds them from the microphone or a program.
package stream

import "context"

// SR is the sample rate a Stream takes (mono float32).
const SR = 16000

// Event is what a Stream recognised. A partial is the utterance still being spoken, as far as it
// goes; it changes until the final for the same utterance replaces it.
type Event struct {
	Final bool
	Text  string
	Start float64 // seconds since the stream began
	End   float64
	// Source is where it was said when the recording mixes microphone and system audio:
	// SourceMic or SourceSystem, whichever was louder over the utterance.
	Source string
}

// Stream is speech recognition fed bit by bit. Not safe for concurrent use.
type Stream interface {
	// Accept takes the next samples and returns the utterances that ended in them.
	Accept(pcm []float32) []Event
	// Preview decodes the utterance going on; false if nothing changed since the last call.
	Preview() (Event, bool)
	// Flush ends the utterance going on and returns it (and anything else still open).
	Flush() []Event
	Close()
}

// Hearer is a Stream that tells what it hears before it recognises it: the Live page shows
// placeholders for it.
type Hearer interface {
	// Pending is the speech going on that no event has covered yet, in seconds since the stream
	// began; ok is false while nobody speaks.
	Pending() (from, to float64, ok bool)
}

// Opener loads what recognition needs (downloading models on first use) and opens a Stream for
// lang: "auto" or a code like de.
type Opener func(ctx context.Context, lang string) (Stream, error)

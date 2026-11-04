// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package jobs is what every queue of recognition jobs shares: the options of a job, its progress
// and what only the work itself can measure. The queues are each program's own (the desktop app's
// in internal/queue, the server's with fairness per instance).
package jobs

import "context"

// Progress reports step of total steps, with a label such as "transcribe".
type Progress func(step, total int, label string)

// Measured is what only the work knows about a job: filled in while it runs, read by the queue
// when it is done.
type Measured struct {
	AudioS  float64 // length of the recording in seconds
	Threads int     // threads the job used
}

type measuredKey struct{}

// WithMeasured returns a context under which SetAudio and SetThreads fill in m.
func WithMeasured(ctx context.Context) (context.Context, *Measured) {
	m := &Measured{}
	return context.WithValue(ctx, measuredKey{}, m), m
}

// SetAudio records the length of the recording of the job running under ctx.
func SetAudio(ctx context.Context, seconds float64) {
	if m, ok := ctx.Value(measuredKey{}).(*Measured); ok {
		m.AudioS = seconds
	}
}

// SetThreads records how many threads the job running under ctx uses.
func SetThreads(ctx context.Context, n int) {
	if m, ok := ctx.Value(measuredKey{}).(*Measured); ok {
		m.Threads = n
	}
}

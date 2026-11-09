// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"cmp"
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestRecordDevice records a microphone by name together with the system sound, as the Live page
// does, and tells how much sound came in the time it ran; what went missing is in the log. Set
// TRANSCRIBE_TEST_MIC to the microphone's name (and TRANSCRIBE_TEST_SECONDS, default 60).
func TestRecordDevice(t *testing.T) {
	name := os.Getenv("TRANSCRIBE_TEST_MIC")
	if name == "" {
		t.Skip("set TRANSCRIBE_TEST_MIC to a microphone's name")
	}
	t.Setenv("TRANSCRIBE_LIVE_INPUT", "")
	t.Setenv("TRANSCRIBE_LIVE_SYSTEM_INPUT", "")
	secs, _ := strconv.Atoi(cmp.Or(os.Getenv("TRANSCRIBE_TEST_SECONDS"), "60"))
	start := time.Now()
	var n, quiet int
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(secs)*time.Second)
	defer cancel()
	err := capture(ctx, SourceBoth, name, func(pcm, mic, sys []float32) {
		n += len(pcm)
		if rms(mic) == 0 {
			quiet += len(mic)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%.1f s recorded in %.1f s, %.1f s of it without the microphone", float64(n)/SR, time.Since(start).Seconds(), float64(quiet)/SR)
}

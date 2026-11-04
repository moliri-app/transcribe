// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"context"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/moliri-app/transcribe/audio"
)

// feed hands sound to the check in pieces of 100 ms, all of it speech, and gives its last verdict.
func feed(c *inputCheck, pcm []float32) string {
	v := ""
	for i := 0; i+SR/10 <= len(pcm); i += SR / 10 {
		v = c.add(pcm[i:i+SR/10], true)
	}
	return v
}

func tones(seconds, amp float64, freqs ...float64) []float32 {
	out := make([]float32, int(seconds*SR))
	for i := range out {
		var s float64
		for _, f := range freqs {
			s += math.Sin(2 * math.Pi * f * float64(i) / SR)
		}
		out[i] = float32(amp * s / float64(len(freqs)))
	}
	return out
}

func TestInputCheck(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	wide := make([]float32, 5*SR)
	for i := range wide {
		wide[i] = float32(0.1 * r.NormFloat64())
	}
	if v := feed(&inputCheck{}, wide); v != "" {
		t.Errorf("full band: %q", v)
	}
	if v := feed(&inputCheck{}, tones(5, 0.2, 300, 1100, 2500)); v != InputNarrow {
		t.Errorf("below 3 kHz: %q", v)
	}
	if v := feed(&inputCheck{}, tones(2, 0.2, 300, 1100, 2500)); v != "" {
		t.Errorf("after 2 s of speech it does not judge yet: %q", v)
	}
	quiet := wide[:]
	for i := range quiet {
		quiet[i] *= 0.03
	}
	if v := feed(&inputCheck{}, quiet); v != InputQuiet {
		t.Errorf("-60 dB: %q", v)
	}
	// silence in between does not count
	c := &inputCheck{}
	for range 100 {
		c.add(make([]float32, SR/10), false)
	}
	if v := c.verdict(); v != "" {
		t.Errorf("silence: %q", v)
	}
}

// With the live bench (see TestLiveBench in pipeline): the clean recording passes, the
// headset one is narrow.
func TestInputCheckBench(t *testing.T) {
	bench := os.Getenv("TRANSCRIBE_LIVE_BENCH")
	if bench == "" {
		t.Skip("set TRANSCRIBE_LIVE_BENCH")
	}
	for file, want := range map[string]string{"clean.wav": "", "headset.wav": InputNarrow} {
		pcm, err := audio.Decode(context.Background(), filepath.Join(bench, file))
		if err != nil {
			t.Skip(err)
		}
		c := &inputCheck{}
		var lo, hi = 1.0, 0.0
		for i := 0; i+SR/10 <= len(pcm); i += SR / 10 {
			if v := c.add(pcm[i:i+SR/10], rms32(pcm[i:i+SR/10]) > 0.01); len(c.total) >= inputEnough {
				var total, high float64
				for k := range c.total {
					total, high = total+c.total[k], high+c.high[k]
				}
				lo, hi = math.Min(lo, high/total), math.Max(hi, high/total)
				if v != want {
					t.Errorf("%s at %.1f s: %q, want %q", file, float64(i)/SR, v, want)
					break
				}
			}
		}
		t.Logf("%s: share above 4.5 kHz %.2g to %.2g", file, lo, hi)
	}
}

func rms32(x []float32) float64 {
	var s float64
	for _, v := range x {
		s += float64(v) * float64(v)
	}
	return math.Sqrt(s / float64(max(1, len(x))))
}

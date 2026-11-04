// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"math"
	"testing"

	"github.com/moliri-app/transcribe/internal/live"
)

func sine(seconds, amp float64) []float32 {
	out := make([]float32, int(seconds*live.SR))
	for i := range out {
		out[i] = float32(amp * math.Sin(2*math.Pi*300*float64(i)/live.SR))
	}
	return out
}

func dB(x []float32) float64 { return 20 * math.Log10(rms(x)) }

// Quiet speech is raised slowly towards -20 dBFS, at most by 20 dB; loud speech is left as it is,
// and a sudden loud sound does not clip.
func TestGainControl(t *testing.T) {
	var g gainControl
	quiet := sine(20, 0.02*math.Sqrt2) // -34 dBFS
	var out []float32
	for i := 0; i < len(quiet); i += live.SR / 10 {
		out = append(out, g.apply(quiet[i:i+live.SR/10], true)...)
	}
	if l := dB(out[:live.SR/10]); l > -33 {
		t.Errorf("first 100 ms at %.1f dB: it must rise slowly", l)
	}
	if l := dB(out[len(out)-live.SR:]); l < -21.5 || l > -19 {
		t.Errorf("after 20 s at %.1f dB, want about -20", l)
	}
	loud := g.apply(sine(0.1, 0.8), true)
	for _, v := range loud {
		if math.Abs(float64(v)) > gainPeak+1e-6 {
			t.Fatalf("a loud sound after quiet speech clips: %.2f", v)
		}
	}

	var h gainControl
	if l := dB(h.apply(sine(5, 0.3), true)); math.Abs(l-dB(sine(5, 0.3))) > 0.01 {
		t.Errorf("loud speech changed to %.1f dB", l)
	}
	var s gainControl
	if l := dB(s.apply(sine(5, 0.001), false)); math.Abs(l-dB(sine(5, 0.001))) > 0.01 {
		t.Errorf("silence raised to %.1f dB", l)
	}
}

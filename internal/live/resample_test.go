// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"encoding/binary"
	"math"
	"testing"
)

func tone(hz float64, rate, n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = float32(0.5 * math.Sin(2*math.Pi*hz*float64(i)/float64(rate)))
	}
	return out
}

func rms(pcm []float32) float64 {
	var s float64
	for _, v := range pcm {
		s += float64(v) * float64(v)
	}
	return math.Sqrt(s / float64(len(pcm)))
}

func TestResamplerKeepsSpeechAndDropsWhatWouldFoldBack(t *testing.T) {
	for _, from := range []int{48000, 44100} {
		for _, c := range []struct {
			hz       float64
			min, max float64
		}{{1000, 0.34, 0.37}, {6000, 0.3, 0.37}, {12000, 0, 0.01}} {
			rs := newResampler(from, SR)
			in := tone(c.hz, from, from) // 1 s
			var out []float32
			for i := 0; i < len(in); i += 480 { // in device-sized pieces
				out = append(out, rs.process(in[i:min(len(in), i+480)])...)
			}
			if n := len(out); n < SR-100 || n > SR {
				t.Errorf("%d Hz → 16 kHz: %d samples for 1 s", from, n)
			}
			if r := rms(out[1000:]); r < c.min || r > c.max { // a sine of 0.5 has an RMS of 0.354
				t.Errorf("%d Hz → 16 kHz, tone of %v Hz: RMS %.3f, want %.2f..%.2f", from, c.hz, r, c.min, c.max)
			}
		}
	}
}

func TestDownmixAveragesTheChannels(t *testing.T) {
	b := make([]byte, 8*2)
	binary.LittleEndian.PutUint32(b[0:], math.Float32bits(0.5))
	binary.LittleEndian.PutUint32(b[4:], math.Float32bits(-0.25))
	binary.LittleEndian.PutUint32(b[8:], math.Float32bits(1))
	binary.LittleEndian.PutUint32(b[12:], math.Float32bits(1))
	got := downmix(b, 2, kindFloat32)
	if len(got) != 2 || got[0] != 0.125 || got[1] != 1 {
		t.Errorf("float32 stereo: %v", got)
	}
	got = downmix([]byte{0x00, 0x40, 0x00, 0xC0}, 1, kindInt16) // 0.5, -0.5
	if len(got) != 2 || got[0] != 0.5 || got[1] != -0.5 {
		t.Errorf("int16 mono: %v", got)
	}
	got = downmix([]byte{0x00, 0x00, 0x40}, 1, kindInt24)
	if len(got) != 1 || got[0] != 0.5 {
		t.Errorf("int24 mono: %v", got)
	}
}

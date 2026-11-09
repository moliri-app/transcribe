// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import "math"

// What the microphone sends can be too poor for recognition: the Live page says so (MicState.Input).
const (
	InputNarrow = "narrow" // telephone quality: practically nothing above 4 kHz, as a Bluetooth headset in hands-free mode sends it
	InputQuiet  = "quiet"  // speech stays very quiet
)

const (
	inputPieces  = 50   // the last 5 s of speech decide
	inputEnough  = 30   // and 3 s of speech at least
	inputEdge    = 5000 // above it, telephone quality has nothing; at 4.5 kHz upsampling 8 kHz still leaves a trace
	inputNarrow  = 1e-4 // energy above inputEdge, relative to 1.5 to 3.4 kHz, below which the input is narrow (-40 dB)
	inputQuietMS = 3e-5 // mean square of speech below which it is very quiet (-45 dBFS)
)

// inputCheck watches the microphone while someone speaks. Telephone quality is told by its edge:
// speech up to 3.4 kHz and next to nothing above. The share of all the energy that lies high up
// does not tell it: a headset that sends the full band but sounds dark has little up there too
// (a tenth of a percent above 4.5 kHz), against the speech band it is still -30 dB rather than
// -60 dB. Not safe for concurrent use.
type inputCheck struct {
	hp, band         []biquad  // above inputEdge; 1.5 to 3.4 kHz
	total, mid, high []float64 // per piece of speech: energy, the part of it in band, and above inputEdge
	n                []int     // samples per piece
}

// add takes the next piece of the microphone, speech or not, and says what is wrong with it,
// if anything.
func (c *inputCheck) add(pcm []float32, speech bool) string {
	if c.hp == nil { // eighth-order Butterworth; fourth-order on either side of the band
		for _, q := range []float64{0.5098, 0.6013, 0.9000, 2.5629} {
			c.hp = append(c.hp, highpass(inputEdge, q))
		}
		for _, q := range []float64{0.5412, 1.3066} {
			c.band = append(c.band, highpass(1500, q), lowpass(3400, q))
		}
	}
	var total, mid, high float64
	for _, v := range pcm {
		x := float64(v)
		h, m := x, x
		for i := range c.hp {
			h = c.hp[i].step(h)
		}
		for i := range c.band {
			m = c.band[i].step(m)
		}
		total += x * x
		mid += m * m
		high += h * h
	}
	if speech && len(pcm) > 0 {
		c.total, c.mid, c.high, c.n = append(c.total, total), append(c.mid, mid), append(c.high, high), append(c.n, len(pcm))
		if len(c.total) > inputPieces {
			c.total, c.mid, c.high, c.n = c.total[1:], c.mid[1:], c.high[1:], c.n[1:]
		}
	}
	return c.verdict()
}

func (c *inputCheck) verdict() string {
	if len(c.total) < inputEnough {
		return ""
	}
	var total, mid, high float64
	n := 0
	for i := range c.total {
		total, mid, high, n = total+c.total[i], mid+c.mid[i], high+c.high[i], n+c.n[i]
	}
	switch {
	case total/float64(n) < inputQuietMS:
		return InputQuiet
	case high < inputNarrow*mid:
		return InputNarrow
	}
	return ""
}

// biquad is a second-order filter (direct form I).
type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

// highpass at f Hz for audio at SR (RBJ's cookbook); several in a row with the right Qs make a
// Butterworth of higher order.
func highpass(f, q float64) biquad {
	w := 2 * math.Pi * f / SR
	alpha := math.Sin(w) / (2 * q)
	cw := math.Cos(w)
	a0 := 1 + alpha
	return biquad{b0: (1 + cw) / 2 / a0, b1: -(1 + cw) / a0, b2: (1 + cw) / 2 / a0, a1: -2 * cw / a0, a2: (1 - alpha) / a0}
}

// lowpass is highpass's counterpart.
func lowpass(f, q float64) biquad {
	w := 2 * math.Pi * f / SR
	alpha := math.Sin(w) / (2 * q)
	cw := math.Cos(w)
	a0 := 1 + alpha
	return biquad{b0: (1 - cw) / 2 / a0, b1: (1 - cw) / a0, b2: (1 - cw) / 2 / a0, a1: -2 * cw / a0, a2: (1 - alpha) / a0}
}

func (b *biquad) step(x float64) float64 {
	y := b.b0*x + b.b1*b.x1 + b.b2*b.x2 - b.a1*b.y1 - b.a2*b.y2
	b.x2, b.x1, b.y2, b.y1 = b.x1, x, b.y1, y
	return y
}

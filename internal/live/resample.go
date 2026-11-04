// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"encoding/binary"
	"math"
)

// sampleKind is how one sample of a device's own format is stored.
type sampleKind int

const (
	kindFloat32 sampleKind = iota
	kindInt16
	kindInt24
	kindInt32
)

// downmix decodes interleaved frames of a device's format into mono samples (the channels'
// average).
func downmix(b []byte, channels int, kind sampleKind) []float32 {
	size := [...]int{4, 2, 3, 4}[kind]
	frames := len(b) / (channels * size)
	out := make([]float32, frames)
	for f := range out {
		var s float32
		for c := range channels {
			p := b[(f*channels+c)*size:]
			switch kind {
			case kindFloat32:
				s += math.Float32frombits(binary.LittleEndian.Uint32(p))
			case kindInt16:
				s += float32(int16(binary.LittleEndian.Uint16(p))) / (1 << 15)
			case kindInt24:
				s += float32(int32(uint32(p[0])<<8|uint32(p[1])<<16|uint32(p[2])<<24)) / (1 << 31)
			case kindInt32:
				s += float32(int32(binary.LittleEndian.Uint32(p))) / (1 << 31)
			}
		}
		out[f] = s / float32(channels)
	}
	return out
}

// resampler converts a stream of mono samples from one rate to another with a windowed sinc,
// cut off below half the lower rate so nothing folds back into what is heard.
type resampler struct {
	step   float64   // input samples per output sample
	half   int       // taps on either side
	kernel []float64 // the filter, kernelRes points per input sample, from -half to half
	buf    []float32 // input still needed
	pos    float64   // where in buf the next output sample lies
}

const kernelRes = 64

func newResampler(from, to int) *resampler {
	r := &resampler{step: float64(from) / float64(to)}
	if from == to {
		return r
	}
	cut := 0.46 / max(1, r.step) // in cycles per input sample
	r.half = int(math.Ceil(8 / (2 * cut)))
	r.kernel = make([]float64, 2*r.half*kernelRes+2)
	for i := range r.kernel {
		x := float64(i)/kernelRes - float64(r.half)
		w := 0.5 + 0.5*math.Cos(math.Pi*x/float64(r.half)) // Hann
		if math.Abs(x) >= float64(r.half) {
			w = 0
		}
		v := 2 * cut
		if x != 0 {
			v = math.Sin(2*math.Pi*cut*x) / (math.Pi * x)
		}
		r.kernel[i] = v * w
	}
	return r
}

// process takes the next input samples and returns the output they complete.
func (r *resampler) process(in []float32) []float32 {
	if r.kernel == nil {
		return in
	}
	r.buf = append(r.buf, in...)
	var out []float32
	for {
		c := int(r.pos)
		if c+r.half >= len(r.buf) {
			break
		}
		var s float64
		for i := max(0, c-r.half+1); i <= c+r.half; i++ {
			k := (r.pos - float64(i) + float64(r.half)) * kernelRes
			j := int(k)
			f := k - float64(j)
			s += float64(r.buf[i]) * (r.kernel[j]*(1-f) + r.kernel[j+1]*f)
		}
		out = append(out, float32(s))
		r.pos += r.step
	}
	if drop := int(r.pos) - r.half; drop > 0 {
		r.buf = append(r.buf[:0], r.buf[drop:]...)
		r.pos -= float64(drop)
	}
	return out
}

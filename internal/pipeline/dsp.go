// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"math"
	"sort"

	"github.com/moliri-app/transcribe/internal/audio"
)

// snapToPause moves a cut to the quietest 100 ms within ±radius.
func snapToPause(pcm []float32, t, radius float64) float64 {
	fr := int(0.02 * audio.SR)
	a := max(0, int((t-radius)*audio.SR))
	b := min(len(pcm), int((t+radius)*audio.SR))
	if b <= a {
		return t
	}
	x := pcm[a:b]
	n := len(x) / fr
	if n < 6 {
		return t
	}
	rms := make([]float64, n)
	for i := 0; i < n; i++ {
		var s float64
		for _, v := range x[i*fr : (i+1)*fr] {
			s += float64(v) * float64(v)
		}
		rms[i] = math.Sqrt(s / float64(fr))
	}
	// 5-frame moving average, same-size convolution (zero padded)
	sm := make([]float64, n)
	var mx float64
	for i := range sm {
		var s float64
		for k := -2; k <= 2; k++ {
			if j := i + k; j >= 0 && j < n {
				s += rms[j]
			}
		}
		sm[i] = s / 5
		mx = math.Max(mx, sm[i])
	}
	best, bv := 0, math.Inf(1)
	for i := range sm {
		pos := float64(i)*0.02 + float64(a)/audio.SR
		v := sm[i] + math.Abs(pos-t)/radius*0.1*(mx+1e-9)
		if v < bv {
			best, bv = i, v
		}
	}
	return float64(a)/audio.SR + (float64(best)+0.5)*0.02
}

// speechActivity is the share of 50 ms frames louder than 30 % of refRMS.
func speechActivity(pcm []float32, a, b, refRMS float64) float64 {
	fr := int(0.05 * audio.SR)
	x := audio.Slice(pcm, a, b)
	n := len(x) / fr
	if n == 0 {
		return 0
	}
	hit := 0
	for i := 0; i < n; i++ {
		var s float64
		for _, v := range x[i*fr : (i+1)*fr] {
			s += float64(v) * float64(v)
		}
		if math.Sqrt(s/float64(fr)) > 0.3*refRMS {
			hit++
		}
	}
	return float64(hit) / float64(n)
}

func rms(x []float32) float64 {
	if len(x) == 0 {
		return 0
	}
	var s float64
	for _, v := range x {
		s += float64(v) * float64(v)
	}
	return math.Sqrt(s / float64(len(x)))
}

func round(v float64, d int) float64 {
	p := math.Pow10(d)
	return math.Round(v*p) / p
}

func sortedKeys(m map[int]float64) []int {
	k := make([]int, 0, len(m))
	for i := range m {
		k = append(k, i)
	}
	sort.Ints(k)
	return k
}

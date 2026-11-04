// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestPCMDecoderSplitsAndResamples(t *testing.T) {
	// 16 kHz s16le, split in the middle of a sample
	b := make([]byte, 8)
	for i, v := range []int16{0, 16384, -16384, 32767} {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(v))
	}
	d := pcmDecoder{ratio: 1}
	got := append(d.decode(b[:3]), d.decode(b[3:])...)
	if len(got) != 4 || got[1] != 0.5 || got[2] != -0.5 {
		t.Fatalf("s16le: %v", got)
	}

	// 48 kHz in pieces of odd length gives a third as many samples, following the signal
	d = pcmDecoder{f32: true, ratio: 3}
	var out []float32
	n := 0
	for _, size := range []int{7, 100, 13, 480} {
		piece := make([]byte, 4*size)
		for i := range size {
			binary.LittleEndian.PutUint32(piece[4*i:], math.Float32bits(float32(n)/48000))
			n++
		}
		out = append(out, d.decode(piece)...)
	}
	if len(out) < n/3-1 || len(out) > n/3+1 {
		t.Fatalf("%d samples at 48 kHz gave %d at 16 kHz", n, len(out))
	}
	for i, v := range out {
		if want := float32(i) / 16000; math.Abs(float64(v-want)) > 1e-4 {
			t.Fatalf("sample %d: %v, want %v", i, v, want)
		}
	}
}

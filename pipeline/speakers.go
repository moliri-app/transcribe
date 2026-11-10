// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"context"
	"math"
	"sort"

	"github.com/moliri-app/transcribe/audio"
	"github.com/moliri-app/transcribe/jobs"
	"github.com/moliri-app/transcribe/sherpa"
)

type SpeakerSeg struct {
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Speaker int     `json:"speaker"`
}

type merge struct {
	d    float32
	a, b int
	seq  int
}

// averageLinkage runs average-linkage agglomerative clustering (nearest-neighbour chain) on the
// cosine distance of unit vectors and returns the merges in order of increasing distance.
func averageLinkage(ctx context.Context, x [][]float32) ([]merge, error) {
	n := len(x)
	if n < 2 {
		return nil, nil
	}
	d := make([]float32, n*n)
	for i := 0; i < n; i++ {
		if err := jobs.Hold(ctx); err != nil {
			return nil, err
		}
		for j := i + 1; j < n; j++ {
			var s float32
			for k, v := range x[i] {
				s += v * x[j][k]
			}
			d[i*n+j], d[j*n+i] = 1-s, 1-s
		}
	}
	size := make([]float32, n)
	active := make([]bool, n)
	for i := range size {
		size[i], active[i] = 1, true
	}
	var chain []int
	var out []merge
	for len(out) < n-1 {
		if err := jobs.Hold(ctx); err != nil {
			return nil, err
		}
		if len(chain) == 0 {
			for i := range active {
				if active[i] {
					chain = append(chain, i)
					break
				}
			}
		}
		var a, b int
		for {
			a = chain[len(chain)-1]
			prev := -1
			if len(chain) > 1 {
				prev = chain[len(chain)-2]
			}
			b = prev
			best := float32(math.Inf(1))
			if prev >= 0 {
				best = d[a*n+prev]
			}
			for k := 0; k < n; k++ {
				if active[k] && k != a && d[a*n+k] < best {
					best, b = d[a*n+k], k
				}
			}
			if b == prev {
				break
			}
			chain = append(chain, b)
		}
		chain = chain[:len(chain)-2]
		out = append(out, merge{d[a*n+b], a, b, len(out)})
		na, nb := size[a], size[b]
		for k := 0; k < n; k++ {
			if active[k] && k != a && k != b {
				v := (na*d[a*n+k] + nb*d[b*n+k]) / (na + nb)
				d[b*n+k], d[k*n+b] = v, v
			}
		}
		active[a] = false
		size[b] = na + nb
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].d < out[j].d })
	return out, nil
}

// cut returns labels 0..k-1 for either k clusters (k>0) or all merges with distance <= threshold.
func cut(n int, merges []merge, k int, threshold float64) []int {
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	clusters := n
	for _, m := range merges {
		if k > 0 && clusters <= k {
			break
		}
		if k <= 0 && float64(m.d) > threshold {
			break
		}
		ra, rb := find(m.a), find(m.b)
		if ra != rb {
			parent[ra] = rb
			clusters--
		}
	}
	ids := map[int]int{}
	labels := make([]int, n)
	for i := range labels {
		r := find(i)
		if _, ok := ids[r]; !ok {
			ids[r] = len(ids)
		}
		labels[i] = ids[r]
	}
	return labels
}

type span struct{ s, e float64 }

func normalize(v []float32) []float32 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	n := float32(math.Sqrt(s) + 1e-9)
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x / n
	}
	return out
}

func embedWindows(ctx context.Context, ex *sherpa.Embedder, pcm []float32, regions []span, win, hop float64) ([][]float32, []span, error) {
	var vecs [][]float32
	var spans []span
	for _, r := range regions {
		a, b := r.s, r.e
		if b-a < 0.5 {
			continue
		}
		var starts []float64
		if b-a > win {
			for s := a; s <= b-win+1e-6; s += hop {
				starts = append(starts, s)
			}
		} else {
			starts = []float64{a}
		}
		for _, s := range starts {
			if err := jobs.Hold(ctx); err != nil {
				return nil, nil, err
			}
			e := math.Min(b, s+win)
			v := ex.Embed(audio.Slice(pcm, s, e))
			if v == nil {
				continue
			}
			vecs = append(vecs, normalize(v))
			spans = append(spans, span{s, e})
		}
	}
	return vecs, spans, nil
}

// diarPiece is how much audio the sherpa diarizer gets per call, in samples. Its call runs to the
// end and cannot be stopped, so a cancelled job leaves at most this much work behind. Pieces of a
// minute change the speech regions of the whole file by about 0.25 % and are faster than one call
// over it, whose clustering grows with the square of its length.
const diarPiece = 60 * audio.SR

// roughSpeech runs the sherpa diarizer over pcm piece by piece and returns its segments, in seconds
// of pcm. The diarizer works apart from the job: on cancel it returns at once, and the diarizer
// finishes the piece it is on (or loading its models) and closes.
func roughSpeech(ctx context.Context, pcm []float32, segModel, embModel string, threads int) ([]sherpa.DiarSegment, error) {
	type result struct {
		segs []sherpa.DiarSegment
		err  error
	}
	done := make(chan result, 1)
	go func() {
		segs, err := diarizePieces(ctx, pcm, segModel, embModel, threads)
		done <- result{segs, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-done:
		return r.segs, r.err
	}
}

func diarizePieces(ctx context.Context, pcm []float32, segModel, embModel string, threads int) ([]sherpa.DiarSegment, error) {
	dz, err := sherpa.NewDiarizer(segModel, embModel, threads)
	if err != nil {
		return nil, err
	}
	defer dz.Close()
	var rough []sherpa.DiarSegment
	for a := 0; a < len(pcm); {
		if err := jobs.Hold(ctx); err != nil {
			return nil, err
		}
		b := a + diarPiece
		if len(pcm)-b < diarPiece/2 {
			b = len(pcm) // no short piece at the end
		}
		off := float64(a) / audio.SR
		for _, s := range dz.Process(pcm[a:b]) {
			s.Start += off
			s.End += off
			rough = append(rough, s)
		}
		a = b
	}
	return rough, nil
}

// DiarizeAHC2 finds speaker turns: sherpa pyannote for speech regions, then own embeddings,
// average-linkage clustering, smoothing and merging of tiny clusters.
// numSpeakers <= 0 means automatic (distance threshold).
// Cancelling ctx stops it within moments, at any stage.
func DiarizeAHC2(ctx context.Context, pcm []float32, segModel, embModel string, numSpeakers int, threshold float64, threads int) ([]SpeakerSeg, error) {
	rough, err := roughSpeech(ctx, pcm, segModel, embModel, threads)
	if err != nil {
		return nil, err
	}
	sort.Slice(rough, func(i, j int) bool { return rough[i].Start < rough[j].Start })
	var regions []span
	for _, s := range rough {
		if n := len(regions); n > 0 && s.Start <= regions[n-1].e+0.3 {
			regions[n-1].e = math.Max(regions[n-1].e, s.End)
		} else {
			regions = append(regions, span{s.Start, s.End})
		}
	}
	ex, err := sherpa.NewEmbedder(embModel, threads)
	if err != nil {
		return nil, err
	}
	defer ex.Close()
	X, spans, err := embedWindows(ctx, ex, pcm, regions, 1.5, 0.75)
	if err != nil || len(X) == 0 {
		return nil, err
	}
	merges, err := averageLinkage(ctx, X)
	if err != nil {
		return nil, err
	}
	k, scraps := numSpeakers, true
	if k > 0 {
		k, scraps = clustersFor(spans, merges, k, minSpeaker)
	}
	labels := cut(len(X), merges, k, threshold)

	// ahc2 smoothing: only an isolated window between two agreeing, touching neighbours
	sm := append([]int(nil), labels...)
	for i := 1; i < len(labels)-1; i++ {
		if spans[i-1].e >= spans[i].s && spans[i].e >= spans[i+1].s && labels[i-1] == labels[i+1] {
			sm[i] = labels[i-1]
		}
	}
	labels = sm

	if scraps {
		mergeSmallClusters(X, spans, labels, minSpeaker)
	}

	var segs []SpeakerSeg
	for i, sp := range spans {
		a := sp.s
		if i > 0 && spans[i-1].e > sp.s {
			a = (sp.s + spans[i-1].e) / 2
		}
		b := sp.e
		if i+1 < len(spans) && spans[i+1].s < sp.e {
			b = (sp.e + spans[i+1].s) / 2
		}
		if n := len(segs); n > 0 && segs[n-1].Speaker == labels[i] && a-segs[n-1].End < 0.3 {
			segs[n-1].End = round(b, 2)
		} else {
			segs = append(segs, SpeakerSeg{round(a, 2), round(b, 2), labels[i]})
		}
	}
	return renumber(segs), nil
}

// minSpeaker is the least speech, in seconds, a cluster needs to count as a speaker; smaller ones
// (a jingle, a cough, a laugh) go to the speaker whose voice is nearest.
const minSpeaker = 8.0

// clustersFor returns how many clusters to cut the tree into so that k of them hold at least
// minTotal seconds of speech each. Cut into exactly k, the tree often splits off a few seconds of
// intro music or noise and leaves all voices in one cluster; cut deeper, the voices part, and
// mergeSmallClusters then gives the scraps to them. When no cut has k such clusters (a short
// recording), it returns k and false: then the clusters stay as they are, small or not.
func clustersFor(spans []span, merges []merge, k int, minTotal float64) (int, bool) {
	n := len(spans)
	parent := make([]int, n)
	tot := make([]float64, n)
	big := 0
	for i, sp := range spans {
		parent[i] = i
		tot[i] = (sp.e - sp.s) / 2 // windows overlap by half
		if tot[i] >= minTotal {
			big++
		}
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	isBig := func(r int) int {
		if tot[r] >= minTotal {
			return 1
		}
		return 0
	}
	// bigAt[c] is the number of big clusters when the tree is cut into c clusters
	bigAt := make([]int, n+1)
	clusters := n
	bigAt[n] = big
	for _, m := range merges {
		ra, rb := find(m.a), find(m.b)
		if ra == rb {
			continue
		}
		big -= isBig(ra) + isBig(rb)
		parent[ra] = rb
		tot[rb] += tot[ra]
		big += isBig(rb)
		clusters--
		bigAt[clusters] = big
	}
	for c := k; c <= n; c++ {
		if bigAt[c] >= k {
			return c, true
		}
	}
	return k, false
}

func mergeSmallClusters(X [][]float32, spans []span, labels []int, minTotal float64) {
	for {
		tot := map[int]float64{}
		for i, sp := range spans {
			tot[labels[i]] += (sp.e - sp.s) / 2
		}
		var small, big []int
		for _, l := range sortedKeys(tot) {
			if tot[l] < minTotal {
				small = append(small, l)
			} else {
				big = append(big, l)
			}
		}
		if len(small) == 0 || len(big) == 0 {
			return
		}
		cent := map[int][]float64{}
		cnt := map[int]float64{}
		for i, l := range labels {
			if cent[l] == nil {
				cent[l] = make([]float64, len(X[i]))
			}
			for k, v := range X[i] {
				cent[l][k] += float64(v)
			}
			cnt[l]++
		}
		for l, c := range cent {
			for k := range c {
				c[k] /= cnt[l]
			}
		}
		l := small[0]
		for _, s := range small {
			if tot[s] < tot[l] {
				l = s
			}
		}
		target, bestSim := big[0], math.Inf(-1)
		for _, b := range big {
			var dot float64
			for k := range cent[l] {
				dot += cent[l][k] * cent[b][k]
			}
			if dot > bestSim {
				target, bestSim = b, dot
			}
		}
		for i := range labels {
			if labels[i] == l {
				labels[i] = target
			}
		}
	}
}

// renumber assigns speaker ids 0..n-1 in order of first appearance.
func renumber(segs []SpeakerSeg) []SpeakerSeg {
	ids := map[int]int{}
	for i := range segs {
		id, ok := ids[segs[i].Speaker]
		if !ok {
			id = len(ids)
			ids[segs[i].Speaker] = id
		}
		segs[i].Speaker = id
	}
	return segs
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr

import (
	"math"
	"sort"
)

// Text detection of PP-OCRv5 (DB, "differentiable binarization"): the model gives each pixel the
// probability that it belongs to text; the regions above a threshold, grown a little, are the
// boxes of the text lines. Pre- and post-processing follow RapidOCR 3.10 (ch_ppocr_det), with
// connected regions instead of OpenCV's contours.
const (
	detLimitSide  = 736 // the shorter side is scaled up to this at least
	detThresh     = 0.3 // pixel is text
	detBoxThresh  = 0.5 // mean probability of a region kept
	detUnclip     = 1.6 // how far a region is grown, relative to its area per perimeter
	detMinSide    = 3
	detCandidates = 1000
	sortRowDelta  = 10 // boxes whose tops are closer than this are one row
)

// quad is a text box: four corners, top left, top right, bottom right, bottom left.
type quad [4]pt

// detInput scales the image for detection: the shorter side at least detLimitSide, both sides a
// multiple of 32.
func detInput(r *raster) (w, h int) {
	ratio := 1.0
	if s := min(r.w, r.h); s < detLimitSide {
		ratio = float64(detLimitSide) / float64(s)
	}
	h = int(math.RoundToEven(float64(int(float64(r.h)*ratio))/32) * 32)
	w = int(math.RoundToEven(float64(int(float64(r.w)*ratio))/32) * 32)
	return max(w, 32), max(h, 32)
}

// detBoxes turns the probability map (w×h) into the text boxes, in pixels of an image of
// dstW×dstH, in reading order.
func detBoxes(prob []float32, w, h, dstW, dstH int) []quad {
	mask := make([]bool, w*h)
	for i, p := range prob {
		mask[i] = p > detThresh
	}
	// dilate with a 2×2 kernel anchored at its centre (OpenCV): a pixel is set when it, its left,
	// upper or upper left neighbour is
	dil := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			dil[i] = mask[i] || (x > 0 && mask[i-1]) || (y > 0 && mask[i-w]) || (x > 0 && y > 0 && mask[i-w-1])
		}
	}

	var boxes []quad
	for _, hull := range regions(dil, w, h, detCandidates) {
		box, side := miniBox(hull)
		if side < detMinSide {
			continue
		}
		if boxScore(prob, w, h, box) < detBoxThresh {
			continue
		}
		box, side = unclip(box)
		if side < detMinSide+2 {
			continue
		}
		for k := range box {
			box[k].x = min(max(math.RoundToEven(box[k].x/float64(w)*float64(dstW)), 0), float64(dstW))
			box[k].y = min(max(math.RoundToEven(box[k].y/float64(h)*float64(dstH)), 0), float64(dstH))
		}
		box = clockwise(box)
		for k := range box {
			box[k].x = math.Trunc(min(max(box[k].x, 0), float64(dstW-1)))
			box[k].y = math.Trunc(min(max(box[k].y, 0), float64(dstH-1)))
		}
		if int(box[0].sub(box[1]).norm()) <= 3 || int(box[0].sub(box[3]).norm()) <= 3 {
			continue
		}
		boxes = append(boxes, box)
	}
	sortBoxes(boxes)
	return boxes
}

// regions finds the 8-connected regions of mask and returns the convex hull of each (pixel
// centres), at most limit of them.
func regions(mask []bool, w, h, limit int) [][]pt {
	label := make([]int32, w*h)
	var hulls [][]pt
	var stack []int
	for start := range mask {
		if !mask[start] || label[start] != 0 || len(hulls) >= limit {
			continue
		}
		id := int32(len(hulls) + 1)
		// leftmost and rightmost pixel of every row the region touches
		rows := map[int][2]int{}
		label[start] = id
		stack = append(stack[:0], start)
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := i%w, i/w
			if e, ok := rows[y]; !ok {
				rows[y] = [2]int{x, x}
			} else {
				rows[y] = [2]int{min(e[0], x), max(e[1], x)}
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					j := ny*w + nx
					if mask[j] && label[j] == 0 {
						label[j] = id
						stack = append(stack, j)
					}
				}
			}
		}
		pts := make([]pt, 0, 2*len(rows))
		for y, e := range rows {
			pts = append(pts, pt{float64(e[0]), float64(y)}, pt{float64(e[1]), float64(y)})
		}
		hulls = append(hulls, convexHull(pts))
	}
	return hulls
}

// convexHull is the hull of pts, counter-clockwise (monotone chain).
func convexHull(pts []pt) []pt {
	sort.Slice(pts, func(i, j int) bool {
		if pts[i].x != pts[j].x {
			return pts[i].x < pts[j].x
		}
		return pts[i].y < pts[j].y
	})
	// drop duplicates (a row of one pixel gives its point twice)
	u := pts[:0]
	for i, p := range pts {
		if i == 0 || p != pts[i-1] {
			u = append(u, p)
		}
	}
	pts = u
	if len(pts) < 3 {
		return pts
	}
	hull := make([]pt, 0, 2*len(pts))
	for _, p := range pts {
		for len(hull) >= 2 && hull[len(hull)-1].sub(hull[len(hull)-2]).cross(p.sub(hull[len(hull)-2])) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, p)
	}
	lower := len(hull) + 1
	for i := len(pts) - 2; i >= 0; i-- {
		p := pts[i]
		for len(hull) >= lower && hull[len(hull)-1].sub(hull[len(hull)-2]).cross(p.sub(hull[len(hull)-2])) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, p)
	}
	return hull[:len(hull)-1]
}

// rect is a rotated rectangle: its centre, the unit vector along its width, width and height.
type rect struct {
	c    pt
	u    pt
	w, h float64
}

func (r rect) corners() [4]pt {
	n := pt{-r.u.y, r.u.x}
	a, b := r.u.mul(r.w/2), n.mul(r.h/2)
	return [4]pt{r.c.sub(a).sub(b), r.c.add(a).sub(b), r.c.add(a).add(b), r.c.sub(a).add(b)}
}

// minAreaRect is the rectangle of least area around the convex hull (rotating calipers).
func minAreaRect(hull []pt) rect {
	switch len(hull) {
	case 0:
		return rect{}
	case 1:
		return rect{c: hull[0], u: pt{1, 0}}
	}
	best, bestArea := rect{}, math.Inf(1)
	for i := range hull {
		e := hull[(i+1)%len(hull)].sub(hull[i])
		l := e.norm()
		if l == 0 {
			continue
		}
		u := e.mul(1 / l)
		n := pt{-u.y, u.x}
		minU, maxU, minN, maxN := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
		for _, p := range hull {
			d := p.sub(hull[i])
			pu, pn := d.dot(u), d.dot(n)
			minU, maxU = min(minU, pu), max(maxU, pu)
			minN, maxN = min(minN, pn), max(maxN, pn)
		}
		if a := (maxU - minU) * (maxN - minN); a < bestArea {
			bestArea = a
			c := hull[i].add(u.mul((minU + maxU) / 2)).add(n.mul((minN + maxN) / 2))
			best = rect{c: c, u: u, w: maxU - minU, h: maxN - minN}
		}
	}
	return best
}

// miniBox is the least rectangle around pts with its corners in the order of RapidOCR's
// get_mini_boxes, and its shorter side.
func miniBox(pts []pt) (quad, float64) {
	r := minAreaRect(pts)
	c := r.corners()
	p := c[:]
	sort.SliceStable(p, func(i, j int) bool { return p[i].x < p[j].x })
	var q quad
	if p[1].y > p[0].y {
		q[0], q[3] = p[0], p[1]
	} else {
		q[0], q[3] = p[1], p[0]
	}
	if p[3].y > p[2].y {
		q[1], q[2] = p[2], p[3]
	} else {
		q[1], q[2] = p[3], p[2]
	}
	return q, min(r.w, r.h)
}

// boxScore is the mean probability inside the box ("fast" score of RapidOCR).
func boxScore(prob []float32, w, h int, box quad) float64 {
	xmin, xmax, ymin, ymax := box[0].x, box[0].x, box[0].y, box[0].y
	for _, p := range box[1:] {
		xmin, xmax = min(xmin, p.x), max(xmax, p.x)
		ymin, ymax = min(ymin, p.y), max(ymax, p.y)
	}
	x0, x1 := min(max(int(math.Floor(xmin)), 0), w-1), min(max(int(math.Ceil(xmax)), 0), w-1)
	y0, y1 := min(max(int(math.Floor(ymin)), 0), h-1), min(max(int(math.Ceil(ymax)), 0), h-1)
	// the polygon as OpenCV fills it: corners truncated to whole pixels
	var poly quad
	for k, p := range box {
		poly[k] = pt{math.Trunc(p.x), math.Trunc(p.y)}
	}
	sign := 0.0
	for k := range poly {
		sign += poly[k].cross(poly[(k+1)%4])
	}
	var sum float64
	n := 0
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if insideQuad(poly, pt{float64(x), float64(y)}, sign) {
				sum += float64(prob[y*w+x])
				n++
			}
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// insideQuad reports whether p lies in the convex polygon q or within half a pixel of its edges.
// sign is the polygon's orientation (twice its signed area).
func insideQuad(q quad, p pt, sign float64) bool {
	for k := range q {
		a, b := q[k], q[(k+1)%4]
		e := b.sub(a)
		l := e.norm()
		if l == 0 {
			continue
		}
		d := e.cross(p.sub(a)) / l
		if sign < 0 {
			d = -d
		}
		if d < -0.5 {
			return false
		}
	}
	return true
}

// unclip grows the box by its area times detUnclip over its perimeter on every side: the model
// marks only the core of a line.
func unclip(box quad) (quad, float64) {
	r := minAreaRect(box[:])
	if r.w+r.h == 0 {
		return box, 0
	}
	d := r.w * r.h * detUnclip / (2 * (r.w + r.h))
	r.w += 2 * d
	r.h += 2 * d
	c := r.corners()
	return miniBox(c[:])
}

// clockwise orders the corners top left, top right, bottom right, bottom left.
func clockwise(b quad) quad {
	p := b[:]
	sort.SliceStable(p, func(i, j int) bool { return p[i].x < p[j].x })
	left, right := []pt{p[0], p[1]}, []pt{p[2], p[3]}
	if left[1].y < left[0].y {
		left[0], left[1] = left[1], left[0]
	}
	if right[1].y < right[0].y {
		right[0], right[1] = right[1], right[0]
	}
	return quad{left[0], right[0], right[1], left[1]}
}

// sortBoxes puts the boxes in reading order: by their top, boxes of one row from left to right.
func sortBoxes(boxes []quad) {
	sort.SliceStable(boxes, func(i, j int) bool { return boxes[i][0].y < boxes[j][0].y })
	row := make([]int, len(boxes))
	for i := 1; i < len(boxes); i++ {
		row[i] = row[i-1]
		if boxes[i][0].y-boxes[i-1][0].y >= sortRowDelta {
			row[i]++
		}
	}
	idx := make([]int, len(boxes))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		if row[idx[a]] != row[idx[b]] {
			return row[idx[a]] < row[idx[b]]
		}
		return boxes[idx[a]][0].x < boxes[idx[b]][0].x
	})
	sorted := make([]quad, len(boxes))
	for i, k := range idx {
		sorted[i] = boxes[k]
	}
	copy(boxes, sorted)
}

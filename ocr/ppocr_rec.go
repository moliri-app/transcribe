// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

// Text recognition of PP-OCRv5 (SVTR with CTC): each line, cut out and scaled to 48 pixels high,
// gives a probability per character of the model's alphabet for each 8-pixel column; the most
// likely character per column, repeats and blanks dropped, is the text. Where its characters fall
// gives the boxes of the words (RapidOCR's cal_rec_boxes).
const (
	recHeight    = 48
	recMinWidth  = 320 // lines are padded to at least this width
	recBatch     = 6
	recTextScore = 0.5 // lines read with less confidence are dropped
)

// crop is one detected line cut out of the image, upright.
type crop struct {
	img     *raster
	toImage homography // crop pixel -> image pixel (of the image detection ran on)
	turned  bool       // turned a quarter: the box was upright, the text vertical
}

// cropLine cuts the box out of r, straightened (RapidOCR's get_rotate_crop_image).
func cropLine(r *raster, b quad) (crop, bool) {
	w := int(max(b[0].sub(b[1]).norm(), b[2].sub(b[3]).norm()))
	h := int(max(b[0].sub(b[3]).norm(), b[1].sub(b[2]).norm()))
	if w <= 0 || h <= 0 {
		return crop{}, false
	}
	m, ok := solveHomography([4]pt{{0, 0}, {float64(w), 0}, {float64(w), float64(h)}, {0, float64(h)}}, b)
	if !ok {
		return crop{}, false
	}
	c := crop{img: r.warp(m, w, h), toImage: m}
	if float64(h)/float64(w) >= 1.5 {
		c.img, c.turned = c.img.rot90(), true
	}
	return c, true
}

// point maps a point of the (possibly turned) crop to the image.
func (c crop) point(p pt) pt {
	if c.turned {
		// rot90: crop pixel (x, y) came from (w-1-y, x) of the unturned crop, w its width
		p = pt{float64(c.img.h) - p.y, p.x}
	}
	return c.toImage.apply(p)
}

// recLine is what recognition read in one crop.
type recLine struct {
	text  string
	score float64
	words []recWord
}

type recWord struct {
	text   string
	p      float64
	x0, x1 float64 // in pixels of the crop
}

// recognise reads the crops, recBatch at a time, those of similar width together.
func (p *PPOCR) recognise(crops []crop, check func() error) ([]recLine, error) {
	out := make([]recLine, len(crops))
	idx := make([]int, len(crops))
	ratio := make([]float64, len(crops))
	for i, c := range crops {
		idx[i] = i
		ratio[i] = float64(c.img.w) / float64(c.img.h)
	}
	sort.SliceStable(idx, func(a, b int) bool { return ratio[idx[a]] < ratio[idx[b]] })
	for beg := 0; beg < len(idx); beg += recBatch {
		if err := check(); err != nil {
			return nil, err
		}
		batch := idx[beg:min(beg+recBatch, len(idx))]
		maxRatio := float64(recMinWidth) / recHeight
		for _, i := range batch {
			maxRatio = max(maxRatio, ratio[i])
		}
		width := int(recHeight * maxRatio)
		plane := 3 * recHeight * width
		in := make([]float32, len(batch)*plane)
		for k, i := range batch {
			rw := min(width, int(math.Ceil(recHeight*ratio[i])))
			crops[i].img.resize(rw, recHeight).chw(in[k*plane:(k+1)*plane], width)
		}
		res, shape, err := p.rec.Run(in, []int64{int64(len(batch)), 3, recHeight, int64(width)})
		if err != nil {
			return nil, err
		}
		if len(shape) != 3 || int(shape[0]) != len(batch) || int(shape[2]) != len(p.chars) {
			return nil, fmt.Errorf("recognition gave shape %v for %d lines and %d characters", shape, len(batch), len(p.chars))
		}
		steps, classes := int(shape[1]), int(shape[2])
		for k, i := range batch {
			// the columns that cover the line, not the padding
			cover := float64(steps) * ratio[i] / maxRatio
			out[i] = p.decode(res[k*steps*classes:(k+1)*steps*classes], steps, classes, float64(crops[i].img.w)/cover)
		}
	}
	return out, nil
}

// decode reads the characters of one line from its probabilities (steps × classes), greedily, and
// places the words; colWidth is the width of one column in pixels of the crop.
func (p *PPOCR) decode(prob []float32, steps, classes int, colWidth float64) recLine {
	type char struct {
		s   string
		col int
		p   float64
	}
	var chars []char
	prev := -1
	for t := 0; t < steps; t++ {
		row := prob[t*classes : (t+1)*classes]
		best := 0
		for c := 1; c < classes; c++ {
			if row[c] > row[best] {
				best = c
			}
		}
		if best != prev && best != 0 {
			chars = append(chars, char{p.chars[best], t, float64(row[best])})
		}
		prev = best
	}
	line := recLine{}
	if len(chars) == 0 {
		return line
	}
	sum := 0.0
	var words [][]char
	var cur []char
	for _, c := range chars {
		sum += c.p
		if strings.TrimSpace(c.s) == "" {
			if len(cur) > 0 {
				words = append(words, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, c)
	}
	if len(cur) > 0 {
		words = append(words, cur)
	}
	line.score = sum / float64(len(chars))

	// width of one character: the mean over words of more than one character
	var widths []float64
	for _, w := range words {
		if len(w) > 1 {
			widths = append(widths, float64(w[len(w)-1].col-w[0].col)*colWidth/float64(len(w)-1))
		}
	}
	lineWidth := colWidth * float64(steps)
	charWidth := lineWidth / float64(len(chars))
	if len(widths) > 0 {
		charWidth = 0
		for _, v := range widths {
			charWidth += v
		}
		charWidth /= float64(len(widths))
	}
	texts := make([]string, 0, len(words))
	for _, w := range words {
		var b strings.Builder
		ps := 0.0
		for _, c := range w {
			b.WriteString(c.s)
			ps += c.p
		}
		x0 := max(math.Trunc((float64(w[0].col)+0.5)*colWidth-charWidth/2), 0)
		x1 := min(math.Trunc((float64(w[len(w)-1].col)+0.5)*colWidth+charWidth/2), lineWidth)
		line.words = append(line.words, recWord{text: b.String(), p: ps / float64(len(w)), x0: x0, x1: x1})
		texts = append(texts, b.String())
	}
	// words that overlap meet halfway
	for i := 0; i+1 < len(line.words); i++ {
		a, b := &line.words[i], &line.words[i+1]
		if a.x1 > b.x0 {
			d := a.x1 - b.x0
			a.x1 -= d / 2
			b.x0 += d - d/2
		}
	}
	line.text = strings.Join(texts, " ")
	return line
}

// alphabet reads the characters of a PP-OCR recognition model from its metadata ("character", one
// per line, as RapidOCR's ONNX models carry them), with CTC's blank first and the space last.
func alphabet(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	v, ok := onnxMetadata(data, "character")
	if !ok {
		return nil, errors.New("the recognition model has no alphabet (metadata \"character\")")
	}
	chars := []string{""}
	for _, l := range strings.Split(strings.TrimRight(strings.ReplaceAll(v, "\r\n", "\n"), "\n"), "\n") {
		chars = append(chars, l)
	}
	return append(chars, " "), nil
}

// onnxMetadata finds a value of metadata_props (field 14 of ModelProto, entries of key 1 and value
// 2) in an ONNX file, reading only the outer level of its protobuf.
func onnxMetadata(b []byte, key string) (string, bool) {
	for len(b) > 0 {
		tag, n := uvarint(b)
		if n <= 0 {
			return "", false
		}
		b = b[n:]
		field, typ := tag>>3, tag&7
		switch typ {
		case 0:
			_, n = uvarint(b)
			if n <= 0 {
				return "", false
			}
			b = b[n:]
		case 1:
			if len(b) < 8 {
				return "", false
			}
			b = b[8:]
		case 5:
			if len(b) < 4 {
				return "", false
			}
			b = b[4:]
		case 2:
			l, n := uvarint(b)
			if n <= 0 || uint64(len(b)-n) < l {
				return "", false
			}
			msg := b[n : n+int(l)]
			b = b[n+int(l):]
			if field != 14 {
				continue
			}
			var k, v string
			for len(msg) > 0 {
				t, n := uvarint(msg)
				if n <= 0 || t&7 != 2 {
					break
				}
				l, m := uvarint(msg[n:])
				if m <= 0 || uint64(len(msg)-n-m) < l {
					break
				}
				s := string(msg[n+m : n+m+int(l)])
				msg = msg[n+m+int(l):]
				switch t >> 3 {
				case 1:
					k = s
				case 2:
					v = s
				}
			}
			if k == key {
				return v, true
			}
		default:
			return "", false
		}
	}
	return "", false
}

func uvarint(b []byte) (uint64, int) {
	var x uint64
	for i, c := range b {
		if i == 10 {
			return 0, -1
		}
		x |= uint64(c&0x7f) << (7 * i)
		if c < 0x80 {
			return x, i + 1
		}
	}
	return 0, 0
}

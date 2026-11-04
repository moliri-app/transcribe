// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMinAreaRectOfTurnedRectangle(t *testing.T) {
	r := rect{c: pt{50, 40}, u: pt{math.Cos(0.2), math.Sin(0.2)}, w: 60, h: 10}
	c := r.corners()
	got := minAreaRect(convexHull(c[:]))
	if math.Abs(got.w*got.h-600) > 1e-6 || math.Abs(min(got.w, got.h)-10) > 1e-6 {
		t.Fatalf("got %+v", got)
	}
	if got.c.sub(r.c).norm() > 1e-6 {
		t.Fatalf("centre %v", got.c)
	}
}

func TestUnclipGrowsEverySide(t *testing.T) {
	box := quad{{10, 10}, {50, 10}, {50, 20}, {10, 20}}
	got, side := unclip(box)
	d := 40.0 * 10 * detUnclip / (2 * 50) // 6.4
	if math.Abs(side-(10+2*d)) > 1e-6 {
		t.Fatalf("shorter side %v, want %v", side, 10+2*d)
	}
	b := boundingBox(got[:])
	if b != (Box{3, 3, 54, 24}) {
		t.Fatalf("box %v", b)
	}
}

// detBoxes finds two lines of a probability map, in reading order, grown to about their full
// height.
func TestDetBoxes(t *testing.T) {
	w, h := 128, 64
	prob := make([]float32, w*h)
	fill := func(x0, y0, x1, y1 int) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				prob[y*w+x] = 0.9
			}
		}
	}
	fill(70, 10, 110, 16) // right, slightly higher: same row as the left one
	fill(10, 12, 60, 18)
	fill(10, 40, 100, 46)
	boxes := detBoxes(prob, w, h, w*2, h*2) // the image is twice the map
	if len(boxes) != 3 {
		t.Fatalf("%d boxes: %v", len(boxes), boxes)
	}
	first, second := boundingBox(boxes[0][:]), boundingBox(boxes[1][:])
	if first[0] > second[0] {
		t.Errorf("row not left to right: %v then %v", first, second)
	}
	third := boundingBox(boxes[2][:])
	if third[1] < 70 || third[3] < 14 || third[3] > 30 {
		t.Errorf("second row %v", third)
	}
}

func TestJoinRows(t *testing.T) {
	word := func(s string, x, y int) Word { return Word{W: s, BBox: Box{x, y, 40, 20}, P: conf(0.9)} }
	parts := []part{
		{box: Box{10, 10, 60, 20}, words: []Word{word("Titel:", 10, 10)}},
		{box: Box{200, 12, 90, 20}, words: []Word{word("Zinn", 200, 12), word("teller", 250, 12)}},
		{box: Box{10, 36, 80, 20}, words: []Word{word("Maße:", 10, 36)}},
		{box: Box{10, 100, 80, 20}, words: []Word{word("Neu", 10, 100)}},
	}
	lines := joinRows(parts)
	if len(lines) != 3 {
		t.Fatalf("%d lines: %+v", len(lines), lines)
	}
	if lines[0].Text != "Titel: Zinn teller" || lines[0].BBox != (Box{10, 10, 280, 22}) {
		t.Errorf("first line %q %v", lines[0].Text, lines[0].BBox)
	}
	if lines[0].Para != 0 || lines[1].Para != 0 || lines[2].Para != 1 {
		t.Errorf("paragraphs %d %d %d", lines[0].Para, lines[1].Para, lines[2].Para)
	}
}

func TestPPOCRClean(t *testing.T) {
	p := &PPOCR{}
	o, err := p.Clean(Options{Languages: "deu+frak2021", Layout: "lines"})
	if err != nil || o.Languages != "latin" || o.Layout != "auto" {
		t.Fatalf("%+v %v", o, err)
	}
	if _, err := p.Clean(Options{Languages: "chi_sim"}); !errors.Is(err, ErrOptions) {
		t.Fatalf("chi_sim: %v", err)
	}
	if _, err := p.Clean(Options{Layout: "columns"}); !errors.Is(err, ErrOptions) {
		t.Fatalf("columns: %v", err)
	}
}

// metadataModel is an ONNX file holding nothing but the alphabet in its metadata.
func metadataModel(chars string) []byte {
	str := func(field byte, s string) []byte { return append([]byte{field<<3 | 2, byte(len(s))}, s...) }
	entry := append(str(1, "character"), str(2, chars)...)
	return append([]byte{0x08, 0x08, 14<<3 | 2, byte(len(entry))}, entry...)
}

func TestAlphabet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rec.onnx")
	if err := os.WriteFile(path, metadataModel("A\nb\n:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chars, err := alphabet(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(chars, "|") != "|A|b|:| " {
		t.Fatalf("%q", chars)
	}
}

// fakeNet stands in for the models: detection marks one block of the image as text, recognition
// reads "Ab A" in every line.
type fakeNet struct{ det bool }

func (f fakeNet) Close() {}

func (f fakeNet) Run(in []float32, shape []int64) ([]float32, []int64, error) {
	n, h, w := int(shape[0]), int(shape[2]), int(shape[3])
	if f.det {
		prob := make([]float32, h*w)
		for y := h / 4; y < h/4+h/8; y++ {
			for x := w / 8; x < w/2; x++ {
				prob[y*w+x] = 0.95
			}
		}
		return prob, []int64{1, 1, int64(h), int64(w)}, nil
	}
	// alphabet: blank, A, b, space; the text sits in the first 12 columns
	steps, classes := w/8, 4
	out := make([]float32, n*steps*classes)
	text := []int{1, 0, 2, 0, 3, 0, 0, 1}
	for k := 0; k < n; k++ {
		for t := 0; t < steps; t++ {
			c := 0
			if t < len(text) {
				c = text[t]
			}
			out[(k*steps+t)*classes+c] = 0.9
		}
	}
	return out, []int64{int64(n), int64(steps), int64(classes)}, nil
}

type fakeNets struct{}

func (fakeNets) Installed() bool { return true }
func (fakeNets) GPU() bool       { return false }
func (fakeNets) Open(_ context.Context, path string, _ int) (Net, error) {
	return fakeNet{det: filepath.Base(path) == PPOCRDet}, nil
}

func TestPPOCRReadFormat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, PPOCRDet), []byte{0x08, 0x08}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, PPOCRRec), metadataModel("A\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	img := image.NewGray(image.Rect(0, 0, 1000, 400))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	img.Set(0, 0, color.Gray{0})
	path := filepath.Join(dir, "page.png")
	f, _ := os.Create(path)
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	p := &PPOCR{Nets: fakeNets{}, ModelDir: dir}
	if info := p.Info(); !info.Ready || info.Languages[0] != "latin" {
		t.Fatalf("info %+v", info)
	}
	reg := NewRegistry("desktop", p)
	e, o, err := reg.Prepare("onnx", Options{Languages: "deu"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := reg.Read(context.Background(), e, path, o, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Engine != "desktop:onnx:ppocr-v5" || res.Pages[0].Width != 1000 || res.Pages[0].Height != 400 {
		t.Fatalf("result %+v", res)
	}
	lines := res.Pages[0].Lines
	if len(lines) != 1 || lines[0].Text != "Ab A" || len(lines[0].Words) != 2 {
		t.Fatalf("lines %+v", lines)
	}
	l, a, b := lines[0].BBox, lines[0].Words[0].BBox, lines[0].Words[1].BBox
	// the block is x 125–500, y 100–150 of the page, grown by unclip on every side by
	// 375 × 50 × 1.6 / 850 ≈ 35 pixels
	want := Box{90, 65, 445, 120}
	for k := range want {
		if math.Abs(float64(l[k]-want[k])) > 4 {
			t.Errorf("line box %v, want about %v", l, want)
			break
		}
	}
	if a[0] < l[0] || a[0]+a[2] > b[0]+1 || b[0]+b[2] > l[0]+l[2] || a[1] < l[1]-1 {
		t.Errorf("word boxes %v %v in %v", a, b, l)
	}
}

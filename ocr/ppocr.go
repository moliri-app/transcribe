// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Net is one loaded ONNX model with one float input and one float output (ort).
type Net interface {
	Run(input []float32, shape []int64) ([]float32, []int64, error)
	Close()
}

// Nets loads ONNX models on the runtime of the app.
type Nets interface {
	// Installed reports whether the runtime is there, nothing left to download.
	Installed() bool
	// Open loads the model at path, after the runtime (downloading it when missing). threads
	// bounds the cores one run uses (0: the runtime's default).
	Open(ctx context.Context, path string, threads int) (Net, error)
	// GPU reports whether the models run on the graphics card; before the first one is loaded,
	// whether they would.
	GPU() bool
}

// The files of the PP-OCRv5 models: text detection (all scripts) and recognition of Latin script.
const (
	PPOCRDet = "ch_PP-OCRv5_det_mobile.onnx"
	PPOCRRec = "latin_PP-OCRv5_rec_mobile.onnx"
)

// ppocrVersion names the engine in results: "desktop:onnx:ppocr-v5".
const ppocrVersion = "onnx:ppocr-v5"

// latinLanguages are Tesseract's names of languages in Latin script: the recognition model reads
// them all, so a caller may name the language as it would for Tesseract.
var latinLanguages = map[string]bool{
	"afr": true, "cat": true, "ces": true, "cym": true, "dan": true, "deu": true, "eng": true, "est": true,
	"eus": true, "fin": true, "fra": true, "frk": true, "fraktur": true, "frak2021": true, "gle": true,
	"glg": true, "hrv": true, "hun": true, "ind": true, "isl": true, "ita": true, "lat": true, "lav": true,
	"lit": true, "ltz": true, "mlt": true, "msa": true, "nld": true, "nor": true, "pol": true, "por": true,
	"ron": true, "slk": true, "slv": true, "spa": true, "sqi": true, "swe": true, "tur": true,
}

// PPOCR reads with PaddleOCR's PP-OCRv5 models (Apache 2.0) the way RapidOCR runs them: detection
// finds the lines, recognition reads each, the columns of its characters place the words. The
// models are downloaded on first use; they run on the app's ONNX runtime, on the graphics card
// when the app uses it. Lines of one row become one line of the result.
type PPOCR struct {
	Nets     Nets
	ModelDir string
	// Fetch downloads one model file into ModelDir; nil = only installed models are used.
	Fetch func(ctx context.Context, file string) error

	mu       sync.Mutex // one reading at a time
	det, rec Net
	threads  int
	chars    []string // the recognition model's alphabet, blank first
}

func (p *PPOCR) have(file string) bool {
	st, err := os.Stat(filepath.Join(p.ModelDir, file))
	return err == nil && st.Size() > 0
}

func (p *PPOCR) Info() Info {
	info := Info{ID: "onnx", Name: "PP-OCRv5", Layouts: []string{"auto"}, Boxes: "words", Languages: []string{}}
	have := p.have(PPOCRDet) && p.have(PPOCRRec)
	switch {
	case have:
		info.Languages = append(info.Languages, "latin")
	case p.Fetch != nil:
		info.Download = []string{"latin"}
	}
	info.Usable = p.Nets != nil && (have || p.Fetch != nil)
	info.Ready = info.Usable && have && p.Nets.Installed()
	if p.Nets != nil {
		info.GPU = p.Nets.GPU()
	} else {
		info.Error = "no ONNX runtime"
	}
	return info
}

// Clean accepts "latin" and the Latin-script languages of Tesseract ("deu", "eng+fra", …), all
// read by the one model, and the layouts of Tesseract, which make no difference here.
func (p *PPOCR) Clean(o Options) (Options, error) {
	for _, l := range strings.Split(o.Languages, "+") {
		l = strings.ToLower(strings.TrimSpace(l))
		if l == "" || l == "latin" || latinLanguages[l] {
			continue
		}
		return o, fmt.Errorf("%w: PP-OCRv5 reads Latin script (languages latin, deu, eng, fra, …), not %q", ErrOptions, l)
	}
	layout := strings.ToLower(strings.TrimSpace(o.Layout))
	if _, ok := Layouts[layout]; layout != "" && !ok {
		return o, fmt.Errorf("%w: layout must be auto, lines, block or sparse, not %q", ErrOptions, o.Layout)
	}
	return Options{Languages: "latin", Layout: "auto"}, nil
}

// load fetches the models that are missing and loads them (again when threads changes).
func (p *PPOCR) load(ctx context.Context, threads int) error {
	if p.Nets == nil {
		return errors.New("no ONNX runtime")
	}
	for _, f := range []string{PPOCRDet, PPOCRRec} {
		if p.have(f) {
			continue
		}
		if p.Fetch == nil {
			return fmt.Errorf("model %s is not installed", f)
		}
		if err := p.Fetch(ctx, f); err != nil {
			return fmt.Errorf("model %s: %w", f, err)
		}
	}
	if p.det != nil && p.threads == threads {
		return nil
	}
	p.close()
	chars, err := alphabet(filepath.Join(p.ModelDir, PPOCRRec))
	if err != nil {
		return err
	}
	det, err := p.Nets.Open(ctx, filepath.Join(p.ModelDir, PPOCRDet), threads)
	if err != nil {
		return err
	}
	rec, err := p.Nets.Open(ctx, filepath.Join(p.ModelDir, PPOCRRec), threads)
	if err != nil {
		det.Close()
		return err
	}
	p.det, p.rec, p.chars, p.threads = det, rec, chars, threads
	return nil
}

func (p *PPOCR) close() {
	if p.det != nil {
		p.det.Close()
	}
	if p.rec != nil {
		p.rec.Close()
	}
	p.det, p.rec = nil, nil
}

// Read recognises the text of the image at path. The first reading downloads the models.
func (p *PPOCR) Read(ctx context.Context, path string, o Options, threads int) (*Result, error) {
	img, err := decodeImage(path)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.load(ctx, threads); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: PP-OCRv5: %v", ErrUnavailable, err)
	}
	lines, err := p.read(ctx, img)
	if err != nil {
		return nil, err
	}
	return &Result{Engine: ppocrVersion, Language: o.Languages, Layout: o.Layout,
		Pages: []Page{{Width: img.w, Height: img.h, Lines: lines}}}, nil
}

// Limits of the image before detection (RapidOCR's preprocessing): the longer side at most
// maxSide, the shorter at least minSide; images much wider than high get black bands above and
// below.
const (
	maxSide     = 2000
	minSide     = 30
	padRatio    = 8 // width per height beyond which an image is padded
	padMinWidth = 30
)

// read finds and reads the lines of one image.
func (p *PPOCR) read(ctx context.Context, orig *raster) ([]Line, error) {
	img, sx, sy := withinBounds(orig)
	pad := 0
	if float64(img.w)/float64(img.h) > padRatio || img.h <= padMinWidth {
		newH := max(int(float64(img.w)/padRatio), padMinWidth) * 2
		pad = int(math.Abs(float64(newH-img.h)) / 2)
		img = img.padVertical(pad)
	}
	// back to pixels of the original
	toOrig := func(q pt) pt {
		return pt{min(max(q.x*sx, 0), float64(orig.w)), min(max((q.y-float64(pad))*sy, 0), float64(orig.h))}
	}

	dw, dh := detInput(img)
	in := make([]float32, 3*dw*dh)
	img.resize(dw, dh).chw(in, dw)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prob, shape, err := p.det.Run(in, []int64{1, 3, int64(dh), int64(dw)})
	if err != nil {
		return nil, fmt.Errorf("detection: %w", err)
	}
	if len(shape) != 4 || shape[2] != int64(dh) || shape[3] != int64(dw) {
		return nil, fmt.Errorf("detection gave shape %v for %d×%d", shape, dw, dh)
	}
	boxes := detBoxes(prob, dw, dh, img.w, img.h)

	crops := make([]crop, 0, len(boxes))
	kept := boxes[:0]
	for _, b := range boxes {
		if c, ok := cropLine(img, b); ok {
			crops = append(crops, c)
			kept = append(kept, b)
		}
	}
	boxes = kept
	read, err := p.recognise(crops, ctx.Err)
	if err != nil {
		return nil, err
	}

	var parts []part
	for i, r := range read {
		if strings.TrimSpace(r.text) == "" || r.score < recTextScore {
			continue
		}
		var q [4]pt
		for k := range boxes[i] {
			q[k] = toOrig(boxes[i][k])
		}
		pa := part{box: boundingBox(q[:])}
		for _, w := range r.words {
			h := float64(crops[i].img.h)
			c := [4]pt{{w.x0, 0}, {w.x1, 0}, {w.x1, h}, {w.x0, h}}
			for k := range c {
				c[k] = toOrig(crops[i].point(c[k]))
			}
			p := math.Round(w.p*1000) / 1000
			pa.words = append(pa.words, Word{W: w.text, BBox: boundingBox(c[:]), P: &p})
		}
		parts = append(parts, pa)
	}
	return joinRows(parts), nil
}

// withinBounds scales the image so its longer side is at most maxSide and its shorter at least
// minSide (sides then rounded to multiples of 32), and returns the factors back to the original.
func withinBounds(r *raster) (*raster, float64, float64) {
	scaled := func(ratio float64) (int, int) {
		w := int(math.RoundToEven(float64(int(float64(r.w)*ratio))/32) * 32)
		h := int(math.RoundToEven(float64(int(float64(r.h)*ratio))/32) * 32)
		return max(w, 32), max(h, 32)
	}
	out := r
	if m := max(r.w, r.h); m > maxSide {
		out = out.resize(scaled(float64(maxSide) / float64(m)))
	}
	if m := min(out.w, out.h); m < minSide {
		ratio := float64(minSide) / float64(m)
		w := int(math.RoundToEven(float64(int(float64(out.w)*ratio))/32) * 32)
		h := int(math.RoundToEven(float64(int(float64(out.h)*ratio))/32) * 32)
		out = out.resize(max(w, 32), max(h, 32))
	}
	return out, float64(r.w) / float64(out.w), float64(r.h) / float64(out.h)
}

func boundingBox(ps []pt) Box {
	x0, y0, x1, y1 := ps[0].x, ps[0].y, ps[0].x, ps[0].y
	for _, p := range ps[1:] {
		x0, y0, x1, y1 = min(x0, p.x), min(y0, p.y), max(x1, p.x), max(y1, p.y)
	}
	x, y := int(math.Floor(x0)), int(math.Floor(y0))
	return Box{x, y, int(math.Ceil(x1)) - x, int(math.Ceil(y1)) - y}
}

// part is one detected box read: its box and words in pixels of the image.
type part struct {
	box   Box
	words []Word
}

// joinRows makes lines of the parts in reading order: parts side by side on one row (a label and
// its value on an index card) are one line, as Tesseract gives them. A line whose top lies further
// below the line before than three quarters of the usual line height starts a paragraph.
func joinRows(parts []part) []Line {
	var lines []Line
	for _, pa := range parts {
		if len(pa.words) == 0 {
			continue
		}
		if n := len(lines); n > 0 {
			last := &lines[n-1]
			prev := last.BBox
			top, bottom := max(prev[1], pa.box[1]), min(prev[1]+prev[3], pa.box[1]+pa.box[3])
			lastWord := last.Words[len(last.Words)-1].BBox
			if bottom-top >= min(prev[3], pa.box[3])/2 && pa.box[0] >= lastWord[0]+lastWord[2]/2 {
				last.Words = append(last.Words, pa.words...)
				last.BBox = union(prev, pa.box)
				continue
			}
		}
		lines = append(lines, Line{BBox: pa.box, Words: pa.words})
	}
	heights := make([]int, len(lines))
	for i, l := range lines {
		ws := make([]string, len(l.Words))
		for k, w := range l.Words {
			ws[k] = w.W
		}
		lines[i].Text = strings.Join(ws, " ")
		heights[i] = l.BBox[3]
	}
	sort.Ints(heights)
	if len(heights) == 0 {
		return []Line{}
	}
	usual := float64(heights[len(heights)/2])
	para := 0
	for i := range lines {
		if i > 0 {
			prev, cur := lines[i-1].BBox, lines[i].BBox
			if float64(cur[1]-(prev[1]+prev[3])) > 0.75*usual {
				para++
			}
		}
		lines[i].Para = para
	}
	return lines
}

func union(a, b Box) Box {
	x0, y0 := min(a[0], b[0]), min(a[1], b[1])
	x1, y1 := max(a[0]+a[2], b[0]+b[2]), max(a[1]+a[3], b[1]+b[3])
	return Box{x0, y0, x1 - x0, y1 - y0}
}

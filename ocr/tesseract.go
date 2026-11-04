// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Layouts maps the layout option to Tesseract's page segmentation mode.
var Layouts = map[string]int{
	"auto":   3,  // finds columns and blocks itself: pages, documents
	"lines":  4,  // one column of lines read across: forms, index cards with label and value
	"block":  6,  // one block of text: a label, a short notice
	"sparse": 11, // scattered text in no particular order: photos of objects with writing
}

var layoutNames = []string{"auto", "lines", "block", "sparse"}

// Library is Tesseract linked as a library instead of the program: the desktop app on Windows,
// where no tesseract program can be assumed and the library is downloaded on first use.
type Library interface {
	Installed() bool
	// Load downloads the library when it is missing, then loads it.
	Load(ctx context.Context) error
	Version() string // after Load, e.g. "5.0.0"
	// TSV reads the image (its file content) with the models of tessdata and returns Tesseract's
	// TSV output.
	TSV(ctx context.Context, image []byte, tessdata, languages string, psm int) ([]byte, error)
}

// Tesseract reads with Tesseract 5: the tesseract program (the server, Linux and macOS), or the
// library (Lib) with its models downloaded on demand (Windows).
type Tesseract struct {
	Program  string  // path or name of the tesseract program; "" = "tesseract"; not used with Lib
	TessData string  // models directory (TESSDATA_PREFIX); "" = the program's built-in default
	Lib      Library // nil = the program
	// Fetch downloads one model into TessData; nil = only the installed models are offered.
	Fetch func(ctx context.Context, model string) error
	// Downloadable are the models Fetch can get.
	Downloadable []string
	// Hint tells people how to install the program when it is missing.
	Hint string

	once      sync.Once
	version   string
	languages []string
	err       error
}

func (t *Tesseract) program() string {
	if t.Program != "" {
		return t.Program
	}
	return "tesseract"
}

func (t *Tesseract) env(threads int) []string {
	env := os.Environ()
	if t.TessData != "" {
		env = append(env, "TESSDATA_PREFIX="+t.TessData)
	}
	if threads > 0 {
		env = append(env, "OMP_THREAD_LIMIT="+strconv.Itoa(threads))
	}
	return env
}

// Check asks the program for its version and installed models (once) and reports whether it
// runs. With Lib it reports whether the library is there.
func (t *Tesseract) Check() error {
	if t.Lib != nil {
		if !t.Lib.Installed() {
			return errors.New("Tesseract wird beim ersten Lesen heruntergeladen")
		}
		return nil
	}
	t.once.Do(func() {
		out, err := exec.Command(t.program(), "--version").CombinedOutput()
		if err != nil {
			t.err = fmt.Errorf("tesseract: %w", err)
			if t.Hint != "" {
				t.err = fmt.Errorf("%w (%s)", t.err, t.Hint)
			}
			return
		}
		first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
		t.version = strings.Join(strings.Fields(first), "-") // "tesseract 5.3.0" -> "tesseract-5.3.0"
		cmd := exec.Command(t.program(), "--list-langs")
		cmd.Env = t.env(0)
		out, err = cmd.CombinedOutput()
		if err != nil {
			t.err = fmt.Errorf("tesseract --list-langs: %w", err)
			return
		}
		t.languages = parseLanguages(string(out))
		if len(t.languages) == 0 {
			t.err = errors.New("tesseract has no language models")
		}
	})
	return t.err
}

// Version is e.g. "tesseract-5.3.0"; empty while the program does not run or the library is not
// loaded yet.
func (t *Tesseract) Version() string {
	if t.Lib != nil {
		if v := t.Lib.Version(); v != "" {
			return "tesseract-" + v
		}
		return ""
	}
	_ = t.Check()
	return t.version
}

// Languages lists the installed models, without osd (orientation only).
func (t *Tesseract) Languages() []string {
	if t.Lib != nil {
		return modelFiles(t.TessData)
	}
	_ = t.Check()
	return t.languages
}

// modelFiles lists the models (*.traineddata) in dir.
func modelFiles(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var langs []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".traineddata"); ok && !e.IsDir() && name != "osd" {
			langs = append(langs, name)
		}
	}
	sort.Strings(langs)
	return langs
}

func parseLanguages(out string) []string {
	var langs []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of") || strings.Contains(line, " ") || line == "osd" {
			continue
		}
		langs = append(langs, line)
	}
	sort.Strings(langs)
	return langs
}

func (t *Tesseract) Info() Info {
	info := Info{ID: "tesseract", Name: "Tesseract", Layouts: layoutNames, Boxes: "words", Languages: []string{}}
	err := t.Check()
	info.Version = strings.TrimPrefix(t.Version(), "tesseract-")
	info.Languages = append(info.Languages, t.Languages()...)
	for _, m := range t.Downloadable {
		if !slices.Contains(info.Languages, m) {
			info.Download = append(info.Download, m)
		}
	}
	switch {
	case t.Lib != nil:
		info.Usable = true
		info.Ready = err == nil && len(info.Languages) > 0
	default:
		info.Ready = err == nil
		info.Usable = info.Ready
		if err != nil {
			info.Error = err.Error()
		}
	}
	return info
}

// Clean checks the options against the installed and downloadable models. Languages match
// without regard to case ("fraktur" is the script model "Fraktur"); unknown ones are an error
// naming the ones there are.
func (t *Tesseract) Clean(o Options) (Options, error) {
	known := t.Languages()
	for _, m := range t.Downloadable {
		if !slices.Contains(known, m) {
			known = append(known, m)
		}
	}
	sort.Strings(known)
	return Clean(o, known)
}

// Clean checks o against the models of installed (Tesseract's names) and fills in the defaults:
// languages "deu", layout "auto". Errors wrap ErrOptions.
func Clean(o Options, installed []string) (Options, error) {
	langs := strings.TrimSpace(o.Languages)
	if langs == "" {
		langs = "deu"
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range strings.Split(langs, "+") {
		p = strings.TrimSpace(p)
		found := ""
		for _, have := range installed {
			if strings.EqualFold(have, p) {
				found = have
				break
			}
		}
		if found == "" {
			return o, fmt.Errorf("%w: language %q is not installed (installed: %s)", ErrOptions, p, strings.Join(installed, ", "))
		}
		if !seen[found] {
			seen[found] = true
			out = append(out, found)
		}
	}
	layout := strings.ToLower(strings.TrimSpace(o.Layout))
	if layout == "" {
		layout = "auto"
	}
	if _, ok := Layouts[layout]; !ok {
		return o, fmt.Errorf("%w: layout must be auto, lines, block or sparse, not %q", ErrOptions, o.Layout)
	}
	return Options{Languages: strings.Join(out, "+"), Layout: layout}, nil
}

// Read recognises the text of the image at path with options already cleaned. threads bounds the
// cores the program uses (OMP_THREAD_LIMIT). Cancelling ctx stops it. With Lib, the library and
// models it lacks are downloaded first.
func (t *Tesseract) Read(ctx context.Context, path string, o Options, threads int) (*Result, error) {
	var tsv []byte
	var err error
	if t.Lib != nil {
		tsv, err = t.readLib(ctx, path, o)
	} else {
		tsv, err = t.readProgram(ctx, path, o, threads)
	}
	if err != nil {
		return nil, err
	}
	pages, err := ParseTSV(tsv)
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, errors.New("tesseract read no image")
	}
	return &Result{Engine: t.Version(), Language: o.Languages, Layout: o.Layout, Pages: pages}, nil
}

func (t *Tesseract) readProgram(ctx context.Context, path string, o Options, threads int) ([]byte, error) {
	if err := t.Check(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	cmd := exec.CommandContext(ctx, t.program(), path, "stdout", "-l", o.Languages, "--psm", strconv.Itoa(Layouts[o.Layout]), "tsv")
	cmd.Env = t.env(threads)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		msg := lastLines(stderr.String(), 3)
		if strings.Contains(msg, "Unknown format") || strings.Contains(msg, "pixRead") {
			return nil, fmt.Errorf("%w: tesseract: %s", ErrImage, msg)
		}
		return nil, fmt.Errorf("tesseract: %v: %s", err, msg)
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("tesseract read no image: %s", lastLines(stderr.String(), 3))
	}
	return stdout.Bytes(), nil
}

func (t *Tesseract) readLib(ctx context.Context, path string, o Options) ([]byte, error) {
	if err := t.Lib.Load(ctx); err != nil {
		return nil, fmt.Errorf("%w: Tesseract: %v", ErrUnavailable, err)
	}
	have := t.Languages()
	for _, m := range strings.Split(o.Languages, "+") {
		if slices.Contains(have, m) {
			continue
		}
		if t.Fetch == nil {
			return nil, fmt.Errorf("%w: model %s is not installed", ErrUnavailable, m)
		}
		if err := t.Fetch(ctx, m); err != nil {
			return nil, fmt.Errorf("%w: model %s: %v", ErrUnavailable, m, err)
		}
	}
	img, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return t.Lib.TSV(ctx, img, t.TessData, o.Languages, Layouts[o.Layout])
}

// ModelPath is where a model of tessdata lives.
func ModelPath(tessdata, model string) string {
	return filepath.Join(tessdata, model+".traineddata")
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

// ParseTSV turns Tesseract's TSV output (level, page_num, block_num, par_num, line_num, word_num, left,
// top, width, height, conf, text) into pages of lines of words. Empty words and lines without words
// are left out.
func ParseTSV(tsv []byte) ([]Page, error) {
	type lineKey struct{ page, block, par, line int }
	type parKey struct{ page, block, par int }
	var pages []Page
	pageIndex := map[int]int{}  // page_num -> index in pages
	lineAt := map[lineKey]int{} // -> index in its page's lines
	paraOf := map[parKey]int{}  // -> paragraph number on its page
	paras := map[int]int{}      // page_num -> paragraphs so far

	for i, row := range strings.Split(strings.ReplaceAll(string(tsv), "\r\n", "\n"), "\n") {
		if row == "" || (i == 0 && strings.HasPrefix(row, "level")) {
			continue
		}
		f := strings.SplitN(row, "\t", 12)
		if len(f) < 11 {
			return nil, fmt.Errorf("tesseract tsv line %d: %d fields", i+1, len(f))
		}
		var n [10]int
		for k := 0; k < 10; k++ {
			v, err := strconv.Atoi(f[k])
			if err != nil {
				return nil, fmt.Errorf("tesseract tsv line %d field %d: %v", i+1, k+1, err)
			}
			n[k] = v
		}
		level, page, block, par, line := n[0], n[1], n[2], n[3], n[4]
		box := Box{n[6], n[7], n[8], n[9]}
		text := ""
		if len(f) == 12 {
			text = strings.TrimSpace(f[11])
		}

		switch level {
		case 1:
			pageIndex[page] = len(pages)
			pages = append(pages, Page{Width: box[2], Height: box[3], Lines: []Line{}})
		case 4:
			pi, ok := pageIndex[page]
			if !ok {
				continue
			}
			pk := parKey{page, block, par}
			if _, ok := paraOf[pk]; !ok {
				paraOf[pk] = paras[page]
				paras[page]++
			}
			lineAt[lineKey{page, block, par, line}] = len(pages[pi].Lines)
			pages[pi].Lines = append(pages[pi].Lines, Line{BBox: box, Para: paraOf[pk], Words: []Word{}})
		case 5:
			if text == "" {
				continue
			}
			pi, ok := pageIndex[page]
			li, ok2 := lineAt[lineKey{page, block, par, line}]
			if !ok || !ok2 {
				continue
			}
			conf, err := strconv.ParseFloat(f[10], 64)
			if err != nil {
				return nil, fmt.Errorf("tesseract tsv line %d conf: %v", i+1, err)
			}
			p := math.Round(math.Max(0, math.Min(100, conf))*10) / 1000
			l := &pages[pi].Lines[li]
			l.Words = append(l.Words, Word{W: text, BBox: box, P: &p})
		}
	}

	for pi := range pages {
		kept := pages[pi].Lines[:0]
		for _, l := range pages[pi].Lines {
			if len(l.Words) == 0 {
				continue
			}
			ws := make([]string, len(l.Words))
			for k, w := range l.Words {
				ws[k] = w.W
			}
			l.Text = strings.Join(ws, " ")
			kept = append(kept, l)
		}
		pages[pi].Lines = kept
		renumber(pages[pi].Lines)
	}
	return pages, nil
}

// renumber makes the paragraph numbers of the lines kept run 0, 1, 2 … again.
func renumber(lines []Line) {
	next, seen := 0, map[int]int{}
	for i := range lines {
		n, ok := seen[lines[i].Para]
		if !ok {
			n = next
			seen[lines[i].Para] = n
			next++
		}
		lines[i].Para = n
	}
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package ocr reads the text of images: engines behind one interface (Engine, Registry), each with
// a result in one format (Result, format 1), whatever the place it runs at (the desktop app, the
// server). The format is written down as a JSON schema in format.schema.json; Moliri checks results
// against it before it stores them.
package ocr

import (
	_ "embed"
	"fmt"
)

// Format is the version of the result format. It changes only when an old reader would misread a
// new result; readers refuse a format they do not know.
const Format = 1

// Schema is the JSON schema of Result (format.schema.json), the contract with Moliri.
//
//go:embed format.schema.json
var Schema []byte

// Box is [x, y, width, height] in pixels of the image that was read, so a viewer can scale it to
// any size with the page's width and height.
type Box [4]int

// Result is the machine result of reading one image.
type Result struct {
	Format   int    `json:"format"`
	Engine   string `json:"engine"`             // place, engine and version: "desktop:tesseract-5.0.0", "server:tesseract-5.3.0"
	Language string `json:"language,omitempty"` // the models used, e.g. "deu+frak2021"
	Layout   string `json:"layout,omitempty"`
	Pages    []Page `json:"pages"`
}

type Page struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Lines  []Line `json:"lines"`
}

// Line is one line of text in reading order. Engines that locate text give its box; engines that
// give no words with confidence may give it for the line (P).
type Line struct {
	BBox  Box      `json:"bbox,omitzero"`
	Para  int      `json:"para"` // paragraph of the page (0, 1, …): lines of one paragraph belong together
	Text  string   `json:"text"`
	P     *float64 `json:"p,omitempty"` // confidence 0–1 of the whole line, when the words have none
	Words []Word   `json:"words,omitempty"`
}

// Word is one word of a line. An engine without word boxes leaves BBox zero (left out in JSON);
// the editor then marks the line instead of the word. An engine without confidence (Windows OCR)
// leaves P nil; readers take such a word as certain.
type Word struct {
	W    string   `json:"w"`
	BBox Box      `json:"bbox,omitzero"`
	P    *float64 `json:"p,omitempty"` // confidence 0–1
}

// Words counts the words of all pages.
func (r *Result) Words() int {
	n := 0
	for _, p := range r.Pages {
		for _, l := range p.Lines {
			n += len(l.Words)
		}
	}
	return n
}

// Text is the text of all pages, one line per line, pages separated by an empty line.
func (r *Result) Text() string {
	s := ""
	for i, p := range r.Pages {
		if i > 0 {
			s += "\n"
		}
		for _, l := range p.Lines {
			s += l.Text + "\n"
		}
	}
	return s
}

// Check reports what makes a result break the contract of format.schema.json, so an engine's bug
// shows here and not in Moliri. Nil = the result is fine.
func (r *Result) Check() error {
	if r.Format != Format {
		return fmt.Errorf("format %d, want %d", r.Format, Format)
	}
	if r.Engine == "" {
		return fmt.Errorf("engine is empty")
	}
	if r.Pages == nil {
		return fmt.Errorf("pages is missing")
	}
	box := func(b Box, where string) error {
		if b != (Box{}) && (b[0] < 0 || b[1] < 0 || b[2] < 0 || b[3] < 0) {
			return fmt.Errorf("%s: negative box %v", where, b)
		}
		return nil
	}
	for pi, p := range r.Pages {
		if p.Width <= 0 || p.Height <= 0 {
			return fmt.Errorf("page %d: size %d×%d", pi, p.Width, p.Height)
		}
		if p.Lines == nil {
			return fmt.Errorf("page %d: lines is missing", pi)
		}
		for li, l := range p.Lines {
			where := fmt.Sprintf("page %d line %d", pi, li)
			if l.Para < 0 {
				return fmt.Errorf("%s: para %d", where, l.Para)
			}
			if l.P != nil && (*l.P < 0 || *l.P > 1) {
				return fmt.Errorf("%s: p %v", where, *l.P)
			}
			if err := box(l.BBox, where); err != nil {
				return err
			}
			for wi, w := range l.Words {
				where := fmt.Sprintf("%s word %d", where, wi)
				if w.W == "" {
					return fmt.Errorf("%s: empty", where)
				}
				if w.P != nil && (*w.P < 0 || *w.P > 1) {
					return fmt.Errorf("%s: p %v", where, *w.P)
				}
				if err := box(w.BBox, where); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Options of one reading; an engine's Clean fills in its defaults.
type Options struct {
	Languages string `json:"languages"` // models joined by "+", e.g. "deu+eng", "deu+frak2021"
	Layout    string `json:"layout"`    // auto, lines, block or sparse
}

// Info describes an engine for the connection test in Moliri and the app's own pages.
type Info struct {
	ID        string   `json:"id"`   // "tesseract", "onnx", "os", "api"
	Name      string   `json:"name"` // for people
	Version   string   `json:"version,omitempty"`
	Ready     bool     `json:"ready"`              // reads now, nothing left to download
	Usable    bool     `json:"usable"`             // reads, perhaps after downloading what it lacks first
	Languages []string `json:"languages"`          // installed models
	Download  []string `json:"download,omitempty"` // models fetched on first use
	Layouts   []string `json:"layouts"`
	Boxes     string   `json:"boxes"` // what it locates: "words", "lines" or "none"
	GPU       bool     `json:"gpu"`   // runs on the graphics card (the app's GPU choice)
	Error     string   `json:"error,omitempty"`
}

// Engine is one way of reading images.
type Engine interface {
	Info() Info
	// Clean checks options against what the engine can read (installed or downloadable) and fills
	// in its defaults; errors wrap ErrOptions.
	Clean(o Options) (Options, error)
	// Read recognises the text of the image at path with options already cleaned, downloading what
	// it lacks first. threads bounds the cores it uses (0 = its default). Format and Engine of the
	// result are filled in by the Registry.
	Read(ctx context.Context, path string, o Options, threads int) (*Result, error)
}

// ByChoice is implemented by engines that read only when a request names them or the app's
// settings make them the default: the AI service (api) costs money and sends the image away, so a
// request that names no engine never falls to it.
type ByChoice interface {
	ByChoice() bool
}

func byChoice(e Engine) bool {
	c, ok := e.(ByChoice)
	return ok && c.ByChoice()
}

var (
	ErrUnknownEngine = errors.New("unknown OCR engine")
	ErrOptions       = errors.New("invalid OCR options")
	ErrUnavailable   = errors.New("OCR engine not available")
	ErrImage         = errors.New("unreadable image")
	ErrService       = errors.New("OCR service failed") // a service the engine sends the image to refused or failed
)

// Registry holds the engines of one place, in order of preference.
type Registry struct {
	Place string // "desktop" or "server": the first part of Result.Engine
	// Default names the engine a request without one reads with (the app's setting, read at every
	// request); "", an unknown or an unusable engine: the first usable one that is not ByChoice.
	Default func() string
	engines []engineEntry
}

type engineEntry struct {
	id string
	e  Engine
}

func NewRegistry(place string, engines ...Engine) *Registry {
	r := &Registry{Place: place}
	for _, e := range engines {
		r.engines = append(r.engines, engineEntry{e.Info().ID, e})
	}
	return r
}

// Engines describes all engines, usable or not; the default (Default) first.
func (r *Registry) Engines() []Info {
	out := make([]Info, 0, len(r.engines))
	def := r.defaultID()
	for _, x := range r.engines {
		if info := x.e.Info(); x.id == def {
			out = append([]Info{info}, out...)
		} else {
			out = append(out, info)
		}
	}
	return out
}

// defaultID is the engine chosen in the settings, "" for none.
func (r *Registry) defaultID() string {
	if r.Default == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(r.Default()))
}

// DefaultEngine is the id of the engine a request without one reads with, "" if none can read.
func (r *Registry) DefaultEngine() string {
	e, err := r.Get("")
	if err != nil {
		return ""
	}
	return e.Info().ID
}

// Get finds an engine by its id; "" is the default: the one chosen in the settings when it is
// usable, else the first usable one that is not ByChoice.
func (r *Registry) Get(id string) (Engine, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		if def := r.defaultID(); def != "" {
			for _, x := range r.engines {
				if x.id == def && x.e.Info().Usable {
					return x.e, nil
				}
			}
		}
		for _, x := range r.engines {
			if !byChoice(x.e) && x.e.Info().Usable {
				return x.e, nil
			}
		}
		return nil, fmt.Errorf("%w: no engine installed", ErrUnavailable)
	}
	for _, x := range r.engines {
		if x.id == id {
			return x.e, nil
		}
	}
	ids := make([]string, len(r.engines))
	for i, x := range r.engines {
		ids[i] = x.id
	}
	return nil, fmt.Errorf("%w %q (engines: %s)", ErrUnknownEngine, id, strings.Join(ids, ", "))
}

// Prepare picks the engine and checks the options, so a caller can refuse a request before it
// queues it.
func (r *Registry) Prepare(id string, o Options) (Engine, Options, error) {
	e, err := r.Get(id)
	if err != nil {
		return nil, o, err
	}
	if info := e.Info(); !info.Usable {
		msg := info.Error
		if msg == "" {
			msg = info.Name + " is not installed"
		}
		return nil, o, fmt.Errorf("%w: %s", ErrUnavailable, msg)
	}
	o, err = e.Clean(o)
	return e, o, err
}

// Read reads the image at path with the engine and options from Prepare and stamps the result
// with the format and the place.
func (r *Registry) Read(ctx context.Context, e Engine, path string, o Options, threads int) (*Result, error) {
	res, err := e.Read(ctx, path, o, threads)
	if err != nil {
		return nil, err
	}
	res.Format = Format
	if r.Place != "" {
		res.Engine = r.Place + ":" + res.Engine
	}
	if err := res.Check(); err != nil {
		return nil, fmt.Errorf("engine %s broke the result format: %w", e.Info().ID, err)
	}
	return res, nil
}

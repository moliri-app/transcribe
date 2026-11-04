// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package ocrapi is the OCR engine "api": an AI service reads the image, through one of three
// interfaces (OpenAI-compatible chat completions, Anthropic Messages, Gemini generateContent),
// with the user's own key from the app's settings. Prompt and the reading of the answer are those
// of Moliri's server path (static/api/_func/ai_ocr.php), so both places give the same result:
// lines with a box in pixels of the image, words without boxes (format 1, internal/ocr).
//
// The engine is chosen only by name or as the app's default (ocr.ByChoice): it costs money and the
// image leaves the computer.
package ocrapi

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moliri-app/transcribe/internal/ocr"
	"github.com/moliri-app/transcribe/internal/prefs"
)

// The interfaces a service speaks.
const (
	OpenAI    = "openai"    // POST {url}/chat/completions
	Anthropic = "anthropic" // POST {url}/messages
	Gemini    = "gemini"    // POST {url}/models/{model}:generateContent
)

// Provider is a service one can choose, with its usual address and model.
type Provider struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // OpenAI, Anthropic or Gemini
	URL   string `json:"url"`
	Model string `json:"model"`
	Keys  string `json:"keys,omitempty"`  // where one gets a key
	NoKey bool   `json:"noKey,omitempty"` // works without a key (a server of one's own)
}

// Providers are the services one can choose; the first is the default.
var Providers = []Provider{
	{ID: "openai", Label: "OpenAI", Kind: OpenAI, URL: "https://api.openai.com/v1", Model: "gpt-5-mini",
		Keys: "https://platform.openai.com/api-keys"},
	{ID: "anthropic", Label: "Anthropic (Claude)", Kind: Anthropic, URL: "https://api.anthropic.com/v1", Model: "claude-opus-5-5",
		Keys: "https://console.anthropic.com/settings/keys"},
	{ID: "gemini", Label: "Google Gemini", Kind: Gemini, URL: "https://generativelanguage.googleapis.com/v1beta", Model: "gemini-2.5-pro",
		Keys: "https://aistudio.google.com/apikey"},
	{ID: "mistral", Label: "Mistral", Kind: OpenAI, URL: "https://api.mistral.ai/v1", Model: "mistral-medium-latest",
		Keys: "https://console.mistral.ai/api-keys"},
	{ID: "custom", Label: "Eigener Server (OpenAI-kompatibel, etwa Ollama)", Kind: OpenAI, URL: "http://localhost:11434/v1",
		Model: "qwen2.5vl:7b", NoKey: true},
}

// Find is the provider with id, or the default.
func Find(id string) Provider {
	for _, p := range Providers {
		if p.ID == id {
			return p
		}
	}
	return Providers[0]
}

// Config is the service chosen in the settings.
type Config struct {
	Provider string
	URL      string // its base address, like https://api.openai.com/v1
	Model    string
	Key      string
}

// Of is the service stored in the settings (prefs.Prefs.OCRAPI).
func Of(s prefs.Service) Config {
	return Config{Provider: s.Provider, URL: s.URL, Model: s.Model, Key: s.Key}
}

func (c Config) provider() Provider { return Find(c.Provider) }

// Filled is c with the service's own address and model where c has none.
func (c Config) Filled() Config {
	p := c.provider()
	c.Provider = p.ID
	c.URL = strings.TrimSpace(c.URL)
	c.Model = strings.TrimSpace(c.Model)
	if c.URL == "" {
		c.URL = p.URL
	}
	if c.Model == "" {
		c.Model = p.Model
	}
	return c
}

// Label names the service for messages.
func (c Config) Label() string { return c.provider().Label }

// Kind is the interface the service speaks.
func (c Config) Kind() string { return c.provider().Kind }

func (c Config) base() string { return strings.TrimRight(c.URL, "/") }

// Usable explains what is missing before the service can be used; nil if nothing.
func (c Config) Usable() error {
	switch {
	case c.URL == "":
		return errors.New("Für den KI-Dienst fehlt die Adresse (Einstellungen, Texterkennung)")
	case c.Model == "":
		return errors.New("Für den KI-Dienst fehlt das Modell (Einstellungen, Texterkennung)")
	case c.Key == "" && !c.provider().NoKey:
		return ErrNoKey
	}
	return nil
}

// ErrNoKey is returned when no key is stored for a service that needs one.
var ErrNoKey = errors.New("Für den KI-Dienst ist noch kein Schlüssel hinterlegt (Einstellungen, Texterkennung)")

// Timeout is how long a reading may take at most: large models think for a minute on a dense page.
const Timeout = 150 * time.Second

// Languages are the models the engine lists. The model reads every language and script; these
// are the ones callers like Moliri offer (German, English, Fraktur and their pairs), plus a few
// the prompt names. Any other name of Tesseract's kind is accepted too (Clean).
var Languages = []string{"deu", "eng", "fra", "ita", "nld", "spa", "frak2021"}

var layouts = []string{"auto", "lines", "block", "sparse"}

// Engine is the engine "api".
type Engine struct {
	// Config is the service of the settings, read at every request (the key may change while the
	// app runs).
	Config func() Config
	// Timeout bounds one reading; 0 = Timeout.
	Timeout time.Duration
}

func (e *Engine) config() Config {
	if e.Config == nil {
		return Config{}.Filled()
	}
	return e.Config().Filled()
}

// ByChoice: the engine reads only when named or chosen as the app's default.
func (e *Engine) ByChoice() bool { return true }

func (e *Engine) Info() ocr.Info {
	c := e.config()
	info := ocr.Info{ID: "api", Name: "KI-Dienst (" + c.Label() + ")", Version: c.Model,
		Languages: append([]string{}, Languages...), Layouts: layouts, Boxes: "lines"}
	if err := c.Usable(); err != nil {
		info.Error = err.Error()
	} else {
		info.Ready, info.Usable = true, true
	}
	return info
}

var languageName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{1,31}$`)

// Clean accepts any language names of Tesseract's kind ("deu", "eng+fra", "deu+frak2021"): the
// model reads all of them, the prompt names those it knows. Default "deu"; layouts as Tesseract's.
func (e *Engine) Clean(o ocr.Options) (ocr.Options, error) {
	langs := strings.TrimSpace(o.Languages)
	if langs == "" {
		langs = "deu"
	}
	var out []string
	seen := map[string]bool{}
	for _, l := range strings.Split(langs, "+") {
		l = strings.ToLower(strings.TrimSpace(l))
		if !languageName.MatchString(l) {
			return o, fmt.Errorf("%w: language %q (names like deu, eng or frak2021, joined by +)", ocr.ErrOptions, l)
		}
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	layout := strings.ToLower(strings.TrimSpace(o.Layout))
	if layout == "" {
		layout = "auto"
	}
	if _, ok := ocr.Layouts[layout]; !ok {
		return o, fmt.Errorf("%w: layout must be auto, lines, block or sparse, not %q", ocr.ErrOptions, o.Layout)
	}
	return ocr.Options{Languages: strings.Join(out, "+"), Layout: layout}, nil
}

// Read sends the image (its longest side at most MaxSide pixels, as JPEG) with the prompt to the
// service and turns the answer into a result in the pixels of the image at path. threads does not
// matter here.
func (e *Engine) Read(ctx context.Context, path string, o ocr.Options, _ int) (*ocr.Result, error) {
	c := e.config()
	if err := c.Usable(); err != nil {
		return nil, fmt.Errorf("%w: %s", ocr.ErrUnavailable, err)
	}
	img, err := PrepareImage(path, MaxSide)
	if err != nil {
		return nil, err
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = Timeout
	}
	content, err := Ask(ctx, c, Prompt(o.Languages, o.Layout), img, timeout)
	if err != nil {
		return nil, err
	}
	return Parse(content, img.Size, EngineName(c.Model), o.Languages, o.Layout), nil
}

// EngineName is the result's engine: "api:" and the model (the registry puts the place before
// it, "desktop:api:gpt-5-mini"), short enough for Moliri's 100 characters with the place.
func EngineName(model string) string {
	model = strings.Join(strings.Fields(model), "-")
	if model == "" {
		model = "model"
	}
	name := "api:" + model
	const max = 92 // + "desktop:" = 100
	if len(name) > max {
		cut := max
		for cut > 0 && !utf8.RuneStart(name[cut]) {
			cut--
		}
		name = name[:cut]
	}
	return name
}

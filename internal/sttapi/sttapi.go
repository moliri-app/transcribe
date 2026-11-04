// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package sttapi recognises speech through a service instead of the models on this computer:
// one that speaks the OpenAI-compatible transcription API (the common standard: OpenAI, Groq,
// Mistral, local servers like Speaches or LocalAI), or ElevenLabs with its own. Whole recordings
// go over HTTP; live transcription over ElevenLabs' WebSocket, or utterance by utterance over
// HTTP where the service has nothing like it.
package sttapi

import (
	"context"
	"errors"
	"io"
	"math"
	"strings"

	"github.com/moliri-app/transcribe/internal/live"
)

// The protocols a service speaks.
const (
	OpenAI     = "openai"
	ElevenLabs = "elevenlabs"
)

// Provider is a service one can choose, with its usual address and model.
type Provider struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"` // OpenAI or ElevenLabs
	URL      string `json:"url"`
	Model    string `json:"model"`
	Keys     string `json:"keys,omitempty"`     // where one gets a key
	NoKey    bool   `json:"noKey,omitempty"`    // works without a key (a server of one's own)
	Speakers bool   `json:"speakers,omitempty"` // tells the speakers apart
	Realtime bool   `json:"realtime,omitempty"` // shows live text while one speaks
}

// Providers are the services one can choose; the first is the default.
var Providers = []Provider{
	{ID: "openai", Label: "OpenAI", Kind: OpenAI, URL: "https://api.openai.com/v1", Model: "whisper-1",
		Keys: "https://platform.openai.com/api-keys"},
	{ID: "groq", Label: "Groq", Kind: OpenAI, URL: "https://api.groq.com/openai/v1", Model: "whisper-large-v3",
		Keys: "https://console.groq.com/keys"},
	{ID: "mistral", Label: "Mistral (Voxtral)", Kind: OpenAI, URL: "https://api.mistral.ai/v1", Model: "voxtral-mini-latest",
		Keys: "https://console.mistral.ai/api-keys"},
	{ID: "elevenlabs", Label: "ElevenLabs", Kind: ElevenLabs, URL: "https://api.elevenlabs.io", Model: "scribe_v2",
		Keys: "https://elevenlabs.io/app/settings/api-keys", Speakers: true, Realtime: true},
	{ID: "custom", Label: "Eigener Server (OpenAI-kompatibel)", Kind: OpenAI, URL: "http://localhost:8000/v1", Model: "whisper-1",
		NoKey: true},
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

func (c Config) provider() Provider { return Find(c.Provider) }

// Filled is c with the service's own address and model where c has none.
func (c Config) Filled() Config {
	p := c.provider()
	c.Provider = p.ID
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

// WholeFile reports whether the service takes a recording as it is, in any format and length;
// otherwise it gets WAV pieces of at most MaxPiece seconds (OpenAI takes 25 MB at most).
func (c Config) WholeFile() bool { return c.provider().Kind == ElevenLabs }

// Speakers reports whether the service tells speakers apart.
func (c Config) Speakers() bool { return c.provider().Speakers }

// MaxPiece is the longest piece of a recording sent at once where WholeFile is false.
const MaxPiece = 600.0

func (c Config) base() string { return strings.TrimRight(c.URL, "/") }

// Usable explains what is missing before the service can be used; nil if nothing.
func (c Config) Usable() error {
	if c.URL == "" {
		return errors.New("Für die API fehlt die Adresse (Einstellungen, Erkennung).")
	}
	if c.Key == "" && !c.provider().NoKey {
		return ErrNoKey
	}
	return nil
}

// ErrNoKey is returned when no API key is stored for a service that needs one.
var ErrNoKey = errors.New("Für die API ist noch kein Schlüssel hinterlegt (Einstellungen, Erkennung).")

// Request says what is known about a recording.
type Request struct {
	Language string  // a code like de; "" lets the service find out
	Speakers int     // how many speak at most; 0: the service finds out
	Diarize  bool    // tell the speakers apart, where the service can
	Length   float64 // seconds of audio, where known (to place words when the service gives no times)
}

// Word is a word or a sound (laughter, applause) the service heard.
type Word struct {
	Text    string
	Start   float64
	End     float64
	P       float64 // how sure the service is, 0 to 1
	Speaker string  // the service's name for the speaker; "" if it tells none apart
	Event   bool    // a sound, its name in Text
}

// Result is the transcript of a recording.
type Result struct {
	Language string // two-letter code where known, like de
	Duration float64
	Words    []Word
}

// Transcribe sends a recording and waits for its transcript.
func Transcribe(ctx context.Context, c Config, audio io.Reader, name string, o Request) (*Result, error) {
	if err := c.Usable(); err != nil {
		return nil, err
	}
	if c.provider().Kind == ElevenLabs {
		return elevenLabsTranscribe(ctx, c, audio, name, o)
	}
	return openAITranscribe(ctx, c, audio, name, o)
}

// OpenLive opens live transcription for lang ("auto" or a code like de); it implements live.Opener
// with the service given.
func OpenLive(ctx context.Context, c Config, lang string) (live.Stream, error) {
	if err := c.Usable(); err != nil {
		return nil, err
	}
	if lang == "auto" {
		lang = ""
	}
	if c.provider().Kind == ElevenLabs {
		return elevenLabsLive(ctx, c, lang)
	}
	return newPieceStream(c, lang), nil
}

// languages are names and ISO 639-3 codes services answer with, as the two-letter codes the app uses.
var languages = map[string]string{
	"deu": "de", "german": "de", "eng": "en", "english": "en", "fra": "fr", "french": "fr", "spa": "es", "spanish": "es",
	"ita": "it", "italian": "it", "nld": "nl", "dutch": "nl", "pol": "pl", "polish": "pl", "por": "pt", "portuguese": "pt",
	"tur": "tr", "turkish": "tr", "rus": "ru", "russian": "ru", "ukr": "uk", "ukrainian": "uk", "ara": "ar", "arabic": "ar",
	"ces": "cs", "czech": "cs", "dan": "da", "danish": "da", "swe": "sv", "swedish": "sv", "nor": "no", "norwegian": "no",
	"fin": "fi", "finnish": "fi", "hun": "hu", "hungarian": "hu", "ell": "el", "greek": "el", "ron": "ro", "romanian": "ro",
	"hrv": "hr", "croatian": "hr", "srp": "sr", "serbian": "sr", "bul": "bg", "bulgarian": "bg", "slk": "sk", "slovak": "sk",
	"slv": "sl", "slovenian": "sl", "jpn": "ja", "japanese": "ja", "zho": "zh", "cmn": "zh", "chinese": "zh",
	"kor": "ko", "korean": "ko", "fas": "fa", "persian": "fa", "heb": "he", "hebrew": "he", "hin": "hi", "hindi": "hi",
}

func shortLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) == 2 {
		return s
	}
	return languages[s]
}

// spread places the words of text evenly over [a, b] by their length, for a service that gives no times.
func spread(text string, a, b, p float64) []Word {
	fields := strings.Fields(text)
	total := 0
	for _, f := range fields {
		total += len([]rune(f)) + 1
	}
	var out []Word
	t := a
	for _, f := range fields {
		d := (b - a) * float64(len([]rune(f))+1) / float64(total)
		out = append(out, Word{Text: f, Start: t, End: t + d, P: p})
		t += d
	}
	return out
}

func prob(logprob float64) float64 { return math.Exp(math.Min(0, logprob)) }

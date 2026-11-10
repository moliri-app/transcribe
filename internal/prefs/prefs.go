// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package prefs stores the user's defaults (processing options, drop behaviour, acceleration) in <data>/settings.json.
package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

type Prefs struct {
	Preset   string `json:"preset"`
	Speakers int    `json:"speakers"` // 0 = automatic
	Language string `json:"language"`
	Threads  int    `json:"threads"`
	DropMode string `json:"dropMode"` // "ask": show the queue options on drop, "auto": start right away
	Accel    string `json:"accel"`    // "cpu" or "cuda"
	GpuAsked bool   `json:"gpuAsked"` // the one-time GPU question was answered

	// Recognition is where speech is recognised: "local" (the models on this computer), "api" (a
	// service, no models are loaded) or "both". With both, LiveEngine and FileEngine say which one
	// live transcription (and dictating) and whole recordings use by default.
	Recognition string  `json:"recognition"`
	LiveEngine  string  `json:"liveEngine"`
	FileEngine  string  `json:"fileEngine"`
	API         Service `json:"api"`

	// LiveOff switches the background mode off: programs may not have the app record the
	// microphone, nothing is kept loaded (audio and images programs send are still processed,
	// recording on the Live page and dictating still work). On (the default), live recognition
	// stays loaded so a recording starts at once.
	LiveOff      bool     `json:"liveOff,omitempty"`
	LiveDevice   string   `json:"liveDevice,omitempty"`   // microphone for live transcription; "" = the system's default
	LiveSource   string   `json:"liveSource,omitempty"`   // what live transcription records: "mic" (""), "system" or "both"
	LiveLanguage string   `json:"liveLanguage,omitempty"` // what is spoken in a recording: "" (auto) or a code like de
	LiveOrigins  []string `json:"liveOrigins,omitempty"`  // web pages allowed to use live transcription besides localhost
	// „Meine Stimme“: the user's own words, which live transcription writes as they are written,
	// and whether punctuation they say aloud („Komma“, „neuer Absatz“) stays a word (on by default).
	LiveWords      []string `json:"liveWords,omitempty"`
	LiveNoCommands bool     `json:"liveNoCommands,omitempty"`

	// What programs transcribe over the live API is kept as notes on the Live page, unless
	// LiveAPINoSave; with LiveAPIKeepHours, those notes are deleted that long after they ended
	// (0 keeps them).
	LiveAPINoSave    bool `json:"liveApiNoSave,omitempty"`
	LiveAPIKeepHours int  `json:"liveApiKeepHours,omitempty"`

	Background bool `json:"background,omitempty"` // keep running in the notification area when the window is closed
	// NoBackgroundHint: the user declined the hint, shown once a program asks for the app, to start
	// it with Windows and keep it running in the background.
	NoBackgroundHint bool `json:"noBackgroundHint,omitempty"`

	// Text recognition (OCR) for programs: OCREngine is the engine a request that names none reads
	// with ("" = the first usable one, never the AI service); OCRAPI is the AI service of the
	// engine "api" (internal/ocrapi), with a key of its own, apart from the speech service.
	OCREngine string  `json:"ocrEngine,omitempty"`
	OCRAPI    Service `json:"ocrApi,omitzero"`

	DictateKey         string `json:"dictateKey,omitempty"`         // global shortcut for dictating: one of DictateKeys, or "off"
	DictateNoInsert    bool   `json:"dictateNoInsert,omitempty"`    // dictated text is not inserted at the cursor
	DictateNoClipboard bool   `json:"dictateNoClipboard,omitempty"` // dictated text does not go to the clipboard

	// Folders are the connected folders: their recordings (also in subfolders) stand in the
	// library where they are, without a copy in audio/ (internal/media.Linked). Unless SidecarOff,
	// a transcript of one is kept next to it as well (internal/sidecar), for others who open the
	// folder. FolderHint is the folder next to the EXE once the user declined to connect it.
	Folders    []string `json:"folders,omitempty"`
	SidecarOff bool     `json:"sidecarOff,omitempty"`
	FolderHint string   `json:"folderHint,omitempty"`
}

// Service is a service the app sends recordings or images to: the speech recognition service
// (internal/sttapi) or the AI service of text recognition (internal/ocrapi): which one of its
// list, at what address, with which model and key. Empty, the ones of the service (or the first)
// apply.
type Service struct {
	Provider string `json:"provider,omitempty"`
	URL      string `json:"url,omitempty"`
	Model    string `json:"model,omitempty"`
	Key      string `json:"key,omitempty"`
}

// DictateKeys are the shortcuts one can choose for dictating, the default first, each with its
// name as a German keyboard labels the keys.
var DictateKeys = []struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}{
	{"ctrl+alt+space", "Strg + Alt + Leertaste"},
	{"ctrl+shift+space", "Strg + Umschalt + Leertaste"},
	{"ctrl+alt+d", "Strg + Alt + D"},
}

// DictateLabel is the name of a shortcut from DictateKeys.
func DictateLabel(key string) string {
	for _, k := range DictateKeys {
		if k.Key == key {
			return k.Label
		}
	}
	return key
}

var mu sync.Mutex

func Default() Prefs {
	return Prefs{Preset: "auto", Language: "auto", Threads: DefaultThreads(), DropMode: "ask", Accel: "cpu",
		Recognition: Local, LiveEngine: Local, FileEngine: Local}
}

// DefaultThreads is how many threads recognition on this computer uses unless set otherwise:
// about one per core (half the logical processors), at least 4 and at most 16. The settings no
// longer show the number.
func DefaultThreads() int { return max(4, min(16, runtime.NumCPU()/2)) }

// Where speech is recognised.
const (
	Local = "local"
	API   = "api"
	Both  = "both"
)

// Engine is what recognises speech when the user chose want ("local", "api" or "" for the
// default def): only what Recognition allows.
func (p Prefs) Engine(want, def string) string {
	switch p.Recognition {
	case Local, API:
		return p.Recognition
	}
	if want == Local || want == API {
		return want
	}
	return def
}

// UsesModels reports whether the models on this computer may be used (and so be downloaded).
func (p Prefs) UsesModels() bool { return p.Recognition != API }

func (p *Prefs) normalize() {
	d := Default()
	switch p.Preset {
	case "auto", "interview", "gespraech", "vortrag", "musik":
	default:
		p.Preset = d.Preset
	}
	if p.Language == "" {
		p.Language = d.Language
	}
	if p.Speakers < 0 || p.Speakers > 20 {
		p.Speakers = 0
	}
	if p.Threads < 1 || p.Threads > 64 {
		p.Threads = d.Threads
	}
	if p.DropMode != "auto" {
		p.DropMode = "ask"
	}
	if p.Accel != "cuda" {
		p.Accel = "cpu"
	}
	if p.Recognition != API && p.Recognition != Both {
		p.Recognition = Local
	}
	if p.LiveEngine != API {
		p.LiveEngine = Local
	}
	if p.FileEngine != API {
		p.FileEngine = Local
	}
	if p.Recognition != Both { // one way to recognise: it is the default for everything
		p.LiveEngine, p.FileEngine = p.Recognition, p.Recognition
	}
	p.OCREngine = strings.ToLower(strings.TrimSpace(p.OCREngine))
	if !engineID(p.OCREngine) {
		p.OCREngine = ""
	}
	p.LiveAPIKeepHours = max(0, p.LiveAPIKeepHours)
	p.LiveWords = normWords(p.LiveWords)
	p.Folders = normFolders(p.Folders)
	if p.DictateKey != "off" && DictateLabel(p.DictateKey) == p.DictateKey {
		p.DictateKey = DictateKeys[0].Key
	}
}

// engineID reports whether s looks like the id of an OCR engine ("tesseract", "api").
func engineID(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// normFolders keeps the connected folders that are absolute paths, each once.
func normFolders(in []string) []string {
	var out []string
	for _, f := range in {
		f = strings.TrimSpace(f)
		if f == "" || !filepath.IsAbs(f) {
			continue
		}
		f = filepath.Clean(f)
		if !slices.ContainsFunc(out, func(o string) bool { return SamePath(o, f) }) {
			out = append(out, f)
		}
	}
	return out
}

// SamePath reports whether two cleaned paths name the same folder (on Windows regardless of case).
func SamePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// MaxLiveWords is how many own words one may keep; MaxLiveWord how long one may be (in letters).
const (
	MaxLiveWords = 200
	MaxLiveWord  = 60
)

// normWords keeps the own words that are words: single spaces, none twice (in any case), none too
// long, not too many.
func normWords(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range in {
		w = strings.Join(strings.Fields(w), " ")
		if w == "" || utf8.RuneCountInString(w) > MaxLiveWord || seen[strings.ToLower(w)] || len(out) == MaxLiveWords {
			continue
		}
		seen[strings.ToLower(w)] = true
		out = append(out, w)
	}
	return out
}

func path(dataDir string) string { return filepath.Join(dataDir, "settings.json") }

// Load returns the stored preferences; missing or broken files give the defaults.
func Load(dataDir string) Prefs {
	mu.Lock()
	defer mu.Unlock()
	return load(dataDir)
}

func load(dataDir string) Prefs {
	p := Default()
	if b, err := os.ReadFile(path(dataDir)); err == nil {
		_ = json.Unmarshal(b, &p)
	}
	p.normalize()
	return p
}

// Update loads the preferences, lets f change them, then validates and saves the result.
func Update(dataDir string, f func(*Prefs)) (Prefs, error) {
	mu.Lock()
	defer mu.Unlock()
	p := load(dataDir)
	f(&p)
	p.normalize()
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return p, err
	}
	tmp := path(dataDir) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return p, err
	}
	return p, os.Rename(tmp, path(dataDir))
}

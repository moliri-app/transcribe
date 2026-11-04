// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/moliri-app/transcribe/internal/ocrapi"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/internal/setup"
	"github.com/moliri-app/transcribe/internal/sttapi"
	"github.com/moliri-app/transcribe/jobs"
	"github.com/moliri-app/transcribe/models"
	"github.com/moliri-app/transcribe/sherpa"
)

// prefsView is the stored defaults as the pages see them: the API keys only as their last
// characters, and the services one can choose (speech recognition, and the AI service of text
// recognition).
type prefsView struct {
	prefs.Prefs
	API          apiView           `json:"api"`
	Providers    []sttapi.Provider `json:"apiProviders"`
	OCRAPI       apiView           `json:"ocrApi"`
	OCRProviders []ocrapi.Provider `json:"ocrApiProviders"`
}

type apiView struct {
	Provider string `json:"provider"`
	URL      string `json:"url"`
	Model    string `json:"model"`
	KeySet   bool   `json:"keySet"`
	KeyHint  string `json:"keyHint,omitempty"`
}

func apiConfig(p prefs.Prefs) sttapi.Config {
	return sttapi.Config{Provider: p.API.Provider, URL: p.API.URL, Model: p.API.Model, Key: p.API.Key}.Filled()
}

func ocrAPIConfig(p prefs.Prefs) ocrapi.Config { return ocrapi.Of(p.OCRAPI).Filled() }

// keyHint is the end of a key, to tell keys apart without showing them; none for short ones.
func keyHint(key string) string {
	if k := []rune(key); len(k) > 8 {
		return string(k[len(k)-4:])
	}
	return ""
}

func viewPrefs(p prefs.Prefs) prefsView {
	c := apiConfig(p)
	o := ocrAPIConfig(p)
	v := prefsView{Prefs: p, Providers: sttapi.Providers, OCRProviders: ocrapi.Providers,
		API:    apiView{Provider: c.Provider, URL: c.URL, Model: c.Model, KeySet: c.Key != "", KeyHint: keyHint(c.Key)},
		OCRAPI: apiView{Provider: o.Provider, URL: o.URL, Model: o.Model, KeySet: o.Key != "", KeyHint: keyHint(o.Key)}}
	v.Prefs.API = prefs.Service{}
	v.Prefs.OCRAPI = prefs.Service{}
	return v
}

// chooseEngine settles what recognises the recordings of a new job (engine in m: "local", "api"
// or none for the default) as far as the settings allow.
func (s *Server) chooseEngine(m map[string]any) error {
	p := prefs.Load(s.cfg.DataDir)
	want, _ := m["engine"].(string)
	e := p.Engine(want, p.FileEngine)
	if e == prefs.API {
		if err := apiConfig(p).Usable(); err != nil {
			return jobs.JobError(err.Error())
		}
	}
	m["engine"] = e
	return nil
}

// serviceInput changes a service of the settings; nil fields stay.
type serviceInput struct {
	Provider *string `json:"provider"` // another service: its address and model, no key
	URL      *string `json:"url"`
	Model    *string `json:"model"`
	Key      *string `json:"key"`
}

// apply changes sv, whose provider (as filled in) is current.
func (a *serviceInput) apply(sv *prefs.Service, current string) {
	if a.Provider != nil && *a.Provider != current {
		*sv = prefs.Service{Provider: *a.Provider}
	}
	if a.URL != nil {
		sv.URL = strings.TrimSpace(*a.URL)
	}
	if a.Model != nil {
		sv.Model = strings.TrimSpace(*a.Model)
	}
	if a.Key != nil {
		sv.Key = strings.TrimSpace(*a.Key)
	}
}

// ocrEngineKnown reports whether id may be the default OCR engine: "" (automatic) or one of the app's.
func (s *Server) ocrEngineKnown(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return true
	}
	if s.cfg.OCR == nil {
		return false
	}
	for _, e := range s.cfg.OCR.Engines() {
		if e.ID == id {
			return true
		}
	}
	return false
}

// prefsAPI reads (GET) or partially updates (POST) the stored defaults.
func (s *Server) prefsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, 200, viewPrefs(prefs.Load(s.cfg.DataDir)))
		return
	}
	if r.Method != http.MethodPost || !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	var in struct {
		Preset   *string `json:"preset"`
		Speakers *int    `json:"speakers"`
		Language *string `json:"language"`
		Threads  *int    `json:"threads"`
		DropMode *string `json:"dropMode"`
		Accel    *string `json:"accel"`

		Recognition *string       `json:"recognition"`
		LiveEngine  *string       `json:"liveEngine"`
		FileEngine  *string       `json:"fileEngine"`
		API         *serviceInput `json:"api"`

		OCREngine *string       `json:"ocrEngine"`
		OCRAPI    *serviceInput `json:"ocrApi"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if in.Accel != nil && *in.Accel == "cuda" && models.FindLib(setup.CudaLibDir(s.cfg.DataDir)) == "" {
		fail(w, 400, "Die GPU-Runtime ist noch nicht heruntergeladen.")
		return
	}
	if in.Language != nil {
		l, err := jobs.NormLanguage(*in.Language)
		if err != nil {
			fail(w, 400, "Sprache: automatisch oder Codes wie de oder de,en")
			return
		}
		in.Language = &l
	}
	if in.API != nil && in.API.Provider != nil && sttapi.Find(*in.API.Provider).ID != *in.API.Provider {
		fail(w, 400, "Unbekannter Dienst: "+*in.API.Provider)
		return
	}
	if in.OCRAPI != nil && in.OCRAPI.Provider != nil && ocrapi.Find(*in.OCRAPI.Provider).ID != *in.OCRAPI.Provider {
		fail(w, 400, "Unbekannter Dienst: "+*in.OCRAPI.Provider)
		return
	}
	if in.OCREngine != nil && !s.ocrEngineKnown(*in.OCREngine) {
		fail(w, 400, "Unbekannte Texterkennung: "+*in.OCREngine)
		return
	}
	before := prefs.Load(s.cfg.DataDir)
	p, err := prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) {
		if in.Recognition != nil {
			p.Recognition = *in.Recognition
		}
		if in.LiveEngine != nil {
			p.LiveEngine = *in.LiveEngine
		}
		if in.FileEngine != nil {
			p.FileEngine = *in.FileEngine
		}
		if a := in.API; a != nil {
			a.apply(&p.API, apiConfig(*p).Provider)
		}
		if a := in.OCRAPI; a != nil {
			a.apply(&p.OCRAPI, ocrAPIConfig(*p).Provider)
		}
		if in.OCREngine != nil {
			p.OCREngine = *in.OCREngine
		}
		if in.Preset != nil {
			p.Preset = *in.Preset
		}
		if in.Speakers != nil {
			p.Speakers = *in.Speakers
		}
		if in.Language != nil {
			p.Language = *in.Language
		}
		if in.Threads != nil {
			p.Threads = *in.Threads
		}
		if in.DropMode != nil {
			p.DropMode = *in.DropMode
		}
		if in.Accel != nil {
			p.Accel = *in.Accel
			p.GpuAsked = true
		}
	})
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if s.Live != nil {
		s.Live.Warm() // live transcription may recognise elsewhere now
	}
	if p.UsesModels() && !before.UsesModels() {
		// the models may be used again: fetch what is missing, as on the first start
		go func() {
			if err := setup.EnsureAll(context.Background(), s.cfg.DataDir, true); err != nil {
				log.Println("setup:", err)
			}
		}()
	}
	writeJSON(w, 200, viewPrefs(p))
}

var (
	gpuMu    sync.Mutex
	gpuCache setup.GPU
	gpuAt    time.Time
)

// cachedGPU detects the GPU at most once a minute, or now if fresh.
func cachedGPU(dataDir string, fresh bool) setup.GPU {
	gpuMu.Lock()
	defer gpuMu.Unlock()
	if fresh || time.Since(gpuAt) > time.Minute {
		gpuCache, gpuAt = setup.DetectGPU(dataDir), time.Now()
	}
	return gpuCache
}

// gpuAPI reports the GPU state: what was found, what is installed and what the runtime currently uses.
func (s *Server) gpuAPI(w http.ResponseWriter, r *http.Request) {
	p := prefs.Load(s.cfg.DataDir)
	installed := models.FindLib(setup.CudaLibDir(s.cfg.DataDir)) != ""
	g := cachedGPU(s.cfg.DataDir, r.URL.Query().Get("fresh") != "")
	active := "cpu"
	if sherpa.Loaded() {
		active = sherpa.Provider()
	}
	wanted := "cpu"
	if p.Accel == "cuda" && installed && g.Ready {
		wanted = "cuda"
	}
	writeJSON(w, 200, map[string]any{
		"gpu":           g,
		"installed":     installed,
		"accel":         p.Accel,
		"asked":         p.GpuAsked,
		"active":        active,
		"restartNeeded": sherpa.Loaded() && wanted != active,
		// the question is only worth asking if CUDA could really be used, with libraries the app can fetch itself
		"offer": g.Present && g.Supported && (g.Ready || g.Fetch > 0) && !installed && !p.GpuAsked,
	})
}

// gpuChoice handles "enable" (choose CUDA and download its runtime, after the NVIDIA libraries it lacks)
// and "dismiss" (stay on CPU).
func (s *Server) gpuChoice(enable bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !allowedPost(r) {
			http.Error(w, "forbidden", 403)
			return
		}
		if !enable {
			_, _ = prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) { p.GpuAsked, p.Accel = true, "cpu" })
			writeJSON(w, 200, map[string]any{"ok": true})
			return
		}
		g := cachedGPU(s.cfg.DataDir, true)
		if !g.Present || !g.Supported {
			fail(w, 400, "Keine unterstützte NVIDIA-GPU gefunden.")
			return
		}
		_, _ = prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) { p.GpuAsked = true })
		if !g.Ready && g.Fetch == 0 {
			fail(w, 400, "Es fehlen NVIDIA-Bibliotheken, die die App nicht selbst laden kann: "+strings.Join(g.Missing, ", "))
			return
		}
		go func() {
			err := setup.SetUpGPU(context.Background(), s.cfg.DataDir, !g.Ready)
			if !g.Ready {
				cachedGPU(s.cfg.DataDir, true)
			}
			if err == nil {
				_, _ = prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) { p.Accel = "cuda" })
			}
		}()
		writeJSON(w, 202, map[string]any{"ok": true})
	}
}

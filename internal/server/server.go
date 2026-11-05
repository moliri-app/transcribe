// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/moliri-app/transcribe/annotate"
	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/queue"
	"github.com/moliri-app/transcribe/ocr"
	"github.com/moliri-app/transcribe/internal/update"
)

type Config struct {
	DataDir       string
	Runner        queue.Runner
	Retranscriber annotate.Retranscriber // may be nil
	Threads       int
	Live          live.Opener     // may be nil: no live transcription
	Warmer        live.Warmer     // may be nil: live recognition is loaded only when a recording starts
	OCR           *ocr.Registry   // may be nil: no text recognition
	Updater       *update.Updater // may be nil: a build that never looks for updates
}

type Server struct {
	cfg   Config
	Queue *queue.Queue
	Live  *live.Hub // nil without Config.Live
	// Dictation is the global shortcut for dictating, nil where there is none; set it before
	// serving.
	Dictation Dictation
	mux       *http.ServeMux
	started   time.Time
	lastPing  atomic.Int64 // unix nanos of the last UI heartbeat
	lastBye   atomic.Int64
}

// WindowGone reports whether the UI stopped sending heartbeats: no ping for
// `grace`, or no ping at all within `firstWait` of startup.
func (s *Server) WindowGone(grace, firstWait time.Duration) bool {
	last := s.lastPing.Load()
	if last == 0 {
		return time.Since(s.started) > firstWait
	}
	// the page said goodbye (closed) and did not come back (reload/navigation) within 5s
	if bye := s.lastBye.Load(); bye > last && time.Since(time.Unix(0, bye)) > 5*time.Second {
		return true
	}
	return time.Since(time.Unix(0, last)) > grace
}

// WindowOpen reports whether a window of the app is open: it sent a heartbeat a moment ago (one
// comes every 5 s) and did not say goodbye since.
func (s *Server) WindowOpen() bool {
	last := s.lastPing.Load()
	return last != 0 && s.lastBye.Load() < last && time.Since(time.Unix(0, last)) < 12*time.Second
}

func (s *Server) bye(w http.ResponseWriter, r *http.Request) {
	s.lastBye.Store(time.Now().UnixNano())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	s.lastPing.Store(time.Now().UnixNano())
	w.WriteHeader(http.StatusNoContent)
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, started: time.Now(), Queue: queue.New(cfg.DataDir, cfg.Runner, cfg.Threads)}
	if cfg.Live != nil {
		s.Live = live.NewHub(cfg.DataDir, cfg.Live)
		s.Live.Warmer = cfg.Warmer
	}
	m := http.NewServeMux()
	m.HandleFunc("/api/ping", s.ping)
	m.HandleFunc("/api/bye", s.bye)
	m.HandleFunc("/api/instance", s.instance)
	m.HandleFunc("/api/background", s.background)
	m.HandleFunc("/api/dictate", s.dictate)
	m.HandleFunc("/api/update", s.updateAPI)
	m.HandleFunc("/", s.root)
	m.HandleFunc("/audio/", s.audio)
	m.HandleFunc("/results/", s.results)
	m.HandleFunc("/api/setup", s.setup)
	m.HandleFunc("/api/settings", s.settings)
	m.HandleFunc("/api/settings/download", s.settingsDownload)
	m.HandleFunc("/api/settings/open", s.settingsOpen)
	m.HandleFunc("/api/prefs", s.prefsAPI)
	m.HandleFunc("/api/gpu", s.gpuAPI)
	m.HandleFunc("/api/gpu/enable", s.gpuChoice(true))
	m.HandleFunc("/api/gpu/dismiss", s.gpuChoice(false))
	m.HandleFunc("/api/index", s.index)
	m.HandleFunc("/api/peaks", s.peaks)
	m.HandleFunc("/api/thumbs", s.thumbs)
	m.HandleFunc("/api/export", s.export)
	m.HandleFunc("/api/clips/export", s.clipExport)
	m.HandleFunc("/api/queue", s.queueList)
	m.HandleFunc("/api/queue/", s.queueAction)
	m.HandleFunc("/api/upload", s.upload)
	m.HandleFunc("/api/annotations", s.annotations)
	m.HandleFunc("/api/apply", s.apply)
	m.HandleFunc("/api/live", s.liveSocket)
	m.HandleFunc("/api/live/events", s.liveEvents)
	m.HandleFunc("/api/live/", s.liveAPI)
	m.HandleFunc("/api/ocr", s.ocrRead)
	m.HandleFunc("/api/ocr/engines", s.ocrEngines)
	m.HandleFunc("/api/ocr/schema", s.ocrSchema)
	s.mux = m
	return s
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		s.mux.ServeHTTP(w, r)
	})
}

// Start runs the job worker until ctx is cancelled.
func (s *Server) Start(ctx context.Context) { s.Queue.Start(ctx) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	b, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(b)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"ok": false, "error": msg})
}

func localHost(h string) bool { return h == "localhost" || h == "127.0.0.1" }

// allowedPost rejects requests from foreign pages (CSRF / DNS rebinding).
func allowedPost(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	if !localHost(host) {
		return false
	}
	if vals, ok := r.Header["Origin"]; ok {
		u, err := url.Parse(vals[0])
		if err != nil || !localHost(u.Hostname()) {
			return false
		}
	}
	return true
}

func validStem(stem string) bool {
	if stem == "" || stem == "." || stem == ".." || strings.ContainsAny(stem, "/\\:\x00") {
		return false
	}
	return true
}

func (s *Server) audioDir() string { return media.AudioDir(s.cfg.DataDir) }

func (s *Server) resultsDir() string { return media.ResultsDir(s.cfg.DataDir) }

// safeJoin maps a URL-relative path below root, refusing anything that could escape it.
func safeJoin(root, rel string) (string, bool) {
	if rel == "" || strings.ContainsAny(rel, "\\:\x00") {
		return "", false
	}
	for _, p := range strings.Split(rel, "/") {
		if p == ".." {
			return "", false
		}
	}
	return filepath.Join(root, filepath.FromSlash(rel)), true
}

func (s *Server) serveBelow(root, prefix string, w http.ResponseWriter, r *http.Request) {
	p, ok := safeJoin(root, strings.TrimPrefix(r.URL.Path, prefix))
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}

func (s *Server) audio(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/audio/")
	if name == "" || strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}
	s.serveBelow(s.audioDir(), "/audio/", w, r)
}

func (s *Server) results(w http.ResponseWriter, r *http.Request) {
	s.serveBelow(s.resultsDir(), "/results/", w, r)
}

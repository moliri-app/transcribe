// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/internal/llm"
	"github.com/moliri-app/transcribe/internal/natfilter"
	"github.com/moliri-app/transcribe/internal/setup"
	"github.com/moliri-app/transcribe/models"
)

// Moliri's filter in plain words, answered by the language model on this computer
// (docs/filter-api.md): GET /api/filter says which model and whether it is there, POST
// /api/filter asks a question, GET /api/filter?id=… tells how far it got and the answer, POST
// /api/filter/cancel drops it, POST /api/filter/download fetches the model. A question takes
// minutes on a laptop's processor, longer than a page should hold one request open, so it runs
// in the background and the page looks after it. Web pages need the same approval as for text
// recognition.

// Chatter is the language model (llm.Server).
type Chatter interface {
	Ready() bool
	Loaded() bool
	Warm(ctx context.Context) error
	Chat(ctx context.Context, req llm.Request, progress func(llm.Progress)) (llm.Answer, error)
}

const (
	maxFilterQueue = 20
	// filterKeep is how long an answer waits for the page to fetch it.
	filterKeep = 10 * time.Minute
)

// filterFetching is set while a download the page asked for runs, filterWarming while the model
// loads after a GET.
var filterFetching, filterWarming atomic.Bool

type filterJob struct {
	ID       string
	in       natfilter.Input
	cancel   context.CancelFunc
	ctx      context.Context
	created  time.Time
	started  time.Time
	finished time.Time
	status   string // pending, processing, done, failed
	progress llm.Progress
	result   natfilter.Result
	seconds  float64
	errText  string
	errKind  string
	call     *live.Call // on the Live page
	stage    string     // of progress, as the Live page was told
}

// filterQueue holds the questions: one is asked at a time, in the order they came.
type filterQueue struct {
	mu      sync.Mutex
	jobs    map[string]*filterJob
	pending []*filterJob
	running bool
}

func (s *Server) filterInfo() map[string]any {
	info := map[string]any{
		"ok":          true,
		"model":       s.cfg.FilterModel,
		"installed":   s.cfg.Filter.Ready(),
		"loaded":      s.cfg.Filter.Loaded(),
		"size":        s.cfg.FilterSizeMB,
		"comparisons": natfilter.Comparisons,
	}
	if d := s.filterDevice(); d != nil {
		info["device"] = d
	}
	items, _ := models.Snapshot()
	for _, it := range items {
		if it.Name == "llm" && (it.State == "downloading" || it.State == "error") {
			info["download"] = map[string]any{"state": it.State, "done": it.Done, "total": it.Total, "error": it.Error}
		}
	}
	s.filters.mu.Lock()
	info["queued"] = len(s.filters.pending)
	s.filters.mu.Unlock()
	return info
}

// filterDevice says what the model runs on: the build ("cuda13", "cuda12", "vulkan", "cpu") and
// the graphics card's name. The running build, or the one picked for this computer while the
// model is not loaded; nil before that is picked.
func (s *Server) filterDevice() map[string]string {
	build, name := setup.LLMDevice(s.cfg.DataDir)
	if d, ok := s.cfg.Filter.(interface{ Device() string }); ok && d.Device() != "" && d.Device() != build {
		build, name = d.Device(), ""
	}
	if build == "" {
		return nil
	}
	return map[string]string{"build": build, "name": name}
}

func (s *Server) filterAPI(w http.ResponseWriter, r *http.Request) {
	if !s.pageAccess(w, r, "den Filter in eigenen Worten") {
		return
	}
	if s.cfg.Filter == nil {
		fail(w, http.StatusServiceUnavailable, "Der Filter in eigenen Worten ist in diesem Programm nicht verfügbar")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if id := r.URL.Query().Get("id"); id != "" {
			s.filterState(w, id)
			return
		}
		// With ?warm=1 a page says a question is near (the input got focus): the model loads
		// meanwhile, some seconds, so that the question need not wait for it. Not on every
		// look: the model takes some gigabytes of memory.
		if r.URL.Query().Get("warm") == "1" && s.cfg.Filter.Ready() && !s.cfg.Filter.Loaded() && filterWarming.CompareAndSwap(false, true) {
			go func() {
				defer filterWarming.Store(false)
				if err := s.cfg.Filter.Warm(context.Background()); err != nil {
					log.Println("filter:", err)
				}
			}()
		}
		writeJSON(w, 200, s.filterInfo())
	case http.MethodPost:
		s.filterAsk(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) filterAsk(w http.ResponseWriter, r *http.Request) {
	var in natfilter.Input
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, "JSON: "+err.Error())
		return
	}
	in = natfilter.Clean(in)
	if in.Text == "" || in.Catalog == "" || len(in.Fields) == 0 {
		fail(w, http.StatusBadRequest, "text, catalog und fields werden gebraucht")
		return
	}
	request := filterRequest(in)
	if !s.cfg.Filter.Ready() {
		s.Live.Logged(live.KindFilter, clientName(r), s.cfg.FilterModel, request, errors.New("Das Sprachmodell ist noch nicht heruntergeladen"))
		fail(w, http.StatusServiceUnavailable, "Das Sprachmodell für den Filter in eigenen Worten ist in Transcribe noch nicht geladen")
		return
	}
	q := &s.filters
	q.mu.Lock()
	q.sweep()
	if len(q.pending) >= maxFilterQueue {
		q.mu.Unlock()
		s.Live.Logged(live.KindFilter, clientName(r), s.cfg.FilterModel, request, errors.New("Zu viele Fragen warten schon"))
		fail(w, http.StatusTooManyRequests, "Zu viele Fragen warten schon")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &filterJob{ID: newJobID(), in: in, ctx: ctx, cancel: cancel, created: time.Now(), status: "pending",
		call: s.Live.Call(live.KindFilter, clientName(r), s.cfg.FilterModel, request)}
	if q.running || len(q.pending) > 0 {
		job.call.Status("Wartet, bis die Fragen davor beantwortet sind …")
	}
	if q.jobs == nil {
		q.jobs = map[string]*filterJob{}
	}
	q.jobs[job.ID] = job
	q.pending = append(q.pending, job)
	ahead := len(q.pending) - 1
	if q.running {
		ahead++
	}
	if !q.running {
		q.running = true
		go s.filterWork()
	}
	q.mu.Unlock()
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "id": job.ID, "status": "pending", "ahead": ahead})
}

// filterWork asks the waiting questions one after the other.
func (s *Server) filterWork() {
	q := &s.filters
	for {
		q.mu.Lock()
		if len(q.pending) == 0 {
			q.running = false
			q.mu.Unlock()
			return
		}
		job := q.pending[0]
		q.pending = q.pending[1:]
		job.status, job.started = "processing", time.Now()
		q.mu.Unlock()
		job.call.Status("Rechnet …")

		key := sha256.Sum256([]byte(natfilter.Prefix(job.in.Catalog)))
		msgs := natfilter.Messages(job.in.Text, job.in.Catalog)
		req := llm.Request{JSON: true, MaxTokens: natfilter.MaxTokens, CacheKey: hex.EncodeToString(key[:12])}
		for _, m := range msgs {
			req.Messages = append(req.Messages, llm.Message{Role: m.Role, Content: m.Content})
		}
		ans, err := s.cfg.Filter.Chat(job.ctx, req, func(p llm.Progress) {
			q.mu.Lock()
			job.progress = p
			tell := p.Stage != job.stage
			job.stage = p.Stage
			q.mu.Unlock()
			if tell {
				job.call.Status(filterStages[p.Stage])
			}
		})

		q.mu.Lock()
		job.finished = time.Now()
		job.seconds = ans.Seconds
		switch {
		case job.ctx.Err() != nil:
			job.status, job.errKind, job.errText = "failed", "cancelled", "Abgebrochen"
		case errors.Is(err, llm.ErrNotInstalled):
			job.status, job.errKind, job.errText = "failed", "not_installed", "Das Sprachmodell ist in Transcribe nicht geladen"
		case errors.Is(err, llm.ErrTooLong):
			job.status, job.errKind, job.errText = "failed", "too_long", "Der Feldkatalog ist für das Sprachmodell zu lang"
		case err != nil:
			job.status, job.errKind, job.errText = "failed", "failed", err.Error()
		default:
			if res, ok := natfilter.Parse(ans.Content, job.in.Fields); ok {
				job.status, job.result = "done", res
			} else {
				content := []rune(ans.Content)
				if len(content) > 200 {
					content = content[:200]
				}
				job.status, job.errKind, job.errText = "failed", "no_json", string(content)
			}
		}
		job.cancel()
		var failed error
		if job.status == "failed" {
			failed = errors.New(job.errText)
			if job.errKind == "no_json" {
				failed = errors.New("Die Antwort des Sprachmodells enthält kein JSON")
			}
		}
		facts := s.filterFacts(job, ans)
		q.mu.Unlock()
		job.call.End(ans.Content, failed, facts...)
		if err != nil && job.ctx.Err() == nil {
			log.Println("filter:", err)
		}
	}
}

// sweep forgets answers nobody fetched. Called with mu held.
func (q *filterQueue) sweep() {
	for id, j := range q.jobs {
		if !j.finished.IsZero() && time.Since(j.finished) > filterKeep {
			delete(q.jobs, id)
		}
	}
}

func (s *Server) filterState(w http.ResponseWriter, id string) {
	q := &s.filters
	q.mu.Lock()
	defer q.mu.Unlock()
	job := q.jobs[id]
	if job == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "Diese Frage gibt es nicht (mehr)", "code": "not-found"})
		return
	}
	out := map[string]any{"ok": true, "id": job.ID, "status": job.status, "ahead": 0}
	switch job.status {
	case "pending":
		ahead := 0
		for _, p := range q.pending {
			if p == job {
				break
			}
			ahead++
		}
		for _, j := range q.jobs {
			if j.status == "processing" {
				ahead++ // the one being asked
			}
		}
		out["ahead"] = ahead
		out["waited"] = time.Since(job.created).Seconds()
	case "processing":
		out["stage"] = job.progress.Stage
		out["progress"] = map[string]int{"done": job.progress.Done, "total": job.progress.Total, "cached": job.progress.Cached}
		out["elapsed"] = time.Since(job.started).Seconds()
		out["waited"] = time.Since(job.created).Seconds()
	case "done":
		out["conditions"] = job.result.Conditions
		out["not_understood"] = job.result.NotUnderstood
		out["dropped"] = job.result.Dropped
		out["seconds"] = job.seconds
		out["model"] = s.cfg.FilterModel
		if d := s.filterDevice(); d != nil {
			out["device"] = d
		}
	case "failed":
		out["error"] = job.errText
		out["error_kind"] = job.errKind
	}
	writeJSON(w, 200, out)
}

// filterCancel drops a question: waiting, it is never asked; being asked, the model stops.
func (s *Server) filterCancel(w http.ResponseWriter, r *http.Request) {
	if !s.pageAccess(w, r, "den Filter in eigenen Worten") {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "JSON: "+err.Error())
		return
	}
	q := &s.filters
	q.mu.Lock()
	job := q.jobs[body.ID]
	if job != nil {
		job.cancel()
		for i, p := range q.pending {
			if p == job {
				q.pending = append(q.pending[:i], q.pending[i+1:]...)
				job.status, job.errKind, job.errText, job.finished = "failed", "cancelled", "Abgebrochen", time.Now()
				job.call.End("", errors.New("Abgebrochen, bevor sie gestellt wurde"))
				break
			}
		}
	}
	q.mu.Unlock()
	writeJSON(w, 200, map[string]any{"ok": true})
}

// filterDownload starts fetching the model in the background; GET /api/filter shows how far it got.
func (s *Server) filterDownload(w http.ResponseWriter, r *http.Request) {
	if !s.pageAccess(w, r, "den Filter in eigenen Worten") {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg.Filter == nil || s.cfg.FilterFetch == nil {
		fail(w, http.StatusServiceUnavailable, "Der Filter in eigenen Worten ist in diesem Programm nicht verfügbar")
		return
	}
	if !s.cfg.Filter.Ready() && filterFetching.CompareAndSwap(false, true) {
		models.Track("llm", "downloading", 0, 0, nil)
		go func() {
			defer filterFetching.Store(false)
			if err := s.cfg.FilterFetch(context.Background()); err != nil {
				log.Println("filter download:", err)
			}
		}()
	}
	writeJSON(w, 200, s.filterInfo())
}

func newJobID() string {
	b := make([]byte, 9)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// filterStages say on the Live page what the model is doing.
var filterStages = map[string]string{"loading": "Lädt das Sprachmodell …", "reading": "Liest die Frage …", "writing": "Schreibt die Antwort …"}

// filterRequest is what the Live page shows of a question: the words, then what the model is
// given, its prompt with the catalog of fields.
func filterRequest(in natfilter.Input) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Frage: %s\nFelder: %d\n", in.Text, len(in.Fields))
	for _, m := range natfilter.Messages(in.Text, in.Catalog) {
		fmt.Fprintf(&b, "\n[%s]\n%s\n", m.Role, m.Content)
	}
	return b.String()
}

// filterFacts are what helps to see how a question went: the wait, the model's work, the result.
// Called with the queue's mu held.
func (s *Server) filterFacts(job *filterJob, ans llm.Answer) []live.Fact {
	var facts []live.Fact
	add := func(label, value string) { facts = append(facts, live.Fact{Label: label, Value: value}) }
	if !job.started.IsZero() {
		add("Gewartet", live.Seconds(job.started.Sub(job.created)))
	}
	if ans.Seconds > 0 {
		add("Rechenzeit", live.Seconds(time.Duration(ans.Seconds*float64(time.Second))))
	}
	if ans.PromptTokens > 0 {
		add("Tokens gelesen", fmt.Sprintf("%d, davon %d schon zwischengespeichert", ans.PromptTokens, ans.CachedTokens))
	}
	if ans.Tokens > 0 {
		add("Tokens geschrieben", fmt.Sprint(ans.Tokens))
	}
	if d := s.filterDevice(); d != nil {
		add("Gerät", strings.TrimSpace(d["build"]+" "+d["name"]))
	}
	if job.status == "done" {
		add("Ergebnis", fmt.Sprintf("%d Bedingungen, %d nicht verstanden, %d verworfen",
			len(job.result.Conditions), len(job.result.NotUnderstood), len(job.result.Dropped)))
	}
	return facts
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package llm runs a generative language model on this computer: llama.cpp's server program as a
// child process of the app, on 127.0.0.1 and a port of its own, started on the first question and
// ended after a quarter of an hour without one. Moliri's filter in plain words asks it
// (docs/filter-api.md).
//
// A model on a laptop's processor reads a long prompt slowly (Qwen 2.5 7B: some minutes for a
// field catalog of 3000 tokens), so the server keeps what it read: in memory for the prompts of
// the last questions, and on disk under a key the caller gives (the catalog's), so that the next
// question with the same beginning reads only what differs, also after a restart of the app.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moliri-app/transcribe/proc"
)

// ErrNotInstalled is returned while the server program or the model is missing.
var ErrNotInstalled = errors.New("llm: the language model is not downloaded")

// ErrTooLong is returned when the prompt does not fit into the context.
var ErrTooLong = errors.New("llm: the prompt is longer than the model's context")

// Server is llama.cpp's server with one model.
type Server struct {
	// Program is llama-server, Model the model's (first) GGUF file.
	Program, Model string
	// Builds lists llama.cpp's builds to run the model with, best first (nil: Program on the
	// processor). A build that fails to start, or ends during a question, is passed over for the
	// next one until the app restarts: a graphics card that fails falls back to the processor.
	Builds func(ctx context.Context) ([]Build, error)
	// CacheDir keeps what the model read under the callers' keys (empty: nothing on disk).
	CacheDir string
	// Installed reports whether program and model are complete (nil: both files exist).
	Installed func() bool
	// Context is the most tokens of prompt and answer together (0: 16384).
	Context int
	// Threads bounds the cores the model uses (0: llama.cpp's choice).
	Threads int
	// Idle is how long the model stays in memory without a question (0: 15 minutes).
	Idle time.Duration
	// KeepCaches is how many prompts stay on disk (0: 6).
	KeepCaches int

	run sync.Mutex // one question at a time

	mu      sync.Mutex
	cmd     *exec.Cmd
	exited  chan struct{}
	base    string // http://127.0.0.1:<port>
	log     *tail
	timer   *time.Timer
	slotKey string          // whose prompt the slot holds
	inRAM   map[string]bool // keys asked since the start: llama.cpp keeps their prompts in memory
	build   string          // the build running now
	broken  map[string]bool // builds that failed since the app started
}

// Build is one of llama.cpp's builds: its server program and the arguments that put the model
// on its device.
type Build struct {
	// Name says which build: "cuda13", "vulkan", "cpu".
	Name    string
	Program string
	// Args choose the device, as "-ngl 0" for the processor.
	Args []string
}

// cpuArgs keep the model on the processor.
var cpuArgs = []string{"-ngl", "0"}

// Message is one turn of the conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is one question.
type Request struct {
	Messages []Message
	// JSON holds the answer to a JSON object.
	JSON bool
	// MaxTokens bounds the answer (0: 600).
	MaxTokens int
	// CacheKey names the beginning of the prompt that the next questions share (a hash of it):
	// what the model read is kept on disk under it and read back on the next question with
	// the same key. Empty: nothing is kept.
	CacheKey string
}

// Progress is how far a question got.
type Progress struct {
	// Stage: "loading" (the model loads), "reading" (the prompt), "writing" (the answer).
	Stage string `json:"stage"`
	// Done and Total: tokens of the prompt read (reading), tokens written (writing, no Total).
	Done  int `json:"done"`
	Total int `json:"total"`
	// Cached: tokens of the prompt that were read before and need not be read again.
	Cached int `json:"cached"`
}

// Answer is the model's reply.
type Answer struct {
	Content string
	// PromptTokens of which CachedTokens were read before; Tokens written.
	PromptTokens, CachedTokens, Tokens int
	// Seconds the question took, loading the model included.
	Seconds float64
}

func (s *Server) installed() bool {
	if s.Installed != nil {
		return s.Installed()
	}
	for _, p := range []string{s.Program, s.Model} {
		if _, err := os.Stat(p); err != nil {
			return false
		}
	}
	return true
}

// Ready reports whether program and model are there.
func (s *Server) Ready() bool { return s.installed() }

// Device names the build the model runs on now ("cuda13", "vulkan", "cpu"), empty while it is
// not loaded.
func (s *Server) Device() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.base == "" {
		return ""
	}
	return s.build
}

// Loaded reports whether the model is in memory now.
func (s *Server) Loaded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cmd != nil && s.base != ""
}

func (s *Server) idle() time.Duration {
	if s.Idle > 0 {
		return s.Idle
	}
	return 15 * time.Minute
}

// Warm loads the model so that the next question need not wait for it.
func (s *Server) Warm(ctx context.Context) error {
	_, err := s.start(ctx)
	return err
}

// start returns the address of the running server, starting it first.
func (s *Server) start(ctx context.Context) (string, error) {
	for {
		s.mu.Lock()
		if s.base != "" {
			s.timer.Reset(s.idle())
			base := s.base
			s.mu.Unlock()
			return base, nil
		}
		if s.cmd != nil {
			// Another caller starts it: wait for it.
			exited := s.exited
			s.mu.Unlock()
			if err := s.awaitStart(ctx, exited); err != nil {
				return "", err
			}
			continue
		}
		s.mu.Unlock()
		if !s.installed() {
			return "", ErrNotInstalled
		}
		builds, err := s.builds(ctx)
		if err != nil {
			return "", err
		}
		b := builds[0]
		base, err := s.launch(ctx, b)
		if errors.Is(err, errBusy) {
			continue
		}
		if err != nil && ctx.Err() == nil && len(builds) > 1 {
			// The graphics card failed: the next build takes over.
			log.Printf("llm: %s failed, trying %s: %v", b.Name, builds[1].Name, err)
			s.mu.Lock()
			s.broken[b.Name] = true
			s.mu.Unlock()
			continue
		}
		return base, err
	}
}

// awaitStart waits until another caller's start is through, either way.
func (s *Server) awaitStart(ctx context.Context, exited chan struct{}) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-exited:
			return nil
		case <-time.After(200 * time.Millisecond):
		}
		if s.Loaded() {
			return nil
		}
	}
}

// builds returns the builds still to try, best first; the last one is never passed over.
func (s *Server) builds(ctx context.Context) ([]Build, error) {
	all := []Build{{Name: "cpu", Program: s.Program, Args: cpuArgs}}
	if s.Builds != nil {
		var err error
		if all, err = s.Builds(ctx); err != nil {
			return nil, err
		}
		if len(all) == 0 {
			return nil, ErrNotInstalled
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken == nil {
		s.broken = map[string]bool{}
	}
	var out []Build
	for i, b := range all {
		if !s.broken[b.Name] || (i == len(all)-1 && len(out) == 0) {
			out = append(out, b)
		}
	}
	return out, nil
}

// errBusy: another caller started the server meanwhile.
var errBusy = errors.New("llm: started by another caller")

// launch starts build b and waits until it answers.
func (s *Server) launch(ctx context.Context, b Build) (string, error) {
	s.mu.Lock()
	if s.cmd != nil || s.base != "" {
		s.mu.Unlock()
		return "", errBusy
	}
	port, err := freePort()
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	n := s.Context
	if n <= 0 {
		n = 16384
	}
	args := []string{"-m", s.Model, "--host", "127.0.0.1", "--port", strconv.Itoa(port),
		"-c", strconv.Itoa(n), "-np", "1", "--no-webui", "--no-warmup",
		// Progress comes per batch: 512 tokens are some ten seconds of reading on a laptop.
		"-b", "512",
		// llama.cpp keeps the prompts of the last questions in memory: about 0.06 MB a token
		// for Qwen 2.5 7B, so 2 GB hold some ten field catalogs.
		"--cache-ram", "2048"}
	args = append(args, b.Args...)
	if s.Threads > 0 {
		args = append(args, "-t", strconv.Itoa(s.Threads))
	}
	if s.CacheDir != "" {
		if err := os.MkdirAll(s.CacheDir, 0o755); err == nil {
			args = append(args, "--slot-save-path", s.CacheDir)
		}
	}
	cmd := proc.Hide(exec.Command(b.Program, args...))
	cmd.Dir = filepath.Dir(b.Program)
	lines := &tail{}
	cmd.Stdout, cmd.Stderr = lines, lines
	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		return "", fmt.Errorf("llm: %w", err)
	}
	// Ended with the app, also when it crashes (Windows: a job object).
	bindToApp(cmd.Process)
	exited := make(chan struct{})
	s.cmd, s.exited, s.log, s.slotKey, s.inRAM, s.build = cmd, exited, lines, "", map[string]bool{}, b.Name
	go func() {
		cmd.Wait()
		close(exited)
		s.mu.Lock()
		if s.cmd == cmd {
			s.cmd, s.base = nil, ""
		}
		s.mu.Unlock()
	}()
	s.mu.Unlock()

	base := "http://127.0.0.1:" + strconv.Itoa(port)
	for {
		select {
		case <-ctx.Done():
			s.Stop()
			return "", ctx.Err()
		case <-exited:
			return "", s.failure()
		case <-time.After(250 * time.Millisecond):
		}
		if healthy(ctx, base) {
			break
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != cmd {
		return "", s.failureLocked(lines)
	}
	s.base = base
	if s.timer == nil {
		s.timer = time.AfterFunc(s.idle(), s.idleStop)
	} else {
		s.timer.Reset(s.idle())
	}
	return base, nil
}

// ended reports whether the server process ended, and then passes its build over from now on.
func (s *Server) ended() bool {
	s.mu.Lock()
	exited, name := s.exited, s.build
	s.mu.Unlock()
	if exited == nil {
		return false
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		return false
	}
	s.mu.Lock()
	s.broken[name] = true
	s.mu.Unlock()
	return true
}

// failure says why the server ended, in its own last words.
func (s *Server) failure() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failureLocked(s.log)
}

func (s *Server) failureLocked(log *tail) error {
	words := ""
	if log != nil {
		words = log.last(3)
	}
	if words == "" {
		return errors.New("llm: the language model server ended")
	}
	return errors.New("llm: the language model server ended: " + words)
}

// idleStop ends the server after Idle without a question, unless one runs.
func (s *Server) idleStop() {
	if !s.run.TryLock() {
		s.mu.Lock()
		s.timer.Reset(s.idle())
		s.mu.Unlock()
		return
	}
	defer s.run.Unlock()
	s.Stop()
}

// Stop ends the server and gives its memory back.
func (s *Server) Stop() {
	s.mu.Lock()
	cmd := s.cmd
	s.cmd, s.base = nil, ""
	s.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}

func healthy(ctx context.Context, base string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", base+"/health", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == http.StatusOK
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Chat asks the model. progress (may be nil) hears how far it got: loading, reading, writing.
// Questions run one after the other.
func (s *Server) Chat(ctx context.Context, req Request, progress func(Progress)) (Answer, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	s.run.Lock()
	defer s.run.Unlock()
	began := time.Now()
	if !s.Loaded() {
		progress(Progress{Stage: "loading"})
	}
	base, err := s.start(ctx)
	if err != nil {
		return Answer{}, err
	}
	s.restore(ctx, base, req.CacheKey)
	ans, err := s.complete(ctx, base, req, progress)
	if err != nil && ctx.Err() == nil && !errors.Is(err, ErrTooLong) && s.ended() {
		// The server fell over during the question (a graphics card out of memory or lost):
		// the next build answers it.
		log.Printf("llm: the server ended during a question, asking again: %v", err)
		progress(Progress{Stage: "loading"})
		if base, err = s.start(ctx); err != nil {
			return Answer{}, err
		}
		s.restore(ctx, base, req.CacheKey)
		ans, err = s.complete(ctx, base, req, progress)
	}
	if err != nil {
		return Answer{}, err
	}
	s.save(ctx, base, req.CacheKey)
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Reset(s.idle())
	}
	s.mu.Unlock()
	ans.Seconds = time.Since(began).Seconds()
	return ans, nil
}

func (s *Server) complete(ctx context.Context, base string, req Request, progress func(Progress)) (Answer, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 600
	}
	body := map[string]any{
		"messages":        req.Messages,
		"stream":          true,
		"return_progress": true,
		"cache_prompt":    true,
		"id_slot":         0,
		"temperature":     0,
		"max_tokens":      maxTokens,
	}
	if req.JSON {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	data, _ := json.Marshal(body)
	hreq, _ := http.NewRequestWithContext(ctx, "POST", base+"/v1/chat/completions", bytes.NewReader(data))
	hreq.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(hreq)
	if err != nil {
		if ctx.Err() != nil {
			return Answer{}, ctx.Err()
		}
		return Answer{}, fmt.Errorf("llm: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&e)
		if strings.Contains(e.Error.Type, "exceed_context") || strings.Contains(e.Error.Message, "context") {
			return Answer{}, fmt.Errorf("%w: %s", ErrTooLong, e.Error.Message)
		}
		return Answer{}, fmt.Errorf("llm: %s: %s", res.Status, e.Error.Message)
	}

	var ans Answer
	var content strings.Builder
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if line == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			PromptProgress *struct {
				Total     int `json:"total"`
				Cache     int `json:"cache"`
				Processed int `json:"processed"`
			} `json:"prompt_progress"`
			Timings *struct {
				CacheN    int `json:"cache_n"`
				PromptN   int `json:"prompt_n"`
				Predicted int `json:"predicted_n"`
			} `json:"timings"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(line), &chunk) != nil {
			continue
		}
		if chunk.Error != nil {
			return Answer{}, errors.New("llm: " + chunk.Error.Message)
		}
		if p := chunk.PromptProgress; p != nil {
			ans.PromptTokens, ans.CachedTokens = p.Total, p.Cache
			progress(Progress{Stage: "reading", Done: p.Processed, Total: p.Total, Cached: p.Cache})
		}
		for _, c := range chunk.Choices {
			if c.Delta.Content != "" {
				content.WriteString(c.Delta.Content)
				ans.Tokens++
				progress(Progress{Stage: "writing", Done: ans.Tokens})
			}
		}
		if t := chunk.Timings; t != nil {
			ans.CachedTokens = t.CacheN
			ans.PromptTokens = t.CacheN + t.PromptN
			if t.Predicted > 0 {
				ans.Tokens = t.Predicted
			}
		}
	}
	if err := sc.Err(); err != nil {
		if ctx.Err() != nil {
			return Answer{}, ctx.Err()
		}
		return Answer{}, fmt.Errorf("llm: %w", err)
	}
	ans.Content = content.String()
	return ans, nil
}

// Speed is how fast a build runs the model: tokens of prompt read and of answer written a second.
type Speed struct {
	Read  float64 `json:"read"`
	Write float64 `json:"write"`
}

// Seconds estimates how long questions take at this speed: what a first question about a
// project reads of its catalog (spread over the next ones, which read only the sentence) and
// writes.
func (sp Speed) Seconds() float64 {
	if sp.Read <= 0 || sp.Write <= 0 {
		return 0
	}
	return 1000/sp.Read + 120/sp.Write
}

// Measure starts build b with model, has it read a prompt of some 700 tokens and write 32, and
// says how fast it did both.
func Measure(ctx context.Context, b Build, model string) (Speed, error) {
	s := &Server{Program: b.Program, Model: model, Builds: func(context.Context) ([]Build, error) { return []Build{b}, nil }}
	defer s.Stop()
	base, err := s.start(ctx)
	if err != nil {
		return Speed{}, err
	}
	// The first batch on a graphics card also prepares its kernels: not part of the speed.
	if _, err := measureOnce(ctx, base, "Hallo", 1); err != nil {
		return Speed{}, err
	}
	var prompt strings.Builder
	for i := 1; prompt.Len() < 2800; i++ {
		fmt.Fprintf(&prompt, "- `Feld %d` „Feld %d“ · Text (`text`) · Beispiel: `Wert %d`\n", i, i, i*7)
	}
	return measureOnce(ctx, base, prompt.String(), 32)
}

func measureOnce(ctx context.Context, base, prompt string, n int) (Speed, error) {
	data, _ := json.Marshal(map[string]any{"prompt": prompt, "n_predict": n, "temperature": 0,
		"cache_prompt": false, "ignore_eos": true})
	req, _ := http.NewRequestWithContext(ctx, "POST", base+"/completion", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Speed{}, fmt.Errorf("llm: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Speed{}, fmt.Errorf("llm: measuring: %s", res.Status)
	}
	var out struct {
		Timings struct {
			PromptPerSecond    float64 `json:"prompt_per_second"`
			PredictedPerSecond float64 `json:"predicted_per_second"`
		} `json:"timings"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return Speed{}, fmt.Errorf("llm: measuring: %w", err)
	}
	return Speed{Read: out.Timings.PromptPerSecond, Write: out.Timings.PredictedPerSecond}, nil
}

// cacheFile is where the prompt under key is kept on disk: one per build, as a graphics card may
// keep it in another form than the processor.
func cacheFile(build, key string) string {
	if build == "" || build == "cpu" {
		return "prompt-" + key + ".bin"
	}
	return "prompt-" + key + "-" + build + ".bin"
}

// restore reads back what the model read for key, unless the server has it in memory already.
// Failing, the model just reads the prompt again.
func (s *Server) restore(ctx context.Context, base, key string) {
	if key == "" || s.CacheDir == "" {
		return
	}
	s.mu.Lock()
	skip := s.slotKey == key || s.inRAM[key]
	file := cacheFile(s.build, key)
	s.mu.Unlock()
	if skip {
		return
	}
	path := filepath.Join(s.CacheDir, file)
	if _, err := os.Stat(path); err != nil {
		return
	}
	if slotAction(ctx, base, "restore", file) == nil {
		now := time.Now()
		os.Chtimes(path, now, now) // kept longest when used most recently
		s.mu.Lock()
		s.slotKey = key
		s.mu.Unlock()
	}
}

// save keeps what the model read for key on disk, and forgets the oldest prompts beyond KeepCaches.
func (s *Server) save(ctx context.Context, base, key string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	s.slotKey = key
	s.inRAM[key] = true
	file := cacheFile(s.build, key)
	s.mu.Unlock()
	if s.CacheDir == "" || slotAction(ctx, base, "save", file) != nil {
		return
	}
	keep := s.KeepCaches
	if keep <= 0 {
		keep = 6
	}
	files, _ := filepath.Glob(filepath.Join(s.CacheDir, "prompt-*.bin"))
	if len(files) <= keep {
		return
	}
	mod := map[string]time.Time{}
	for _, f := range files {
		if st, err := os.Stat(f); err == nil {
			mod[f] = st.ModTime()
		}
	}
	sort.Slice(files, func(i, j int) bool { return mod[files[i]].After(mod[files[j]]) })
	for _, f := range files[keep:] {
		os.Remove(f)
	}
}

func slotAction(ctx context.Context, base, action, filename string) error {
	data, _ := json.Marshal(map[string]string{"filename": filename})
	req, _ := http.NewRequestWithContext(ctx, "POST", base+"/slots/0?action="+action, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("llm: slot %s: %s", action, res.Status)
	}
	return nil
}

// tail keeps the last lines the server wrote, to say why it ended.
type tail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > 16<<10 {
		t.buf = t.buf[len(t.buf)-8<<10:]
	}
	return len(p), nil
}

func (t *tail) last(n int) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(string(t.buf)), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.Join(lines, " · ")
}

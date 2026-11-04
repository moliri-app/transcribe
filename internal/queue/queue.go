// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/moliri-app/transcribe/fileutil"
	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/jobs"
)

const (
	Queued    = "queued"
	Running   = "running"
	Canceling = "canceling"
	Canceled  = "canceled"
	Done      = "done"
	Failed    = "error"

	LogLines = 40
)

var ErrUnknownJob = errors.New("unknown job")

// Runner must produce results/<stem>/_final.json; audioFile is a file name inside <data>/audio.
type Runner func(ctx context.Context, dataDir, audioFile string, o jobs.Options, p jobs.Progress) error

type Job struct {
	ID       string       `json:"id"`
	Audio    string       `json:"audio"`
	Stem     string       `json:"stem"`
	Options  jobs.Options `json:"options"`
	Status   string       `json:"status"`
	Progress float64      `json:"progress"`
	Step     *string      `json:"step"`
	Log      []string     `json:"log"`
	Created  float64      `json:"created"`
	Started  *float64     `json:"started"`
	Finished *float64     `json:"finished"`
	Error    *string      `json:"error"`
	Note     *string      `json:"note"`
}

func (j *Job) active() bool {
	return j.Status == Queued || j.Status == Running || j.Status == Canceling
}

type Queue struct {
	DataDir        string
	DefaultThreads int

	run    Runner
	mu     sync.Mutex
	jobs   []*Job
	cancel context.CancelFunc // of the running job
	wake   chan struct{}
	done   chan struct{}
}

func New(dataDir string, run Runner, defaultThreads int) *Queue {
	if defaultThreads <= 0 {
		defaultThreads = 4
	}
	q := &Queue{DataDir: dataDir, DefaultThreads: defaultThreads, run: run, wake: make(chan struct{}, 1)}
	q.load()
	return q
}

func now() float64 { return float64(time.Now().UnixNano()) / 1e9 }

func ptr[T any](v T) *T { return &v }

func (q *Queue) path() string { return filepath.Join(q.DataDir, "results", "_queue.json") }

func (q *Queue) load() {
	var f struct {
		Jobs []*Job `json:"jobs"`
	}
	if fileutil.ReadJSON(q.path(), &f) != nil {
		return
	}
	for _, j := range f.Jobs {
		switch j.Status {
		case Running:
			j.Status, j.Progress, j.Step = Queued, 0, nil
			j.Note = ptr("nach Neustart erneut eingereiht")
		case Canceling:
			j.Status, j.Finished = Canceled, ptr(now())
		}
		if j.Log == nil {
			j.Log = []string{}
		}
		q.jobs = append(q.jobs, j)
	}
	q.save()
}

// save must be called with mu held (or before the queue is shared).
func (q *Queue) save() {
	_ = os.MkdirAll(filepath.Dir(q.path()), 0o755)
	jobs := q.jobs
	if jobs == nil {
		jobs = []*Job{}
	}
	_ = fileutil.WriteJSON(q.path(), map[string]any{"jobs": jobs})
}

func (q *Queue) poke() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func newID() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Busy reports whether any job is queued or running.
func (q *Queue) Busy() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range q.jobs {
		if j.active() {
			return true
		}
	}
	return false
}

// List returns a snapshot of all jobs.
func (q *Queue) List() []Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Job, len(q.jobs))
	for i, j := range q.jobs {
		out[i] = *j
		out[i].Log = append([]string{}, j.Log...)
	}
	return out
}

func (q *Queue) find(id string) *Job {
	for _, j := range q.jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// Add queues files; active duplicates are skipped. It returns the number added.
func (q *Queue) Add(audio []string, raw map[string]any) (int, error) {
	o, err := jobs.CleanOptions(raw, q.DefaultThreads)
	if err != nil {
		return 0, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	added := 0
	for _, name := range audio {
		if name == "" || strings.ContainsAny(name, `/\`) || !media.IsMedia(name) {
			return added, jobs.JobError("unknown file: " + name)
		}
		if st, err := os.Stat(filepath.Join(media.AudioDir(q.DataDir), name)); err != nil || st.IsDir() {
			return added, jobs.JobError("unknown file: " + name)
		}
		dup := false
		for _, j := range q.jobs {
			if j.Audio == name && j.active() {
				dup = true
			}
		}
		if dup {
			continue
		}
		q.jobs = append(q.jobs, &Job{
			ID: newID(), Audio: name, Stem: media.Stem(name), Options: o,
			Status: Queued, Log: []string{}, Created: now(),
		})
		added++
	}
	q.save()
	if added > 0 {
		q.poke()
	}
	return added, nil
}

func (q *Queue) cancelLocked(j *Job) {
	switch j.Status {
	case Queued:
		j.Status, j.Finished = Canceled, ptr(now())
	case Running:
		j.Status = Canceling
		if q.cancel != nil {
			q.cancel()
		}
	}
}

func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	j := q.find(id)
	if j == nil {
		return ErrUnknownJob
	}
	q.cancelLocked(j)
	q.save()
	return nil
}

func (q *Queue) CancelAll() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range q.jobs {
		q.cancelLocked(j)
	}
	q.save()
}

func (q *Queue) Retry(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	j := q.find(id)
	if j == nil {
		return ErrUnknownJob
	}
	if j.active() {
		return jobs.JobError("job is still active")
	}
	j.Status, j.Progress, j.Step, j.Log = Queued, 0, nil, []string{}
	j.Error, j.Note, j.Started, j.Finished = nil, nil, nil, nil
	q.save()
	q.poke()
	return nil
}

func (q *Queue) Remove(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	j := q.find(id)
	if j == nil {
		return ErrUnknownJob
	}
	if j.Status == Running || j.Status == Canceling {
		return jobs.JobError("cancel the running job first")
	}
	for i, x := range q.jobs {
		if x == j {
			q.jobs = append(q.jobs[:i:i], q.jobs[i+1:]...)
			break
		}
	}
	q.save()
	return nil
}

type logKey struct{}

// Logf appends a line to the log of the job running under ctx.
func Logf(ctx context.Context, format string, a ...any) {
	if f, ok := ctx.Value(logKey{}).(func(string)); ok {
		f(fmt.Sprintf(format, a...))
	}
}

func (q *Queue) appendLog(j *Job, line string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	j.Log = append(j.Log, line)
	if len(j.Log) > LogLines {
		j.Log = j.Log[len(j.Log)-LogLines:]
	}
	q.save()
}

// Start runs the worker until ctx is cancelled; Wait blocks until it has stopped.
func (q *Queue) Start(ctx context.Context) {
	q.done = make(chan struct{})
	go q.loop(ctx)
}

func (q *Queue) Wait() {
	if q.done != nil {
		<-q.done
	}
}

func (q *Queue) loop(ctx context.Context) {
	defer close(q.done)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for ctx.Err() == nil {
		if !q.runNext(ctx) {
			select {
			case <-ctx.Done():
			case <-q.wake:
			case <-tick.C:
			}
		}
	}
}

func (q *Queue) runNext(parent context.Context) bool {
	q.mu.Lock()
	var j *Job
	for _, x := range q.jobs {
		if x.Status == Queued {
			j = x
			break
		}
	}
	if j == nil {
		q.mu.Unlock()
		return false
	}
	ctx, cancel := context.WithCancel(parent)
	q.cancel = cancel
	j.Status, j.Started, j.Progress, j.Step = Running, ptr(now()), 0, ptr("Start")
	j.Log, j.Error = []string{}, nil
	q.save()
	audio, o := j.Audio, j.Options
	q.mu.Unlock()
	defer cancel()

	ctx = context.WithValue(ctx, logKey{}, func(s string) { q.appendLog(j, s) })
	prog := func(step, total int, label string) {
		q.mu.Lock()
		defer q.mu.Unlock()
		if total > 0 {
			j.Progress = float64(step-1) / float64(total)
		}
		j.Step = ptr(label)
		q.save()
	}
	err := q.safeRun(ctx, audio, o, prog)

	q.mu.Lock()
	defer q.mu.Unlock()
	q.cancel = nil
	if parent.Err() != nil && j.Status == Running {
		return true // shutting down: stays "running", requeued on next start
	}
	j.Finished = ptr(now())
	switch {
	case j.Status == Canceling:
		j.Status = Canceled
	case err != nil:
		j.Status, j.Error = Failed, ptr(err.Error())
	default:
		j.Status, j.Progress, j.Step = Done, 1, nil
	}
	q.save()
	return true
}

func (q *Queue) safeRun(ctx context.Context, audio string, o jobs.Options, p jobs.Progress) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return q.run(ctx, q.DataDir, audio, o, p)
}

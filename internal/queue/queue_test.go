// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package queue

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/jobs"
)

func newData(t *testing.T, names ...string) string {
	t.Helper()
	d := t.TempDir()
	_ = os.MkdirAll(filepath.Join(d, "audio"), 0o755)
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(d, "audio", n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func status(q *Queue, id string) string {
	for _, j := range q.List() {
		if j.ID == id {
			return j.Status
		}
	}
	return ""
}

func waitFor(t *testing.T, q *Queue, id, want string) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if status(q, id) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s: status %q, want %q", id, status(q, id), want)
}

func TestLifecycle(t *testing.T) {
	d := newData(t, "a.mp3", "b.mp3", "c.mp3")
	block := make(chan struct{})
	fail := errors.New("boom")
	run := func(ctx context.Context, _, audio string, o jobs.Options, p jobs.Progress) error {
		p(2, 4, "Schritt")
		Logf(ctx, "läuft %s", audio)
		switch audio {
		case "a.mp3":
			return nil
		case "b.mp3":
			return fail
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-block:
			return nil
		}
	}
	q := New(d, run, 0)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	if _, err := q.Add([]string{"nope.mp3"}, nil); err == nil {
		t.Error("unknown file accepted")
	}
	if _, err := q.Add([]string{"a.mp3"}, map[string]any{"threads": 0}); err == nil {
		t.Error("threads 0 accepted")
	}
	n, err := q.Add([]string{"a.mp3", "b.mp3", "c.mp3"}, nil)
	if err != nil || n != 3 {
		t.Fatalf("add: %d %v", n, err)
	}
	if n, _ := q.Add([]string{"a.mp3"}, nil); n != 0 {
		t.Error("duplicate active job added")
	}
	ids := map[string]string{}
	for _, j := range q.List() {
		ids[j.Audio] = j.ID
	}
	q.Start(ctx)

	waitFor(t, q, ids["a.mp3"], Done)
	waitFor(t, q, ids["b.mp3"], Failed)
	waitFor(t, q, ids["c.mp3"], Running)
	// the job is running before its first progress report
	for i := 0; i < 400 && q.List()[2].Step != nil && *q.List()[2].Step != "Schritt"; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	for _, j := range q.List() {
		switch j.Audio {
		case "a.mp3":
			if j.Progress != 1 || j.Step != nil || len(j.Log) != 1 || j.Log[0] != "läuft a.mp3" {
				t.Errorf("done job: %+v", j)
			}
		case "b.mp3":
			if j.Error == nil || *j.Error != "boom" {
				t.Errorf("error job: %+v", j)
			}
		case "c.mp3":
			if j.Progress != 0.25 || *j.Step != "Schritt" {
				t.Errorf("progress: %v %v", j.Progress, j.Step)
			}
		}
	}

	if err := q.Remove(ids["c.mp3"]); err == nil {
		t.Error("removed running job")
	}
	if err := q.Retry(ids["c.mp3"]); err == nil {
		t.Error("retried active job")
	}
	if err := q.Cancel(ids["c.mp3"]); err != nil {
		t.Fatal(err)
	}
	waitFor(t, q, ids["c.mp3"], Canceled)

	if err := q.Retry(ids["b.mp3"]); err != nil {
		t.Fatal(err)
	}
	waitFor(t, q, ids["b.mp3"], Failed)
	if err := q.Remove(ids["a.mp3"]); err != nil {
		t.Fatal(err)
	}
	if err := q.Remove(ids["a.mp3"]); !errors.Is(err, ErrUnknownJob) {
		t.Errorf("second remove: %v", err)
	}
	if err := q.Cancel("zzz"); !errors.Is(err, ErrUnknownJob) {
		t.Errorf("cancel unknown: %v", err)
	}

	// persistence
	q2 := New(d, run, 0)
	if len(q2.List()) != 2 {
		t.Errorf("reloaded %d jobs", len(q2.List()))
	}
}

func TestCancelQueuedAndAll(t *testing.T) {
	d := newData(t, "a.mp3", "b.mp3")
	q := New(d, nil, 0) // never started
	q.Add([]string{"a.mp3", "b.mp3"}, nil)
	ids := q.List()
	if err := q.Cancel(ids[0].ID); err != nil || status(q, ids[0].ID) != Canceled {
		t.Error("queued job not canceled")
	}
	q.CancelAll()
	if status(q, ids[1].ID) != Canceled || q.List()[1].Finished == nil {
		t.Error("cancel-all failed")
	}
}

func TestRestartRecovery(t *testing.T) {
	d := newData(t, "a.mp3", "b.mp3")
	q := New(d, nil, 0)
	q.Add([]string{"a.mp3", "b.mp3"}, nil)
	q.mu.Lock()
	q.jobs[0].Status, q.jobs[0].Progress = Running, 0.5
	q.jobs[1].Status = Canceling
	q.save()
	q.mu.Unlock()

	q2 := New(d, nil, 0)
	l := q2.List()
	// still running after a restart: the app crashed on it, so it is not run again by itself
	if l[0].Status != Failed || l[0].Error == nil || *l[0].Error != Interrupted || l[0].Finished == nil {
		t.Errorf("running not marked as interrupted: %+v", l[0])
	}
	if l[1].Status != Canceled || l[1].Finished == nil {
		t.Errorf("canceling not canceled: %+v", l[1])
	}
}

func TestShutdownRequeues(t *testing.T) {
	d := newData(t, "a.mp3")
	started := make(chan struct{})
	q := New(d, func(ctx context.Context, _, _ string, _ jobs.Options, _ jobs.Progress) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}, 0)
	ctx, stop := context.WithCancel(context.Background())
	q.Add([]string{"a.mp3"}, nil)
	q.Start(ctx)
	<-started
	stop()
	q.Wait()
	if j := q.List()[0]; j.Status != Queued || j.Note == nil || *j.Note != "nach Neustart erneut eingereiht" {
		t.Errorf("after shutdown: %+v", j)
	}
	if s := New(d, nil, 0).List()[0].Status; s != Queued {
		t.Errorf("after restart: %s", s)
	}
}

// A paused queue holds its running job where it is and starts no other, also after a restart;
// a live transcription holds it the same way until it ends.
func TestPause(t *testing.T) {
	d := newData(t, "a.mp3", "b.mp3")
	var steps atomic.Int32
	finish := make(chan struct{})
	run := func(ctx context.Context, _, audio string, o jobs.Options, p jobs.Progress) error {
		if audio == "b.mp3" {
			return nil
		}
		for {
			if err := jobs.Hold(ctx); err != nil {
				return err
			}
			steps.Add(1)
			select {
			case <-finish:
				return nil
			case <-time.After(time.Millisecond):
			}
		}
	}
	q := New(d, run, 0)
	ctx, stop := context.WithCancel(context.Background())
	q.Start(ctx)
	if _, err := q.Add([]string{"a.mp3", "b.mp3"}, nil); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, j := range q.List() {
		ids[j.Audio] = j.ID
	}
	waitFor(t, q, ids["a.mp3"], Running)

	q.Pause()
	waitFor(t, q, ids["a.mp3"], Paused)
	time.Sleep(20 * time.Millisecond)
	n := steps.Load()
	time.Sleep(50 * time.Millisecond)
	if steps.Load() != n {
		t.Error("the paused job went on working")
	}
	if user, live := q.Paused(); !user || live {
		t.Errorf("paused: user %v, live %v", user, live)
	}

	release := q.Hold()
	q.Resume()
	time.Sleep(50 * time.Millisecond)
	if status(q, ids["a.mp3"]) != Paused || steps.Load() != n {
		t.Error("the job went on while a live transcription held the queue")
	}
	release()
	release() // a second release changes nothing
	waitFor(t, q, ids["a.mp3"], Running)
	if steps.Load() == n {
		time.Sleep(50 * time.Millisecond)
		if steps.Load() == n {
			t.Error("the job did not go on")
		}
	}

	// shut down while paused: the job waits again after the restart, the queue still paused
	q.Pause()
	waitFor(t, q, ids["a.mp3"], Paused)
	stop()
	q.Wait()
	if got := status(q, ids["a.mp3"]); got != Queued {
		t.Fatalf("after shutdown: %q", got)
	}
	q2 := New(d, run, 0)
	ctx2, stop2 := context.WithCancel(context.Background())
	defer stop2()
	q2.Start(ctx2)
	time.Sleep(50 * time.Millisecond)
	if got := status(q2, ids["a.mp3"]); got != Queued {
		t.Errorf("after the restart the paused queue started its job: %q", got)
	}
	q2.Resume()
	waitFor(t, q2, ids["a.mp3"], Running)
	close(finish)
	waitFor(t, q2, ids["a.mp3"], Done)
	waitFor(t, q2, ids["b.mp3"], Done)
}

func TestCancelPaused(t *testing.T) {
	d := newData(t, "a.mp3")
	run := func(ctx context.Context, _, audio string, o jobs.Options, p jobs.Progress) error {
		for {
			if err := jobs.Hold(ctx); err != nil {
				return err
			}
			time.Sleep(time.Millisecond)
		}
	}
	q := New(d, run, 0)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	q.Start(ctx)
	if _, err := q.Add([]string{"a.mp3"}, nil); err != nil {
		t.Fatal(err)
	}
	id := q.List()[0].ID
	waitFor(t, q, id, Running)
	release := q.Hold()
	defer release()
	waitFor(t, q, id, Paused)
	if err := q.Remove(id); err == nil {
		t.Error("a paused job was removed")
	}
	if err := q.Cancel(id); err != nil {
		t.Fatal(err)
	}
	waitFor(t, q, id, Canceled)
}

// TestBackgroundPriority: the app's priority is lowered only while a job works, not while a live
// transcription holds it, and given back when it ends.
func TestBackgroundPriority(t *testing.T) {
	d := newData(t, "a.mp3")
	finish := make(chan struct{})
	run := func(ctx context.Context, _, _ string, _ jobs.Options, _ jobs.Progress) error {
		for {
			if err := jobs.Hold(ctx); err != nil {
				return err
			}
			select {
			case <-finish:
				return nil
			case <-time.After(time.Millisecond):
			}
		}
	}
	q := New(d, run, 0)
	var low atomic.Bool
	var calls atomic.Int32
	q.Background = func(on bool) { low.Store(on); calls.Add(1) }
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	q.Start(ctx)
	id := mustAdd(t, q, "a.mp3")
	waitFor(t, q, id, Running)
	if !low.Load() {
		t.Error("a running job left the priority normal")
	}
	release := q.Hold()
	if low.Load() {
		t.Error("the priority stayed low while a live transcription held the queue")
	}
	release()
	if !low.Load() {
		t.Error("the priority did not go down again after the hold")
	}
	close(finish)
	waitFor(t, q, id, Done)
	if low.Load() {
		t.Error("the priority stayed low after the job")
	}
	if n := calls.Load(); n != 4 {
		t.Errorf("Background called %d times, want 4 (only on changes)", n)
	}
}

func mustAdd(t *testing.T, q *Queue, audio string) string {
	t.Helper()
	if _, err := q.Add([]string{audio}, nil); err != nil {
		t.Fatal(err)
	}
	for _, j := range q.List() {
		if j.Audio == audio {
			return j.ID
		}
	}
	t.Fatal("job not found")
	return ""
}

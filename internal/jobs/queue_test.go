// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package jobs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	run := func(ctx context.Context, _, audio string, o Options, p Progress) error {
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
	if l[0].Status != Queued || l[0].Progress != 0 || l[0].Note == nil || *l[0].Note != "nach Neustart erneut eingereiht" {
		t.Errorf("running not requeued: %+v", l[0])
	}
	if l[1].Status != Canceled || l[1].Finished == nil {
		t.Errorf("canceling not canceled: %+v", l[1])
	}
}

func TestShutdownKeepsRunning(t *testing.T) {
	d := newData(t, "a.mp3")
	started := make(chan struct{})
	q := New(d, func(ctx context.Context, _, _ string, _ Options, _ Progress) error {
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
	if s := q.List()[0].Status; s != Running {
		t.Errorf("status after shutdown: %s", s)
	}
	if s := New(d, nil, 0).List()[0].Status; s != Queued {
		t.Errorf("after restart: %s", s)
	}
}

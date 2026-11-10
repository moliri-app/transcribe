// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/jobs"
)

func TestInstance(t *testing.T) {
	s, d := newServer(t)
	var got struct{ App, DataDir string }
	_ = json.Unmarshal(do(s, "GET", "/api/instance", "", nil).Body.Bytes(), &got)
	if abs, _ := filepath.Abs(d); got.App != "transcribe" || got.DataDir != abs {
		t.Errorf("got %+v, want the data dir %s", got, abs)
	}
}

// Only the background preference: switching autostart would write the user's registry.
func TestBackgroundPref(t *testing.T) {
	s, _ := newServer(t)
	get := func() (st struct{ Background bool }) {
		_ = json.Unmarshal(do(s, "GET", "/api/background", "", nil).Body.Bytes(), &st)
		return st
	}
	if get().Background {
		t.Fatal("keeps running in the background before it was turned on")
	}
	if w := do(s, "POST", "/api/background", `{"background":true}`, map[string]string{"Origin": "https://evil.example"}); w.Code != 403 {
		t.Errorf("foreign page: got %d, want 403", w.Code)
	}
	if w := do(s, "POST", "/api/background", `{"background":true}`, nil); w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	if !get().Background {
		t.Error("the background preference was not stored")
	}
}

func TestAccessListsRequestsAndDeclinesTheHint(t *testing.T) {
	run := func(context.Context, string, string, jobs.Options, jobs.Progress) error { return nil }
	noLive := func(context.Context, string) (live.Stream, error) { return nil, errors.New("no live in tests") }
	s := New(Config{DataDir: t.TempDir(), Runner: run, Live: noLive})
	s.Live.Refused("https://moliri.example", "Moliri")
	type state struct {
		Blocked []struct{ Origin, Client string }
		Hint    bool
	}
	get := func() (st state) {
		_ = json.Unmarshal(do(s, "GET", "/api/access", "", nil).Body.Bytes(), &st)
		return st
	}
	if b := get().Blocked; len(b) != 1 || b[0].Origin != "https://moliri.example" || b[0].Client != "Moliri" {
		t.Errorf("blocked: %+v", b)
	}
	if w := do(s, "POST", "/api/access", `{"hint":false}`, map[string]string{"Origin": "https://evil.example"}); w.Code != 403 {
		t.Errorf("foreign page: got %d, want 403", w.Code)
	}
	if w := do(s, "POST", "/api/access", `{"hint":false}`, nil); w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	if get().Hint {
		t.Error("the hint stays after it was declined")
	}
}

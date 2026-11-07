// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package llm

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestMain lets the test binary stand in for llama-server: with LLM_FAKE_SERVER set, it answers
// /health and every question with "ok", or ends at once when its arguments hold --fake-crash.
func TestMain(m *testing.M) {
	if os.Getenv("LLM_FAKE_SERVER") == "" {
		os.Exit(m.Run())
	}
	port := ""
	for i, a := range os.Args {
		if a == "--port" && i+1 < len(os.Args) {
			port = os.Args[i+1]
		}
		if a == "--fake-crash" {
			fmt.Fprintln(os.Stderr, "ggml_vulkan: device lost")
			os.Exit(1)
		}
	}
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {})
	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")
	})
	http.ListenAndServe("127.0.0.1:"+port, nil)
}

// TestFallBack: a build that fails to start (a missing program, a server that ends while it
// loads) is passed over for the next one, and stays passed over; the last build is tried even
// when it failed before.
func TestFallBack(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(t.TempDir(), "model.gguf")
	os.WriteFile(model, nil, 0o644)
	t.Setenv("LLM_FAKE_SERVER", "1")
	s := &Server{Program: exe, Model: model, Builds: func(context.Context) ([]Build, error) {
		return []Build{
			{Name: "cuda13", Program: filepath.Join(t.TempDir(), "missing.exe")},
			{Name: "vulkan", Program: exe, Args: []string{"--fake-crash"}},
			{Name: "cpu", Program: exe, Args: cpuArgs},
		}, nil
	}}
	defer s.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ans, err := s.Chat(ctx, Request{Messages: []Message{{Role: "user", Content: "?"}}}, nil)
	if err != nil || ans.Content != "ok" {
		t.Fatalf("answer %q, %v", ans.Content, err)
	}
	if s.Device() != "cpu" {
		t.Errorf("runs on %q, want cpu", s.Device())
	}
	s.Stop()
	time.Sleep(100 * time.Millisecond)
	bs, _ := s.builds(ctx)
	if names := buildNames(bs); !slices.Equal(names, []string{"cpu"}) {
		t.Errorf("after the failure: %v", names)
	}
	s.broken["cpu"] = true
	if bs, _ := s.builds(ctx); len(bs) != 1 || bs[0].Name != "cpu" {
		t.Errorf("the last build is passed over: %v", buildNames(bs))
	}
}

func buildNames(bs []Build) []string {
	var out []string
	for _, b := range bs {
		out = append(out, b.Name)
	}
	return out
}

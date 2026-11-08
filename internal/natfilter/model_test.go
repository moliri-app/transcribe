// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package natfilter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/internal/llm"
)

// TestWithModel asks the real model, when TRANSCRIBE_LLM_DIR names a folder with llama/ (the
// server program) and the model's GGUF files, and TRANSCRIBE_FILTER_CATALOG a catalog text
// (Moliri's), with TRANSCRIBE_FILTER_FIELDS the field catalog as JSON (entries with `value`).
// It asks twice, then once more after a restart, and logs the times: the second and third
// question should read only the sentence. TRANSCRIBE_LLM_BUILD names another build's folder
// beside llama/ (llama-cuda13, llama-vulkan) and TRANSCRIBE_LLM_ARGS its device arguments
// (default "-ngl 0").
func TestWithModel(t *testing.T) {
	dir, catalogFile := os.Getenv("TRANSCRIBE_LLM_DIR"), os.Getenv("TRANSCRIBE_FILTER_CATALOG")
	if dir == "" || catalogFile == "" {
		t.Skip("TRANSCRIBE_LLM_DIR and TRANSCRIBE_FILTER_CATALOG not set")
	}
	catalog, err := os.ReadFile(catalogFile)
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct{ Value string }
	if data, err := os.ReadFile(os.Getenv("TRANSCRIBE_FILTER_FIELDS")); err == nil {
		json.Unmarshal(data, &entries)
	}
	var fields []string
	for _, e := range entries {
		fields = append(fields, e.Value)
	}
	models, _ := filepath.Glob(filepath.Join(dir, "*00001-of-*.gguf"))
	if len(models) == 0 {
		t.Fatal("no model in", dir)
	}
	build := llm.Build{Name: "test", Program: filepath.Join(dir, "llama", "llama-server.exe"), Args: []string{"-ngl", "0"}}
	if b := os.Getenv("TRANSCRIBE_LLM_BUILD"); b != "" {
		build.Program = filepath.Join(dir, b, "llama-server.exe")
	}
	if a := os.Getenv("TRANSCRIBE_LLM_ARGS"); a != "" {
		build.Args = strings.Fields(a)
	}
	newServer := func() *llm.Server {
		return &llm.Server{Program: build.Program, Model: models[0], CacheDir: filepath.Join(dir, "cache"),
			Builds: func(context.Context) ([]llm.Build, error) { return []llm.Build{build}, nil }}
	}
	key := sha256.Sum256([]byte(Prefix(string(catalog))))
	ask := func(s *llm.Server, text string) {
		req := llm.Request{JSON: true, MaxTokens: MaxTokens, CacheKey: hex.EncodeToString(key[:12])}
		for _, m := range Messages(text, string(catalog)) {
			req.Messages = append(req.Messages, llm.Message{Role: m.Role, Content: m.Content})
		}
		var last llm.Progress
		began := time.Now()
		ans, err := s.Chat(context.Background(), req, func(p llm.Progress) {
			if p.Stage != last.Stage || p.Stage == "reading" {
				t.Logf("%6.1f s %s %d/%d (cached %d)", time.Since(began).Seconds(), p.Stage, p.Done, p.Total, p.Cached)
			}
			last = p
		})
		if err != nil {
			t.Fatal(err)
		}
		res, ok := Parse(ans.Content, fields)
		t.Logf("%q: %.1f s, prompt %d (cached %d), %d tokens written; %v %+v", text, ans.Seconds, ans.PromptTokens, ans.CachedTokens, ans.Tokens, ok, res)
	}
	s := newServer()
	ask(s, "Gemälde nach 1900 ohne Standort")
	ask(s, "Skulpturen aus Japan")
	s.Stop()
	time.Sleep(time.Second)
	s = newServer()
	defer s.Stop()
	ask(s, "Objekte von Frans Hals mit Bild")
}

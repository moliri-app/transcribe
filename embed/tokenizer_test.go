// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package embed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// modelDir is a folder holding EmbeddingGemma's files (TRANSCRIBE_EMBED_MODEL), else the test skips.
func modelDir(t *testing.T) string {
	dir := os.Getenv("TRANSCRIBE_EMBED_MODEL")
	if dir == "" {
		t.Skip("TRANSCRIBE_EMBED_MODEL not set (a folder with EmbeddingGemma's tokenizer.json and onnx/)")
	}
	return dir
}

// TestTokenizerMatchesBrowser compares the tokens with those of transformers.js (testdata/tokens.json,
// written by @huggingface/tokenizers 0.2.0 as Moliri's browser model uses it).
func TestTokenizerMatchesBrowser(t *testing.T) {
	tok, err := LoadTokenizer(filepath.Join(modelDir(t), "tokenizer.json"), maxTokens)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("testdata/tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Text string  `json:"text"`
		IDs  []int64 `json:"ids"`
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if got := tok.Encode(c.Text); !slices.Equal(got, c.IDs) {
			t.Errorf("%q:\n got %v\nwant %v", c.Text, got, c.IDs)
		}
	}
}

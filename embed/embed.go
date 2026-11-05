// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package embed turns texts into sentence embeddings with EmbeddingGemma (300M, 8-bit, the ONNX
// export of onnx-community), on the onnxruntime of speech recognition, on the CPU. Moliri finds
// fields by meaning with it (docs/embed-api.md); it runs the same model in the browser with
// transformers.js, and this package computes the vectors the same way (mean of the last hidden
// state over the tokens, normalized), so vectors of either place can be compared with each other.
package embed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/moliri-app/transcribe/ort"
)

const (
	// ModelID names the model as Hugging Face (and Moliri) does.
	ModelID = "onnx-community/embeddinggemma-300m-ONNX"
	// Dtype is the precision of its weights, as transformers.js names it.
	Dtype = "q8"
	// Dim is the length of a vector.
	Dim = 768

	// EmbeddingGemma's prompts for retrieval: the query and the documents get different ones.
	QueryPrefix    = "task: search result | query: "
	DocumentPrefix = "title: none | text: "

	maxTokens = 2048
)

// Files are the model's files below Model.Dir.
var Files = []string{"tokenizer.json", "onnx/model_quantized.onnx", "onnx/model_quantized.onnx_data"}

// Kind says which prompt a text gets.
type Kind string

const (
	Query    Kind = "query"
	Document Kind = "document"
)

// ErrNotInstalled is returned while the model's files are missing.
var ErrNotInstalled = errors.New("embed: the model is not downloaded")

// Model is EmbeddingGemma in Dir, loaded on first use and given back after Idle without one.
type Model struct {
	Dir string
	// Runtime gives the folder of the onnxruntime library, downloading it first when it is missing.
	Runtime func(ctx context.Context) (string, error)
	// Threads bounds the cores one run uses (0: the runtime's default).
	Threads int
	// Idle is how long the model stays in memory without a request (0: 15 minutes).
	Idle time.Duration

	mu    sync.Mutex
	tok   *Tokenizer
	sess  *ort.Session
	timer *time.Timer
}

// Installed reports whether the model's files are all there.
func (m *Model) Installed() bool {
	for _, f := range Files {
		if st, err := os.Stat(filepath.Join(m.Dir, filepath.FromSlash(f))); err != nil || st.Size() == 0 {
			return false
		}
	}
	return true
}

// Loaded reports whether the model is in memory now.
func (m *Model) Loaded() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sess != nil
}

func (m *Model) load(ctx context.Context) (*Tokenizer, *ort.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idle := m.Idle
	if idle <= 0 {
		idle = 15 * time.Minute
	}
	if m.sess != nil {
		m.timer.Reset(idle)
		return m.tok, m.sess, nil
	}
	if !m.Installed() {
		return nil, nil, ErrNotInstalled
	}
	tok, err := LoadTokenizer(filepath.Join(m.Dir, "tokenizer.json"), maxTokens)
	if err != nil {
		return nil, nil, err
	}
	dir, err := m.Runtime(ctx)
	if err != nil {
		return nil, nil, err
	}
	rt, err := ort.Load(dir)
	if err != nil {
		return nil, nil, err
	}
	// On the CPU always, as in the browser. The export keeps its weights in 8 bits and turns them
	// into floats in every run, about a second each on a laptop, whatever the text; turning them
	// once at loading (QDQ handling off, so that constant folding takes them) makes a query take
	// a tenth of that, for 1.4 GB of memory instead of 0.5 GB and 10 s of loading instead of 2.
	// The vectors stay the same: the floats are the ones each run would compute.
	sess, err := rt.NewSessionWith(filepath.Join(m.Dir, "onnx", "model_quantized.onnx"), m.Threads, false,
		map[string]string{"session.disable_quant_qdq": "1"})
	if err != nil {
		return nil, nil, err
	}
	m.tok, m.sess = tok, sess
	if m.timer == nil {
		m.timer = time.AfterFunc(idle, m.unload)
	} else {
		m.timer.Reset(idle)
	}
	return tok, sess, nil
}

func (m *Model) unload() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sess != nil {
		m.sess.Close()
	}
	m.tok, m.sess = nil, nil
}

// Close gives the model's memory back.
func (m *Model) Close() {
	m.mu.Lock()
	if m.timer != nil {
		m.timer.Stop()
	}
	m.mu.Unlock()
	m.unload()
}

// Warm loads the model ahead of the first request.
func (m *Model) Warm(ctx context.Context) error {
	_, _, err := m.load(ctx)
	return err
}

// batch is how many texts run through the model at once: on the CPU a batch takes little longer
// than one text. Moliri's browser model reads a catalog in batches of the same size.
const batch = 8

// Embed returns one normalized vector of Dim values per text, each text with the prompt of kind.
// Texts run in batches, padded to the longest; the 8-bit model quantizes a batch as a whole, which
// moves a vector by less than 1e-6 in cosine (as in the browser).
func (m *Model) Embed(ctx context.Context, texts []string, kind Kind) ([][]float32, error) {
	prefix := DocumentPrefix
	switch kind {
	case Query:
		prefix = QueryPrefix
	case Document:
	default:
		return nil, fmt.Errorf("embed: unknown kind %q (query or document)", kind)
	}
	tok, sess, err := m.load(ctx)
	if err != nil {
		return nil, err
	}
	out := make([][]float32, 0, len(texts))
	for i := 0; i < len(texts); i += batch {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		seqs := make([][]int64, 0, batch)
		for _, t := range texts[i:min(i+batch, len(texts))] {
			seqs = append(seqs, tok.Encode(prefix+t))
		}
		vecs, err := run(sess, seqs)
		if err != nil {
			return nil, err
		}
		out = append(out, vecs...)
	}
	return out, nil
}

// run puts the token sequences through the model as one batch, padded on the right with <pad>
// (id 0) and masked out, and returns the mean of each one's last hidden state, normalized.
func run(sess *ort.Session, seqs [][]int64) ([][]float32, error) {
	n := 0
	for _, s := range seqs {
		n = max(n, len(s))
	}
	b := len(seqs)
	ids := make([]int64, b*n)
	mask := make([]int64, b*n)
	for i, s := range seqs {
		copy(ids[i*n:], s)
		for j := range s {
			mask[i*n+j] = 1
		}
	}
	hidden, shape, err := sess.RunNamed([]ort.Input{
		{Name: "input_ids", Int64: ids, Shape: []int64{int64(b), int64(n)}},
		{Name: "attention_mask", Int64: mask, Shape: []int64{int64(b), int64(n)}},
	}, "last_hidden_state")
	if err != nil {
		return nil, err
	}
	if len(shape) != 3 || shape[0] != int64(b) || shape[1] != int64(n) || shape[2] != Dim {
		return nil, fmt.Errorf("embed: output of shape %v", shape)
	}
	out := make([][]float32, b)
	for i, s := range seqs {
		out[i] = meanNormalized(hidden[i*n*Dim:(i+1)*n*Dim], len(s))
	}
	return out, nil
}

// meanNormalized averages the first n rows of hidden (rows of Dim values; the rest is padding)
// and scales the mean to length 1, as transformers.js does with pooling "mean" and normalize.
func meanNormalized(hidden []float32, n int) []float32 {
	sum := make([]float64, Dim)
	for t := 0; t < n; t++ {
		for k, x := range hidden[t*Dim : (t+1)*Dim] {
			sum[k] += float64(x)
		}
	}
	var norm float64
	for k := range sum {
		sum[k] /= float64(n)
		norm += sum[k] * sum[k]
	}
	norm = math.Sqrt(norm)
	v := make([]float32, Dim)
	for k := range sum {
		if norm > 0 {
			v[k] = float32(sum[k] / norm)
		}
	}
	return v
}

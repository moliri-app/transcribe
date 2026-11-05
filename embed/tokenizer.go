// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package embed

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// Tokenizer is Gemma's tokenizer as its tokenizer.json describes it: BPE over the characters of the
// text with spaces as "▁", bytes for characters the vocabulary lacks, the added tokens (<bos>,
// <unused0>, …) matched as they are, and <bos> … <eos> around it. It follows transformers.js, which
// Moliri runs in the browser, so both give the same tokens and hence the same vectors.
type Tokenizer struct {
	vocab  map[string]int64
	ranks  map[[2]string]int
	added  *trie
	addedN map[string]int64
	unk    int64
	bos    int64
	eos    int64
	max    int // most tokens of a text, <bos> and <eos> included
}

type tokenizerJSON struct {
	AddedTokens []struct {
		ID      int64  `json:"id"`
		Content string `json:"content"`
	} `json:"added_tokens"`
	Normalizer struct {
		Type    string `json:"type"`
		Pattern struct {
			String string `json:"String"`
		} `json:"pattern"`
		Content string `json:"content"`
	} `json:"normalizer"`
	Model struct {
		Type         string           `json:"type"`
		Vocab        map[string]int64 `json:"vocab"`
		Merges       [][2]string      `json:"merges"`
		UnkToken     string           `json:"unk_token"`
		ByteFallback bool             `json:"byte_fallback"`
	} `json:"model"`
}

// LoadTokenizer reads a tokenizer.json of Gemma 3 (EmbeddingGemma's). maxTokens bounds a text's
// tokens (the model's 2048).
func LoadTokenizer(path string, maxTokens int) (*Tokenizer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var j tokenizerJSON
	if err := json.Unmarshal(b, &j); err != nil {
		return nil, fmt.Errorf("tokenizer: %w", err)
	}
	// What this tokenizer does is what Gemma's needs; another one would come out wrong.
	if j.Model.Type != "BPE" || !j.Model.ByteFallback || j.Normalizer.Type != "Replace" ||
		j.Normalizer.Pattern.String != " " || j.Normalizer.Content != "▁" {
		return nil, fmt.Errorf("tokenizer: %s is not Gemma's (BPE, byte fallback, spaces as ▁)", path)
	}
	t := &Tokenizer{vocab: j.Model.Vocab, ranks: make(map[[2]string]int, len(j.Model.Merges)),
		added: &trie{}, addedN: map[string]int64{}, max: maxTokens}
	for i, m := range j.Model.Merges {
		if _, ok := t.ranks[m]; !ok {
			t.ranks[m] = i
		}
	}
	for _, a := range j.AddedTokens {
		t.added.add(a.Content)
		t.addedN[a.Content] = a.ID
		t.vocab[a.Content] = a.ID
	}
	var ok [3]bool
	t.unk, ok[0] = t.vocab[j.Model.UnkToken]
	t.bos, ok[1] = t.vocab["<bos>"]
	t.eos, ok[2] = t.vocab["<eos>"]
	if !ok[0] || !ok[1] || !ok[2] {
		return nil, fmt.Errorf("tokenizer: %s lacks <unk>, <bos> or <eos>", path)
	}
	return t, nil
}

// Encode returns the token ids of text, <bos> first and <eos> last.
func (t *Tokenizer) Encode(text string) []int64 {
	ids := []int64{t.bos}
	for _, sec := range t.added.split(text) {
		if id, ok := t.addedN[sec]; ok {
			ids = append(ids, id)
			continue
		}
		// The normalizer turns spaces into ▁; the pre-tokenizer then splits at spaces, of which
		// there are none left, so the whole section is one word.
		for _, tok := range t.bpe(strings.ReplaceAll(sec, " ", "▁")) {
			if id, ok := t.vocab[tok]; ok {
				ids = append(ids, id)
				continue
			}
			bytes := make([]int64, 0, len(tok))
			for i := 0; i < len(tok); i++ {
				id, ok := t.vocab[fmt.Sprintf("<0x%02X>", tok[i])]
				if !ok {
					bytes = []int64{t.unk}
					break
				}
				bytes = append(bytes, id)
			}
			ids = append(ids, bytes...)
		}
	}
	if t.max > 1 && len(ids) >= t.max {
		ids = ids[:t.max-1]
	}
	return append(ids, t.eos)
}

// bpe merges the characters of word pair by pair, the pair of the lowest rank first and of equal
// ranks the leftmost.
func (t *Tokenizer) bpe(word string) []string {
	if word == "" {
		return nil
	}
	n := utf8.RuneCountInString(word)
	nodes := make([]bpeNode, 0, n)
	for i, r := range []rune(word) {
		nodes = append(nodes, bpeNode{tok: string(r), prev: i - 1, next: i + 1, pos: i})
	}
	nodes[n-1].next = -1
	q := &bpeQueue{}
	push := func(i int) {
		a := &nodes[i]
		if a.next < 0 {
			return
		}
		if r, ok := t.ranks[[2]string{a.tok, nodes[a.next].tok}]; ok {
			heap.Push(q, bpeItem{rank: r, pos: a.pos, node: i, ver: a.ver, right: nodes[a.next].ver})
		}
	}
	for i := range nodes {
		push(i)
	}
	for q.Len() > 0 {
		it := heap.Pop(q).(bpeItem)
		a := &nodes[it.node]
		if a.dead || a.ver != it.ver || a.next < 0 || nodes[a.next].dead || nodes[a.next].ver != it.right {
			continue
		}
		b := &nodes[a.next]
		a.tok += b.tok
		b.dead = true
		a.next = b.next
		if a.next >= 0 {
			nodes[a.next].prev = it.node
		}
		a.ver++
		if a.prev >= 0 {
			push(a.prev)
		}
		push(it.node)
	}
	out := make([]string, 0, n)
	for i := 0; i >= 0; i = nodes[i].next {
		out = append(out, nodes[i].tok)
	}
	return out
}

type bpeNode struct {
	tok        string
	prev, next int
	pos        int  // position of its first character, for ties
	ver        int  // raised on every change, so queued pairs that no longer hold are skipped
	dead       bool // merged into its left neighbour
}

type bpeItem struct {
	rank, pos, node int
	ver, right      int // versions of the left and the right node when queued
}

type bpeQueue []bpeItem

func (q bpeQueue) Len() int { return len(q) }
func (q bpeQueue) Less(i, j int) bool {
	if q[i].rank != q[j].rank {
		return q[i].rank < q[j].rank
	}
	return q[i].pos < q[j].pos
}
func (q bpeQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *bpeQueue) Push(x any)   { *q = append(*q, x.(bpeItem)) }
func (q *bpeQueue) Pop() any {
	old := *q
	it := old[len(old)-1]
	*q = old[:len(old)-1]
	return it
}

// trie finds the added tokens in a text, the longest at each place, from the left.
type trie struct {
	next map[rune]*trie
	end  string
}

func (t *trie) add(word string) {
	n := t
	for _, r := range word {
		if n.next == nil {
			n.next = map[rune]*trie{}
		}
		c := n.next[r]
		if c == nil {
			c = &trie{}
			n.next[r] = c
		}
		n = c
	}
	n.end = word
}

// split cuts text into the added tokens it holds and the stretches between them.
func (t *trie) split(text string) []string {
	var out []string
	start := 0
	for i := 0; i < len(text); {
		n, match := t, ""
		for j := i; j < len(text); {
			r, size := utf8.DecodeRuneInString(text[j:])
			if n = n.next[r]; n == nil {
				break
			}
			j += size
			if n.end != "" {
				match = n.end
			}
		}
		if match == "" {
			_, size := utf8.DecodeRuneInString(text[i:])
			i += size
			continue
		}
		if i > start {
			out = append(out, text[start:i])
		}
		out = append(out, match)
		i += len(match)
		start = i
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

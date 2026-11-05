// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package embed

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"
)

// TestEmbed runs the model on a few field names and queries. It needs the model
// (TRANSCRIBE_EMBED_MODEL) and an onnxruntime library (TRANSCRIBE_ORT_LIB: its folder, e.g.
// <data>/lib). With TRANSCRIBE_EMBED_OUT it writes the vectors there, to compare them with the
// browser's.
func TestEmbed(t *testing.T) {
	dir, lib := modelDir(t), os.Getenv("TRANSCRIBE_ORT_LIB")
	if lib == "" {
		t.Skip("TRANSCRIBE_ORT_LIB not set")
	}
	m := &Model{Dir: dir, Runtime: func(context.Context) (string, error) { return lib, nil }}
	defer m.Close()
	start := time.Now()
	if err := m.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Logf("loaded in %v", time.Since(start).Round(time.Millisecond))

	fields := []string{"Urheber", "Erwerbungsvermerk", "Inventarnummer", "Höhe · Breite · Tiefe"}
	start = time.Now()
	docs, err := m.Embed(context.Background(), fields, Document)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%v per field", (time.Since(start) / time.Duration(len(fields))).Round(time.Millisecond))
	queries := []string{"Künstler", "Ankauf"}
	start = time.Now()
	qs, err := m.Embed(context.Background(), queries, Query)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%v per query", (time.Since(start) / time.Duration(len(queries))).Round(time.Millisecond))

	for _, v := range append(docs, qs...) {
		var n float64
		for _, x := range v {
			n += float64(x) * float64(x)
		}
		if len(v) != Dim || math.Abs(n-1) > 1e-4 {
			t.Fatalf("vector of %d values, length² %v", len(v), n)
		}
	}
	// the meaning the field search is for: »Künstler« is closest to »Urheber«, »Ankauf« to the note of acquisition
	for qi, want := range []int{0, 1} {
		best, bestScore := -1, -2.0
		for di, d := range docs {
			if s := dot(qs[qi], d); s > bestScore {
				best, bestScore = di, s
			}
		}
		t.Logf("%s → %s (%.3f)", queries[qi], fields[best], bestScore)
		if best != want {
			t.Errorf("%s: closest is %s, want %s", queries[qi], fields[best], fields[want])
		}
	}
	if out := os.Getenv("TRANSCRIBE_EMBED_OUT"); out != "" {
		b, _ := json.Marshal(map[string]any{"fields": fields, "documents": docs, "queries": queries, "query_vectors": qs})
		if err := os.WriteFile(out, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func dot(a, b []float32) float64 {
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package memo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeptForTheNextStart(t *testing.T) {
	dir := t.TempDir()
	s := In(dir, "test")
	if In(dir, "test") != s {
		t.Fatal("the same name gives another store")
	}
	type v struct{ N int }
	s.Put("a", "1", v{7})
	var got v
	if !s.Get("a", "1", &got) || got.N != 7 {
		t.Fatalf("Get = %+v", got)
	}
	if s.Get("a", "2", &got) {
		t.Fatal("a value of another stamp was given")
	}
	Flush()
	if _, err := os.Stat(filepath.Join(dir, "cache", "test.json")); err != nil {
		t.Fatal("not written:", err)
	}
	// a new run reads the file
	next := &Store{path: filepath.Join(dir, "cache", "test.json")}
	got = v{}
	if !next.Get("a", "1", &got) || got.N != 7 {
		t.Fatalf("next run: Get = %+v", got)
	}
}

func TestFileStamp(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if FileStamp(p) != "" {
		t.Fatal("a missing file has a stamp")
	}
	_ = os.WriteFile(p, []byte("x"), 0o644)
	a := FileStamp(p)
	_ = os.WriteFile(p, []byte("xy"), 0o644)
	if a == "" || a == FileStamp(p) {
		t.Fatalf("stamp did not change: %q", a)
	}
}

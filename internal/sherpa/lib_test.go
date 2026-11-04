// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sherpa

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlaceholderLib(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, libFile()), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	err := load(dir)
	if err == nil || !strings.Contains(err.Error(), "Platzhalter") {
		t.Fatalf("want a placeholder message, got %v", err)
	}
}

func TestDamagedLib(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, libFile()), []byte("not a library"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := load(dir)
	if err == nil || strings.Contains(err.Error(), "%1") {
		t.Fatalf("want a readable message, got %v", err)
	}
	t.Log(err)
}

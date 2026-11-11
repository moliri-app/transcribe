// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package media

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moliri-app/transcribe/internal/prefs"
)

// The first listing after a start is the one the last run kept, without waiting for the walk;
// the walk then renews it in the background.
func TestListingKeptForTheNextStart(t *testing.T) {
	data, folder := t.TempDir(), t.TempDir()
	if _, err := prefs.Update(data, func(p *prefs.Prefs) { p.Folders = []string{folder} }); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(folder, "a.mp3"), nil, 0o644)
	if got := LinkedIn(data); len(got) != 1 {
		t.Fatalf("first listing: %v", got)
	}
	if _, err := os.Stat(listingPath(data)); err != nil {
		t.Fatal("listing not kept:", err)
	}

	// a new run: a.mp3 is gone, but the kept listing comes first
	_ = os.Remove(filepath.Join(folder, "a.mp3"))
	scanMu.Lock()
	delete(scans, data)
	delete(kept, data)
	scanMu.Unlock()
	if got := LinkedIn(data); len(got) != 1 || got[0].Name != LinkedName(filepath.Join(folder, "a.mp3")) {
		t.Fatalf("kept listing: %v", got)
	}
	// after Forget, never the kept one again
	Forget(data)
	if got := LinkedIn(data); len(got) != 0 {
		t.Fatalf("after Forget: %v", got)
	}
}

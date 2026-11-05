// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package autostart

import (
	"path/filepath"
	"testing"
)

func TestCommand(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "transcribe.exe")
	if got, want := command(exe, ""), `"`+exe+`" --background`; got != want {
		t.Errorf("default data dir: got %s, want %s", got, want)
	}
	other := filepath.Join(dir, "elsewhere")
	if got, want := command(exe, other), `"`+exe+`" --background --data "`+other+`"`; got != want {
		t.Errorf("own data dir: got %s, want %s", got, want)
	}
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"slices"
	"testing"
)

func TestRelaunchArgs(t *testing.T) {
	for _, c := range []struct {
		args      []string
		noBrowser bool
		want      []string
	}{
		// a double click: the new version must not open a second window
		{nil, false, []string{"-port", "8765", "-background"}},
		{[]string{"-background"}, false, []string{"-background", "-port", "8765"}},
		{[]string{"--background=false"}, false, []string{"--background=false", "-port", "8765", "-background"}},
		// the port this app got replaces the one asked for
		{[]string{"--data", `C:\x y`, "--port", "1234", "--threads", "2"}, false,
			[]string{"--data", `C:\x y`, "--threads", "2", "-port", "8765", "-background"}},
		{[]string{"-port=1234", "-no-browser"}, true, []string{"-no-browser", "-port", "8765"}},
	} {
		if got := relaunchArgs(c.args, 8765, c.noBrowser); !slices.Equal(got, c.want) {
			t.Errorf("relaunchArgs(%q, %v) = %q, want %q", c.args, c.noBrowser, got, c.want)
		}
	}
}

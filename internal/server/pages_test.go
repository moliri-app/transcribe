// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"strings"
	"testing"
	"testing/fstest"
)

func withDist(t *testing.T, files fstest.MapFS) {
	t.Helper()
	old := dist
	dist = files
	t.Cleanup(func() { dist = old })
}

func TestPages(t *testing.T) {
	s, _ := newServer(t)
	withDist(t, fstest.MapFS{
		"index.html":                {Data: []byte("<!-- neu -->")},
		"_app/immutable/entry/a.js": {Data: []byte("export{}")},
		"favicon.svg":               {Data: []byte("<svg/>")},
		"PLATZHALTER.txt":           {Data: []byte("x")},
		"_app/version.json":         {Data: []byte("{}")},
	})
	for _, p := range []string{"/", "/queue", "/settings", "/einstellungen", "/editor", "/editor?neu", "/lizenzen", "/bausteine"} {
		if w := do(s, "GET", p, "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "<!-- neu -->") {
			t.Errorf("%s: %d %q", p, w.Code, w.Body.String())
		}
	}
	// the addresses of the static pages from before lead to the same page
	for from, to := range map[string]string{"/index.html": "/", "/queue.html": "/queue", "/settings.html": "/settings", "/editor.html?x=1": "/editor?x=1"} {
		if w := do(s, "GET", from, "", nil); w.Code != 301 || w.Header().Get("Location") != to {
			t.Errorf("%s: %d %q, want 301 to %s", from, w.Code, w.Header().Get("Location"), to)
		}
	}

	w := do(s, "GET", "/_app/immutable/entry/a.js", "", nil)
	if w.Code != 200 || w.Body.String() != "export{}" || !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("asset: %d %q %q", w.Code, w.Body.String(), w.Header().Get("Cache-Control"))
	}
	if w := do(s, "GET", "/_app/version.json", "", nil); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("version.json: %d %q", w.Code, w.Header().Get("Cache-Control"))
	}
	if w := do(s, "GET", "/favicon.svg", "", nil); w.Code != 200 {
		t.Errorf("favicon: %d", w.Code)
	}
	for _, p := range []string{"/_app/", "/_app/immutable", "/nirgends", "/web/appbar.js", "/web/fonts/fonts.css"} {
		if w := do(s, "GET", p, "", nil); w.Code != 404 {
			t.Errorf("%s: %d, want 404", p, w.Code)
		}
	}
}

func TestNewPageWithoutBuild(t *testing.T) {
	s, _ := newServer(t)
	withDist(t, fstest.MapFS{"PLATZHALTER.txt": {Data: []byte("x")}})
	w := do(s, "GET", "/lizenzen", "", nil)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "pnpm build") {
		t.Errorf("no build: %d %q", w.Code, w.Body.String())
	}
	if w := do(s, "GET", "/editor", "", nil); w.Code != 503 {
		t.Errorf("editor without build: %d", w.Code)
	}
}

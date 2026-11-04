// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"io/fs"
	"net/http"
	"slices"
	"strings"

	"github.com/moliri-app/transcribe/web"
)

// The browser UI is the SvelteKit app in frontend/, built into web/dist. Every page is the same
// index.html; its router shows the page for the address.
var pages = []string{"/", "/queue", "/live", "/settings", "/einstellungen", "/editor", "/lizenzen", "/bausteine"}

// The static pages the UI had before stood under these addresses; bookmarks to them still arrive.
var moved = map[string]string{
	"/index.html":    "/",
	"/queue.html":    "/queue",
	"/settings.html": "/settings",
	"/editor.html":   "/editor",
}

// dist is the UI; a variable so tests can put a build in.
var dist fs.FS = web.Dist

func (s *Server) root(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if slices.Contains(pages, path) {
		page(w)
		return
	}
	if to, ok := moved[path]; ok {
		if r.URL.RawQuery != "" {
			to += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, to, http.StatusMovedPermanently)
		return
	}
	// files of the UI (/_app/…, /favicon.svg); its index.html only as a page
	if name := strings.TrimPrefix(path, "/"); name != "index.html" && fs.ValidPath(name) {
		if st, err := fs.Stat(dist, name); err == nil && !st.IsDir() {
			if strings.HasPrefix(name, "_app/immutable/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFileFS(w, r, dist, name)
			return
		}
	}
	http.NotFound(w, r)
}

// page serves the UI's single page.
func page(w http.ResponseWriter) {
	b, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`<!doctype html><html lang="de"><meta charset="utf-8"><title>Transcribe by Moliri</title>` +
			`<p style="font:18px/1.5 system-ui,sans-serif;margin:2rem">Die Oberfläche ist in diesem Programm nicht enthalten. ` +
			`Sie entsteht mit <code>pnpm install</code> und <code>pnpm build</code> in <code>frontend/</code>; danach das Programm neu bauen (<code>build.ps1</code>).</p>`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

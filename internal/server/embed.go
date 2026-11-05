// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync/atomic"
	"unicode/utf8"

	"github.com/moliri-app/transcribe/embed"
	"github.com/moliri-app/transcribe/models"
)

// Sentence embeddings for Moliri's field search by meaning (docs/embed-api.md): GET /api/embed says
// which model and whether it is there, POST /api/embed turns texts into vectors, POST
// /api/embed/download fetches the model. Web pages need the same approval as for text recognition.

const (
	maxEmbedTexts = 64
	maxEmbedText  = 4000 // bytes of one text
)

// embedFetching is set while a download the page asked for runs, embedWarming while the model
// loads after a GET.
var embedFetching, embedWarming atomic.Bool

func (s *Server) embedInfo() map[string]any {
	info := map[string]any{
		"ok":        true,
		"model":     embed.ModelID,
		"dtype":     embed.Dtype,
		"dim":       embed.Dim,
		"prefixes":  map[string]string{"query": embed.QueryPrefix, "document": embed.DocumentPrefix},
		"installed": s.cfg.Embed.Installed(),
		"loaded":    s.cfg.Embed.Loaded(),
		"size":      embedSizeMB,
	}
	items, _ := models.Snapshot()
	for _, it := range items {
		if it.Name == "embed" && (it.State == "downloading" || it.State == "error") {
			info["download"] = map[string]any{"state": it.State, "done": it.Done, "total": it.Total, "error": it.Error}
		}
	}
	return info
}

// embedSizeMB is the download, for the page to say what a click costs.
const embedSizeMB = 295

func (s *Server) embedAPI(w http.ResponseWriter, r *http.Request) {
	if !s.pageAccess(w, r, "die Suche nach Bedeutung") {
		return
	}
	if s.cfg.Embed == nil {
		fail(w, http.StatusServiceUnavailable, "Die Suche nach Bedeutung ist in diesem Programm nicht verfügbar")
		return
	}
	switch r.Method {
	case http.MethodGet:
		info := s.embedInfo()
		// A page asks when a search is near (a field picker got focus): the model loads
		// meanwhile, about ten seconds, so the first query need not wait for it.
		if s.cfg.Embed.Installed() && !s.cfg.Embed.Loaded() && embedWarming.CompareAndSwap(false, true) {
			go func() {
				defer embedWarming.Store(false)
				if err := s.cfg.Embed.Warm(context.Background()); err != nil {
					log.Println("embed:", err)
				}
			}()
		}
		writeJSON(w, 200, info)
	case http.MethodPost:
		s.embedTexts(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) embedTexts(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Texts []string   `json:"texts"`
		Kind  embed.Kind `json:"kind"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "JSON: "+err.Error())
		return
	}
	if body.Kind != embed.Query && body.Kind != embed.Document {
		fail(w, http.StatusBadRequest, "kind muss query oder document sein")
		return
	}
	if len(body.Texts) == 0 || len(body.Texts) > maxEmbedTexts {
		fail(w, http.StatusBadRequest, "texts: 1 bis 64 Texte")
		return
	}
	for _, t := range body.Texts {
		if len(t) > maxEmbedText || !utf8.ValidString(t) {
			fail(w, http.StatusBadRequest, "texts: ein Text ist länger als 4000 Bytes oder kein UTF-8")
			return
		}
	}
	vectors, err := s.cfg.Embed.Embed(r.Context(), body.Texts, body.Kind)
	switch {
	case errors.Is(err, embed.ErrNotInstalled):
		fail(w, http.StatusServiceUnavailable, "Das Modell für die Suche nach Bedeutung ist in Transcribe noch nicht geladen")
		return
	case err != nil:
		if r.Context().Err() != nil {
			return // the caller went away
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "model": embed.ModelID, "dtype": embed.Dtype, "dim": embed.Dim, "vectors": vectors})
}

// embedDownload starts fetching the model in the background; GET /api/embed shows how far it got.
func (s *Server) embedDownload(w http.ResponseWriter, r *http.Request) {
	if !s.pageAccess(w, r, "die Suche nach Bedeutung") {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg.Embed == nil || s.cfg.EmbedFetch == nil {
		fail(w, http.StatusServiceUnavailable, "Die Suche nach Bedeutung ist in diesem Programm nicht verfügbar")
		return
	}
	if !s.cfg.Embed.Installed() && embedFetching.CompareAndSwap(false, true) {
		models.Track("embed", "downloading", 0, 0, nil)
		go func() {
			defer embedFetching.Store(false)
			if err := s.cfg.EmbedFetch(context.Background()); err != nil {
				log.Println("embed download:", err)
			}
		}()
	}
	writeJSON(w, 200, s.embedInfo())
}

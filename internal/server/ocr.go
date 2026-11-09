// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/ocr"
)

// Text recognition (OCR) for other programs (docs/ocr-api.md): POST /api/ocr reads an image,
// GET /api/ocr/engines lists the engines, GET /api/ocr/schema is the result's JSON schema. Web pages
// need the same approval as for live transcription (the Live page), and may call from another
// address (CORS).

const maxOCRImage = 64 << 20

// ocrAccess answers the preflight and sets the CORS headers; false = the request is answered.
func (s *Server) ocrAccess(w http.ResponseWriter, r *http.Request) bool {
	return s.pageAccess(w, r, "die Texterkennung")
}

// pageAccess lets programs and approved pages use a service of the app (what, in the refusal):
// it answers the preflight and sets the CORS headers; false = the request is answered.
func (s *Server) pageAccess(w http.ResponseWriter, r *http.Request, what string) bool {
	if !fromLocalHost(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	origin := r.Header.Get("Origin")
	if origin != "" {
		// a refused page may read the refusal too: it says where to allow it, and holds nothing else
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Add("Vary", "Origin")
	}
	if r.Method == http.MethodOptions {
		h := w.Header()
		h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
		h.Set("Access-Control-Max-Age", "600")
		if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
			h.Set("Access-Control-Allow-Private-Network", "true") // a public page calling localhost (Chrome)
		}
		w.WriteHeader(http.StatusNoContent)
		return false
	}
	if !s.originAllowed(origin, clientName(r)) {
		fail(w, http.StatusForbidden, "Diese Seite darf "+what+" nicht nutzen. In Transcribe auf der Seite „Live“ erlauben: "+live.Origin(origin))
		return false
	}
	return true
}

// originAllowed is the approval of the Live page: programs, pages on this computer, pages allowed
// there. A page turned away is listed there to be allowed.
func (s *Server) originAllowed(origin, client string) bool {
	if s.Live == nil {
		return live.Origin(origin) == ""
	}
	if s.Live.Allowed(origin) {
		return true
	}
	s.Live.Refused(origin, client)
	return false
}

func (s *Server) ocrEngines(w http.ResponseWriter, r *http.Request) {
	if !s.ocrAccess(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg.OCR == nil {
		fail(w, http.StatusServiceUnavailable, "Texterkennung ist nicht verfügbar")
		return
	}
	accel := "cpu"
	if prefs.Load(s.cfg.DataDir).Accel == "cuda" {
		accel = "cuda"
	}
	writeJSON(w, 200, map[string]any{
		"ok":      true,
		"format":  ocr.Format,
		"place":   s.cfg.OCR.Place,
		"accel":   accel, // the app's GPU choice; each engine says whether it runs on the GPU
		"default": s.cfg.OCR.DefaultEngine(),
		"engines": s.cfg.OCR.Engines(),
	})
}

func (s *Server) ocrSchema(w http.ResponseWriter, r *http.Request) {
	if !s.ocrAccess(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/schema+json")
	_, _ = w.Write(ocr.Schema)
}

// ocrRead reads one image: multipart/form-data with the file in "image" and the fields engine,
// languages, layout, or the image as the body with those as query parameters.
func (s *Server) ocrRead(w http.ResponseWriter, r *http.Request) {
	if !s.ocrAccess(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg.OCR == nil {
		fail(w, http.StatusServiceUnavailable, "Texterkennung ist nicht verfügbar")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxOCRImage)
	var body io.Reader = r.Body
	field := r.URL.Query().Get
	image := r.Header.Get("Content-Type")
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt == "multipart/form-data" {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			fail(w, http.StatusBadRequest, "Formular: "+err.Error())
			return
		}
		defer r.MultipartForm.RemoveAll()
		f, hdr, err := r.FormFile("image")
		if err != nil {
			fail(w, http.StatusBadRequest, "image fehlt")
			return
		}
		defer f.Close()
		body, field = f, r.FormValue
		image = strings.TrimSpace(hdr.Filename + ", " + hdr.Header.Get("Content-Type"))
	}
	asked := ocr.Options{Languages: field("languages"), Layout: field("layout")}
	engine, o, err := s.cfg.OCR.Prepare(field("engine"), asked)
	if err != nil {
		s.Live.Logged(live.KindOCR, clientName(r), field("engine"), ocrRequest(image, 0, asked), err)
		fail(w, ocrStatus(err), err.Error())
		return
	}
	info := engine.Info()
	tmp, err := os.CreateTemp("", "transcribe-ocr-*")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, body)
	tmp.Close()
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		s.Live.Logged(live.KindOCR, clientName(r), engineName(info), ocrRequest(image, 0, o), errors.New("Bild größer als 64 MB"))
		fail(w, http.StatusRequestEntityTooLarge, "Bild größer als 64 MB")
		return
	case err != nil:
		fail(w, http.StatusBadRequest, err.Error())
		return
	case n == 0:
		fail(w, http.StatusBadRequest, "Bild fehlt")
		return
	}
	var facts []live.Fact
	if info.GPU {
		facts = append(facts, live.Fact{Label: "Grafikkarte", Value: "ja"})
	}
	call := s.Live.Call(live.KindOCR, clientName(r), engineName(info), ocrRequest(image, n, o), facts...)
	res, err := s.cfg.OCR.Read(r.Context(), engine, tmp.Name(), o, s.cfg.Threads)
	if err != nil {
		if r.Context().Err() != nil {
			call.End("", errGone)
			return // the caller went away
		}
		call.End("", err)
		fail(w, ocrStatus(err), err.Error())
		return
	}
	call.End(res.Text(), nil, ocrFacts(res)...)
	writeJSON(w, 200, res)
}

// errGone ends a call whose program went away before the answer came.
var errGone = errors.New("Das Programm hat nicht mehr auf die Antwort gewartet")

// engineName names an OCR engine for the Live page: its name and version.
func engineName(info ocr.Info) string {
	if info.Version == "" || strings.Contains(info.Name, info.Version) {
		return info.Name
	}
	return info.Name + " " + info.Version
}

// ocrRequest is what the Live page shows of a request: the image (its name and type, size) and the
// options.
func ocrRequest(image string, size int64, o ocr.Options) string {
	var b strings.Builder
	b.WriteString("Bild: " + cmp.Or(strings.Trim(image, ", "), "ohne Angabe"))
	if size > 0 {
		fmt.Fprintf(&b, ", %s", byteSize(size))
	}
	b.WriteString("\nSprachen: " + cmp.Or(o.Languages, "Standard"))
	b.WriteString("\nLayout: " + cmp.Or(o.Layout, "Standard"))
	return b.String()
}

func ocrFacts(res *ocr.Result) []live.Fact {
	lines := 0
	for _, p := range res.Pages {
		lines += len(p.Lines)
	}
	facts := []live.Fact{
		{Label: "Erkannt", Value: fmt.Sprintf("%d Zeilen, %d Wörter", lines, res.Words())},
		{Label: "Engine", Value: res.Engine},
	}
	if len(res.Pages) > 0 {
		facts = append(facts, live.Fact{Label: "Bildgröße", Value: fmt.Sprintf("%d × %d Pixel", res.Pages[0].Width, res.Pages[0].Height)})
	}
	if res.Language != "" {
		facts = append(facts, live.Fact{Label: "Modelle", Value: res.Language})
	}
	return facts
}

// byteSize says a size for people: "812 KB", "2,4 MB".
func byteSize(n int64) string {
	if n < 1<<20 {
		return fmt.Sprintf("%d KB", max(1, (n+512)>>10))
	}
	return strings.Replace(fmt.Sprintf("%.1f MB", float64(n)/(1<<20)), ".", ",", 1)
}

func ocrStatus(err error) int {
	switch {
	case errors.Is(err, ocr.ErrOptions):
		return http.StatusBadRequest
	case errors.Is(err, ocr.ErrUnknownEngine):
		return http.StatusNotFound
	case errors.Is(err, ocr.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, ocr.ErrImage):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ocr.ErrService):
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}

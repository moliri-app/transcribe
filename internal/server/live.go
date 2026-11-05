// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/jobs"
)

// Live transcription for other programs (protocol in docs/live-api.md) and the Live page.

// fromLocalHost reports whether the request was addressed to this computer by name (not a
// foreign name that resolves here: DNS rebinding).
func fromLocalHost(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return localHost(host) || host == "::1"
}

// clientName is who asks, as the Live page shows it: the name the program gave, else its page.
func clientName(r *http.Request) string {
	if n := strings.TrimSpace(r.URL.Query().Get("name")); n != "" {
		return truncate(n, 60)
	}
	if u, err := url.Parse(r.Header.Get("Origin")); err == nil && u.Host != "" {
		return u.Host
	}
	return ""
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

type wsMsg struct {
	Type    string   `json:"type"`
	Text    string   `json:"text,omitempty"`
	Start   *float64 `json:"start,omitempty"`
	End     *float64 `json:"end,omitempty"`
	Note    string   `json:"note,omitempty"`
	Mode    string   `json:"mode,omitempty"`
	Message string   `json:"message,omitempty"`
	Source  string   `json:"source,omitempty"` // "mic" or "system", when the recording mixes both
}

// liveSocket is the WebSocket a program talks to: mode=audio (default) sends audio and gets the
// text back, mode=mic has the app record the microphone and gets the text from now until it says
// stop.
func (s *Server) liveSocket(w http.ResponseWriter, r *http.Request) {
	if s.Live == nil {
		http.Error(w, "live transcription is not available", http.StatusServiceUnavailable)
		return
	}
	if !fromLocalHost(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Header.Get("Upgrade") == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusUpgradeRequired)
		fmt.Fprintf(w, "Live-Transkription: WebSocket auf ws://%s/api/live (siehe docs/live-api.md)\n", r.Host)
		return
	}
	q := r.URL.Query()
	mode := q.Get("mode")
	if mode == "" {
		mode = "audio"
	}
	rate, _ := strconv.Atoi(q.Get("sample_rate"))
	if rate == 0 {
		rate = live.SR
	}
	enc := q.Get("encoding")
	if enc == "" {
		enc = "s16le"
	}
	origin := r.Header.Get("Origin")
	client := clientName(r)

	// any page may connect; whether it may transcribe is told over the socket, which a page can read
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	c.SetReadLimit(1 << 20)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	out := make(chan wsMsg, 256)
	wrote := make(chan struct{})
	go func() {
		defer close(wrote)
		for m := range out {
			b, _ := json.Marshal(m)
			wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.Write(wctx, websocket.MessageText, b)
			wcancel()
			if err != nil {
				cancel()
				for range out {
				}
				return
			}
		}
	}()
	send := func(m wsMsg) {
		select {
		case out <- m:
		case <-ctx.Done():
		}
	}
	closeWith := func(code websocket.StatusCode, reason string) {
		close(out)
		<-wrote
		_ = c.Close(code, truncate(reason, 120))
	}
	refuse := func(msg string) {
		send(wsMsg{Type: "error", Message: msg})
		closeWith(websocket.StatusPolicyViolation, msg)
	}

	switch {
	case !s.Live.Allowed(origin):
		s.Live.Refused(origin, client)
		refuse("Diese Seite darf die Live-Transkription nicht nutzen. In Transcribe auf der Seite „Live“ erlauben: " + live.Origin(origin))
		return
	case mode != "audio" && mode != "mic":
		refuse("mode: audio oder mic")
		return
	case mode == "audio" && (rate < 8000 || rate > 96000):
		refuse("sample_rate: 8000 bis 96000")
		return
	case mode == "audio" && enc != "s16le" && enc != "f32le":
		refuse("encoding: s16le oder f32le")
		return
	}
	events := func(ev live.Event) {
		t := "partial"
		if ev.Final {
			t = "final"
		}
		send(wsMsg{Type: t, Text: ev.Text, Start: &ev.Start, End: &ev.End, Source: ev.Source})
	}

	if mode == "mic" {
		l, err := s.Live.Listen(ctx, client, events)
		if err != nil {
			refuse(err.Error())
			return
		}
		send(wsMsg{Type: "ready", Mode: mode, Note: l.Note()})
		// the program says stop or leaves; the note ends either way
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			for {
				typ, b, err := c.Read(ctx)
				if err != nil || (typ == websocket.MessageText && isStop(b)) {
					return
				}
			}
		}()
		select {
		case <-stopped:
			l.Stop()
		case <-l.Done():
		}
		if err := l.Err(); err != nil {
			send(wsMsg{Type: "error", Message: err.Error()})
			closeWith(websocket.StatusInternalError, err.Error())
			return
		}
		send(wsMsg{Type: "done"})
		closeWith(websocket.StatusNormalClosure, "")
		return
	}

	sess, err := s.Live.Audio(ctx, client, events)
	if err != nil {
		refuse(err.Error())
		return
	}
	send(wsMsg{Type: "ready", Mode: mode, Note: sess.Note()})
	dec := pcmDecoder{f32: enc == "f32le", ratio: float64(rate) / live.SR}
	var left error
	gone := false
	for {
		typ, b, err := c.Read(ctx)
		if err != nil {
			// closing without stop is fine; a connection that breaks off is noted
			if st := websocket.CloseStatus(err); st != websocket.StatusNormalClosure && st != websocket.StatusGoingAway {
				left = errors.New("Verbindung abgebrochen")
			}
			gone = true
			break
		}
		if typ == websocket.MessageText {
			if isStop(b) {
				break
			}
			continue
		}
		if pcm := dec.decode(b); len(pcm) > 0 {
			sess.Write(pcm)
		}
	}
	sess.End(left)
	if gone {
		closeWith(websocket.StatusNormalClosure, "")
		return
	}
	send(wsMsg{Type: "done"})
	closeWith(websocket.StatusNormalClosure, "")
}

func isStop(b []byte) bool {
	var m struct{ Type string }
	return json.Unmarshal(b, &m) == nil && m.Type == "stop"
}

// pcmDecoder turns binary messages into 16 kHz samples: samples may be split across messages, and
// other rates are resampled (linearly; 16 kHz from the sender sounds best).
type pcmDecoder struct {
	f32   bool
	ratio float64 // input rate / 16 kHz
	rest  []byte
	pos   float64 // position of the next output sample, in input samples after prev
	prev  float32
	have  bool
}

func (d *pcmDecoder) decode(b []byte) []float32 {
	b = append(d.rest, b...)
	size := 2
	if d.f32 {
		size = 4
	}
	n := len(b) / size
	in := make([]float32, n)
	for i := range in {
		if d.f32 {
			in[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
		} else {
			in[i] = float32(int16(binary.LittleEndian.Uint16(b[2*i:]))) / 32768
		}
	}
	d.rest = append([]byte(nil), b[n*size:]...)
	if d.ratio == 1 || n == 0 {
		return in
	}
	x := in
	if d.have {
		x = append([]float32{d.prev}, in...)
	}
	d.have = true
	var out []float32
	for d.pos+1 < float64(len(x)) {
		i := int(d.pos)
		f := float32(d.pos - float64(i))
		out = append(out, x[i]*(1-f)+x[i+1]*f)
		d.pos += d.ratio
	}
	d.pos -= float64(len(x) - 1)
	d.prev = x[len(x)-1]
	return out
}

// liveEvents streams the Live page's state: a snapshot, then every change (Server-Sent Events).
func (s *Server) liveEvents(w http.ResponseWriter, r *http.Request) {
	if s.Live == nil {
		http.Error(w, "live transcription is not available", http.StatusServiceUnavailable)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	snap, updates, cancel := s.Live.Subscribe()
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	write := func(b []byte) bool {
		_, err := fmt.Fprintf(w, "data: %s\n\n", b)
		fl.Flush()
		return err == nil
	}
	if !write(snap) {
		return
	}
	keep := time.NewTicker(20 * time.Second)
	defer keep.Stop()
	for {
		select {
		case b, ok := <-updates:
			if !ok || !write(b) {
				return
			}
		case <-keep.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// liveAPI answers the Live page: microphones, the recording, notes, who may connect and what is
// kept of what programs transcribe.
func (s *Server) liveAPI(w http.ResponseWriter, r *http.Request) {
	if s.Live == nil {
		http.Error(w, "live transcription is not available", http.StatusServiceUnavailable)
		return
	}
	action := strings.TrimPrefix(r.URL.Path, "/api/live/")
	if r.Method == http.MethodGet {
		switch action {
		case "devices":
			writeJSON(w, 200, live.ListMicrophones(r.Context(), s.cfg.DataDir))
		case "note":
			n := s.Live.Note(r.URL.Query().Get("id"))
			if n == nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(n.Text()))
		default:
			http.NotFound(w, r)
		}
		return
	}
	if r.Method != http.MethodPost || !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	var in struct {
		Device   *string  `json:"device"`
		Source   string   `json:"source"`
		Language string   `json:"language"`
		Save     bool     `json:"save"`
		ID       string   `json:"id"`
		Origin   string   `json:"origin"`
		On       bool     `json:"on"`
		Words    []string `json:"words"`
		Hours    *int     `json:"keepHours"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in) != nil {
		http.Error(w, "bad request", 400)
		return
	}
	var err error
	switch action {
	case "device":
		if in.Device == nil {
			err = errors.New("device fehlt")
			break
		}
		_, err = prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) { p.LiveDevice = *in.Device })
		s.Live.InputChanged()
	case "language":
		lang, e := jobs.NormLanguage(in.Language)
		if e != nil {
			err = errors.New("Sprache: auto oder Codes wie de oder de,en")
			break
		}
		_, err = prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) { p.LiveLanguage = strings.TrimPrefix(lang, "auto") })
		s.Live.Warm() // German has a model of its own
	case "source":
		_, err = prefs.Update(s.cfg.DataDir, func(p *prefs.Prefs) { p.LiveSource = live.NormSource(in.Source) })
		s.Live.InputChanged()
	case "mode":
		err = s.Live.SetEnabled(in.On)
	case "api-save":
		err = s.Live.SetAPINotes(&in.On, nil)
	case "api-keep":
		if in.Hours == nil || *in.Hours < 0 {
			err = errors.New("keepHours: 0 (behalten) oder Stunden")
			break
		}
		err = s.Live.SetAPINotes(nil, in.Hours)
	case "words":
		err = s.Live.SetWords(in.Words)
	case "commands":
		err = s.Live.SetCommands(in.On)
	case "start":
		err = s.Live.StartDauer(context.Background(), in.Save)
	case "stop":
		s.Live.StopDauer()
	case "delete":
		n := s.Live.Note(in.ID)
		if err = s.Live.Delete(in.ID); err == nil && n != nil && n.Audio != "" {
			s.dropRecording(n.Audio)
		}
	case "allow":
		err = s.Live.Allow(in.Origin, true)
	case "forbid":
		err = s.Live.Allow(in.Origin, false)
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// dropRecording removes the recording a deleted Live note saved in audio/, so the library does not
// offer it for transcription any more. One that was transcribed in full stays: it has a place in
// the library of its own, as does one waiting in the queue.
func (s *Server) dropRecording(name string) {
	if strings.ContainsAny(name, `/\`) || !media.IsMedia(name) {
		return
	}
	stem := media.Stem(name)
	rdir := filepath.Join(s.resultsDir(), stem)
	if _, err := os.Stat(filepath.Join(rdir, "_final.json")); err == nil {
		return
	}
	for _, j := range s.Queue.List() {
		if j.Audio == name && (j.Status == jobs.Queued || j.Status == jobs.Running || j.Status == jobs.Canceling) {
			return
		}
	}
	if err := os.Remove(filepath.Join(s.audioDir(), name)); err != nil && !os.IsNotExist(err) {
		log.Println("live: removing the recording of a deleted note:", err)
		return
	}
	_ = os.RemoveAll(rdir) // what a cancelled transcription left
}

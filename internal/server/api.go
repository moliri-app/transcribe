// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/moliri-app/transcribe/models"

	"github.com/moliri-app/transcribe/annotate"
	"github.com/moliri-app/transcribe/export"
	"github.com/moliri-app/transcribe/fileutil"
	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/queue"
	"github.com/moliri-app/transcribe/internal/results"
	"github.com/moliri-app/transcribe/jobs"
)

// pyQuote mirrors Python's urllib.parse.quote (safe="/").
func pyQuote(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '_', c == '.', c == '-', c == '~', c == '/':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

type variant struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

func variants(rdir, stem, kind string) []variant {
	out := []variant{}
	files, _ := filepath.Glob(filepath.Join(rdir, "_"+kind+"@*.json"))
	sort.Strings(files)
	for _, f := range files {
		base := filepath.Base(f)
		id := strings.TrimSuffix(strings.SplitN(base, "@", 2)[1], ".json")
		out = append(out, variant{id, "results/" + stem + "/" + base})
	}
	return out
}

func (s *Server) buildIndex() []map[string]any {
	items := []map[string]any{}
	entries, _ := os.ReadDir(s.audioDir())
	sort.SliceStable(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !media.IsMedia(name) {
			continue
		}
		stem := media.Stem(name)
		rdir := filepath.Join(s.resultsDir(), stem)
		engines := []map[string]any{}
		if _, err := os.Stat(filepath.Join(rdir, "_final.json")); err == nil {
			engines = append(engines, map[string]any{
				"id": "★ Ergebnis", "file": "results/" + stem + "/_final.json", "final": true})
		}
		files, _ := filepath.Glob(filepath.Join(rdir, "*.json"))
		sort.Strings(files)
		for _, f := range files {
			base := filepath.Base(f)
			if strings.HasPrefix(base, "_") {
				continue
			}
			var meta struct {
				Engine  json.RawMessage `json:"engine"`
				Speed   json.RawMessage `json:"speed"`
				Elapsed json.RawMessage `json:"elapsed"`
				Threads json.RawMessage `json:"threads"`
			}
			if fileutil.ReadJSON(f, &meta) != nil || len(meta.Engine) == 0 {
				continue
			}
			engines = append(engines, map[string]any{
				"id": meta.Engine, "file": "results/" + stem + "/" + base,
				"speed": rawOrNull(meta.Speed), "elapsed": rawOrNull(meta.Elapsed), "threads": rawOrNull(meta.Threads),
			})
		}
		item := map[string]any{
			"name": name, "stem": stem, "url": "audio/" + pyQuote(name),
			"video":   media.HasVideo(filepath.Join(s.audioDir(), name)),
			"engines": engines, "speakers": variants(rdir, stem, "speakers"), "events": variants(rdir, stem, "events"),
			"added": added(filepath.Join(s.audioDir(), name)),
		}
		if sum, ok := summarize(rdir); ok {
			item["summary"] = sum
		}
		items = append(items, item)
	}
	return items
}

func rawOrNull(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return json.RawMessage("null")
	}
	return r
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.buildIndex())
}

func (s *Server) peaks(w http.ResponseWriter, r *http.Request) {
	stem := r.URL.Query().Get("stem")
	if !validStem(stem) {
		http.Error(w, "bad stem", 400)
		return
	}
	b, err := media.Peaks(s.cfg.DataDir, stem)
	switch {
	case errors.Is(err, media.ErrNoMedia):
		http.Error(w, "no media", 404)
	case err != nil:
		http.Error(w, "peaks failed: "+err.Error(), 500)
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(b)
	}
}

func (s *Server) thumbs(w http.ResponseWriter, r *http.Request) {
	stem := r.URL.Query().Get("stem")
	if !validStem(stem) {
		http.Error(w, "bad stem", 400)
		return
	}
	meta, err := media.Thumbs(s.cfg.DataDir, stem)
	switch {
	case err != nil:
		http.Error(w, "thumbnails failed", 500)
	case meta == nil:
		http.Error(w, "no video", 404)
	default:
		writeJSON(w, 200, meta)
	}
}

func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	stem, format := q.Get("stem"), q.Get("format")
	if !validStem(stem) {
		http.Error(w, "bad stem", 400)
		return
	}
	if _, ok := export.Formats[format]; !ok {
		http.Error(w, "bad format", 400)
		return
	}
	body, ctype, name, err := results.Export(s.cfg.DataDir, stem, format)
	if errors.Is(err, results.ErrNoTranscript) {
		http.Error(w, "no transcript yet", 404)
		return
	} else if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+pyQuote(name))
	_, _ = w.Write(body)
}

// clipExport serves saved clips (ids, comma separated) as one file: format mp3 or mp4 cuts them out of the
// recording and joins them in time order, txt writes their transcript.
func (s *Server) clipExport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	stem, format := q.Get("stem"), q.Get("format")
	if !validStem(stem) {
		http.Error(w, "bad stem", 400)
		return
	}
	if !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	if format != "mp3" && format != "mp4" && format != "txt" {
		http.Error(w, "bad format", 400)
		return
	}
	final, ann, err := results.Load(s.cfg.DataDir, stem)
	if errors.Is(err, results.ErrNoTranscript) {
		http.Error(w, "no transcript yet", 404)
		return
	} else if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	want := map[string]bool{}
	for _, id := range strings.Split(q.Get("ids"), ",") {
		want[id] = true
	}
	var clips []annotate.Clip
	for _, c := range ann.Clips {
		if want[c.ID] && c.End > c.Start {
			clips = append(clips, c)
		}
	}
	if len(clips) == 0 {
		http.Error(w, "no such clip", 404)
		return
	}
	sort.SliceStable(clips, func(i, j int) bool { return clips[i].Start < clips[j].Start })
	name := clipFileName(clips[0].Title)
	if len(clips) > 1 {
		name = fmt.Sprintf("%s – Zusammenschnitt (%d Clips)", stem, len(clips))
	}
	disposition := "attachment; filename*=UTF-8''" + pyQuote(name+"."+format)
	if format == "txt" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", disposition)
		_, _ = io.WriteString(w, export.ClipsTXT(final, ann, clips))
		return
	}
	src := media.FindMedia(s.cfg.DataDir, stem)
	if src == "" {
		http.Error(w, "audio not found", 404)
		return
	}
	video := format == "mp4"
	if video && !media.HasVideo(src) {
		http.Error(w, "recording has no video", 400)
		return
	}
	tmp, err := os.MkdirTemp("", "clips-")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer os.RemoveAll(tmp)
	ranges := make([]media.Range, len(clips))
	for i, c := range clips {
		ranges[i] = media.Range{A: c.Start, B: c.End}
	}
	out := filepath.Join(tmp, "clip."+format)
	if err := media.Cut(r.Context(), src, ranges, video, out); err != nil {
		http.Error(w, "cutting failed: "+err.Error(), 500)
		return
	}
	f, err := os.Open(out)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Content-Type", map[bool]string{true: "video/mp4", false: "audio/mpeg"}[video])
	w.Header().Set("Content-Disposition", disposition)
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// clipFileName makes a clip title safe as a file name on every system.
func clipFileName(title string) string {
	name := strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, strings.TrimSpace(title))
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "Clip"
	}
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	return name
}

func (s *Server) queueList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"jobs": s.Queue.List()})
}

func readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r.Body, 64<<20))
}

func (s *Server) queueAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/queue/"), "/")
	var err error
	switch {
	case len(parts) == 1 && parts[0] == "add":
		body, _ := readBody(r)
		var m map[string]any
		if m, err = jobs.DecodeOptions(body); err != nil {
			break
		}
		list, ok := m["audio"].([]any)
		if !ok || len(list) == 0 {
			err = jobs.JobError("audio: list of file names expected")
			break
		}
		names := make([]string, 0, len(list))
		for _, v := range list {
			n, _ := v.(string)
			names = append(names, n)
		}
		if err = s.chooseEngine(m); err != nil {
			break
		}
		var added int
		if added, err = s.Queue.Add(names, m); err == nil {
			writeJSON(w, 200, map[string]any{"ok": true, "added": added})
			return
		}
	case len(parts) == 1 && parts[0] == "cancel-all":
		s.Queue.CancelAll()
	case len(parts) == 2 && parts[1] == "cancel":
		err = s.Queue.Cancel(parts[0])
	case len(parts) == 2 && parts[1] == "retry":
		err = s.Queue.Retry(parts[0])
	case len(parts) == 2 && parts[1] == "remove":
		err = s.Queue.Remove(parts[0])
	default:
		http.NotFound(w, r)
		return
	}
	var je jobs.JobError
	switch {
	case err == nil:
		writeJSON(w, 200, map[string]any{"ok": true})
	case errors.Is(err, queue.ErrUnknownJob):
		fail(w, 404, err.Error())
	case errors.As(err, &je):
		fail(w, 400, err.Error())
	default:
		fail(w, 500, err.Error())
	}
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	name := r.URL.Query().Get("name")
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if name == "" || name == "." || name == ".." || !media.IsMedia(name) {
		fail(w, 400, "unsupported file type")
		return
	}
	dir := s.audioDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 2; ; n++ {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			break
		}
		name = fmt.Sprintf("%s (%d)%s", stem, n, ext)
	}
	target := filepath.Join(dir, name)
	part := target + ".part"
	f, err := os.Create(part)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	n, err := io.Copy(f, r.Body)
	f.Close()
	if err != nil || (r.ContentLength >= 0 && n < r.ContentLength) {
		os.Remove(part)
		fail(w, 400, "upload interrupted")
		return
	}
	if err := os.Rename(part, target); err != nil {
		os.Remove(part)
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "name": name})
}

// stemPost handles the checks shared by /api/annotations and /api/apply.
func (s *Server) stemPost(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return "", false
	}
	if !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return "", false
	}
	stem := r.URL.Query().Get("stem")
	if st, err := os.Stat(results.Dir(s.cfg.DataDir, stem)); !validStem(stem) || err != nil || !st.IsDir() {
		http.Error(w, "unknown stem", 400)
		return "", false
	}
	return stem, true
}

func (s *Server) annotations(w http.ResponseWriter, r *http.Request) {
	stem, ok := s.stemPost(w, r)
	if !ok {
		return
	}
	body, _ := readBody(r)
	var data map[string]json.RawMessage
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			fail(w, 400, "invalid JSON")
			return
		}
	}
	if raw, ok := data["stem"]; ok && string(raw) != "null" {
		var st string
		if json.Unmarshal(raw, &st) != nil || st != stem {
			http.Error(w, "stem mismatch", 409)
			return
		}
	}
	pick := func(key, empty string) json.RawMessage {
		if v, ok := data[key]; ok {
			return v
		}
		return json.RawMessage(empty)
	}
	ann := map[string]json.RawMessage{
		"speakers": pick("speakers", "{}"), "spans": pick("spans", "[]"), "edits": pick("edits", "[]"), "clips": pick("clips", "[]"),
	}
	rdir := results.Dir(s.cfg.DataDir, stem)
	target := filepath.Join(rdir, "_annotations.json")
	if old, err := os.ReadFile(target); err == nil {
		keepHistory(rdir, old)
	}
	if err := fileutil.WriteJSON(target, ann); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// keepHistory saves the previous annotations at most every two minutes.
func keepHistory(rdir string, old []byte) {
	hist := filepath.Join(rdir, "_annotations_history")
	_ = os.MkdirAll(hist, 0o755)
	var last time.Time
	files, _ := filepath.Glob(filepath.Join(hist, "*.json"))
	for _, f := range files {
		if st, err := os.Stat(f); err == nil && st.ModTime().After(last) {
			last = st.ModTime()
		}
	}
	if time.Since(last) > 120*time.Second {
		_ = fileutil.WriteAtomic(filepath.Join(hist, time.Now().Format("20060102-150405")+".json"), old)
	}
}

func (s *Server) apply(w http.ResponseWriter, r *http.Request) {
	stem, ok := s.stemPost(w, r)
	if !ok {
		return
	}
	if media.FindMedia(s.cfg.DataDir, stem) == "" {
		http.Error(w, "audio not found", 404)
		return
	}
	if err := results.Apply(r.Context(), s.cfg.DataDir, stem, s.cfg.Retranscriber); err != nil {
		msg := err.Error()
		if len(msg) > 2000 {
			msg = msg[len(msg)-2000:]
		}
		fail(w, 500, msg)
		return
	}
	raw, err := os.ReadFile(filepath.Join(results.Dir(s.cfg.DataDir, stem), "_final.json"))
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	var f struct {
		Speakers  json.RawMessage   `json:"speakers"`
		Words     []json.RawMessage `json:"words"`
		Annotated bool              `json:"annotated"`
	}
	_ = json.Unmarshal(raw, &f)
	var sp map[string]json.RawMessage
	_ = json.Unmarshal(f.Speakers, &sp)
	py := map[bool]string{true: "True", false: "False"}[f.Annotated]
	writeJSON(w, 200, map[string]any{
		"ok":    true,
		"log":   fmt.Sprintf("-> %s/_final.json: %d Sprecher, %d Wörter, annotiert: %s", stem, len(sp), len(f.Words), py),
		"final": json.RawMessage(raw),
	})
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	items, ready := models.Snapshot()
	writeJSON(w, 200, map[string]any{"ready": ready, "items": items})
}

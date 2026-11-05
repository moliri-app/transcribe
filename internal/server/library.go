// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"cmp"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/moliri-app/transcribe/fileutil"
	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/queue"
	"github.com/moliri-app/transcribe/model"
)

// The library page shows each transcribed recording with what it holds: how long, who speaks,
// how it begins and how much people have worked on it. Reading a long transcript takes a moment,
// so the summary is kept until _final.json or _annotations.json changes.

// summary is what the library shows of a transcribed recording.
type summary struct {
	Duration float64   `json:"duration"`
	Speakers []speaker `json:"speakers"` // in the order they first speak; the preview begins with the first
	Preview  string    `json:"preview"`  // the first words
	Edits    int       `json:"edits"`    // corrections by hand
	Clips    int       `json:"clips"`
	Done     int64     `json:"done"` // when the transcript was last written (unix seconds)
}

// speaker is who speaks: id is the speaker's number, which picks the color as in the editor.
type speaker struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type summaryEntry struct {
	stamp string
	sum   summary
}

var summaries sync.Map // results dir -> summaryEntry

const previewLen = 240

// summarize reads the summary of the recording whose results are in rdir; ok is false if it has
// no transcript.
func summarize(rdir string) (summary, bool) {
	fst, err := os.Stat(filepath.Join(rdir, "_final.json"))
	if err != nil {
		return summary{}, false
	}
	stamp := fst.ModTime().String()
	if ast, err := os.Stat(filepath.Join(rdir, "_annotations.json")); err == nil {
		stamp += ast.ModTime().String()
	}
	if e, ok := summaries.Load(rdir); ok && e.(summaryEntry).stamp == stamp {
		return e.(summaryEntry).sum, true
	}
	var fin model.Final
	if fileutil.ReadJSON(filepath.Join(rdir, "_final.json"), &fin) != nil {
		return summary{}, false
	}
	s := summary{Duration: fin.Duration, Speakers: []speaker{}, Done: fst.ModTime().Unix()}
	// what people said about the speakers wins over _final.json, which the editor may not have
	// rewritten yet: their names, and who was merged into whom
	var ann model.Annotations
	if fileutil.ReadJSON(filepath.Join(rdir, "_annotations.json"), &ann) == nil {
		s.Edits, s.Clips = len(ann.Edits), len(ann.Clips)
	}
	who := func(id int) (int, string) {
		for range 8 { // a chain of merges, but never a loop
			info, ok := ann.Speakers[strconv.Itoa(id)]
			if !ok || info.MergeInto == "" {
				break
			}
			to, err := strconv.Atoi(info.MergeInto)
			if err != nil {
				break
			}
			id = to
		}
		key := strconv.Itoa(id)
		return id, cmp.Or(ann.Speakers[key].Name, fin.Speakers[key].Name, "S"+strconv.Itoa(id+1))
	}
	var text strings.Builder
	for _, seg := range fin.Segments {
		if seg.Speaker != nil {
			if id, name := who(*seg.Speaker); !slices.ContainsFunc(s.Speakers, func(p speaker) bool { return p.ID == id }) {
				s.Speakers = append(s.Speakers, speaker{id, name})
			}
		}
		if text.Len() < previewLen {
			if t := strings.TrimSpace(seg.Text); t != "" {
				if text.Len() > 0 {
					text.WriteByte(' ')
				}
				text.WriteString(t)
			}
		}
	}
	s.Preview = cut(text.String(), previewLen)
	summaries.Store(rdir, summaryEntry{stamp, s})
	return s, true
}

// cut shortens s to about n bytes at a word boundary, with an ellipsis.
func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	i := strings.LastIndexByte(s[:n], ' ')
	if i < n/2 {
		i = n
		for i > 0 && !utf8.RuneStart(s[i]) {
			i--
		}
	}
	return strings.TrimRight(s[:i], " ,;:") + " …"
}

// added is when the recording came into audio/ (unix seconds).
func added(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return time.Now().Unix()
	}
	return st.ModTime().Unix()
}

// deleteRecording handles POST /api/delete?name=<file in audio/>: the recording goes from the
// library with all it has in results/<stem>/, transcript and corrections included. A job of it
// still waiting in the queue is cancelled; while one runs, the recording stays (409).
func (s *Server) deleteRecording(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if !allowedPost(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" || strings.ContainsAny(name, `/\:`) || !media.IsMedia(name) {
		fail(w, 400, "unknown file")
		return
	}
	if st, err := os.Stat(filepath.Join(s.audioDir(), name)); err != nil || st.IsDir() {
		fail(w, 404, "unknown file")
		return
	}
	if err := s.Queue.Forget(name); errors.Is(err, queue.ErrBusy) {
		fail(w, 409, "Die Aufnahme wird gerade transkribiert. Brich das zuerst ab.")
		return
	}
	if err := s.removeRecording(name); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// removeRecording deletes audio/<name> and, unless another recording of the same stem is left,
// results/<stem>/.
func (s *Server) removeRecording(name string) error {
	if err := os.Remove(filepath.Join(s.audioDir(), name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	stem := media.Stem(name)
	if media.FindMedia(s.cfg.DataDir, stem) != "" {
		return nil
	}
	rdir := filepath.Join(s.resultsDir(), stem)
	summaries.Delete(rdir)
	return os.RemoveAll(rdir)
}

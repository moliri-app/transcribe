// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/moliri-app/transcribe/internal/fileutil"
)

// Kinds of notes.
const (
	KindAudio  = "audio"  // a program sent audio and got the text back
	KindMic    = "mic"    // a program had the app listen to the microphone
	KindDauer  = "dauer"  // a recording started on the Live page, for as long as it runs
	KindDiktat = "diktat" // dictated with the global shortcut, to be inserted where the cursor is
)

// Note is one request (or one recording on the Live page) and what was said in it, as it is
// kept in <data>/live/<id>.json.
type Note struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	Client        string     `json:"client,omitempty"` // who asked: the name it gave, else the address of its page
	Started       time.Time  `json:"started"`
	Ended         *time.Time `json:"ended,omitempty"`
	Lines         []Line     `json:"lines"`
	Partial       string     `json:"partial,omitempty"`       // the utterance going on
	PartialSource string     `json:"partialSource,omitempty"` // and where it is said (see Line.Source)
	Audio         string     `json:"audio,omitempty"`         // recording saved in audio/
	Language      string     `json:"language,omitempty"`      // chosen for the recording: auto or a code like de
	Source        string     `json:"source,omitempty"`        // what the app recorded (SourceMic, SourceSystem, SourceBoth); empty when a program sent the audio
	Error         string     `json:"error,omitempty"`
	Transient     bool       `json:"transient,omitempty"` // kept only for a while in memory, never saved (apinotes.go)
	Loading       bool       `json:"loading,omitempty"`   // recognition is still being loaded
	Waiting       bool       `json:"waiting,omitempty"`   // recognition is ready, the recording has not given sound yet
}

// Line is one utterance; its times count seconds since the note started.
type Line struct {
	Text   string  `json:"text"`
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Source string  `json:"source,omitempty"` // SourceMic or SourceSystem, when the recording mixes both
}

var sourceNames = map[string]string{SourceMic: "Mikrofon", SourceSystem: "Systemaudio"}

// Text is the note as plain text, one utterance per line; where the recording mixed microphone
// and system audio, each change of source is headed by its name.
func (n *Note) Text() string {
	var b strings.Builder
	source := ""
	for i, l := range n.Lines {
		if l.Source != "" && l.Source != source {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(sourceNames[l.Source] + ":\n")
			source = l.Source
		}
		b.WriteString(l.Text)
		if !strings.HasSuffix(l.Text, "\n") { // a line break said at its end is one already
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (n *Note) clone() *Note {
	c := *n
	c.Lines = append([]Line{}, n.Lines...)
	return &c
}

// byProgram reports whether notes of kind come from a program over the live API.
func byProgram(kind string) bool { return kind == KindAudio || kind == KindMic }

// keepNotes is how many notes the Live page gets; older ones stay on disk.
const keepNotes = 200

func notesDir(dataDir string) string { return filepath.Join(dataDir, "live") }

func validID(id string) bool {
	return id != "" && !strings.ContainsAny(id, `/\:.`+"\x00")
}

func saveNote(dataDir string, n *Note) error {
	if err := os.MkdirAll(notesDir(dataDir), 0o755); err != nil {
		return err
	}
	return fileutil.WriteJSON(filepath.Join(notesDir(dataDir), n.ID+".json"), n)
}

func removeNote(dataDir, id string) error {
	err := os.Remove(filepath.Join(notesDir(dataDir), id+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// loadNotes reads the newest notes, oldest first.
func loadNotes(dataDir string) []*Note {
	names := noteFiles(dataDir)
	if len(names) > keepNotes {
		names = names[len(names)-keepNotes:]
	}
	var out []*Note
	for _, name := range names {
		if n := readNote(dataDir, name); n != nil {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Started.Before(out[j].Started) })
	return out
}

// noteFiles are the names of the notes' files, oldest first (ids start with the time).
func noteFiles(dataDir string) []string {
	entries, _ := os.ReadDir(notesDir(dataDir))
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// readNote reads the note in file name, nil if it is broken. A note that never ended (the app was
// closed while it ran) ends where its last utterance does.
func readNote(dataDir, name string) *Note {
	b, err := os.ReadFile(filepath.Join(notesDir(dataDir), name))
	if err != nil {
		return nil
	}
	var n Note
	if json.Unmarshal(b, &n) != nil || !validID(n.ID) {
		return nil
	}
	n.Partial, n.Loading, n.Waiting = "", false, false
	if n.Lines == nil {
		n.Lines = []Line{}
	}
	if n.Ended == nil {
		end := n.Started
		if k := len(n.Lines); k > 0 {
			end = end.Add(time.Duration(n.Lines[k-1].End * float64(time.Second)))
		}
		n.Ended = &end
	}
	return &n
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"log"
	"slices"
	"strings"
	"time"

	"github.com/moliri-app/transcribe/internal/prefs"
)

// What programs transcribe over the live API is saved as a note like everything else, unless the
// settings say otherwise (prefs.LiveAPINoSave): then its note is only held in memory, while it
// runs and for transientKeep after it ended, so that the Live page shows it and the program can
// still read the text. Saved notes of programs can be
// deleted some time after they ended (prefs.LiveAPIKeepHours). Notes started on the Live page and
// dictations are not touched.

// transientKeep is how long a note that is not saved stays after it ended.
var transientKeep = 5 * time.Minute

// pruneEvery is how often old notes of programs are looked for.
const pruneEvery = 10 * time.Minute

// idTime is how a note's id begins: the time it started.
const idTime = "20060102-150405"

// save writes note n unless it is not to be saved; with h.mu held.
func (h *Hub) save(n *Note) {
	if n.Transient {
		return
	}
	if err := saveNote(h.dataDir, n); err != nil {
		log.Println("live: saving note:", err)
	}
}

// forgetLater drops note n transientKeep from now if it is not saved; with h.mu held.
func (h *Hub) forgetLater(n *Note) {
	if !n.Transient {
		return
	}
	time.AfterFunc(transientKeep, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.drop(n)
		removeImage(h.dataDir, n.ID)
	})
}

// drop takes note n off the Live page, its file aside; with h.mu held.
func (h *Hub) drop(n *Note) {
	if h.byID[n.ID] != n {
		return
	}
	delete(h.byID, n.ID)
	h.notes = slices.DeleteFunc(h.notes, func(x *Note) bool { return x == n })
	h.broadcastLocked(map[string]any{"type": "removed", "id": n.ID})
}

// SetAPINotes sets whether the notes of programs are saved and how many hours after they ended
// they are deleted (0 keeps them); nil leaves either as it is. It applies to notes starting from
// now; deleting, to those saved before too.
func (h *Hub) SetAPINotes(save *bool, keepHours *int) error {
	p, err := prefs.Update(h.dataDir, func(p *prefs.Prefs) {
		if save != nil {
			p.LiveAPINoSave = !*save
		}
		if keepHours != nil {
			p.LiveAPIKeepHours = *keepHours
		}
	})
	if err != nil {
		return err
	}
	h.broadcast(map[string]any{"type": "apiNotes", "save": !p.LiveAPINoSave, "keepHours": p.LiveAPIKeepHours})
	go h.prune()
	return nil
}

func (h *Hub) pruneLoop() {
	for {
		h.prune()
		time.Sleep(pruneEvery)
	}
}

// prune deletes the saved notes of programs that ended longer ago than the settings keep them,
// also those too old for the Live page to show.
func (h *Hub) prune() {
	h.pruneMu.Lock()
	defer h.pruneMu.Unlock()
	hours := prefs.Load(h.dataDir).LiveAPIKeepHours
	if hours <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	for _, name := range noteFiles(h.dataDir) {
		id := strings.TrimSuffix(name, ".json")
		if len(id) >= len(idTime) {
			if t, err := time.ParseInLocation(idTime, id[:len(idTime)], time.Local); err == nil && !t.Before(cutoff) {
				break // the ones after it started later still
			}
		}
		n := readNote(h.dataDir, name)
		if n == nil || n.ID != id || !byProgram(n.Kind) || !n.Ended.Before(cutoff) {
			continue
		}
		h.mu.Lock()
		m := h.byID[id]
		if m == nil || (m.Ended != nil && m.Ended.Before(cutoff)) {
			if err := removeNote(h.dataDir, id); err != nil {
				log.Println("live: deleting an old note:", err)
			} else if m != nil {
				h.drop(m)
			}
		}
		h.mu.Unlock()
	}
}

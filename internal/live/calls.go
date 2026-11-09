// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moliri-app/transcribe/internal/prefs"
)

// Besides transcriptions, programs use the app's other services: text recognition, the filter in
// plain words, search by meaning. Each such request is a call: a note of its own kind that holds
// what was asked, what came back, which engine or model answered and what helps to see what
// happened (sizes, times, tokens). The Live page lists calls with the transcriptions; they are
// kept and deleted like the notes of programs (apinotes.go). Their ids end with their kind, so
// that a flood of calls does not push the recordings off the Live page: each keeps keepNotes.

// Kinds of calls.
const (
	KindOCR    = "ocr"    // text recognition read an image
	KindFilter = "filter" // the language model turned words into a filter
	KindEmbed  = "embed"  // texts were turned into vectors for search by meaning
)

// isCall reports whether notes of kind are calls.
func isCall(kind string) bool { return kind == KindOCR || kind == KindFilter || kind == KindEmbed }

// callID reports whether the note with this id (or file name) is a call.
func callID(id string) bool {
	id = strings.TrimSuffix(id, ".json")
	i := strings.LastIndexByte(id, '-')
	return i >= 0 && isCall(id[i+1:])
}

// Fact is something about a call worth knowing when looking into it, such as how long it took.
type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// maxCallText is how much of a request or response a call keeps.
const maxCallText = 20000

func clip(s string) string {
	if len(s) <= maxCallText {
		return s
	}
	cut := maxCallText
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return fmt.Sprintf("%s\n… (gekürzt, insgesamt %d Zeichen)", s[:cut], utf8.RuneCountInString(s))
}

// Call is a request to a service of the app, as the Live page shows it. A nil *Call does nothing,
// so that servers without a hub need not check.
type Call struct {
	h *Hub
	n *Note
}

// Call notes that a program (client) asks service kind with request, to be answered by model.
func (h *Hub) Call(kind, client, model, request string, facts ...Fact) *Call {
	if h == nil {
		return nil
	}
	transient := prefs.Load(h.dataDir).LiveAPINoSave
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	now := time.Now()
	n := &Note{ID: fmt.Sprintf("%s-%d-%s", now.Format(idTime), h.seq, kind), Kind: kind, Client: client, Started: now,
		Lines: []Line{}, Model: model, Request: clip(request), Facts: facts, Transient: transient}
	h.add(n)
	h.sendNote(n)
	h.save(n)
	return &Call{h: h, n: n}
}

// Logged notes a call that is over as soon as it is asked, as one that was refused.
func (h *Hub) Logged(kind, client, model, request string, err error, facts ...Fact) {
	h.Call(kind, client, model, request).End("", err, facts...)
}

// Status says what the call is doing now ("" when nothing worth saying), for the Live page.
func (c *Call) Status(s string) {
	if c == nil {
		return
	}
	c.h.mu.Lock()
	defer c.h.mu.Unlock()
	if c.n.Ended != nil || c.n.Partial == s {
		return
	}
	c.n.Partial = s
	c.h.broadcastLocked(map[string]any{"type": "partial", "id": c.n.ID, "text": s})
}

// Model names the engine or model that answers, once it is known.
func (c *Call) Model(model string) {
	if c == nil {
		return
	}
	c.h.mu.Lock()
	defer c.h.mu.Unlock()
	c.n.Model = model
	c.h.sendNote(c.n)
}

// End finishes the call with the response, or err; facts come after those given when it began.
func (c *Call) End(response string, err error, facts ...Fact) {
	if c == nil {
		return
	}
	h, n := c.h, c.n
	h.mu.Lock()
	defer h.mu.Unlock()
	if n.Ended != nil {
		return
	}
	now := time.Now()
	n.Ended, n.Partial = &now, ""
	n.Response = clip(response)
	if err != nil {
		n.Error = err.Error()
	}
	n.Facts = append(n.Facts, facts...)
	h.sendNote(n)
	h.save(n)
	h.forgetLater(n)
}

// add puts n on the Live page; with h.mu held. Beyond keepNotes, the oldest of its group (calls
// or the others) goes.
func (h *Hub) add(n *Note) {
	h.notes = append(h.notes, n)
	h.byID[n.ID] = n
	call := isCall(n.Kind)
	count := 0
	for _, x := range h.notes {
		if isCall(x.Kind) == call {
			count++
		}
	}
	if count <= keepNotes {
		return
	}
	for i, x := range h.notes {
		if isCall(x.Kind) == call {
			h.notes = append(h.notes[:i:i], h.notes[i+1:]...)
			delete(h.byID, x.ID)
			return
		}
	}
}

// Seconds says a duration for people: "0,4 s", "12 s", "2 min 5 s".
func Seconds(d time.Duration) string {
	s := d.Seconds()
	switch {
	case s < 10:
		return strings.Replace(fmt.Sprintf("%.1f s", s), ".", ",", 1)
	case s < 60:
		return fmt.Sprintf("%.0f s", s)
	}
	return fmt.Sprintf("%d min %d s", int(s)/60, int(s)%60)
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/moliri-app/transcribe/internal/live"
	"github.com/moliri-app/transcribe/internal/prefs"
)

// Dictating anywhere: a global shortcut records the microphone until it is pressed again (Esc
// cancels), then inserts what was said where the cursor is, in whatever program is in front, and
// puts it into the clipboard; the settings can turn off either or both. A small window above the
// taskbar shows that it listens and what it understood, like a subtitle; a click on it opens the
// dictation on the Live page, where every dictation is kept. Only Windows has the shortcut; it
// needs the tray icon, whose window receives it.

var errNoTray = errors.New("Das Tastenkürzel wirkt nur, solange Transcribe mit seinem Symbol im Infobereich läuft.")

// dictation is the shortcut and the dictation going on; it is the server's Dictation.
type dictation struct {
	hub  *live.Hub
	data string

	mu   sync.Mutex
	tray *tray
	err  error      // why the shortcut does not work
	on   *dictating // the dictation going on
}

type dictating struct {
	key     string         // the shortcut that ends it
	l       *live.Listener // nil while it starts
	stop    bool           // the shortcut came again while it started
	cancel  bool           // Esc came while it started
	heard   bool           // the microphone gives sound
	lines   []string
	partial string
}

// overlay is what the small window shows; an empty title hides it.
type overlay struct {
	dot   uint32 // colour of the dot before the title (RGB)
	title string
	hint  string // beside the title, smaller
	body  string // below, up to two lines
	note  string // the dictation's note, which a click opens on the Live page
}

const (
	dotWait   = 0x8a8a8a
	dotListen = 0xe5484d
	dotDone   = 0x62eb00
)

// attach hands over the tray icon (nil if there is none) and registers the shortcut.
func (d *dictation) attach(t *tray) {
	d.mu.Lock()
	d.tray = t
	d.mu.Unlock()
	if err := d.Apply(); err != nil {
		log.Println("dictate:", err)
	}
}

// Apply registers the shortcut of the settings anew.
func (d *dictation) Apply() error {
	d.mu.Lock()
	t := d.tray
	d.mu.Unlock()
	key := prefs.Load(d.data).DictateKey
	var err error
	switch {
	case t == nil:
		if key != "off" {
			err = errNoTray
		}
	case key == "off":
		t.hotkey("")
	default:
		err = t.hotkey(key)
	}
	d.mu.Lock()
	d.err = err
	d.mu.Unlock()
	return err
}

func (d *dictation) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

// toggle starts a dictation, or ends the one going on and inserts its text.
func (d *dictation) toggle() {
	d.mu.Lock()
	if on := d.on; on != nil {
		if on.l == nil {
			on.stop = true // it ends as soon as it has started
			d.mu.Unlock()
			return
		}
		d.mu.Unlock()
		d.end(on, false)
		return
	}
	on := &dictating{key: prefs.Load(d.data).DictateKey}
	d.on = on
	t := d.tray
	d.mu.Unlock()

	t.escape(true)
	t.show(overlay{dot: dotWait, title: "Diktat wird vorbereitet …", body: "Einen Moment, gleich kannst du sprechen."}, 0)
	began := time.Now()
	l, err := d.hub.Dictate(context.Background(), func(ev live.Event) { d.heard(on, ev) })
	log.Printf("dictate: recognition ready after %.1f s", time.Since(began).Seconds())
	if err != nil {
		d.mu.Lock()
		d.on = nil
		d.mu.Unlock()
		t.escape(false)
		t.show(overlay{dot: dotWait, title: "Diktieren ging nicht", body: err.Error()}, 6*time.Second)
		return
	}
	d.mu.Lock()
	on.l = l
	stop, cancel := on.stop, on.cancel
	d.mu.Unlock()
	if stop || cancel {
		d.end(on, cancel)
		return
	}
	go func() {
		select {
		case <-l.Heard():
			log.Printf("dictate: listening after %.1f s", time.Since(began).Seconds())
			d.mu.Lock()
			on.heard = true
			d.mu.Unlock()
			d.update(on)
		case <-l.Done():
		}
	}()
	go func() {
		<-l.Done()
		d.end(on, false) // the recording ended by itself (the microphone failed): what came is inserted
	}()
}

// cancel ends the dictation going on without inserting anything.
func (d *dictation) cancel() {
	d.mu.Lock()
	on := d.on
	if on != nil && on.l == nil {
		on.cancel = true
		on = nil
	}
	d.mu.Unlock()
	if on != nil {
		d.end(on, true)
	}
}

func (d *dictation) heard(on *dictating, ev live.Event) {
	d.mu.Lock()
	if ev.Final {
		if s := strings.Trim(ev.Text, " "); s != "" { // a line break said at its end stays
			on.lines = append(on.lines, s)
		}
		on.partial = ""
	} else {
		on.partial = strings.Trim(ev.Text, " ")
	}
	d.mu.Unlock()
	d.update(on)
}

// update shows what the dictation going on has understood.
func (d *dictation) update(on *dictating) {
	d.mu.Lock()
	if d.on != on {
		d.mu.Unlock()
		return
	}
	t := d.tray
	text := join(append(on.lines[:len(on.lines):len(on.lines)], on.partial))
	heard := on.heard
	d.mu.Unlock()
	o := overlay{dot: dotListen, title: "Ich höre zu", hint: prefs.DictateLabel(on.key) + ": fertig   Esc: abbrechen", body: tail(text), note: on.l.Note()}
	if !heard {
		o.dot, o.title, o.body = dotWait, "Mikrofon startet …", "Gleich kannst du sprechen."
	} else if text == "" {
		o.body = "Sprich jetzt."
	}
	t.show(o, 0)
}

// end stops the dictation; unless cancel, its text goes where the settings say.
func (d *dictation) end(on *dictating, cancel bool) {
	d.mu.Lock()
	if d.on != on {
		d.mu.Unlock()
		return
	}
	d.on = nil
	t := d.tray
	d.mu.Unlock()
	t.escape(false)
	p := prefs.Load(d.data)
	if !cancel {
		t.show(overlay{dot: dotWait, title: "Einen Moment …", body: "Das zuletzt Gesagte wird noch erkannt."}, 0)
	}
	on.l.Stop() // what is still being said is recognised first
	d.mu.Lock()
	text := join(on.lines)
	d.mu.Unlock()
	if cancel || text == "" {
		// nothing to keep: the Live page does not need to list it
		if err := d.hub.Delete(on.l.Note()); err != nil {
			log.Println("dictate:", err)
		}
	}
	switch {
	case cancel:
		t.show(overlay{dot: dotWait, title: "Diktat abgebrochen", body: "Es wurde nichts eingefügt."}, 2*time.Second)
	case text == "" && on.l.Err() != nil:
		t.show(overlay{dot: dotWait, title: "Diktieren ging nicht", body: on.l.Err().Error()}, 6*time.Second)
	case text == "":
		t.show(overlay{dot: dotWait, title: "Nichts verstanden", body: "Es kam kein Text an. " + prefs.DictateLabel(on.key) + " startet ein neues Diktat."}, 4*time.Second)
	default:
		insert, clipboard := !p.DictateNoInsert, !p.DictateNoClipboard
		if clipboard {
			if err := setClipboard(text); err != nil {
				log.Println("dictate: clipboard:", err)
				t.show(overlay{dot: dotWait, title: "Zwischenablage ging nicht", body: err.Error(), note: on.l.Note()}, 6*time.Second)
				return
			}
		}
		o := overlay{dot: dotDone, body: tail(text), note: on.l.Note()}
		switch {
		case insert && clipboard:
			paste() // Ctrl+V: one step to undo in the program
			o.title, o.hint = "Eingefügt", "liegt auch in der Zwischenablage"
		case insert:
			typeText(text) // typed, so the clipboard stays as it was
			o.title = "Eingefügt"
		case clipboard:
			o.title, o.hint = "In der Zwischenablage", "Strg + V: einfügen"
		default:
			o.title, o.hint = "Auf der Seite Live gespeichert", "Klick: dort öffnen"
		}
		shown := 3 * time.Second
		if !insert {
			shown = 8 * time.Second // time to read it, or to click it
		}
		t.show(o, shown)
	}
}

// join puts the utterances together into one text. Each is recognised on its own, so one that
// starts a sentence may begin in lower case; it gets a capital. A mark said at the start of one
// (live.Attach) ends the one before.
func join(utterances []string) string {
	var b strings.Builder
	for _, u := range utterances {
		if live.StartsWithMark(u) {
			prev, rest := live.Attach(b.String(), u)
			b.Reset()
			b.WriteString(prev)
			u = strings.TrimSpace(rest)
		}
		if u == "" {
			continue
		}
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteByte(' ')
		}
		prev := strings.TrimRight(b.String(), " ")
		if prev == "" || strings.ContainsAny(prev[len(prev)-1:], ".?!\n") {
			r := []rune(u)
			r[0] = unicode.ToUpper(r[0])
			u = string(r)
		}
		b.WriteString(u)
	}
	return b.String()
}

// tail is the end of text, as much as the window shows in two lines.
func tail(text string) string {
	const most = 130
	r := []rune(text)
	if len(r) <= most {
		return text
	}
	end := string(r[len(r)-most:])
	if i := strings.IndexByte(end, ' '); i >= 0 { // from a whole word on
		end = end[i+1:]
	}
	return "…" + end
}

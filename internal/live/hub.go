// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/prefs"
)

// Hub runs the live transcriptions, keeps their notes and tells the Live page about both.
//
// A program that sends audio gets a stream of its own. The microphone is recorded once, however
// many listen: every program that listens and the recording started on the Live page are taps on
// the same run, each getting what was said from the moment it joined.
type Hub struct {
	dataDir string
	open    Opener
	// Warmer keeps live recognition loaded while the live mode is on (mode.go); nil for none. Set
	// it before serving.
	Warmer Warmer

	mu       sync.Mutex
	notes    []*Note // oldest first
	byID     map[string]*Note
	subs     map[chan []byte]struct{}
	watching bool // watchDevices runs
	blocked  []Refusal
	active   int // notes still running
	seq      int
	warm     string // WarmLoading, WarmReady or ""

	warmMu      sync.Mutex
	warmWant    string  // the language Warm asked for last, "" for nothing
	warmHave    *string // what is loaded; nil while it is not known
	warmRunning bool    // warmLoop runs

	pruneMu sync.Mutex // one look for old notes at a time

	micMu sync.Mutex // starting and stopping the microphone
	mic   *run
	dauer *tap
}

// Refusal is a page that asked for live transcription without being allowed to.
type Refusal struct {
	Origin string    `json:"origin"`
	Client string    `json:"client,omitempty"`
	At     time.Time `json:"at"`
}

func NewHub(dataDir string, open Opener) *Hub {
	h := &Hub{dataDir: dataDir, open: open, byID: map[string]*Note{}, subs: map[chan []byte]struct{}{}}
	h.notes = loadNotes(dataDir)
	_ = os.RemoveAll(filepath.Join(notesDir(dataDir), "tmp")) // left by versions with a second pass
	for _, n := range h.notes {
		h.byID[n.ID] = n
	}
	go h.pruneLoop()
	return h
}

// Busy reports whether a transcription is running (the app must not quit then).
func (h *Hub) Busy() bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.active > 0
}

// ---- runs and taps ----

type run struct {
	h        *Hub
	stream   Stream
	audio    chan chunk  // closed by its only writer when the audio ends
	ctl      chan func() // runs inside the loop
	done     chan struct{}
	taps     map[*tap]struct{}  // owned by the loop
	pos      int                // samples given to the stream
	source   string             // the recording: what it records (SourceMic, ...)
	language string             // the recording: what is spoken, auto or a code like de
	cancel   context.CancelFunc // the recording: stops it
	err      error              // why the recording ended, read after done
	loud     [][2]float32       // mixing both sources: how loud each was, per 100 ms
	hearing  bool               // the Live page was told that speech waits for recognition
	check    inputCheck         // the recording: how good the microphone is
	input    string             // and what is wrong with it (InputNarrow, ...), owned by the loop
}

// chunk is the next piece of audio; mic and system say how loud each source was in it (mean
// square) when the recording mixes both, micPCM is what the microphone gave when it is recorded.
type chunk struct {
	pcm         []float32
	mic, system float32
	micPCM      []float32
}

type tap struct {
	note  *Note
	from  int           // run.pos when it joined
	send  func(Event)   // the program asking; nil for the Live page
	wav   *wavWriter    // the recording is saved
	heard chan struct{} // the recording: closed once it gives sound for this tap
	ended chan struct{} // closed once the note is finished
	err   error
}

// newRun starts recognition on s; queue is how many pieces of audio may wait for it, source what
// the recording records ("" for a program's own audio).
func newRun(h *Hub, s Stream, queue int, source string) *run {
	r := &run{h: h, stream: s, source: source, audio: make(chan chunk, queue), ctl: make(chan func()), done: make(chan struct{}), taps: map[*tap]struct{}{}}
	go r.loop()
	return r
}

func (r *run) loop() {
	defer close(r.done)
	defer r.stream.Close()
	for {
		select {
		case c, ok := <-r.audio:
			if !ok {
				r.deliver(r.stream.Flush())
				return
			}
			r.take(c)
		case f := <-r.ctl:
			f()
		}
	}
}

// take hands the next piece of audio to recognition and what it recognised to the taps.
func (r *run) take(c chunk) {
	pcm := c.pcm
	if r.pos == 0 { // the first sound: whoever waited for it is listening from now on
		for t := range r.taps {
			r.h.heard(t.note)
			if t.heard != nil {
				close(t.heard)
			}
		}
	}
	r.pos += len(pcm)
	if r.source == SourceBoth {
		r.loud = append(r.loud, [2]float32{c.mic, c.system})
	}
	for t := range r.taps {
		if t.wav != nil {
			if err := t.wav.Write(pcm); err != nil {
				log.Println("live: saving the recording:", err)
				t.wav.Close()
				t.wav = nil
			}
		}
	}
	evs := r.stream.Accept(pcm)
	r.deliver(evs)
	if len(r.audio) == 0 {
		if ev, ok := r.stream.Preview(); ok {
			evs = append(evs, ev)
			r.deliver([]Event{ev})
		}
	}
	speech := r.tellHearing(len(evs) > 0)
	if c.micPCM != nil {
		// mixing both sources, the microphone counts when it was the louder one
		if v := r.check.add(c.micPCM, speech && (r.source != SourceBoth || c.mic >= c.system)); v != r.input {
			r.input = v
			go r.h.sendMic()
		}
	}
}

// tellHearing tells the Live page how much speech waits for recognition when that changes: speech
// began or ended, or text came for some of it. It reports whether speech is going on.
func (r *run) tellHearing(text bool) bool {
	hs, ok := r.stream.(Hearer)
	if !ok {
		return false
	}
	from, to, on := hs.Pending()
	if on == r.hearing && !text {
		return on
	}
	r.hearing = on
	pending := 0.0
	if on {
		pending = math.Round(max(0, to-from)*100) / 100
	}
	for t := range r.taps {
		r.h.broadcast(map[string]any{"type": "hearing", "id": t.note.ID, "speaking": on, "pending": pending})
	}
	return on
}

// inputChanged forgets what was wrong with the microphone: another one was chosen.
func (r *run) inputChanged() {
	r.check, r.input = inputCheck{}, ""
}

// drain recognises the audio recorded until now that still waits for recognition (on a busy
// computer, recognition falls behind); inside the loop.
func (r *run) drain() {
	for {
		select {
		case c, ok := <-r.audio:
			if !ok {
				return // the loop flushes and ends when it sees this too
			}
			r.take(c)
		default:
			return
		}
	}
}

// do runs f inside the loop and waits for it; false if the run has ended.
func (r *run) do(f func()) bool {
	ran := make(chan struct{})
	select {
	case r.ctl <- func() { f(); close(ran) }:
		<-ran
		return true
	case <-r.done:
		return false
	}
}

func (r *run) deliver(evs []Event) {
	for _, ev := range evs {
		ev.Source = r.whose(ev.Start, ev.End)
		for t := range r.taps {
			from := float64(t.from) / SR
			if ev.End <= from {
				continue // said before this tap joined
			}
			e := ev
			e.Start = math.Round(max(0, e.Start-from)*100) / 100
			e.End = math.Round((e.End-from)*100) / 100
			r.h.record(t.note, e)
			if t.send != nil {
				t.send(e)
			}
		}
	}
}

// whose is the source that was louder from start to end (seconds), when the recording mixes
// both; mixing hands out pieces of 100 ms.
func (r *run) whose(start, end float64) string {
	if r.source != SourceBoth {
		return ""
	}
	var mic, system float64
	for i := max(0, int(start*10)); i < min(len(r.loud), int(math.Ceil(end*10))); i++ {
		mic += float64(r.loud[i][0])
		system += float64(r.loud[i][1])
	}
	if system > mic {
		return SourceSystem
	}
	return SourceMic
}

func meanSquare(pcm []float32) float32 {
	var s float64
	for _, v := range pcm {
		s += float64(v) * float64(v)
	}
	return float32(s / float64(max(1, len(pcm))))
}

// level is how loud pcm is: about -50 dB to 0 dB onto 0..1.
func level(pcm []float32) float64 {
	var s float64
	for _, v := range pcm {
		s += float64(v) * float64(v)
	}
	rms := math.Sqrt(s / float64(max(1, len(pcm))))
	return math.Round(max(0, min(1, 1+20*math.Log10(max(rms, 1e-6))/50))*100) / 100
}

// ---- notes ----

func (h *Hub) newNote(kind, client string) *Note {
	transient := byProgram(kind) && prefs.Load(h.dataDir).LiveAPINoSave
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	now := time.Now()
	n := &Note{ID: fmt.Sprintf("%s-%d", now.Format(idTime), h.seq), Kind: kind, Client: client, Started: now, Lines: []Line{}, Loading: true, Transient: transient}
	h.notes = append(h.notes, n)
	h.byID[n.ID] = n
	if len(h.notes) > keepNotes {
		old := h.notes[0]
		h.notes = h.notes[1:]
		delete(h.byID, old.ID)
	}
	h.active++
	h.sendNote(n)
	return n
}

// ready marks the note as listening: it starts now, not when loading began.
func (h *Hub) ready(n *Note) {
	h.mu.Lock()
	defer h.mu.Unlock()
	n.Loading, n.Started = false, time.Now()
	h.sendNote(n)
	h.save(n)
}

// waiting marks the note as ready for sound that has not come yet (a microphone takes a moment
// to start, a Bluetooth headset several seconds).
func (h *Hub) waiting(n *Note) {
	h.mu.Lock()
	defer h.mu.Unlock()
	n.Loading, n.Waiting = false, true
	h.sendNote(n)
	h.save(n)
}

// heard ends the waiting of a note: it starts with the first sound.
func (h *Hub) heard(n *Note) {
	h.mu.Lock()
	on := n.Waiting
	n.Waiting = false
	h.mu.Unlock()
	if on {
		h.ready(n)
	}
}

func (h *Hub) record(n *Note, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !ev.Final {
		n.Partial, n.PartialSource = ev.Text, ev.Source
		h.broadcastLocked(map[string]any{"type": "partial", "id": n.ID, "text": ev.Text, "source": ev.Source})
		return
	}
	n.Partial, n.PartialSource = "", ""
	text := ev.Text
	if StartsWithMark(text) {
		// a mark said at the start of the utterance ends the line before (marks.go)
		prev := ""
		if k := len(n.Lines) - 1; k >= 0 {
			prev = n.Lines[k].Text
		}
		prev, text = Attach(prev, text)
		if k := len(n.Lines) - 1; k >= 0 {
			n.Lines[k].Text = prev
		}
		text = strings.TrimSpace(text)
	}
	if text != ev.Text {
		if text != "" {
			n.Lines = append(n.Lines, Line{Text: text, Start: ev.Start, End: ev.End, Source: ev.Source})
		}
		h.sendNote(n)
	} else {
		l := Line{Text: text, Start: ev.Start, End: ev.End, Source: ev.Source}
		n.Lines = append(n.Lines, l)
		h.broadcastLocked(map[string]any{"type": "line", "id": n.ID, "line": l})
	}
	h.save(n)
}

// finish ends the tap's note; err says why, if it did not end as asked.
func (h *Hub) finish(t *tap, err error) {
	empty := false
	if t.wav != nil {
		path := t.wav.f.Name()
		empty = t.wav.n == 0
		if cerr := t.wav.Close(); cerr != nil {
			log.Println("live: saving the recording:", cerr)
		}
		if empty { // no sound came: there is nothing to keep or transcribe
			if rerr := os.Remove(path); rerr != nil {
				log.Println("live: removing the empty recording:", rerr)
			}
		}
		t.wav = nil
	}
	h.mu.Lock()
	now := time.Now()
	n := t.note
	if empty {
		n.Audio = ""
		if err == nil {
			err = errNoSound
		}
	}
	n.Ended, n.Partial, n.PartialSource, n.Loading, n.Waiting = &now, "", "", false, false
	if err != nil {
		n.Error = err.Error()
	}
	h.active--
	h.sendNote(n)
	h.save(n)
	h.forgetLater(n)
	h.mu.Unlock()
	t.err = err
	close(t.ended)
	h.Warm() // the recording may have downloaded what could not be kept loaded before
}

// Notes are the notes the Live page shows, oldest first.
func (h *Hub) Notes() []*Note {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*Note, len(h.notes))
	for i, n := range h.notes {
		out[i] = n.clone()
	}
	return out
}

// Note returns a copy of one note, nil if there is none with that id.
func (h *Hub) Note(id string) *Note {
	h.mu.Lock()
	defer h.mu.Unlock()
	if n := h.byID[id]; n != nil {
		return n.clone()
	}
	return nil
}

// Delete removes a note that has ended (the recording it saved stays in audio/).
func (h *Hub) Delete(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := h.byID[id]
	if n == nil {
		return errors.New("Diese Notiz gibt es nicht.")
	}
	if n.Ended == nil {
		return errors.New("Die Notiz läuft noch.")
	}
	if err := removeNote(h.dataDir, id); err != nil {
		return err
	}
	h.drop(n)
	return nil
}

// ---- a program sends audio ----

// Session is a program's own audio being transcribed.
type Session struct {
	h *Hub
	r *run
	t *tap
}

// Note is the id of the session's note.
func (s *Session) Note() string { return s.t.note.ID }

// Audio opens a session for client; send gets what is recognised. It returns once recognition
// is loaded. The background mode does not matter: what a program sends is always transcribed.
func (h *Hub) Audio(ctx context.Context, client string, send func(Event)) (*Session, error) {
	n := h.newNote(KindAudio, client)
	t := &tap{note: n, send: send, ended: make(chan struct{})}
	stream, err := h.open(ctx, "auto")
	if err != nil {
		h.finish(t, err)
		return nil, err
	}
	r := newRun(h, stream, 64, "")
	r.do(func() { r.taps[t] = struct{}{} })
	h.ready(n)
	return &Session{h: h, r: r, t: t}, nil
}

// Write hands the next samples (16 kHz mono) to recognition.
func (s *Session) Write(pcm []float32) { s.r.audio <- chunk{pcm: pcm} }

// End recognises what is left, sends it and ends the note; err says why if the program left.
func (s *Session) End(err error) {
	close(s.r.audio)
	<-s.r.done
	s.h.finish(s.t, err)
}

// ---- the recording ----
//
// The app records the source chosen on the Live page (the microphone, what the computer plays, or
// both mixed) for the Live page and for the programs listening.

// Listener is a program that has the app listen to the recording.
type Listener struct {
	h *Hub
	t *tap
}

// Note is the id of the listener's note.
func (l *Listener) Note() string { return l.t.note.ID }

// Done is closed when the note has ended, on Stop or because recording failed (Err says why).
func (l *Listener) Done() <-chan struct{} { return l.t.ended }

func (l *Listener) Err() error { return l.t.err }

// Heard is closed once the recording gives sound: from then on, what is said is recognised.
func (l *Listener) Heard() <-chan struct{} { return l.t.heard }

// Stop ends the utterance going on and the note: what was said until now is sent.
func (l *Listener) Stop() { l.h.untap(l.t, nil) }

// Listen records for client from now on; send gets what is recognised.
func (h *Hub) Listen(ctx context.Context, client string, send func(Event)) (*Listener, error) {
	t, err := h.tapMic(ctx, KindMic, client, send, "", "")
	if err != nil {
		return nil, err
	}
	return &Listener{h: h, t: t}, nil
}

// Dictate records the microphone for a dictation with the global shortcut; send gets what is
// recognised. Unless a recording runs already, it records the microphone alone, whatever the
// Live page records.
func (h *Hub) Dictate(ctx context.Context, send func(Event)) (*Listener, error) {
	t, err := h.tapMic(ctx, KindDiktat, "", send, "", SourceMic)
	if err != nil {
		return nil, err
	}
	return &Listener{h: h, t: t}, nil
}

// StartDauer records from the Live page until StopDauer; with save, the
// recording goes to audio/ as well, to be transcribed in full afterwards.
func (h *Hub) StartDauer(ctx context.Context, save bool) error {
	h.micMu.Lock()
	busy := h.dauer != nil
	h.micMu.Unlock()
	if busy {
		return errors.New("Die Aufnahme läuft schon.")
	}
	wav := ""
	if save {
		wav = h.recordingName()
	}
	t, err := h.tapMic(ctx, KindDauer, "", nil, wav, "")
	if err != nil {
		return err
	}
	h.micMu.Lock()
	h.dauer = t
	h.micMu.Unlock()
	h.sendMic()
	go func() {
		<-t.ended
		h.micMu.Lock()
		if h.dauer == t {
			h.dauer = nil
		}
		h.micMu.Unlock()
		h.sendMic()
	}()
	return nil
}

// StopDauer ends the recording started on the Live page.
func (h *Hub) StopDauer() {
	h.micMu.Lock()
	t := h.dauer
	h.micMu.Unlock()
	if t != nil {
		h.untap(t, nil)
		<-t.ended
	}
}

// recordingName is a free name in audio/ for a recording starting now.
func (h *Hub) recordingName() string {
	base := "Live " + time.Now().Format("2006-01-02 15-04")
	name := base + ".wav"
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(media.AudioDir(h.dataDir), name)); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s (%d).wav", base, i)
	}
}

// tapMic joins the recording, starting it if none runs: of source, or of what the Live page
// records if source is "".
func (h *Hub) tapMic(ctx context.Context, kind, client string, send func(Event), wav, source string) (*tap, error) {
	// the background mode only decides whether programs may have the app record; the Live page and
	// dictating record either way
	if kind == KindMic && !h.Enabled() {
		return nil, errOff
	}
	n := h.newNote(kind, client)
	t := &tap{note: n, send: send, heard: make(chan struct{}), ended: make(chan struct{})}
	if wav != "" {
		w, err := createWav(filepath.Join(media.AudioDir(h.dataDir), wav))
		if err != nil {
			h.finish(t, err)
			return nil, err
		}
		t.wav = w
		h.mu.Lock()
		n.Audio = wav
		h.mu.Unlock()
	}
	h.micMu.Lock()
	if h.mic == nil {
		p := prefs.Load(h.dataDir)
		stream, err := h.open(ctx, cmp.Or(p.LiveLanguage, "auto"))
		if err != nil {
			h.micMu.Unlock()
			h.finish(t, err)
			return nil, err
		}
		h.mic = h.startMic(stream, p, cmp.Or(source, NormSource(p.LiveSource)))
	}
	r := h.mic
	if source == SourceMic && r.source == SourceSystem {
		h.micMu.Unlock()
		h.finish(t, errSystemOnly)
		return nil, errSystemOnly
	}
	h.mu.Lock()
	n.Language, n.Source = r.language, r.source
	h.mu.Unlock()
	waits := false
	ok := r.do(func() {
		t.from = r.pos
		r.taps[t] = struct{}{}
		if waits = r.pos == 0; waits { // in the loop, so that the first sound cannot come in between
			h.waiting(n)
		} else {
			close(t.heard)
		}
	})
	h.micMu.Unlock()
	if !ok {
		err := r.err
		if err == nil {
			err = errEnded
		}
		h.finish(t, err)
		return nil, err
	}
	if !waits {
		h.ready(n)
	}
	h.sendMic()
	return t, nil
}

func (h *Hub) startMic(stream Stream, p prefs.Prefs, source string) *run {
	// When recognition falls behind (a busy computer), the recording waits for it here rather
	// than stall: a microphone that is not read loses sound, and so does the mixing.
	r := newRun(h, stream, 3000, source) // five minutes
	r.language = cmp.Or(p.LiveLanguage, "auto")
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go func() {
		pieces := 0
		r.err = capture(ctx, r.source, p.LiveDevice, func(pcm, mic, system []float32) {
			if pieces++; pieces%2 == 0 { // the Live page shows how loud each source is, five times a second
				lv := map[string]float64{}
				if mic != nil {
					lv[SourceMic] = level(mic)
				}
				if system != nil {
					lv[SourceSystem] = level(system)
				}
				h.broadcast(map[string]any{"type": "level", "levels": lv})
			}
			c := chunk{pcm: pcm, micPCM: mic}
			if mic != nil && system != nil {
				c.mic, c.system = meanSquare(mic), meanSquare(system)
			}
			r.audio <- c
		})
		close(r.audio)
	}()
	go func() {
		<-r.done
		cancel()
		h.micMu.Lock()
		if h.mic == r {
			h.mic = nil
		}
		h.micMu.Unlock()
		// recording failed: whoever still listens is told why
		err := r.err
		if err == nil {
			err = errEnded
		}
		for t := range r.taps {
			h.finish(t, err)
		}
		h.sendMic()
	}()
	return r
}

// untap ends a tap on the microphone: the utterance going on is ended and handed out first.
func (h *Hub) untap(t *tap, err error) {
	h.micMu.Lock()
	r := h.mic
	last := false
	ok := r != nil && r.do(func() {
		if _, on := r.taps[t]; !on {
			return
		}
		r.drain() // what was said up to now counts, even if recognition is behind
		r.deliver(r.stream.Flush())
		delete(r.taps, t)
		last = len(r.taps) == 0
		h.finish(t, err)
	})
	if last {
		h.mic = nil
		r.cancel()
	}
	h.micMu.Unlock()
	if !ok {
		<-t.ended // the run has ended and finishes its taps itself
	}
	h.sendMic()
}

// MicState is what the Live page shows about the recording.
type MicState struct {
	Running   bool   `json:"running"`
	Source    string `json:"source,omitempty"` // what is recorded, while it runs
	Dauer     string `json:"dauer,omitempty"`  // note of the recording started on the Live page
	Listeners int    `json:"listeners"`        // programs listening
	Input     string `json:"input,omitempty"`  // what is wrong with the microphone: InputNarrow, InputQuiet
}

func (h *Hub) micState() MicState {
	h.micMu.Lock()
	defer h.micMu.Unlock()
	var s MicState
	if h.dauer != nil {
		s.Dauer = h.dauer.note.ID
	}
	if h.mic != nil {
		s.Running, s.Source = true, h.mic.source
		h.mic.do(func() {
			s.Input = h.mic.input
			for t := range h.mic.taps {
				if t.note.Kind == KindMic {
					s.Listeners++
				}
			}
		})
	}
	return s
}

func (h *Hub) sendMic() {
	h.broadcast(map[string]any{"type": "mic", "mic": h.micState()})
}

// InputChanged is called when another microphone or source was chosen: what was wrong with the
// one recorded is no longer said.
func (h *Hub) InputChanged() {
	h.micMu.Lock()
	r := h.mic
	h.micMu.Unlock()
	if r != nil && r.do(r.inputChanged) {
		h.sendMic()
	}
}

// ---- who may ask ----

// Origin is the scheme, host and port of a page's address, as browsers send it.
func Origin(raw string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(raw), "/"))
}

// Allowed reports whether a request from origin may use live transcription: programs that are no
// web page (no origin), pages on this computer, and pages the user allowed on the Live page.
func (h *Hub) Allowed(origin string) bool {
	origin = Origin(origin)
	if origin == "" {
		return true
	}
	if u, err := url.Parse(origin); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		switch u.Hostname() {
		case "localhost", "127.0.0.1", "::1":
			return true
		}
	}
	return slices.Contains(prefs.Load(h.dataDir).LiveOrigins, origin)
}

// Refused notes a page that was turned away, so the Live page can offer to allow it.
func (h *Hub) Refused(origin, client string) {
	origin = Origin(origin)
	h.mu.Lock()
	h.blocked = slices.DeleteFunc(h.blocked, func(r Refusal) bool { return r.Origin == origin })
	h.blocked = append([]Refusal{{Origin: origin, Client: client, At: time.Now()}}, h.blocked...)
	if len(h.blocked) > 10 {
		h.blocked = h.blocked[:10]
	}
	h.mu.Unlock()
	h.sendAccess()
}

// Allow lets origin use live transcription from now on (allow) or no longer (!allow).
func (h *Hub) Allow(origin string, allow bool) error {
	origin = Origin(origin)
	if origin == "" {
		return errors.New("Adresse fehlt.")
	}
	_, err := prefs.Update(h.dataDir, func(p *prefs.Prefs) {
		p.LiveOrigins = slices.DeleteFunc(p.LiveOrigins, func(o string) bool { return o == origin })
		if allow {
			p.LiveOrigins = append(p.LiveOrigins, origin)
		}
	})
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.blocked = slices.DeleteFunc(h.blocked, func(r Refusal) bool { return r.Origin == origin })
	h.mu.Unlock()
	h.sendAccess()
	return nil
}

func (h *Hub) access() map[string]any {
	h.mu.Lock()
	blocked := append([]Refusal{}, h.blocked...)
	h.mu.Unlock()
	allowed := prefs.Load(h.dataDir).LiveOrigins
	if allowed == nil {
		allowed = []string{}
	}
	return map[string]any{"type": "access", "blocked": blocked, "allowed": allowed}
}

func (h *Hub) sendAccess() { h.broadcast(h.access()) }

// ---- the Live page ----

// Subscribe returns what the Live page shows now and a channel with every change after it
// (JSON messages). The channel is closed when the page falls behind; it then subscribes anew.
func (h *Hub) Subscribe() (snapshot []byte, updates <-chan []byte, cancel func()) {
	mic := h.micState()
	acc := h.access()
	p := prefs.Load(h.dataDir)
	source, language := NormSource(p.LiveSource), cmp.Or(p.LiveLanguage, "auto")
	h.mu.Lock()
	defer h.mu.Unlock()
	notes := make([]*Note, len(h.notes))
	for i, n := range h.notes {
		notes[i] = n.clone()
	}
	snapshot, _ = json.Marshal(map[string]any{"type": "snapshot", "notes": notes, "mic": mic, "source": source, "language": language, "system": SystemAudio(),
		"blocked": acc["blocked"], "allowed": acc["allowed"], "enabled": !p.LiveOff, "warm": h.warm,
		"apiSave": !p.LiveAPINoSave, "apiKeepHours": p.LiveAPIKeepHours,
		"voice": voiceMsg(p)})
	ch := make(chan []byte, 256)
	h.subs[ch] = struct{}{}
	if !h.watching {
		h.watching = true
		go h.watchDevices()
	}
	return snapshot, ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
	}
}

// sendNote and broadcastLocked expect h.mu held; broadcast takes it.
func (h *Hub) sendNote(n *Note) {
	h.broadcastLocked(map[string]any{"type": "note", "note": n.clone()})
}

func (h *Hub) broadcast(msg any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.broadcastLocked(msg)
}

func (h *Hub) broadcastLocked(msg any) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	for ch := range h.subs {
		select {
		case ch <- b:
		default:
			delete(h.subs, ch)
			close(ch)
		}
	}
}

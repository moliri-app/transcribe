// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sttapi

import (
	"bytes"
	"context"
	"encoding/binary"
	"log"
	"math"
	"strings"
	"time"

	"github.com/moliri-app/transcribe/internal/live"
)

// Live transcription for services without a live API of their own: the loudness of the audio tells
// where an utterance ends (a pause), and each utterance goes to the service as a recording of its
// own, one after the other. The text appears after each pause; there is no preview while speaking.
// No model on this computer is needed for it.

const (
	frame      = live.SR * 3 / 100 // 30 ms
	pieceLead  = live.SR * 3 / 10  // kept before speech starts
	pieceQuiet = live.SR * 7 / 10  // a pause this long ends an utterance
	pieceMax   = live.SR * 20      // an utterance longer than this is cut
	pieceMin   = live.SR / 4       // shorter sound is not speech
)

type piece struct {
	pcm   []float32
	start int // sample index
	done  chan struct{}
}

type pieceStream struct {
	cfg  Config
	lang string

	pos      int       // samples taken
	rest     []float32 // less than a frame
	buf      []float32 // the utterance going on (or the lead while silent)
	bufStart int
	speaking bool
	quiet    int     // samples of silence since the last loud frame
	floor    float64 // loudness of the background

	work    chan piece
	results chan live.Event
	closed  bool
}

func newPieceStream(c Config, lang string) *pieceStream {
	s := &pieceStream{cfg: c, lang: lang, floor: 0.003, work: make(chan piece, 64), results: make(chan live.Event, 64)}
	go s.worker()
	return s
}

// worker sends the utterances to the service in order.
func (s *pieceStream) worker() {
	defer close(s.results)
	for p := range s.work {
		if p.pcm != nil {
			if ev, ok := s.recognise(p); ok {
				s.results <- ev
			}
		}
		if p.done != nil {
			close(p.done)
		}
	}
}

func (s *pieceStream) recognise(p piece) (live.Event, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	length := float64(len(p.pcm)) / live.SR
	res, err := Transcribe(ctx, s.cfg, bytes.NewReader(WAV(p.pcm)), "live.wav", Request{Language: s.lang, Length: length})
	if err != nil {
		log.Println("live (API):", err)
		return live.Event{}, false
	}
	var words []string
	for _, w := range res.Words {
		if !w.Event {
			words = append(words, w.Text)
		}
	}
	text := strings.TrimSpace(strings.Join(words, " "))
	if text == "" {
		return live.Event{}, false
	}
	a := float64(p.start) / live.SR
	return live.Event{Final: true, Text: text, Start: round2(a), End: round2(a + length)}, true
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

func rms(x []float32) float64 {
	var s float64
	for _, v := range x {
		s += float64(v) * float64(v)
	}
	return math.Sqrt(s / float64(max(1, len(x))))
}

func (s *pieceStream) Accept(pcm []float32) []live.Event {
	s.rest = append(s.rest, pcm...)
	for len(s.rest) >= frame {
		f := s.rest[:frame]
		s.rest = s.rest[frame:]
		s.buf = append(s.buf, f...)
		s.pos += frame
		level := rms(f)
		loud := level > math.Max(0.01, 4*s.floor)
		switch {
		case loud:
			s.speaking, s.quiet = true, 0
		case s.speaking:
			s.quiet += frame
			if s.quiet >= pieceQuiet {
				s.end(len(s.buf) - s.quiet + pieceLead/2)
			}
		default:
			s.floor = 0.95*s.floor + 0.05*level
			if len(s.buf) > pieceLead { // only the lead is kept while silent
				n := len(s.buf) - pieceLead
				s.buf = append([]float32(nil), s.buf[n:]...)
				s.bufStart += n
			}
		}
		if s.speaking && len(s.buf) >= pieceMax {
			s.end(len(s.buf))
		}
	}
	s.rest = append([]float32(nil), s.rest...)
	return s.take()
}

// end sends buf[:n] as an utterance and keeps the rest.
func (s *pieceStream) end(n int) {
	n = min(n, len(s.buf))
	if n-pieceLead >= pieceMin {
		s.work <- piece{pcm: append([]float32(nil), s.buf[:n]...), start: s.bufStart}
	}
	s.buf = append([]float32(nil), s.buf[n:]...)
	s.bufStart += n
	s.speaking, s.quiet = false, 0
}

// take collects the utterances recognised so far.
func (s *pieceStream) take() []live.Event {
	var out []live.Event
	for {
		select {
		case ev, ok := <-s.results:
			if !ok {
				return out
			}
			out = append(out, ev)
		default:
			return out
		}
	}
}

func (s *pieceStream) Preview() (live.Event, bool) { return live.Event{}, false }

// Flush sends what is open and waits (a while) for everything sent to be recognised.
func (s *pieceStream) Flush() []live.Event {
	if s.speaking {
		s.end(len(s.buf))
	}
	done := make(chan struct{})
	s.work <- piece{done: done}
	select {
	case <-done:
	case <-time.After(30 * time.Second):
	}
	return s.take()
}

func (s *pieceStream) Close() {
	if !s.closed {
		s.closed = true
		close(s.work)
	}
}

// WAV is 16 kHz mono pcm as a WAV file.
func WAV(pcm []float32) []byte {
	n := 2 * len(pcm)
	b := make([]byte, 44+n)
	le := binary.LittleEndian
	copy(b, "RIFF")
	le.PutUint32(b[4:], uint32(36+n))
	copy(b[8:], "WAVEfmt ")
	le.PutUint32(b[16:], 16)
	le.PutUint16(b[20:], 1) // PCM
	le.PutUint16(b[22:], 1) // mono
	le.PutUint32(b[24:], live.SR)
	le.PutUint32(b[28:], 2*live.SR)
	le.PutUint16(b[32:], 2)
	le.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	le.PutUint32(b[40:], uint32(n))
	for i, v := range pcm {
		le.PutUint16(b[44+2*i:], uint16(int16(math.Round(float64(max(-1, min(1, v)))*32767))))
	}
	return b
}

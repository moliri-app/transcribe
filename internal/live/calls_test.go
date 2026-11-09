// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCallKeptAndReadBack(t *testing.T) {
	dir := t.TempDir()
	open := func(context.Context, string) (Stream, error) { return &fakeStream{}, nil }
	h := NewHub(dir, open)
	c := h.Call(KindOCR, "Moliri", "Tesseract 5.5.0", "Bild: scan.png", Fact{Label: "Grafikkarte", Value: "ja"})
	c.Status("Liest …")
	if n := h.Notes()[0]; n.Ended != nil || n.Partial != "Liest …" || !callID(n.ID) {
		t.Fatalf("running call: %+v", n)
	}
	if h.Busy() {
		t.Error("a call keeps the app from quitting")
	}
	c.End("Zinnteller", nil, Fact{Label: "Erkannt", Value: "1 Zeile"})
	c.End("twice", errors.New("no"))

	n := NewHub(dir, open).Notes()[0]
	if n.Kind != KindOCR || n.Client != "Moliri" || n.Model != "Tesseract 5.5.0" || n.Request != "Bild: scan.png" ||
		n.Response != "Zinnteller" || n.Error != "" || n.Partial != "" || len(n.Facts) != 2 || n.Ended == nil {
		t.Fatalf("call read back: %+v", n)
	}
}

func TestCallsDoNotPushRecordingsOff(t *testing.T) {
	dir := t.TempDir()
	open := func(context.Context, string) (Stream, error) { return &fakeStream{}, nil }
	h := NewHub(dir, open)
	s, err := h.Audio(context.Background(), "Moliri", func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	s.End(nil)
	for range keepNotes + 5 {
		h.Logged(KindEmbed, "Moliri", "", "Art: query", nil)
	}
	for _, hub := range []*Hub{h, NewHub(dir, open)} {
		notes := hub.Notes()
		if len(notes) != keepNotes+1 || hub.Note(s.Note()) == nil {
			t.Fatalf("%d notes, recording kept: %v", len(notes), hub.Note(s.Note()) != nil)
		}
	}
}

func TestCallTextClipped(t *testing.T) {
	long := strings.Repeat("ä", maxCallText)
	got := clip(long)
	if len(got) > maxCallText+100 || !strings.Contains(got, "gekürzt, insgesamt 20000 Zeichen") {
		t.Errorf("clipped: %d bytes, ends %q", len(got), got[len(got)-50:])
	}
	if Seconds(400*time.Millisecond) != "0,4 s" || Seconds(125*time.Second) != "2 min 5 s" {
		t.Error(Seconds(400*time.Millisecond), Seconds(125*time.Second))
	}
}

func TestNilCall(t *testing.T) {
	var h *Hub
	c := h.Call(KindFilter, "", "", "")
	c.Status("x")
	c.End("", nil)
	h.Logged(KindFilter, "", "", "", nil)
}

func TestCallImageKeptAndDeleted(t *testing.T) {
	dir := t.TempDir()
	open := func(context.Context, string) (Stream, error) { return &fakeStream{}, nil }
	h := NewHub(dir, open)
	src := filepath.Join(t.TempDir(), "upload")
	if err := os.WriteFile(src, append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...), 0o644); err != nil {
		t.Fatal(err)
	}
	c := h.Call(KindOCR, "Moliri", "Tesseract", "Bild")
	c.Image(src)
	c.End("Text", nil)
	n := h.Notes()[0]
	path := h.ImagePath(n.ID)
	if n.Image != n.ID+".png" || path == "" {
		t.Fatalf("image %q at %q", n.Image, path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	// an image without its note (one not saved, when the app ended) goes at the next start
	orphan := filepath.Join(imagesDir(dir), "20260101-000000-1-ocr.png")
	if err := os.WriteFile(orphan, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	h = NewHub(dir, open)
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphan image stays")
	}
	if h.ImagePath(n.ID) != path {
		t.Error("image lost after a restart")
	}

	if err := h.Delete(n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("image stays after its note was deleted")
	}

	// what is not an image is not kept
	if err := os.WriteFile(src, []byte("%PDF-1.7"), 0o644); err != nil {
		t.Fatal(err)
	}
	c = h.Call(KindOCR, "Moliri", "Tesseract", "Bild")
	c.Image(src)
	if n := h.Notes()[len(h.Notes())-1]; n.Image != "" {
		t.Errorf("kept %q", n.Image)
	}
}

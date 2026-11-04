// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package osocr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/moliri-app/transcribe/internal/ocr"
	"github.com/moliri-app/transcribe/internal/ocr/ocrtest"
)

// TestWindows reads the OCR test set with the recognizer of this Windows, when German is
// installed. Windows has no Fraktur: "deu+frak2021" reads German, and the Fraktur print is only
// checked for the format. The bounds are the measurement of 2026-11-01 with some room.
func TestWindows(t *testing.T) {
	e := New()
	if e == nil {
		t.Fatal("no engine on Windows")
	}
	info := e.Info()
	t.Logf("info: %+v", info)
	if !slices.Contains(info.Languages, "deu") {
		t.Skip("German text recognition is not installed in Windows")
	}
	if info.ID != "os" || !info.Ready || !info.Usable || info.Boxes != "words" || info.Version == "" {
		t.Errorf("info: %+v", info)
	}
	reg := ocr.NewRegistry("desktop", e)
	for _, c := range []struct {
		image, languages, layout string
		maxErrors, minFound      float64
	}{
		{"karteikarte", "deu", "lines", 0.05, 0.85},
		{"karteikarte", "deu+frak2021", "lines", 0.05, 0.85},
		{"etikett", "deu+frak2021", "block", 0.03, 0.9},
		{"fraktur", "deu+frak2021", "auto", 1, 0},
	} {
		t.Run(c.image+"/"+c.languages, func(t *testing.T) {
			gt, err := os.ReadFile(filepath.Join(ocrtest.Dir(), c.image+".gt.txt"))
			if err != nil {
				t.Fatal(err)
			}
			eng, o, err := reg.Prepare("os", ocr.Options{Languages: c.languages, Layout: c.layout})
			if err != nil || o.Languages != "deu" {
				t.Fatalf("prepare: %v %+v", err, o)
			}
			start := time.Now()
			res, err := reg.Read(context.Background(), eng, filepath.Join(ocrtest.Dir(), c.image+".jpg"), o, 0)
			if err != nil {
				t.Fatal(err)
			}
			cer, found := ocr.Score(string(gt), res.Text())
			t.Logf("%s: %.1f %% character errors, %.0f %% words found, %d lines, %d words, %.2f s (%s)",
				c.image, cer*100, found*100, len(res.Pages[0].Lines), res.Words(), time.Since(start).Seconds(), res.Engine)
			if cer > c.maxErrors || found < c.minFound {
				t.Errorf("worse than measured: %.1f %% errors, %.0f %% words", cer*100, found*100)
			}
			pg := res.Pages[0]
			for _, l := range pg.Lines {
				b := l.BBox
				if b[2] == 0 || b[3] == 0 || b[0]+b[2] > pg.Width || b[1]+b[3] > pg.Height {
					t.Errorf("line box %v outside %d×%d: %q", b, pg.Width, pg.Height, l.Text)
				}
				for _, w := range l.Words {
					if w.P != nil || w.BBox[2] == 0 {
						t.Errorf("word %+v", w)
					}
				}
			}
		})
	}

	eng, o, _ := reg.Prepare("os", ocr.Options{Languages: "deu"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reg.Read(ctx, eng, filepath.Join(ocrtest.Dir(), "fraktur.jpg"), o, 0); err == nil {
		t.Error("cancelled reading returned a result")
	}
	if _, err := reg.Read(context.Background(), eng, filepath.Join(ocrtest.Dir(), "fraktur.gt.txt"), o, 0); !errors.Is(err, ocr.ErrImage) {
		t.Errorf("text file as an image: %v", err)
	}
	if _, _, err := reg.Prepare("os", ocr.Options{Languages: "frak2021"}); !errors.Is(err, ocr.ErrOptions) {
		t.Errorf("Fraktur alone: %v", err)
	}
}

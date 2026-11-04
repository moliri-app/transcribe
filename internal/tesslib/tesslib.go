// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package tesslib is Tesseract as a library (ocr.Library) for the desktop app on Windows: the DLLs
// of the NuGet package "Tesseract", downloaded on first use and called through their C API.
package tesslib

import (
	"context"
	"sync"

	"github.com/moliri-app/transcribe/internal/models"
)

// Lib loads the library from the app's data directory, downloading it first when it is missing.
type Lib struct {
	DataDir string

	mu      sync.Mutex // one reading at a time; the library is loaded once
	loaded  bool
	version string
	api     uintptr // TessBaseAPI kept for its languages (loading models takes a while)
	apiLang string
}

func New(dataDir string) *Lib { return &Lib{DataDir: dataDir} }

func (l *Lib) Installed() bool { return models.TesseractLibInstalled(l.DataDir) }

func (l *Lib) Version() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.version
}

func (l *Lib) Load(ctx context.Context) error {
	if err := models.EnsureTesseractLib(ctx, l.DataDir); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.loaded {
		return nil
	}
	v, err := load(models.TesseractLibDir(l.DataDir))
	if err != nil {
		return err
	}
	l.version, l.loaded = v, true
	return nil
}

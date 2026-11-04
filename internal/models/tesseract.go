// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package models

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Tesseract for text recognition (OCR) is not shipped with the app: on Windows the library and
// the models come on first use, each pinned by its checksum. Elsewhere the tesseract program of
// the system is used (apt install tesseract-ocr, brew install tesseract).

// TesseractDir holds the library (lib/) and the models (tessdata/).
func TesseractDir(dataDir string) string { return filepath.Join(dataDir, "tesseract") }

func TesseractLibDir(dataDir string) string  { return filepath.Join(TesseractDir(dataDir), "lib") }
func TesseractDataDir(dataDir string) string { return filepath.Join(TesseractDir(dataDir), "tessdata") }

// The NuGet package "Tesseract" (Charles Weld, Apache 2.0) carries Tesseract 5 and Leptonica as
// DLLs built for Windows; only x64/ is taken from it.
const (
	tesseractPackage    = "https://www.nuget.org/api/v2/package/Tesseract/5.2.0"
	tesseractPackageSum = "202d82fc7c7d8384df7da57206d5e1f456ccdabd648c46e67cdfaa3a911d4795"
)

// TesseractLibFiles are the DLLs of the package, Leptonica first (Tesseract needs it).
var TesseractLibFiles = []string{"leptonica-1.82.0.dll", "tesseract50.dll"}

type tessModel struct{ url, sum string }

// tessModels are the models the server's image has too (server/transcribe in Moliri): tessdata_fast
// 4.1.0 as Debian packages it, and UB Mannheim's frak2021 (tessdata_best, CC0).
var tessModels = map[string]tessModel{
	"deu":      {"https://github.com/tesseract-ocr/tessdata_fast/raw/4.1.0/deu.traineddata", "19d219bbb6672c869d20a9636c6816a81eb9a71796cb93ebe0cb1530e2cdb22d"},
	"eng":      {"https://github.com/tesseract-ocr/tessdata_fast/raw/4.1.0/eng.traineddata", "7d4322bd2a7749724879683fc3912cb542f19906c83bcc1a52132556427170b2"},
	"frk":      {"https://github.com/tesseract-ocr/tessdata_fast/raw/4.1.0/frk.traineddata", "7cd1b541e9d3884b9546a7d292c4f349cdb45ff646b4b43166dfc5099a8ad1a1"},
	"Fraktur":  {"https://github.com/tesseract-ocr/tessdata_fast/raw/4.1.0/script/Fraktur.traineddata", "ac985c3020e42986f8a19e07f1c1b501d47ed7d8e96de69f685bb907aa8db55c"},
	"frak2021": {"https://zenodo.org/records/10125246/files/frak2021-0.905.traineddata?download=1", "1da2384254fa8462c776faf3b43307fe19ce51be0931c623b2fdd560f96e299a"},
}

// TesseractModels are the models the app can download, sorted.
func TesseractModels() []string {
	names := make([]string, 0, len(tessModels))
	for n := range tessModels {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// TesseractLibSupported reports whether the library can be downloaded for this platform.
func TesseractLibSupported() bool { return runtime.GOOS == "windows" && runtime.GOARCH == "amd64" }

var tessMu sync.Mutex

// TesseractLibInstalled reports whether the DLLs are in place.
func TesseractLibInstalled(dataDir string) bool {
	for _, f := range TesseractLibFiles {
		if st, err := os.Stat(filepath.Join(TesseractLibDir(dataDir), f)); err != nil || st.Size() == 0 {
			return false
		}
	}
	return true
}

// EnsureTesseractLib downloads the Tesseract library into TesseractLibDir unless it is there.
func EnsureTesseractLib(ctx context.Context, dataDir string) error {
	tessMu.Lock()
	defer tessMu.Unlock()
	if TesseractLibInstalled(dataDir) {
		return nil
	}
	if !TesseractLibSupported() {
		return fmt.Errorf("no Tesseract download for %s/%s: install tesseract (e.g. apt install tesseract-ocr, brew install tesseract)", runtime.GOOS, runtime.GOARCH)
	}
	dir := TesseractLibDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	archive := filepath.Join(dir, "tesseract-download.tmp")
	defer os.Remove(archive)
	err := download(ctx, "tesseract", tesseractPackage, tesseractPackageSum, archive)
	if err == nil {
		err = unzipTesseract(archive, dir)
	}
	if err != nil {
		track("tesseract", "error", 0, 0, err)
		return err
	}
	track("tesseract", "done", 0, 0, nil)
	return nil
}

func unzipTesseract(archive, dir string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	found := 0
	for _, zf := range zr.File {
		name, ok := strings.CutPrefix(zf.Name, "x64/")
		if !ok || !slices.Contains(TesseractLibFiles, name) {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeAtomically(filepath.Join(dir, name), rc)
		rc.Close()
		if err != nil {
			return err
		}
		found++
	}
	if found < len(TesseractLibFiles) {
		return fmt.Errorf("%s not in the Tesseract package", strings.Join(TesseractLibFiles, ", "))
	}
	return nil
}

// FetchTesseractModel downloads one model into TesseractDataDir unless it is there.
func FetchTesseractModel(ctx context.Context, dataDir, model string) error {
	m, ok := tessModels[model]
	if !ok {
		return fmt.Errorf("no download for the Tesseract model %q", model)
	}
	tessMu.Lock()
	defer tessMu.Unlock()
	dir := TesseractDataDir(dataDir)
	path := filepath.Join(dir, model+".traineddata")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := "tesseract-" + model
	tmp := path + ".tmp"
	defer os.Remove(tmp)
	err := download(ctx, name, m.url, m.sum, tmp)
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		track(name, "error", 0, 0, err)
		return err
	}
	track(name, "done", 0, 0, nil)
	return nil
}

// download fetches url into path and checks its SHA-256.
func download(ctx context.Context, name, url, sum, path string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), counted(name, resp.Body, resp.ContentLength))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != sum {
		return fmt.Errorf("%s: checksum %s, want %s", url, got, sum)
	}
	return nil
}

func writeAtomically(path string, r io.Reader) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

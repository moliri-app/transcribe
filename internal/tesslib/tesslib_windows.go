// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package tesslib

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/moliri-app/transcribe/internal/models"
	"github.com/moliri-app/transcribe/internal/ocr"
)

// The C API of Tesseract (tesseract/capi.h) and the two Leptonica calls that read an image.
var (
	tessVersion      func() *byte
	apiCreate        func() uintptr
	apiDelete        func(api uintptr)
	apiInit3         func(api uintptr, datapath, language string) int32
	apiSetPSM        func(api uintptr, mode int32)
	apiSetImage2     func(api, pix uintptr)
	apiRecognize     func(api, monitor uintptr) int32
	apiGetTsvText    func(api uintptr, page int32) *byte
	apiClear         func(api uintptr)
	deleteText       func(text *byte)
	monitorCreate    func() uintptr
	monitorSetCancel func(monitor, fn uintptr)
	pixReadMem       func(data *byte, size uintptr) uintptr
	pixDestroy       func(pix *uintptr)
	setMsgSeverity   func(level int32) int32

	monitor   uintptr
	cancelled atomic.Pointer[context.Context] // the reading running now; its cancellation stops Tesseract
	loadOnce  sync.Mutex
	loadedDir string
)

func load(dir string) (string, error) {
	loadOnce.Lock()
	defer loadOnce.Unlock()
	if loadedDir != "" {
		return goString(tessVersion()), nil
	}
	// tesseract50.dll finds leptonica next to it
	if k, err := syscall.LoadLibrary("kernel32.dll"); err == nil {
		if p, err := syscall.GetProcAddress(k, "SetDllDirectoryW"); err == nil {
			d, _ := syscall.UTF16PtrFromString(dir)
			syscall.SyscallN(p, uintptr(unsafe.Pointer(d)))
		}
	}
	lept, err := syscall.LoadLibrary(filepath.Join(dir, models.TesseractLibFiles[0]))
	if err != nil {
		return "", fmt.Errorf("leptonica: %w", err)
	}
	tess, err := syscall.LoadLibrary(filepath.Join(dir, models.TesseractLibFiles[1]))
	if err != nil {
		return "", fmt.Errorf("tesseract: %w", err)
	}
	t, l := uintptr(tess), uintptr(lept)
	purego.RegisterLibFunc(&tessVersion, t, "TessVersion")
	purego.RegisterLibFunc(&apiCreate, t, "TessBaseAPICreate")
	purego.RegisterLibFunc(&apiDelete, t, "TessBaseAPIDelete")
	purego.RegisterLibFunc(&apiInit3, t, "TessBaseAPIInit3")
	purego.RegisterLibFunc(&apiSetPSM, t, "TessBaseAPISetPageSegMode")
	purego.RegisterLibFunc(&apiSetImage2, t, "TessBaseAPISetImage2")
	purego.RegisterLibFunc(&apiRecognize, t, "TessBaseAPIRecognize")
	purego.RegisterLibFunc(&apiGetTsvText, t, "TessBaseAPIGetTsvText")
	purego.RegisterLibFunc(&apiClear, t, "TessBaseAPIClear")
	purego.RegisterLibFunc(&deleteText, t, "TessDeleteText")
	purego.RegisterLibFunc(&monitorCreate, t, "TessMonitorCreate")
	purego.RegisterLibFunc(&monitorSetCancel, t, "TessMonitorSetCancelFunc")
	purego.RegisterLibFunc(&pixReadMem, l, "pixReadMem")
	purego.RegisterLibFunc(&pixDestroy, l, "pixDestroy")
	purego.RegisterLibFunc(&setMsgSeverity, l, "setMsgSeverity")
	setMsgSeverity(5) // errors only: no "Info in fopenReadFromMemory" on every image

	// bool cancel(void *cancel_this, int words): asked again and again while Tesseract reads
	monitor = monitorCreate()
	monitorSetCancel(monitor, purego.NewCallback(func(_ uintptr, _ int32) uintptr {
		if c := cancelled.Load(); c != nil && (*c).Err() != nil {
			return 1
		}
		return 0
	}))
	loadedDir = dir
	return goString(tessVersion()), nil
}

// TSV reads one image. The TessBaseAPI stays loaded with its languages for the next image.
func (l *Lib) TSV(ctx context.Context, image []byte, tessdata, languages string, psm int) ([]byte, error) {
	if len(image) == 0 {
		return nil, errors.New("empty image")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.loaded {
		return nil, errors.New("tesseract library not loaded")
	}
	if l.api == 0 || l.apiLang != tessdata+"|"+languages {
		if l.api != 0 {
			apiDelete(l.api)
			l.api = 0
		}
		api := apiCreate()
		// the library opens files with narrow paths: the short (8.3) form keeps a user name with
		// umlauts out of it
		if apiInit3(api, shortPath(tessdata), languages) != 0 {
			apiDelete(api)
			return nil, fmt.Errorf("tesseract could not load the models %s from %s", languages, tessdata)
		}
		l.api, l.apiLang = api, tessdata+"|"+languages
	}
	defer apiClear(l.api)

	pix := pixReadMem(&image[0], uintptr(len(image)))
	if pix == 0 {
		return nil, fmt.Errorf("%w: Tesseract kennt dieses Bildformat nicht", ocr.ErrImage)
	}
	defer pixDestroy(&pix)
	apiSetPSM(l.api, int32(psm))
	apiSetImage2(l.api, pix)
	cancelled.Store(&ctx)
	defer cancelled.Store(nil)
	if apiRecognize(l.api, monitor) != 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("tesseract could not read the image")
	}
	text := apiGetTsvText(l.api, 0)
	if text == nil {
		return nil, errors.New("tesseract returned no text")
	}
	defer deleteText(text)
	return []byte(goString(text)), nil
}

// goString copies a C string the library owns.
func goString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

// shortPath is the 8.3 form of path, or path when there is none.
func shortPath(path string) string {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return path
	}
	buf := make([]uint16, 1024)
	n, err := syscall.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return path
	}
	return syscall.UTF16ToString(buf[:n])
}

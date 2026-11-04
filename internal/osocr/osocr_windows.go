// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package osocr

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"github.com/moliri-app/transcribe/internal/ocr"
)

// New gives the engine "os" with Windows.Media.Ocr (Windows 10 and later), through the Windows
// Runtime directly: the image is decoded by Windows (BitmapDecoder: JPEG, PNG, TIFF, BMP, GIF,
// and WebP or HEIF where their codecs are installed), shrunk to the largest size the recognizer
// takes, and read in one language.
func New() *Engine {
	if procRoGetActivationFactory.Find() != nil || procCreateStreamOnFile.Find() != nil {
		return nil
	}
	return &Engine{sys: windowsOCR{}}
}

var (
	combase                    = syscall.NewLazyDLL("combase.dll")
	procRoInitialize           = combase.NewProc("RoInitialize")
	procRoUninitialize         = combase.NewProc("RoUninitialize")
	procRoGetActivationFactory = combase.NewProc("RoGetActivationFactory")
	procRoActivateInstance     = combase.NewProc("RoActivateInstance")
	procWindowsCreateString    = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString    = combase.NewProc("WindowsDeleteString")
	procWindowsGetStringRaw    = combase.NewProc("WindowsGetStringRawBuffer")
	procCreateStreamOnFile     = syscall.NewLazyDLL("shcore.dll").NewProc("CreateRandomAccessStreamOnFile")
	procRtlGetVersion          = syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion")
)

// The interfaces used, with their IIDs from the Windows SDK (winrt/windows.media.ocr.h and others).
// Methods are called by their place in the vtable: IUnknown 0–2, IInspectable 3–5, then the
// interface's own in the order of the header.
var (
	iidOcrEngineStatics      = mustGUID("5bffa85a-3384-3540-9940-699120d428a8")
	iidLanguageFactory       = mustGUID("9b0252ac-0c27-44f8-b792-9793fb66c63e")
	iidBitmapDecoderStatics  = mustGUID("438ccb26-bcef-4e95-bad6-23a822e58d01")
	iidBitmapFrame           = mustGUID("72a49a1c-8081-438d-91bc-94ecfc8185c6")
	iidBitmapFrameWithBitmap = mustGUID("fe287c9a-420c-4963-87ad-691436e08383")
	iidBitmapTransform       = mustGUID("ae755344-e268-4d35-adcf-e995d31a8d34")
	iidAsyncInfo             = mustGUID("00000036-0000-0000-c000-000000000046")
	iidRandomAccessStream    = mustGUID("905a0fe1-bc53-11df-8c49-001e4fc686da")
)

const (
	// IOcrEngineStatics
	mMaxImageDimension      = 6
	mAvailableLanguages     = 7
	mTryCreateFromLanguage  = 9
	mRecognizeAsync         = 6 // IOcrEngine
	mCreateLanguage         = 6 // ILanguageFactory
	mLanguageTag            = 6 // ILanguage
	mVectorGetAt            = 6 // IVectorView<T>
	mVectorSize             = 7
	mResultLines            = 6 // IOcrResult
	mResultTextAngle        = 7
	mLineWords              = 6 // IOcrLine
	mLineText               = 7
	mWordBoundingRect       = 6 // IOcrWord
	mWordText               = 7
	mReferenceValue         = 6 // IReference<double>
	mDecoderCreateAsync     = 14
	mFramePixelWidth        = 12
	mFramePixelHeight       = 13
	mFrameBitmapConverted   = 7 // IBitmapFrameWithSoftwareBitmap
	mFrameBitmapTransformed = 8
	mTransformScaledWidth   = 7 // IBitmapTransform put_
	mTransformScaledHeight  = 9
	mTransformInterpolation = 11
	mAsyncStatus            = 7 // IAsyncInfo
	mAsyncErrorCode         = 8
	mAsyncCancel            = 9
	mAsyncResults           = 8 // IAsyncOperation<T>

	pixelBgra8       = 87
	alphaPremult     = 0
	interpFant       = 3
	exifIgnore       = 0
	colorDoNotManage = 0

	statusStarted   = 0
	statusCompleted = 1
	statusCanceled  = 2
)

type windowsOCR struct{}

func (windowsOCR) Name() string { return "Windows-Texterkennung" }

// Version is the version of Windows, e.g. "10.0.26300".
func (windowsOCR) Version() string {
	var v struct {
		size, major, minor, build, platform uint32
		csd                                 [128]uint16
	}
	v.size = uint32(unsafe.Sizeof(v))
	if r, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&v))); r != 0 {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.build)
}

func (windowsOCR) Languages() ([]string, error) {
	done, err := enter()
	if err != nil {
		return nil, err
	}
	defer done()
	statics, err := factory("Windows.Media.Ocr.OcrEngine", &iidOcrEngineStatics)
	if err != nil {
		return nil, err
	}
	defer statics.release()
	var vec obj
	if err := statics.call(mAvailableLanguages, vec.out()); err != nil {
		return nil, fmt.Errorf("OCR languages: %w", err)
	}
	defer vec.release()
	var tags []string
	err = each(vec, func(lang obj) error {
		tag, err := lang.text(mLanguageTag)
		if tag != "" {
			tags = append(tags, tag)
		}
		return err
	})
	return tags, err
}

func (windowsOCR) Read(ctx context.Context, path, tag string) (*page, error) {
	done, err := enter()
	if err != nil {
		return nil, err
	}
	defer done()
	var rel releaser
	defer rel.all()

	// the recognizer for the language
	statics, err := factory("Windows.Media.Ocr.OcrEngine", &iidOcrEngineStatics)
	if err != nil {
		return nil, err
	}
	rel.add(statics)
	langs, err := factory("Windows.Globalization.Language", &iidLanguageFactory)
	if err != nil {
		return nil, err
	}
	rel.add(langs)
	h, err := hstring(tag)
	if err != nil {
		return nil, err
	}
	var lang, engine obj
	err = langs.call(mCreateLanguage, h, lang.out())
	freeHString(h)
	if err != nil {
		return nil, fmt.Errorf("%w: language %q: %v", ocr.ErrOptions, tag, err)
	}
	rel.add(lang)
	if err := statics.call(mTryCreateFromLanguage, uintptr(lang.p), engine.out()); err != nil || engine.p == nil {
		return nil, fmt.Errorf("%w: Windows cannot read %q", ocr.ErrUnavailable, tag)
	}
	rel.add(engine)
	var maxDim uint32
	if err := statics.call(mMaxImageDimension, uintptr(unsafe.Pointer(&maxDim))); err != nil {
		return nil, err
	}

	// the image, decoded by Windows
	if abs, err := filepath.Abs(path); err == nil {
		path = abs // the stream wants a full path
	}
	p16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var stream obj
	if r, _, _ := procCreateStreamOnFile.Call(uintptr(unsafe.Pointer(p16)), 0, uintptr(unsafe.Pointer(&iidRandomAccessStream)), stream.out()); int32(r) < 0 {
		return nil, fmt.Errorf("open %s: %w", path, hresult(r))
	}
	rel.add(stream)
	decoders, err := factory("Windows.Graphics.Imaging.BitmapDecoder", &iidBitmapDecoderStatics)
	if err != nil {
		return nil, err
	}
	rel.add(decoders)
	var op obj
	if err := decoders.call(mDecoderCreateAsync, uintptr(stream.p), op.out()); err != nil {
		return nil, fmt.Errorf("%w: %v", ocr.ErrImage, err)
	}
	decoder, err := await(ctx, op)
	if err != nil {
		return nil, imageErr(ctx, err)
	}
	rel.add(decoder)
	frame, err := decoder.query(&iidBitmapFrame)
	if err != nil {
		return nil, err
	}
	rel.add(frame)
	var w, hgt uint32
	if err := frame.call(mFramePixelWidth, uintptr(unsafe.Pointer(&w))); err != nil {
		return nil, err
	}
	if err := frame.call(mFramePixelHeight, uintptr(unsafe.Pointer(&hgt))); err != nil {
		return nil, err
	}
	if w == 0 || hgt == 0 {
		return nil, fmt.Errorf("%w: empty image", ocr.ErrImage)
	}
	withBitmap, err := decoder.query(&iidBitmapFrameWithBitmap)
	if err != nil {
		return nil, err
	}
	rel.add(withBitmap)

	// as BGRA, shrunk to what the recognizer takes
	scale := 1.0
	if maxDim > 0 && max(w, hgt) > maxDim {
		scale = float64(maxDim) / float64(max(w, hgt))
		transform, err := activate("Windows.Graphics.Imaging.BitmapTransform", &iidBitmapTransform)
		if err != nil {
			return nil, err
		}
		rel.add(transform)
		sw := max(1, min(maxDim, uint32(float64(w)*scale+0.5)))
		sh := max(1, min(maxDim, uint32(float64(hgt)*scale+0.5)))
		for _, set := range [][2]uintptr{{mTransformScaledWidth, uintptr(sw)}, {mTransformScaledHeight, uintptr(sh)}, {mTransformInterpolation, interpFant}} {
			if err := transform.call(int(set[0]), set[1]); err != nil {
				return nil, err
			}
		}
		err = withBitmap.call(mFrameBitmapTransformed, pixelBgra8, alphaPremult, uintptr(transform.p), exifIgnore, colorDoNotManage, op.out())
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ocr.ErrImage, err)
		}
	} else if err := withBitmap.call(mFrameBitmapConverted, pixelBgra8, alphaPremult, op.out()); err != nil {
		return nil, fmt.Errorf("%w: %v", ocr.ErrImage, err)
	}
	bitmap, err := await(ctx, op)
	if err != nil {
		return nil, imageErr(ctx, err)
	}
	rel.add(bitmap)

	// read
	if err := engine.call(mRecognizeAsync, uintptr(bitmap.p), op.out()); err != nil {
		return nil, fmt.Errorf("recognize: %w", err)
	}
	result, err := await(ctx, op)
	if err != nil {
		return nil, err
	}
	rel.add(result)

	pg := &page{Width: int(w), Height: int(hgt), Scale: scale}
	var angle obj
	if err := result.call(mResultTextAngle, angle.out()); err == nil && angle.p != nil {
		var a float64
		if angle.call(mReferenceValue, uintptr(unsafe.Pointer(&a))) == nil {
			pg.Angle = a
		}
		angle.release()
	}
	var lines obj
	if err := result.call(mResultLines, lines.out()); err != nil {
		return nil, err
	}
	rel.add(lines)
	err = each(lines, func(line obj) error {
		var rl rawLine
		var err error
		if rl.Text, err = line.text(mLineText); err != nil {
			return err
		}
		var words obj
		if err := line.call(mLineWords, words.out()); err != nil {
			return err
		}
		defer words.release()
		err = each(words, func(word obj) error {
			var r struct{ X, Y, W, H float32 }
			if err := word.call(mWordBoundingRect, uintptr(unsafe.Pointer(&r))); err != nil {
				return err
			}
			text, err := word.text(mWordText)
			rl.Words = append(rl.Words, rawWord{Text: text, X: float64(r.X), Y: float64(r.Y), W: float64(r.W), H: float64(r.H)})
			return err
		})
		pg.Lines = append(pg.Lines, rl)
		return err
	})
	if err != nil {
		return nil, err
	}
	return pg, nil
}

// imageErr names a decoding failure as an unreadable image, unless the reading was cancelled.
func imageErr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("%w: %v", ocr.ErrImage, err)
}

// enter joins the Windows Runtime on this goroutine's thread for one call; done leaves it.
func enter() (done func(), err error) {
	runtime.LockOSThread()
	r, _, _ := procRoInitialize.Call(1) // RO_INIT_MULTITHREADED
	switch hr := int32(r); {
	case hr >= 0: // S_OK, S_FALSE
		return func() { procRoUninitialize.Call(); runtime.UnlockOSThread() }, nil
	case uint32(r) == 0x80010106: // RPC_E_CHANGED_MODE: the thread is single-threaded already, which serves too
		return runtime.UnlockOSThread, nil
	default:
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("RoInitialize: %w", hresult(r))
	}
}

// obj is a pointer to a COM interface.
type obj struct{ p unsafe.Pointer }

// out is the address for a method that hands back an interface. Whatever o held before is
// someone else's to release (await releases its operation).
func (o *obj) out() uintptr {
	o.p = nil
	return uintptr(unsafe.Pointer(&o.p))
}

// call calls method i of the interface's vtable.
func (o obj) call(i int, args ...uintptr) error {
	vtbl := *(*unsafe.Pointer)(o.p)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(i)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(o.p)}, args...)...)
	if int32(r) < 0 {
		return hresult(r)
	}
	return nil
}

func (o *obj) release() {
	if o.p != nil {
		o.call(2)
		o.p = nil
	}
}

func (o obj) query(iid *guid) (obj, error) {
	var q obj
	if err := o.call(0, uintptr(unsafe.Pointer(iid)), q.out()); err != nil {
		return obj{}, fmt.Errorf("QueryInterface: %w", err)
	}
	return q, nil
}

// text calls a getter that hands back a string.
func (o obj) text(i int) (string, error) {
	var h uintptr
	if err := o.call(i, uintptr(unsafe.Pointer(&h))); err != nil {
		return "", err
	}
	defer freeHString(h)
	var n uint32
	r, _, _ := procWindowsGetStringRaw.Call(h, uintptr(unsafe.Pointer(&n)))
	if r == 0 || n == 0 {
		return "", nil
	}
	return syscall.UTF16ToString(unsafe.Slice(*(**uint16)(unsafe.Pointer(&r)), n)), nil
}

// each calls fn with every item of an IVectorView; the items are released after fn.
func each(vec obj, fn func(obj) error) error {
	var n uint32
	if err := vec.call(mVectorSize, uintptr(unsafe.Pointer(&n))); err != nil {
		return err
	}
	for i := range n {
		var item obj
		if err := vec.call(mVectorGetAt, uintptr(i), item.out()); err != nil {
			return err
		}
		err := fn(item)
		item.release()
		if err != nil {
			return err
		}
	}
	return nil
}

// await waits for an IAsyncOperation and hands back its result. Cancelling ctx cancels it.
func await(ctx context.Context, op obj) (obj, error) {
	defer op.release()
	info, err := op.query(&iidAsyncInfo)
	if err != nil {
		return obj{}, err
	}
	defer info.release()
	for wait := 100 * time.Microsecond; ; wait = min(wait*2, 10*time.Millisecond) {
		var status int32
		if err := info.call(mAsyncStatus, uintptr(unsafe.Pointer(&status))); err != nil {
			return obj{}, err
		}
		switch status {
		case statusStarted:
			select {
			case <-ctx.Done():
				info.call(mAsyncCancel)
				return obj{}, ctx.Err()
			case <-time.After(wait):
			}
			continue
		case statusCompleted:
			var res obj
			if err := op.call(mAsyncResults, res.out()); err != nil {
				return obj{}, err
			}
			return res, nil
		case statusCanceled:
			return obj{}, context.Canceled
		default:
			var code int32
			info.call(mAsyncErrorCode, uintptr(unsafe.Pointer(&code)))
			return obj{}, hresult(uint32(code))
		}
	}
}

func factory(class string, iid *guid) (obj, error) {
	h, err := hstring(class)
	if err != nil {
		return obj{}, err
	}
	defer freeHString(h)
	var f obj
	if r, _, _ := procRoGetActivationFactory.Call(h, uintptr(unsafe.Pointer(iid)), f.out()); int32(r) < 0 {
		return obj{}, fmt.Errorf("%s: %w", class, hresult(r))
	}
	return f, nil
}

// activate makes an instance of a runtime class and asks it for the interface iid.
func activate(class string, iid *guid) (obj, error) {
	h, err := hstring(class)
	if err != nil {
		return obj{}, err
	}
	defer freeHString(h)
	var inst obj
	if r, _, _ := procRoActivateInstance.Call(h, inst.out()); int32(r) < 0 {
		return obj{}, fmt.Errorf("%s: %w", class, hresult(r))
	}
	defer inst.release()
	return inst.query(iid)
}

func hstring(s string) (uintptr, error) {
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var h uintptr
	if r, _, _ := procWindowsCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&h))); int32(r) < 0 {
		return 0, hresult(r)
	}
	return h, nil
}

func freeHString(h uintptr) {
	if h != 0 {
		procWindowsDeleteString.Call(h)
	}
}

// releaser releases the objects of one reading, last first.
type releaser []obj

func (r *releaser) add(o obj) { *r = append(*r, o) }

func (r *releaser) all() {
	for i := len(*r) - 1; i >= 0; i-- {
		(*r)[i].release()
	}
}

type guid struct {
	data1        uint32
	data2, data3 uint16
	data4        [8]byte
}

func mustGUID(s string) guid {
	var g guid
	hex := func(from, to int) uint64 {
		v, err := strconv.ParseUint(s[from:to], 16, 64)
		if err != nil {
			panic("bad GUID " + s)
		}
		return v
	}
	if len(s) != 36 {
		panic("bad GUID " + s)
	}
	g.data1 = uint32(hex(0, 8))
	g.data2 = uint16(hex(9, 13))
	g.data3 = uint16(hex(14, 18))
	g.data4[0], g.data4[1] = byte(hex(19, 21)), byte(hex(21, 23))
	for i := range 6 {
		g.data4[2+i] = byte(hex(24+2*i, 26+2*i))
	}
	return g
}

type hresult uint32

func (h hresult) Error() string { return fmt.Sprintf("HRESULT 0x%08X", uint32(h)) }

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package live

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// What the computer plays is recorded with WASAPI loopback on the default output device; the
// DirectShow that ffmpeg offers has no loopback. An output device hands loopback nothing while
// nothing plays, so the app plays silence on it alongside: the recording then runs on without
// gaps, at the device's pace. When the default output changes (a headset is plugged in), the
// recording moves on to the new one. The microphone Windows has as its default ("System" on the
// Live page) is recorded the same way, without loopback, and followed the same way. A microphone
// chosen by name is recorded through WASAPI as well: DirectShow (ffmpeg) took up to ten seconds
// to give the first sound of a Bluetooth headset, and at times gave none.
//
// The COM interfaces are called through their vtables. Each call passes its pointers to
// syscall.SyscallN directly, which keeps what they point to in place until the call returns.

type guid struct {
	a    uint32
	b, c uint16
	d    [8]byte
}

var (
	clsidMMDeviceEnumerator = guid{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = guid{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidIAudioClient         = guid{0x1CB9AD4C, 0xDBFA, 0x4C32, [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}}
	iidIAudioCaptureClient  = guid{0xC8ADBD64, 0xE71E, 0x48A0, [8]byte{0xA4, 0xDE, 0x18, 0x5C, 0x39, 0x5C, 0xD3, 0x17}}
	iidIAudioRenderClient   = guid{0xF294ACFC, 0x3146, 0x4483, [8]byte{0xA7, 0xBF, 0xAD, 0xDC, 0xA7, 0xC2, 0x60, 0xE2}}
	pkeyDeviceFriendlyName  = propertyKey{guid{0xA45C254E, 0xDF1C, 0x4EFD, [8]byte{0x80, 0x20, 0x67, 0xD1, 0x46, 0xA8, 0x50, 0xE0}}, 14}
	subtypeIEEEFloat        = guid{0x00000003, 0x0000, 0x0010, [8]byte{0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}}
)

var (
	ole32            = syscall.NewLazyDLL("ole32.dll")
	coInitializeEx   = ole32.NewProc("CoInitializeEx")
	coUninitialize   = ole32.NewProc("CoUninitialize")
	coCreateInstance = ole32.NewProc("CoCreateInstance")
	coTaskMemFree    = ole32.NewProc("CoTaskMemFree")
	propVariantClear = ole32.NewProc("PropVariantClear")
)

type propertyKey struct {
	fmtid guid
	pid   uint32
}

const (
	clsctxAll         = 0x17
	eRender           = 0
	eCapture          = 1
	eConsole          = 0
	deviceStateActive = 1
	stgmRead          = 0
	vtLPWSTR          = 31
	streamLoopback    = 0x00020000
	bufferSilent      = 0x2
	captureBuffer     = 20_000_000 // 2 s in 100 ns; see recordDevice
	renderBuffer      = 2_000_000  // 200 ms
	rpcEChangedMode   = 0x80010106
	errNotFound       = 0x80070490
	deviceInvalidated = 0x88890004
)

// vtable method numbers
const (
	mRelease = 2

	mEnumAudioEndpoints      = 3 // IMMDeviceEnumerator
	mGetDefaultAudioEndpoint = 4

	mGetCount = 3 // IMMDeviceCollection
	mItem     = 4

	mActivate          = 3 // IMMDevice
	mOpenPropertyStore = 4
	mGetID             = 5

	mGetValue = 5 // IPropertyStore

	mInitialize        = 3 // IAudioClient
	mGetBufferSize     = 4
	mGetCurrentPadding = 6
	mGetMixFormat      = 8
	mStart             = 10
	mStop              = 11
	mGetService        = 14

	mGetBuffer         = 3 // IAudioCaptureClient and IAudioRenderClient
	mReleaseBuffer     = 4
	mGetNextPacketSize = 5 // IAudioCaptureClient
)

type comObj struct{ vtbl *[16]uintptr }

func (o *comObj) fn(m int) uintptr { return o.vtbl[m] }

func (o *comObj) release() {
	if o != nil {
		syscall.SyscallN(o.fn(mRelease), uintptr(unsafe.Pointer(o)))
	}
}

type hresult uint32

func (h hresult) Error() string { return fmt.Sprintf("Audio: Fehler 0x%08X", uint32(h)) }

func check(hr, _ uintptr, _ syscall.Errno) error {
	if int32(hr) < 0 {
		return hresult(uint32(hr))
	}
	return nil
}

var errDeviceChanged = errors.New("the default device changed")

// withEnumerator runs f with a device enumerator, on a thread of its own for COM.
func withEnumerator(f func(enum *comObj) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := coInitializeEx.Call(0, 0) // multithreaded
	if int32(hr) >= 0 {
		defer coUninitialize.Call()
	} else if uint32(hr) != rpcEChangedMode {
		return hresult(uint32(hr))
	}
	var enum *comObj
	hr, _, _ = coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&enum)))
	if int32(hr) < 0 {
		return hresult(uint32(hr))
	}
	defer enum.release()
	return f(enum)
}

// loopback records the default output device until ctx ends.
func loopback(ctx context.Context, piece func([]float32)) error {
	return followDefault(ctx, eRender, piece)
}

// defaultMic records the default microphone until ctx ends.
func defaultMic(ctx context.Context, piece func([]float32)) error {
	return followDefault(ctx, eCapture, piece)
}

// namedMic records the microphone Windows calls name until ctx ends; ok is false if there is
// none of that name (then ffmpeg tries it). When it goes away for a moment (a Bluetooth headset
// reconnects), the recording waits for it to come back.
func namedMic(ctx context.Context, name string, piece func([]float32)) (ok bool, err error) {
	err = withEnumerator(func(enum *comObj) error {
		pick := func() (*comObj, string, error) { return captureByName(enum, name) }
		if dev, _, err := pick(); err != nil {
			return err
		} else {
			dev.release()
		}
		ok = true
		for {
			err := recordDevice(ctx, enum, eCapture, pick, piece)
			if ctx.Err() != nil {
				return nil
			}
			var h hresult
			if !errors.Is(err, errNoDevice) && !(errors.As(err, &h) && h == deviceInvalidated) {
				return err
			}
			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				return nil
			}
		}
	})
	if !ok && errors.Is(err, errNoDevice) {
		err = nil
	}
	return ok, err
}

// captureByName is the active microphone Windows calls name and its id; errNoDevice if there is none.
func captureByName(enum *comObj, name string) (*comObj, string, error) {
	var coll *comObj
	if err := check(syscall.SyscallN(enum.fn(mEnumAudioEndpoints), uintptr(unsafe.Pointer(enum)), eCapture, deviceStateActive,
		uintptr(unsafe.Pointer(&coll)))); err != nil {
		return nil, "", err
	}
	defer coll.release()
	var n uint32
	if err := check(syscall.SyscallN(coll.fn(mGetCount), uintptr(unsafe.Pointer(coll)), uintptr(unsafe.Pointer(&n)))); err != nil {
		return nil, "", err
	}
	for i := range n {
		var dev *comObj
		if err := check(syscall.SyscallN(coll.fn(mItem), uintptr(unsafe.Pointer(coll)), uintptr(i), uintptr(unsafe.Pointer(&dev)))); err != nil {
			return nil, "", err
		}
		if friendlyName(dev) == name {
			id, err := deviceID(dev)
			if err != nil {
				dev.release()
				return nil, "", err
			}
			return dev, id, nil
		}
		dev.release()
	}
	return nil, "", errNoDevice
}

// followDefault records the default device of flow (eRender: what it plays), moving on to the
// next default when it changes or goes away.
func followDefault(ctx context.Context, flow uintptr, piece func([]float32)) error {
	return withEnumerator(func(enum *comObj) error {
		pick := func() (*comObj, string, error) { return defaultDevice(enum, flow) }
		for {
			err := recordDevice(ctx, enum, flow, pick, piece)
			if ctx.Err() != nil {
				return nil
			}
			var h hresult
			if !errors.Is(err, errDeviceChanged) && !(errors.As(err, &h) && h == deviceInvalidated) {
				return err
			}
			select {
			case <-time.After(200 * time.Millisecond):
			case <-ctx.Done():
				return nil
			}
		}
	})
}

// defaultDevice is the default device of flow and its id.
func defaultDevice(enum *comObj, flow uintptr) (*comObj, string, error) {
	var dev *comObj
	if err := check(syscall.SyscallN(enum.fn(mGetDefaultAudioEndpoint), uintptr(unsafe.Pointer(enum)), flow, eConsole,
		uintptr(unsafe.Pointer(&dev)))); err != nil {
		if h, _ := err.(hresult); h == errNotFound {
			if flow == eCapture {
				return nil, "", errNoDevice
			}
			return nil, "", errNoOutput
		}
		return nil, "", err
	}
	id, err := deviceID(dev)
	if err != nil {
		dev.release()
		return nil, "", err
	}
	return dev, id, nil
}

func deviceID(dev *comObj) (string, error) {
	var id *uint16
	if err := check(syscall.SyscallN(dev.fn(mGetID), uintptr(unsafe.Pointer(dev)), uintptr(unsafe.Pointer(&id)))); err != nil {
		return "", err
	}
	defer coTaskMemFree.Call(uintptr(unsafe.Pointer(id)))
	return utf16z(id), nil
}

// utf16z reads a NUL-terminated UTF-16 string.
func utf16z(p *uint16) string {
	n := 0
	for *(*uint16)(unsafe.Add(unsafe.Pointer(p), 2*n)) != 0 {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}

// friendlyName is the name Windows shows for dev, "" if it has none.
func friendlyName(dev *comObj) string {
	var store *comObj
	if check(syscall.SyscallN(dev.fn(mOpenPropertyStore), uintptr(unsafe.Pointer(dev)), stgmRead, uintptr(unsafe.Pointer(&store)))) != nil {
		return ""
	}
	defer store.release()
	var v [3]uint64 // PROPVARIANT
	if check(syscall.SyscallN(store.fn(mGetValue), uintptr(unsafe.Pointer(store)), uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)),
		uintptr(unsafe.Pointer(&v)))) != nil {
		return ""
	}
	defer propVariantClear.Call(uintptr(unsafe.Pointer(&v)))
	if uint16(v[0]) != vtLPWSTR || v[1] == 0 {
		return ""
	}
	return utf16z(*(**uint16)(unsafe.Pointer(&v[1])))
}

// deviceState is DeviceState from Windows: stamp holds the ids of the microphones connected and
// of the default.
func deviceState() (stamp, def string, ok bool) {
	err := withEnumerator(func(enum *comObj) error {
		var coll *comObj
		if err := check(syscall.SyscallN(enum.fn(mEnumAudioEndpoints), uintptr(unsafe.Pointer(enum)), eCapture, deviceStateActive,
			uintptr(unsafe.Pointer(&coll)))); err != nil {
			return err
		}
		defer coll.release()
		var n uint32
		if err := check(syscall.SyscallN(coll.fn(mGetCount), uintptr(unsafe.Pointer(coll)), uintptr(unsafe.Pointer(&n)))); err != nil {
			return err
		}
		var b strings.Builder
		for i := range n {
			var dev *comObj
			if err := check(syscall.SyscallN(coll.fn(mItem), uintptr(unsafe.Pointer(coll)), uintptr(i), uintptr(unsafe.Pointer(&dev)))); err != nil {
				return err
			}
			id, err := deviceID(dev)
			dev.release()
			if err != nil {
				return err
			}
			b.WriteString(id + "\n")
		}
		if dev, id, err := defaultDevice(enum, eCapture); err == nil {
			def = friendlyName(dev)
			dev.release()
			b.WriteString("default " + id)
		}
		stamp = b.String()
		return nil
	})
	return stamp, def, err == nil
}

// audioClient opens an audio client on dev in shared mode, in the device's own format.
func audioClient(dev *comObj, flags, duration uintptr) (client *comObj, format []byte, err error) {
	if err := check(syscall.SyscallN(dev.fn(mActivate), uintptr(unsafe.Pointer(dev)), uintptr(unsafe.Pointer(&iidIAudioClient)),
		clsctxAll, 0, uintptr(unsafe.Pointer(&client)))); err != nil {
		return nil, nil, err
	}
	var wf *byte
	if err := check(syscall.SyscallN(client.fn(mGetMixFormat), uintptr(unsafe.Pointer(client)), uintptr(unsafe.Pointer(&wf)))); err != nil {
		client.release()
		return nil, nil, err
	}
	defer coTaskMemFree.Call(uintptr(unsafe.Pointer(wf)))
	extra := binary.LittleEndian.Uint16(unsafe.Slice(wf, 18)[16:]) // cbSize
	format = append([]byte{}, unsafe.Slice(wf, 18+int(extra))...)
	if err := check(syscall.SyscallN(client.fn(mInitialize), uintptr(unsafe.Pointer(client)), 0, flags, duration, 0,
		uintptr(unsafe.Pointer(wf)), 0)); err != nil {
		client.release()
		return nil, nil, err
	}
	return client, format, nil
}

func service(client *comObj, iid *guid) (*comObj, error) {
	var s *comObj
	err := check(syscall.SyscallN(client.fn(mGetService), uintptr(unsafe.Pointer(client)), uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&s))))
	return s, err
}

// sampleFormat reads channels, rate and sample kind from a WAVEFORMATEX(TENSIBLE).
func sampleFormat(f []byte) (channels, rate int, kind sampleKind, err error) {
	le := binary.LittleEndian
	tag, channels, rate, bits := le.Uint16(f), int(le.Uint16(f[2:])), int(le.Uint32(f[4:])), le.Uint16(f[14:])
	float := tag == 3
	if tag == 0xFFFE && len(f) >= 40 {
		var sub guid
		sub.a, sub.b, sub.c = le.Uint32(f[24:]), le.Uint16(f[28:]), le.Uint16(f[30:])
		copy(sub.d[:], f[32:40])
		float = sub == subtypeIEEEFloat
	}
	switch {
	case float && bits == 32:
		kind = kindFloat32
	case !float && bits == 16:
		kind = kindInt16
	case !float && bits == 24:
		kind = kindInt24
	case !float && bits == 32:
		kind = kindInt32
	default:
		return 0, 0, 0, fmt.Errorf("Audio: unbekanntes Format (%d Bit)", bits)
	}
	if channels < 1 {
		return 0, 0, 0, errors.New("Audio: Format ohne Kanäle")
	}
	return channels, rate, kind, nil
}

// recordDevice records the device pick gives (of flow) until ctx ends or pick gives another. The
// device's buffer holds 2 s: sound the app has not read by then is lost. With 200 ms, a computer
// busy with recognizing speech on its processor read a Bluetooth headset too late for a fifth of
// its sound, so that the microphone fell further and further behind the system sound.
func recordDevice(ctx context.Context, enum *comObj, flow uintptr, pick func() (*comObj, string, error), piece func([]float32)) error {
	dev, id, err := pick()
	if err != nil {
		return err
	}
	defer dev.release()
	var flags uintptr
	if flow == eRender {
		flags = streamLoopback
	}
	client, format, err := audioClient(dev, flags, captureBuffer)
	if err != nil {
		return err
	}
	defer client.release()
	channels, rate, kind, err := sampleFormat(format)
	if err != nil {
		return err
	}
	block := int(binary.LittleEndian.Uint16(format[12:]))
	capt, err := service(client, &iidIAudioCaptureClient)
	if err != nil {
		return err
	}
	defer capt.release()

	var quiet *silence
	if flow == eRender {
		if quiet, err = playSilence(dev); err != nil {
			log.Println("live: playing silence for loopback:", err) // only gaps while nothing plays
		} else {
			defer quiet.close()
		}
	}

	if err := check(syscall.SyscallN(client.fn(mStart), uintptr(unsafe.Pointer(client)))); err != nil {
		return err
	}
	defer syscall.SyscallN(client.fn(mStop), uintptr(unsafe.Pointer(client)))

	rs := newResampler(rate, SR)
	gaps := gapLog{name: friendlyName(dev), read: time.Now(), last: time.Now()}
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for ticks := 1; ; ticks++ {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
		if quiet != nil {
			quiet.fill()
		}
		gaps.tick()
		for {
			var n uint32
			if err := check(syscall.SyscallN(capt.fn(mGetNextPacketSize), uintptr(unsafe.Pointer(capt)), uintptr(unsafe.Pointer(&n)))); err != nil {
				return err
			}
			if n == 0 {
				break
			}
			var data *byte
			var frames, flags uint32
			if err := check(syscall.SyscallN(capt.fn(mGetBuffer), uintptr(unsafe.Pointer(capt)), uintptr(unsafe.Pointer(&data)),
				uintptr(unsafe.Pointer(&frames)), uintptr(unsafe.Pointer(&flags)), 0, 0)); err != nil {
				return err
			}
			gaps.packet()
			var mono []float32
			if flags&bufferSilent != 0 || data == nil {
				mono = make([]float32, frames)
			} else {
				mono = downmix(unsafe.Slice(data, int(frames)*block), channels, kind)
			}
			if err := check(syscall.SyscallN(capt.fn(mReleaseBuffer), uintptr(unsafe.Pointer(capt)), uintptr(frames))); err != nil {
				return err
			}
			if pcm := rs.process(mono); len(pcm) > 0 {
				piece(pcm)
			}
		}
		if ticks%100 == 0 { // once a second: still the device to record (the default)?
			if d, now, err := pick(); err == nil {
				d.release()
				if now != id {
					return errDeviceChanged
				}
			}
		}
	}
}

// gapLog writes to the log when a device stops delivering sound and when it goes on, and when the
// app read it so late (a busy computer) that its sound may be lost: what the log should tell when
// parts of a recording are missing.
type gapLog struct {
	name       string
	read, last time.Time // when the device was last read, and when sound last came
	quiet      bool      // logged that none came for a while
}

func (g *gapLog) tick() {
	now := time.Now()
	if late := now.Sub(g.read); late > time.Second {
		log.Printf("live: %q was not read for %.1f s (a busy computer); beyond %d s its sound is lost", g.name, late.Seconds(), captureBuffer/10_000_000)
	}
	g.read = now
	if !g.quiet && now.Sub(g.last) > time.Second {
		g.quiet = true
		log.Printf("live: %q has delivered no sound for 1 s", g.name)
	}
}

func (g *gapLog) packet() {
	if g.quiet {
		g.quiet = false
		log.Printf("live: %q delivers sound again, after %.1f s without", g.name, time.Since(g.last).Seconds())
	}
	g.last = time.Now()
}

// silence plays silence on an output device, so that its loopback keeps running.
type silence struct {
	client, render *comObj
	size           uint32
}

func playSilence(dev *comObj) (*silence, error) {
	client, _, err := audioClient(dev, 0, renderBuffer)
	if err != nil {
		return nil, err
	}
	s := &silence{client: client}
	if err := check(syscall.SyscallN(client.fn(mGetBufferSize), uintptr(unsafe.Pointer(client)), uintptr(unsafe.Pointer(&s.size)))); err != nil {
		client.release()
		return nil, err
	}
	if s.render, err = service(client, &iidIAudioRenderClient); err != nil {
		client.release()
		return nil, err
	}
	s.fill()
	if err := check(syscall.SyscallN(client.fn(mStart), uintptr(unsafe.Pointer(client)))); err != nil {
		s.render.release()
		client.release()
		return nil, err
	}
	return s, nil
}

// fill tops the device's buffer up with silence.
func (s *silence) fill() {
	var padding uint32
	if check(syscall.SyscallN(s.client.fn(mGetCurrentPadding), uintptr(unsafe.Pointer(s.client)), uintptr(unsafe.Pointer(&padding)))) != nil {
		return
	}
	n := s.size - padding
	if n == 0 {
		return
	}
	var data *byte
	if check(syscall.SyscallN(s.render.fn(mGetBuffer), uintptr(unsafe.Pointer(s.render)), uintptr(n), uintptr(unsafe.Pointer(&data)))) != nil {
		return
	}
	syscall.SyscallN(s.render.fn(mReleaseBuffer), uintptr(unsafe.Pointer(s.render)), uintptr(n), bufferSilent)
}

func (s *silence) close() {
	syscall.SyscallN(s.client.fn(mStop), uintptr(unsafe.Pointer(s.client)))
	s.render.release()
	s.client.release()
}

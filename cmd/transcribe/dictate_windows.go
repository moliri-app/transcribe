// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// The Windows side of dictating (dictate.go): the shortcut and Esc are hotkeys of the tray
// window, the small window that shows the dictation is a second window on the tray's thread, and
// the text goes in through the clipboard and a Ctrl+V sent to the program in front, or typed
// as keystrokes when the clipboard is to stay as it was.

var (
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	procRegisterHotKey    = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey  = user32.NewProc("UnregisterHotKey")
	procSendMessage       = user32.NewProc("SendMessageW")
	procSendInput         = user32.NewProc("SendInput")
	procGetAsyncKeyState  = user32.NewProc("GetAsyncKeyState")
	procOpenClipboard     = user32.NewProc("OpenClipboard")
	procEmptyClipboard    = user32.NewProc("EmptyClipboard")
	procSetClipboardData  = user32.NewProc("SetClipboardData")
	procCloseClipboard    = user32.NewProc("CloseClipboard")
	procShowWindow        = user32.NewProc("ShowWindow")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
	procInvalidateRect    = user32.NewProc("InvalidateRect")
	procUpdateWindow      = user32.NewProc("UpdateWindow")
	procBeginPaint        = user32.NewProc("BeginPaint")
	procEndPaint          = user32.NewProc("EndPaint")
	procFillRect          = user32.NewProc("FillRect")
	procDrawText          = user32.NewProc("DrawTextW")
	procGetDC             = user32.NewProc("GetDC")
	procReleaseDC         = user32.NewProc("ReleaseDC")
	procGetForegroundWnd  = user32.NewProc("GetForegroundWindow")
	procMonitorFromWindow = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfo    = user32.NewProc("GetMonitorInfoW")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreateFont       = gdi32.NewProc("CreateFontW")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procGetStockObject   = gdi32.NewProc("GetStockObject")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procEllipse          = gdi32.NewProc("Ellipse")
	procGetDeviceCaps    = gdi32.NewProc("GetDeviceCaps")
	procCreateCompatDC   = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatBmp  = gdi32.NewProc("CreateCompatibleBitmap")
	procDeleteDC         = gdi32.NewProc("DeleteDC")
	procBitBlt           = gdi32.NewProc("BitBlt")

	procGlobalAlloc   = kernel32.NewProc("GlobalAlloc")
	procGlobalLock    = kernel32.NewProc("GlobalLock")
	procGlobalUnlock  = kernel32.NewProc("GlobalUnlock")
	procGlobalFree    = kernel32.NewProc("GlobalFree")
	procRtlMoveMemory = kernel32.NewProc("RtlMoveMemory")

	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	wmHotkey  = 0x0312
	wmPaint   = 0x000F
	wmMouseAc = 0x0021

	wmSetHotkey = 0x8000 + 3 // WM_APP+3: register the shortcut in tray.key
	wmEscape    = 0x8000 + 4 // WM_APP+4: wParam 1 registers Esc, 0 lets it go
	wmOverlay   = 0x8000 + 5 // WM_APP+5: show tray.ov as it is now

	hotkeyDictate = 1
	hotkeyEscape  = 2

	modAlt      = 0x1
	modControl  = 0x2
	modShift    = 0x4
	modNoRepeat = 0x4000

	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12
	vkEscape  = 0x1B
	vkSpace   = 0x20
	vkLWin    = 0x5B
	vkRWin    = 0x5C
	vkV       = 0x56
)

// hotkeys are the shortcuts one can choose (prefs.DictateKeys) as RegisterHotKey takes them.
var hotkeys = map[string][2]uintptr{
	"ctrl+alt+space":   {modControl | modAlt, vkSpace},
	"ctrl+shift+space": {modControl | modShift, vkSpace},
	"ctrl+alt+d":       {modControl | modAlt, 'D'},
}

// hotkey registers key as the shortcut for dictating, "" none.
func (t *tray) hotkey(key string) error {
	if t == nil {
		return errNoTray
	}
	t.mu.Lock()
	t.key = key
	t.mu.Unlock()
	procSendMessage.Call(t.wnd, wmSetHotkey, 0, 0)
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.keyErr
}

// setHotkey runs on the tray's thread.
func (t *tray) setHotkey() {
	procUnregisterHotKey.Call(t.wnd, hotkeyDictate)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.keyErr = nil
	hk, ok := hotkeys[t.key]
	if !ok {
		return
	}
	if r, _, _ := procRegisterHotKey.Call(t.wnd, hotkeyDictate, hk[0]|modNoRepeat, hk[1]); r == 0 {
		t.keyErr = errors.New("Ein anderes Programm benutzt dieses Tastenkürzel schon. Wähle ein anderes.")
	}
}

// escape catches Esc everywhere (on) while a dictation goes on, to cancel it.
func (t *tray) escape(on bool) {
	if t == nil {
		return
	}
	w := uintptr(0)
	if on {
		w = 1
	}
	procPostMessage.Call(t.wnd, wmEscape, w, 0)
}

// show shows o in the small window, for d if d > 0, else until the next show.
func (t *tray) show(o overlay, d time.Duration) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.ov = o
	t.ovSeq++
	seq := t.ovSeq
	t.mu.Unlock()
	procPostMessage.Call(t.wnd, wmOverlay, 0, 0)
	if d > 0 {
		time.AfterFunc(d, func() {
			t.mu.Lock()
			if t.ovSeq == seq {
				t.ov = overlay{}
			}
			t.mu.Unlock()
			procPostMessage.Call(t.wnd, wmOverlay, 0, 0)
		})
	}
}

// dictateMsg handles the tray window's messages for dictating; false if message is none of them.
func (t *tray) dictateMsg(message, wParam uintptr) bool {
	switch message {
	case wmHotkey:
		switch wParam {
		case hotkeyDictate:
			go t.actions.dictate()
		case hotkeyEscape:
			go t.actions.cancelDictate()
		}
	case wmSetHotkey:
		t.setHotkey()
	case wmEscape:
		procUnregisterHotKey.Call(t.wnd, hotkeyEscape)
		if wParam == 1 {
			procRegisterHotKey.Call(t.wnd, hotkeyEscape, modNoRepeat, vkEscape)
		}
	case wmOverlay:
		t.paintOverlay()
	default:
		return false
	}
	return true
}

// ---- the small window ----

const (
	wsPopup         = 0x80000000
	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080
	wsExNoActivate  = 0x08000000
	csDropShadow    = 0x00020000
	swHide          = 0
	swpNoActivate   = 0x0010
	swpShowWindow   = 0x0040
	hwndTopmost     = ^uintptr(0) // -1
	maNoActivate    = 3
	dtWordBreak     = 0x0010
	dtSingleLine    = 0x0020
	dtVCenter       = 0x0004
	dtRight         = 0x0002
	dtEndEllipsis   = 0x8000
	dtNoPrefix      = 0x0800
	dtEditControl   = 0x2000
	nullPen         = 8
	transparent     = 1
	logPixelsY      = 90
	monitorPrimary  = 1
	dwmCornerPref   = 33
	dwmCornerRound  = 2
	overlayBg       = 0x1b1c1d
	overlayText     = 0xfafafa
	overlayTextSoft = 0xb4b4b4
)

type rect struct{ left, top, right, bottom int32 }

type paintStruct struct {
	hdc       uintptr
	erase     int32
	paint     rect
	restore   int32
	incUpdate int32
	reserved  [32]byte
}

type monitorInfo struct {
	size    uint32
	monitor rect
	work    rect
	flags   uint32
}

// colorref turns 0xRRGGBB into the 0x00BBGGRR GDI takes.
func colorref(rgb uint32) uintptr {
	return uintptr(rgb>>16&0xff | rgb&0xff00 | rgb&0xff<<16)
}

// paintOverlay shows, updates or hides the small window as t.ov says; on the tray's thread.
func (t *tray) paintOverlay() {
	t.mu.Lock()
	o := t.ov
	t.mu.Unlock()
	if o.title == "" {
		if t.ovWnd != 0 {
			procShowWindow.Call(t.ovWnd, swHide)
		}
		return
	}
	if t.ovWnd == 0 {
		cls, _ := syscall.UTF16PtrFromString("TranscribeDiktat")
		bg, _, _ := procCreateSolidBrush.Call(colorref(overlayBg)) // what shows before the first paint
		wc := wndClassEx{style: csDropShadow, wndProc: syscall.NewCallback(wndProc), className: cls, background: bg}
		wc.size = uint32(unsafe.Sizeof(wc))
		procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
		title, _ := syscall.UTF16PtrFromString("Diktat")
		t.ovWnd, _, _ = procCreateWindowEx.Call(wsExTopmost|wsExToolWindow|wsExNoActivate, uintptr(unsafe.Pointer(cls)),
			uintptr(unsafe.Pointer(title)), wsPopup, 0, 0, 0, 0, 0, 0, 0, 0)
		if t.ovWnd == 0 {
			return
		}
		pref := uint32(dwmCornerRound) // rounded corners on Windows 11
		procDwmSetWindowAttribute.Call(t.ovWnd, dwmCornerPref, uintptr(unsafe.Pointer(&pref)), 4)
	}
	// at the bottom of the screen the program in front is on, above the taskbar
	s := t.scale()
	w, h := int32(620*s), int32(104*s)
	mon, _, _ := procMonitorFromWindow.Call(foreground(), monitorPrimary)
	mi := monitorInfo{}
	mi.size = uint32(unsafe.Sizeof(mi))
	procGetMonitorInfo.Call(mon, uintptr(unsafe.Pointer(&mi)))
	wa := mi.work
	w = min(w, wa.right-wa.left-int32(16*s))
	x, y := wa.left+(wa.right-wa.left-w)/2, wa.bottom-h-int32(28*s)
	procSetWindowPos.Call(t.ovWnd, hwndTopmost, uintptr(x), uintptr(y), uintptr(w), uintptr(h), swpNoActivate|swpShowWindow)
	procInvalidateRect.Call(t.ovWnd, 0, 0)
	procUpdateWindow.Call(t.ovWnd) // painted now, before the screen shows it empty
}

func foreground() uintptr {
	w, _, _ := procGetForegroundWnd.Call()
	return w
}

// scale is the screen's DPI over 96 (the app is DPI aware).
func (t *tray) scale() float64 {
	dc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, dc)
	dpi, _, _ := procGetDeviceCaps.Call(dc, logPixelsY)
	if dpi == 0 {
		return 1
	}
	return float64(dpi) / 96
}

// overlayProc is the small window's window procedure.
func (t *tray) overlayProc(wnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmMouseAc:
		return maNoActivate // a click on it leaves the program in front where it is
	case wmLButtonUp:
		t.mu.Lock()
		id := t.ov.note
		t.mu.Unlock()
		if id != "" {
			go t.actions.openNote(id)
		}
		return 0
	case wmPaint:
		t.mu.Lock()
		o := t.ov
		t.mu.Unlock()
		var ps paintStruct
		dc, _, _ := procBeginPaint.Call(wnd, uintptr(unsafe.Pointer(&ps)))
		// drawn aside and copied in one go, so the text does not flicker as it grows
		s := t.scale()
		w, h := uintptr(620*s), uintptr(104*s)
		mem, _, _ := procCreateCompatDC.Call(dc)
		bmp, _, _ := procCreateCompatBmp.Call(dc, w, h)
		old, _, _ := procSelectObject.Call(mem, bmp)
		t.drawOverlay(mem, o)
		const srcCopy = 0x00CC0020
		procBitBlt.Call(dc, 0, 0, w, h, mem, 0, 0, srcCopy)
		procSelectObject.Call(mem, old)
		procDeleteObject.Call(bmp)
		procDeleteDC.Call(mem)
		procEndPaint.Call(wnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	}
	r, _, _ := procDefWindowProc.Call(wnd, message, wParam, lParam)
	return r
}

func (t *tray) drawOverlay(dc uintptr, o overlay) {
	s := t.scale()
	px := func(v float64) int32 { return int32(v * s) }
	var all rect
	all.right, all.bottom = px(620), px(104)
	bg, _, _ := procCreateSolidBrush.Call(colorref(overlayBg))
	procFillRect.Call(dc, uintptr(unsafe.Pointer(&all)), bg)
	procDeleteObject.Call(bg)

	dot, _, _ := procCreateSolidBrush.Call(colorref(o.dot))
	oldBrush, _, _ := procSelectObject.Call(dc, dot)
	pen, _, _ := procGetStockObject.Call(nullPen)
	oldPen, _, _ := procSelectObject.Call(dc, pen)
	procEllipse.Call(dc, uintptr(px(20)), uintptr(px(22)), uintptr(px(34)), uintptr(px(36)))
	procSelectObject.Call(dc, oldPen)
	procSelectObject.Call(dc, oldBrush)
	procDeleteObject.Call(dot)

	procSetBkMode.Call(dc, transparent)
	text := func(str string, size float64, weight int, rgb uint32, r rect, flags uintptr) {
		if str == "" {
			return
		}
		face, _ := syscall.UTF16PtrFromString("Segoe UI")
		f, _, _ := procCreateFont.Call(uintptr(-px(size)), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
		old, _, _ := procSelectObject.Call(dc, f)
		procSetTextColor.Call(dc, colorref(rgb))
		u, _ := syscall.UTF16FromString(str)
		procDrawText.Call(dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&r)), flags|dtNoPrefix)
		procSelectObject.Call(dc, old)
		procDeleteObject.Call(f)
	}
	head := rect{px(44), px(14), all.right - px(20), px(44)}
	text(o.hint, 14, 400, overlayTextSoft, head, dtSingleLine|dtVCenter|dtRight|dtEndEllipsis)
	text(o.title, 18, 600, overlayText, head, dtSingleLine|dtVCenter|dtEndEllipsis)
	body := rect{px(44), px(48), all.right - px(20), all.bottom - px(10)}
	text(strings.TrimSpace(o.body), 16, 400, overlayText, body, dtWordBreak|dtEditControl|dtEndEllipsis)
}

// ---- clipboard and Ctrl+V ----

// setClipboard puts text into the clipboard (as the window of the tray).
func setClipboard(text string) error {
	runtime.LockOSThread() // the clipboard is opened and closed on the same thread
	defer runtime.UnlockOSThread()
	var owner uintptr
	if theTray != nil {
		owner = theTray.wnd
	}
	opened := false
	for range 20 { // another program may have it open for a moment
		if r, _, _ := procOpenClipboard.Call(owner); r != 0 {
			opened = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !opened {
		return errors.New("Die Zwischenablage ist von einem anderen Programm belegt.")
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()
	u, _ := syscall.UTF16FromString(strings.ReplaceAll(text, "\n", "\r\n"))
	const gmemMoveable, cfUnicodeText = 0x0002, 13
	mem, _, err := procGlobalAlloc.Call(gmemMoveable, uintptr(len(u)*2))
	if mem == 0 {
		return err
	}
	p, _, _ := procGlobalLock.Call(mem)
	procRtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)*2))
	procGlobalUnlock.Call(mem)
	if r, _, err := procSetClipboardData.Call(cfUnicodeText, mem); r == 0 {
		procGlobalFree.Call(mem)
		return err
	}
	return nil
}

type keyInput struct {
	typ   uint32
	_     uint32 // the union after it is aligned for its pointer-sized member
	vk    uint16
	scan  uint16
	flags uint32
	time  uint32
	extra uintptr
	_     [8]byte // INPUT is as large as its largest member, a MOUSEINPUT
}

// typeText types text into the program in front, as keystrokes of its characters, once the
// keys of the shortcut are let go. A line break is the Enter key.
func typeText(text string) {
	waitForKeysUp()
	const inputKeyboard, keyUp, unicode, vkReturn = 1, 0x0002, 0x0004, 0x0D
	var in []keyInput
	u, _ := syscall.UTF16FromString(text)
	for _, c := range u[:len(u)-1] { // without the closing 0
		if c == '\n' {
			in = append(in, keyInput{typ: inputKeyboard, vk: vkReturn}, keyInput{typ: inputKeyboard, vk: vkReturn, flags: keyUp})
			continue
		}
		in = append(in, keyInput{typ: inputKeyboard, scan: c, flags: unicode}, keyInput{typ: inputKeyboard, scan: c, flags: unicode | keyUp})
	}
	if len(in) > 0 {
		procSendInput.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0]))
	}
}

// paste sends Ctrl+V to the program in front, once the keys of the shortcut are let go (else it
// would get Ctrl+Alt+V).
func paste() {
	waitForKeysUp()
	const inputKeyboard, keyUp = 1, 0x0002
	in := []keyInput{
		{typ: inputKeyboard, vk: vkControl},
		{typ: inputKeyboard, vk: vkV},
		{typ: inputKeyboard, vk: vkV, flags: keyUp},
		{typ: inputKeyboard, vk: vkControl, flags: keyUp},
	}
	procSendInput.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0]))
}

// waitForKeysUp waits (up to two seconds) until the keys of the shortcut are let go.
func waitForKeysUp() {
	for range 100 {
		down := false
		for _, vk := range []uintptr{vkShift, vkControl, vkMenu, vkLWin, vkRWin, vkSpace, 'D'} {
			if r, _, _ := procGetAsyncKeyState.Call(vk); r&0x8000 != 0 {
				down = true
			}
		}
		if !down {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
}

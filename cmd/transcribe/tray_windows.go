// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// The icon in the notification area (system tray): a hidden window receives its
// clicks; a left click opens the app, a right click shows the menu.

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")

	procRegisterClassEx          = user32.NewProc("RegisterClassExW")
	procCreateWindowEx           = user32.NewProc("CreateWindowExW")
	procDefWindowProc            = user32.NewProc("DefWindowProcW")
	procDestroyWindow            = user32.NewProc("DestroyWindow")
	procGetMessage               = user32.NewProc("GetMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessage          = user32.NewProc("DispatchMessageW")
	procPostMessage              = user32.NewProc("PostMessageW")
	procPostQuitMessage          = user32.NewProc("PostQuitMessage")
	procRegisterWindowMessage    = user32.NewProc("RegisterWindowMessageW")
	procCreatePopupMenu          = user32.NewProc("CreatePopupMenu")
	procAppendMenu               = user32.NewProc("AppendMenuW")
	procSetMenuDefaultItem       = user32.NewProc("SetMenuDefaultItem")
	procTrackPopupMenu           = user32.NewProc("TrackPopupMenu")
	procDestroyMenu              = user32.NewProc("DestroyMenu")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procSetProcessDPIAware       = user32.NewProc("SetProcessDPIAware")
	procCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	procShellNotifyIcon          = shell32.NewProc("Shell_NotifyIconW")
)

const (
	wmNull    = 0x0000
	wmDestroy = 0x0002
	wmClose   = 0x0010
	wmTray    = 0x8000 + 1 // WM_APP+1: the icon's callback message
	wmNotify  = 0x8000 + 2 // WM_APP+2: show the pending notification

	wmLButtonUp     = 0x0202
	wmRButtonUp     = 0x0205
	ninBalloonClick = 0x0405 // NIN_BALLOONUSERCLICK

	nimAdd        = 0
	nimModify     = 1
	nimDelete     = 2
	nifMessage    = 0x01
	nifIcon       = 0x02
	nifTip        = 0x04
	nifInfo       = 0x10
	niifUser      = 0x04
	niifLargeIcon = 0x20

	mfString       = 0x0000
	mfGrayed       = 0x0001
	mfSeparator    = 0x0800
	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	smCxIcon   = 11
	smCxSmIcon = 49
)

const (
	menuOpen = iota + 1
	menuSettings
	menuQuit
	menuUpdate
)

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

type notifyIconData struct {
	size        uint32
	wnd         uintptr
	id          uint32
	flags       uint32
	callbackMsg uint32
	icon        uintptr
	tip         [128]uint16
	state       uint32
	stateMask   uint32
	info        [256]uint16
	version     uint32
	infoTitle   [64]uint16
	infoFlags   uint32
	guid        [16]byte
	balloonIcon uintptr
}

type msg struct {
	wnd     uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
	private uint32
}

type tray struct {
	wnd            uintptr
	actions        trayActions
	icon, bigIcon  uintptr
	taskbarCreated uintptr
	done           chan struct{}

	mu      sync.Mutex
	balloon [2]string // title and text for wmNotify
	click   func()    // what a click on the notification does; nil opens the app
	key     string    // the shortcut for dictating, "" none (dictate_windows.go)
	keyErr  error     // why it could not be registered
	ov      overlay   // what the small window shows while dictating
	ovSeq   int       // counts the shows, so a timed hide only hides its own
	ovWnd   uintptr   // the small window, once it was shown; used on the tray's thread only
}

// the window procedure is a C callback without context; there is one tray per process
var theTray *tray

// startTray shows the icon; nil when that fails (the app still runs, only without icon).
func startTray(a trayActions) *tray {
	t := &tray{actions: a, done: make(chan struct{})}
	ready := make(chan bool)
	go t.loop(ready)
	if !<-ready {
		return nil
	}
	return t
}

// loop creates the window and the icon, then pumps messages on this locked thread.
func (t *tray) loop(ready chan<- bool) {
	runtime.LockOSThread()
	defer close(t.done)
	procSetProcessDPIAware.Call() // a sharp icon and menu on scaled displays

	cls, _ := syscall.UTF16PtrFromString("TranscribeTray")
	wc := wndClassEx{wndProc: syscall.NewCallback(wndProc), className: cls}
	wc.size = uint32(unsafe.Sizeof(wc))
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	title, _ := syscall.UTF16PtrFromString("Transcribe by Moliri")
	wnd, _, err := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)),
		0, 0, 0, 0, 0, 0, 0, 0, 0)
	if wnd == 0 {
		log.Println("tray:", err)
		ready <- false
		return
	}
	t.wnd = wnd
	theTray = t
	tc, _ := syscall.UTF16PtrFromString("TaskbarCreated")
	t.taskbarCreated, _, _ = procRegisterWindowMessage.Call(uintptr(unsafe.Pointer(tc)))
	small, _, _ := procGetSystemMetrics.Call(smCxSmIcon)
	large, _, _ := procGetSystemMetrics.Call(smCxIcon)
	t.icon, t.bigIcon = makeIcon(int(small)), makeIcon(int(large))
	if !t.add() {
		// right after sign-in the taskbar may not be there yet; TaskbarCreated adds the icon then
		log.Println("tray: no taskbar yet, the icon follows when it appears")
	}
	ready <- true

	var m msg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (t *tray) data(flags uint32) *notifyIconData {
	d := &notifyIconData{wnd: t.wnd, id: 1, flags: flags, callbackMsg: wmTray, icon: t.icon}
	d.size = uint32(unsafe.Sizeof(*d))
	copy(d.tip[:len(d.tip)-1], utf16("Transcribe by Moliri"))
	return d
}

func (t *tray) add() bool {
	r, _, _ := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(t.data(nifMessage|nifIcon|nifTip))))
	return r != 0
}

// notify shows a notification at the icon (on Windows 10 and 11 as a toast); a click on it opens
// the app.
func (t *tray) notify(title, text string) { t.notifyThen(title, text, nil) }

// notifyThen shows a notification at the icon, and a click on it does click (nil: opens the app).
func (t *tray) notifyThen(title, text string, click func()) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.balloon = [2]string{title, text}
	t.click = click
	t.mu.Unlock()
	procPostMessage.Call(t.wnd, wmNotify, 0, 0)
}

// close removes the icon; without it a dead icon stays until the mouse passes over it.
func (t *tray) close() {
	if t == nil {
		return
	}
	procPostMessage.Call(t.wnd, wmClose, 0, 0)
	select {
	case <-t.done:
	case <-time.After(time.Second):
	}
}

func wndProc(wnd, message, wParam, lParam uintptr) uintptr {
	t := theTray
	switch {
	case t != nil && t.ovWnd != 0 && wnd == t.ovWnd:
		return t.overlayProc(wnd, message, wParam, lParam)
	case t == nil || wnd != t.wnd:
	case t.dictateMsg(message, wParam):
		return 0
	case message == wmTray:
		switch lParam & 0xffff {
		case wmLButtonUp:
			go t.actions.open()
		case ninBalloonClick:
			t.mu.Lock()
			click := t.click
			t.mu.Unlock()
			if click == nil {
				click = t.actions.open
			}
			go click()
		case wmRButtonUp:
			t.menu()
		}
		return 0
	case message == wmNotify:
		t.mu.Lock()
		title, text := t.balloon[0], t.balloon[1]
		t.mu.Unlock()
		d := t.data(nifInfo)
		copy(d.infoTitle[:len(d.infoTitle)-1], utf16(title))
		copy(d.info[:len(d.info)-1], utf16(text))
		d.infoFlags = niifUser | niifLargeIcon
		d.balloonIcon = t.bigIcon
		if r, _, err := procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(d))); r == 0 {
			log.Println("tray: notification:", err)
		}
		return 0
	case message == t.taskbarCreated:
		t.add() // the taskbar appeared, or Explorer was restarted
		return 0
	case message == wmClose:
		procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(t.data(0))))
		procDestroyWindow.Call(wnd)
		return 0
	case message == wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(wnd, message, wParam, lParam)
	return r
}

func (t *tray) menu() {
	m, _, _ := procCreatePopupMenu.Call()
	defer procDestroyMenu.Call(m)
	item := func(id uintptr, text string) {
		p, _ := syscall.UTF16PtrFromString(text)
		procAppendMenu.Call(m, mfString, id, uintptr(unsafe.Pointer(p)))
	}
	item(menuOpen, "Transcribe öffnen")
	item(menuSettings, "Einstellungen")
	if t.actions.updateItem != nil {
		if text, enabled := t.actions.updateItem(); text != "" {
			p, _ := syscall.UTF16PtrFromString(text)
			flags := uintptr(mfString)
			if !enabled {
				flags |= mfGrayed
			}
			procAppendMenu.Call(m, mfSeparator, 0, 0)
			procAppendMenu.Call(m, flags, menuUpdate, uintptr(unsafe.Pointer(p)))
		}
	}
	procAppendMenu.Call(m, mfSeparator, 0, 0)
	item(menuQuit, "Beenden")
	procSetMenuDefaultItem.Call(m, menuOpen, 0)

	var pt struct{ x, y int32 }
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// the menu only closes on a click elsewhere if our window is in front
	procSetForegroundWindow.Call(t.wnd)
	cmd, _, _ := procTrackPopupMenu.Call(m, tpmRightButton|tpmReturnCmd, uintptr(pt.x), uintptr(pt.y), 0, t.wnd, 0)
	procPostMessage.Call(t.wnd, wmNull, 0, 0)
	switch cmd {
	case menuOpen:
		go t.actions.open()
	case menuSettings:
		go t.actions.settings()
	case menuQuit:
		go t.actions.quit()
	case menuUpdate:
		go t.actions.update()
	}
}

// makeIcon draws the app icon (frontend/static/favicon.svg: four level bars on a
// dark tile, the second one green) at size pixels and turns it into an HICON.
func makeIcon(size int) uintptr {
	size = max(size, 16)
	type shape struct {
		x, y, w, h, r float64
		rgb           uint32
	}
	shapes := []shape{
		{0, 0, 24, 24, 5, 0x1b1c1d},
		{3.5, 9, 2.6, 6, 1.3, 0xfafafa},
		{8, 4.5, 2.6, 15, 1.3, 0x62eb00},
		{12.5, 7, 2.6, 10, 1.3, 0xfafafa},
		{17, 9.5, 2.6, 5, 1.3, 0xfafafa},
	}
	inside := func(s shape, x, y float64) bool {
		if x < s.x || x > s.x+s.w || y < s.y || y > s.y+s.h {
			return false
		}
		dx := max(s.x+s.r-x, 0, x-(s.x+s.w-s.r))
		dy := max(s.y+s.r-y, 0, y-(s.y+s.h-s.r))
		return dx*dx+dy*dy <= s.r*s.r
	}
	const n = 4 // samples per axis, for smooth edges
	scale := 24 / float64(size)
	pixels := make([]byte, 0, size*size*4)
	for py := size - 1; py >= 0; py-- { // a DIB is stored bottom-up
		for px := 0; px < size; px++ {
			var r, g, b, hits uint32
			for sy := range n {
				for sx := range n {
					x := (float64(px) + (float64(sx)+0.5)/n) * scale
					y := (float64(py) + (float64(sy)+0.5)/n) * scale
					for i := len(shapes) - 1; i >= 0; i-- {
						if inside(shapes[i], x, y) {
							c := shapes[i].rgb
							r, g, b, hits = r+c>>16, g+c>>8&0xff, b+c&0xff, hits+1
							break
						}
					}
				}
			}
			if hits == 0 {
				pixels = append(pixels, 0, 0, 0, 0)
				continue
			}
			pixels = append(pixels, byte(b/hits), byte(g/hits), byte(r/hits), byte(hits*255/(n*n)))
		}
	}
	var buf bytes.Buffer
	// BITMAPINFOHEADER; the height counts the colour image and the (empty) AND mask
	binary.Write(&buf, binary.LittleEndian, struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		XPelsPerM, YPelsPerM   int32
		ClrUsed, ClrImportant  uint32
	}{40, int32(size), int32(2 * size), 1, 32, 0, uint32(len(pixels)), 0, 0, 0, 0})
	buf.Write(pixels)
	buf.Write(make([]byte, size*((size+31)/32*4)))
	b := buf.Bytes()
	h, _, _ := procCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 1, 0x00030000,
		uintptr(size), uintptr(size), 0)
	return h
}

func utf16(s string) []uint16 {
	u, _ := syscall.UTF16FromString(s)
	return u
}

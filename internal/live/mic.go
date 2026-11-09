// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package live

import (
	"bytes"
	"cmp"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/proc"
)

// The app records the microphone through ffmpeg: DirectShow on Windows, PulseAudio on Linux,
// AVFoundation on macOS. On Windows, WASAPI records the microphone chosen (system_windows.go), and
// ffmpeg only one WASAPI does not know by that name. With no microphone chosen ("System" on the
// Live page) it records the system's default one; on Windows through WASAPI, so that the recording
// moves on when another becomes the default (a headset is plugged in). What the computer plays (the others in an online meeting) comes from
// WASAPI loopback on Windows (system_windows.go) and from the PulseAudio monitor of the default
// output on Linux; both together are mixed here. TRANSCRIBE_LIVE_INPUT names a file to play in
// real time instead of the microphone (for tests and demos without one),
// TRANSCRIBE_LIVE_SYSTEM_INPUT one instead of the system sound; silence follows its end. Several
// files (separated like PATH) play in turn, one per recording.

// What the app records.
const (
	SourceMic    = "mic"    // the microphone chosen on the Live page, or the system's default
	SourceSystem = "system" // what the computer plays
	SourceBoth   = "both"   // both, mixed
)

// NormSource is s if it names a source, else the microphone.
func NormSource(s string) string {
	switch s {
	case SourceSystem, SourceBoth:
		return s
	}
	return SourceMic
}

// SystemAudio reports whether this computer can record what it plays.
func SystemAudio() bool {
	return runtime.GOOS != "darwin" || os.Getenv("TRANSCRIBE_LIVE_SYSTEM_INPUT") != ""
}

var dshowAudio = regexp.MustCompile(`"([^"]+)"\s+\(audio\)`)

// Devices lists the recording devices ffmpeg sees, in its order. Only Windows names them;
// elsewhere the list is empty and the system default is recorded.
func Devices(ctx context.Context) []string {
	if names, _, _, ok := testDevices(); ok {
		return names
	}
	if runtime.GOOS != "windows" {
		return nil
	}
	var out bytes.Buffer
	cmd := proc.Hide(exec.CommandContext(ctx, media.Tool("ffmpeg"), "-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy"))
	cmd.Stdout, cmd.Stderr = &out, &out
	_ = cmd.Run() // always fails: "dummy" is no device
	var names []string
	for _, m := range dshowAudio.FindAllStringSubmatch(out.String(), -1) {
		names = append(names, m[1])
	}
	return names
}

var micFiles, systemFiles atomic.Int64

// testInput is the next file named in env, "" if env is not set.
func testInput(env string, n *atomic.Int64) string {
	files := filepath.SplitList(os.Getenv(env))
	if len(files) == 0 {
		return ""
	}
	return files[int(n.Add(1)-1)%len(files)]
}

// play hands out file in real time, in 100 ms pieces, and silence after its end, until ctx ends.
// The app keeps the time itself (rather than ffmpeg in real time), so that on a busy computer it
// catches up after a stall, as a device does.
func play(ctx context.Context, file string, piece func([]float32)) error {
	cmd := proc.Hide(exec.CommandContext(ctx, media.Tool("ffmpeg"), "-nostdin", "-v", "error", "-i", file,
		"-ac", "1", "-ar", "16000", "-f", "s16le", "-"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return captureError(cmp.Or(strings.TrimSpace(stderr.String()), err.Error()))
	}
	pcm := s16le(b)
	start := time.Now()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for pos := 0; ; {
		for due := int(time.Since(start).Seconds() * SR); pos+SR/10 <= due; pos += SR / 10 {
			p := make([]float32, SR/10)
			if pos < len(pcm) {
				copy(p, pcm[pos:])
			}
			piece(p)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

func micArgs(device string) []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"-f", "dshow", "-audio_buffer_size", "50", "-i", "audio=" + device}
	case "darwin":
		if device == "" {
			device = "default"
		}
		return []string{"-f", "avfoundation", "-i", ":" + device}
	default:
		if device == "" {
			device = "default"
		}
		return []string{"-f", "pulse", "-i", device}
	}
}

// capture records source until ctx ends and hands out 100 ms pieces as 16 kHz mono samples: the
// recording, and what the microphone and the system sound each gave to it (nil if not recorded).
// It returns when the recording ends; the error tells why if it failed.
func capture(ctx context.Context, source, device string, piece func(pcm, mic, system []float32)) error {
	switch source {
	case SourceSystem:
		return recordSystem(ctx, chunks(func(pcm []float32) { piece(pcm, nil, pcm) }))
	case SourceBoth:
		return mixed(ctx, device, piece)
	default:
		return recordMic(ctx, device, func(pcm []float32) { piece(pcm, pcm, nil) })
	}
}

// recordMic records the microphone in 100 ms pieces.
func recordMic(ctx context.Context, device string, piece func([]float32)) error {
	if f := testInput("TRANSCRIBE_LIVE_INPUT", &micFiles); f != "" {
		return play(ctx, f, piece)
	}
	if runtime.GOOS == "windows" {
		if device == "" {
			return defaultMic(ctx, chunks(piece))
		}
		if ok, err := namedMic(ctx, device, chunks(piece)); ok {
			return err
		}
	}
	return record(ctx, micArgs(device), errEnded, piece)
}

// recordSystem records what the computer plays, in pieces of any size.
func recordSystem(ctx context.Context, piece func([]float32)) error {
	if f := testInput("TRANSCRIBE_LIVE_SYSTEM_INPUT", &systemFiles); f != "" {
		return play(ctx, f, piece)
	}
	switch runtime.GOOS {
	case "windows":
		return loopback(ctx, piece)
	case "darwin":
		return errNoSystem
	default:
		return record(ctx, []string{"-f", "pulse", "-i", "@DEFAULT_MONITOR@"}, errSystemEnded, piece)
	}
}

// Mixing: the microphone sets the pace. The system sound waits in a buffer of mixDelay before it
// is added, so pieces that come a little early or late on either side leave no gaps. When the
// system sound stalls, silence stands in for it until the buffer has filled again. When the
// microphone stalls (a Bluetooth headset that delivers nothing for a while, sound lost on a busy
// computer), the system sound piles
// up; once it is mixLag ahead, it sets the pace itself, with silence for the microphone, until the
// microphone gives sound again. So nothing of what the others in a meeting say is lost.
const (
	mixDelay = SR / 5 // 200 ms
	mixLag   = SR     // 1 s
)

func mixed(ctx context.Context, device string, piece func(pcm, mic, system []float32)) error {
	mic := func(ctx context.Context, piece func([]float32)) error { return recordMic(ctx, device, piece) }
	return mix(ctx, mic, recordSystem, piece)
}

// mix mixes what mic records (in 100 ms pieces) and what system records (in pieces of any size).
func mix(ctx context.Context, mic, system func(context.Context, func([]float32)) error, piece func(pcm, mic, system []float32)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var out sync.Mutex // one piece handed out at a time, in order
	var mu sync.Mutex  // guards what follows
	var buf []float32
	filling := true
	alone := 0 // samples of the system sound handed out without the microphone since it last gave sound
	sysErr := make(chan error, 1)
	go func() {
		err := system(ctx, func(pcm []float32) {
			mu.Lock()
			buf = append(buf, pcm...)
			mu.Unlock()
			if !out.TryLock() {
				return // the microphone hands out a piece just now
			}
			defer out.Unlock()
			for {
				mu.Lock()
				if len(buf) < mixDelay+SR/10 || alone == 0 && len(buf) <= mixDelay+mixLag {
					mu.Unlock()
					return
				}
				if alone == 0 {
					log.Printf("live: the microphone fell %d s behind the system audio (it gave no sound); the system audio goes on with silence for it", mixLag/SR)
				}
				sys := slices.Clone(buf[:SR/10])
				buf = buf[SR/10:]
				alone += len(sys)
				filling = false
				mu.Unlock()
				piece(sys, make([]float32, len(sys)), sys)
			}
		})
		sysErr <- err
		cancel() // without the system sound the recording ends
	}()
	err := mic(ctx, func(mic []float32) {
		out.Lock()
		defer out.Unlock()
		sys := make([]float32, len(mic))
		mu.Lock()
		if alone > 0 {
			log.Printf("live: the microphone gives sound again; %.1f s of the system audio went on without it", float64(alone)/SR)
			alone = 0
		}
		if filling && len(buf) >= mixDelay {
			filling = false
		}
		if !filling {
			n := copy(sys, buf)
			buf = buf[n:]
			filling = n < len(sys)
		}
		mu.Unlock()
		pcm := make([]float32, len(mic))
		for i := range pcm {
			pcm[i] = mic[i] + sys[i]
		}
		piece(pcm, mic, sys)
	})
	cancel()
	err = cmp.Or(err, <-sysErr)
	if alone > 0 {
		log.Printf("live: the microphone gave no sound up to the end; %.1f s of the system audio went on without it", float64(alone)/SR)
	}
	return err
}

// chunks gathers pieces of any size into 100 ms ones.
func chunks(piece func([]float32)) func([]float32) {
	var buf []float32
	return func(pcm []float32) {
		buf = append(buf, pcm...)
		for len(buf) >= SR/10 {
			piece(slices.Clone(buf[:SR/10]))
			buf = buf[SR/10:]
		}
	}
}

// record runs ffmpeg on input and hands out 100 ms pieces as 16 kHz mono samples until ctx ends.
// It returns nil once ctx has ended, and ended if the input stopped by itself.
func record(ctx context.Context, input []string, ended error, piece func([]float32)) error {
	args := append([]string{"-nostdin", "-hide_banner", "-v", "error"}, input...)
	args = append(args, "-ac", "1", "-ar", "16000", "-f", "s16le", "-")
	cmd := proc.Hide(exec.CommandContext(ctx, media.Tool("ffmpeg"), args...))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		if ctx.Err() != nil {
			return nil // stopped before it began
		}
		return err
	}
	buf := make([]byte, SR/10*2)
	for {
		if _, err := io.ReadFull(out, buf); err != nil {
			break
		}
		piece(s16le(buf))
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		return nil
	}
	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		return captureError(msg)
	}
	if err != nil {
		return err
	}
	return ended
}

type captureError string

func (e captureError) Error() string { return "Aufnahme fehlgeschlagen: " + string(e) }

var (
	errNoDevice    = captureError("kein Mikrofon gefunden")
	errEnded       = captureError("das Mikrofon hat aufgehört zu liefern")
	errNoOutput    = captureError("kein Ausgabegerät für das Systemaudio gefunden")
	errSystemEnded = captureError("das Systemaudio hat aufgehört zu liefern")
	errNoSystem    = captureError("Systemaudio aufzunehmen geht auf dem Mac noch nicht")
	errSystemOnly  = errors.New("Gerade läuft eine Aufnahme nur vom Systemaudio. Zum Diktieren erst diese Aufnahme auf der Seite Live beenden.")
	errNoSound     = errors.New("Es kam kein Ton an, bevor die Aufnahme beendet wurde, darum ist nichts gespeichert.")
)

func s16le(b []byte) []float32 {
	pcm := make([]float32, len(b)/2)
	for i := range pcm {
		pcm[i] = float32(int16(binary.LittleEndian.Uint16(b[2*i:]))) / 32768
	}
	return pcm
}

// wavWriter saves 16 kHz mono 16-bit PCM; the sizes in the header are written on Close.
type wavWriter struct {
	f *os.File
	n uint32
}

func createWav(path string) (*wavWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := &wavWriter{f: f}
	if err := w.header(); err != nil {
		f.Close()
		return nil, err
	}
	return w, nil
}

func (w *wavWriter) header() error {
	h := make([]byte, 44)
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], 36+w.n)
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1) // PCM
	binary.LittleEndian.PutUint16(h[22:], 1) // mono
	binary.LittleEndian.PutUint32(h[24:], SR)
	binary.LittleEndian.PutUint32(h[28:], SR*2)
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], w.n)
	_, err := w.f.WriteAt(h, 0)
	return err
}

func (w *wavWriter) Write(pcm []float32) error {
	b := make([]byte, 2*len(pcm))
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(int16(max(-1, min(1, v))*32767)))
	}
	_, err := w.f.WriteAt(b, int64(44+w.n))
	w.n += uint32(len(b))
	return err
}

func (w *wavWriter) Close() error {
	err := w.header()
	if cerr := w.f.Close(); err == nil {
		err = cerr
	}
	return err
}

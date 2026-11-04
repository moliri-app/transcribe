// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/moliri-app/transcribe/internal/autostart"
	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/internal/runner"
	"github.com/moliri-app/transcribe/internal/server"
	"github.com/moliri-app/transcribe/internal/setup"
	"github.com/moliri-app/transcribe/internal/update"
)

func defaultDataDir() string {
	if d := os.Getenv("TRANSCRIBE_DATA"); d != "" {
		return d
	}
	// portable: everything lives in a folder next to the executable
	exe, err := os.Executable()
	if err != nil {
		return "data"
	}
	return filepath.Join(filepath.Dir(exe), "data")
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// fail reports a fatal error in a message box (the Windows build has no console).
func fail(err error) {
	log.Println("Fehler:", err)
	showError("Fehler: " + err.Error())
	os.Exit(1)
}

// setupLog writes the log to <data>/transcribe.log (and stderr when a console exists).
func setupLog(dataDir string) {
	_ = os.MkdirAll(dataDir, 0o755)
	f, err := os.OpenFile(filepath.Join(dataDir, "transcribe.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err == nil {
		log.SetOutput(io.MultiWriter(f, os.Stderr))
	}
}

func main() {
	data := flag.String("data", defaultDataDir(), "data directory (audio/, results/, models/)")
	port := flag.Int("port", 8765, "HTTP port")
	noBrowser := flag.Bool("no-browser", false, "do not open the browser")
	background := flag.Bool("background", false, "start without a window, in the notification area (as at sign-in)")
	threads := flag.Int("threads", prefs.DefaultThreads(), "default thread count for new jobs")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	// before setupLog, which would empty the running app's log
	if url := runningInstance(*data); url != "" {
		fmt.Println("Transcribe läuft schon:", url)
		if !*noBrowser && !*background {
			openAppWindow(url, *data)
		}
		return
	}
	setupLog(*data)
	log.Println("Transcribe", version)

	// an update puts the new EXE in place and ends the app; the new one starts once this one is done
	exe, _ := os.Executable()
	var restartArgs []string
	defer func() {
		if restartArgs != nil {
			log.Println("starting the new version")
			relaunch(exe, restartArgs)
		}
	}()
	go update.Cleanup(exe) // the EXE the last update replaced

	for _, d := range []string{"audio", "results", "models"} {
		if err := os.MkdirAll(filepath.Join(*data, d), 0o755); err != nil {
			fail(err)
		}
	}
	media.SetDataDir(*data)
	pl := runner.New(*data)
	upd := newUpdater(exe)
	srv := server.New(server.Config{DataDir: *data, Runner: pl.Run, Retranscriber: pl, Threads: *threads, Live: pl.OpenLive, Warmer: pl,
		OCR: ocrEngines(*data, pl), Updater: upd})
	srv.Live.Warm() // while the live mode is on, recognition is loaded once it can be
	go func() {
		if err := setup.EnsureAll(context.Background(), *data, prefs.Load(*data).UsesModels()); err != nil {
			log.Println("setup:", err)
		}
		srv.Live.Warm() // on the first start, the models it needs have only just come
	}()
	dict := &dictation{hub: srv.Live, data: *data}
	if runtime.GOOS == "windows" {
		srv.Dictation = dict
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	srv.Start(ctx)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Printf("port %d is busy, picking a free one", *port)
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		fail(err)
	}
	url := fmt.Sprintf("http://localhost:%d", ln.Addr().(*net.TCPAddr).Port)
	fmt.Println(url, "(data:", *data+")")
	hs := &http.Server{Handler: srv.Handler()}
	go func() {
		if err := hs.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			fail(err)
		}
	}()
	writeInstance(*data, url)
	defer removeInstance(*data)
	var tr *tray
	if upd != nil {
		upd.Idle = func() bool { return !srv.Queue.Busy() && !srv.Live.Busy() }
		upd.Restart = func() {
			restartArgs = relaunchArgs(os.Args[1:], ln.Addr().(*net.TCPAddr).Port, *noBrowser && !*background)
			stop()
		}
		upd.Found = func(v string) {
			tr.notify("Transcribe "+strings.TrimPrefix(v, "v")+" ist da",
				"Ein Klick auf „Aktualisieren“ im Menü des Symbols oder in den Einstellungen installiert die neue Version.")
		}
	}
	if *noBrowser && !*background {
		dict.attach(nil) // no tray icon, so no shortcut; the settings say why
	}

	if !*noBrowser || *background {
		tr = startTray(trayActions{
			open:     func() { openAppWindow(url, *data) },
			settings: func() { openAppWindow(url+"/settings", *data) },
			quit:     stop,

			dictate:       dict.toggle,
			cancelDictate: dict.cancel,
			openNote:      func(id string) { openAppWindow(url+"/live?aufnahme="+neturl.QueryEscape(id), *data) },

			updateItem: func() (string, bool) { return updateMenuItem(upd) },
			update: func() {
				if upd.Install() == nil {
					openAppWindow(url+"/settings#programm", *data)
				}
			},
		})
		defer tr.close()
		dict.attach(tr)
		firstWait := 90 * time.Second
		if *background {
			firstWait = math.MaxInt64 // started without a window: wait until one was opened and closed again
		} else {
			openAppWindow(url, *data)
		}
		go func() {
			told := false
			for range time.Tick(3 * time.Second) {
				switch {
				case !srv.WindowGone(150*time.Second, firstWait):
				case prefs.Load(*data).Background || autostart.Enabled():
					if !told && srv.WindowSeen() {
						log.Println("window closed, running on in the background")
						tr.notify("Transcribe läuft im Hintergrund weiter",
							"Aufträge und Live-Anfragen laufen weiter. Ein Klick auf das Symbol öffnet das Fenster wieder.")
						told = true
					}
				case !srv.Queue.Busy() && !srv.Live.Busy():
					// window closed, no job or live transcription active
					log.Println("window closed, shutting down")
					stop()
					return
				}
			}
		}()
	}

	if upd != nil {
		go upd.Run(ctx)
	}

	<-ctx.Done()
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = hs.Shutdown(sctx)
	srv.Queue.Wait()
}

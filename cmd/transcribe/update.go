// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/moliri-app/transcribe/internal/update"
)

// version is the release's tag (v1.2.3), set when building:
// go build -ldflags "-X main.version=v1.2.3" (.github/workflows/release.yml). build.ps1 sets
// dev-<commit>; a build without a release version never looks for updates.
var version string

// newUpdater looks for newer releases of the Windows EXE. Elsewhere it only names the version: on
// Linux and macOS the user fetches a new release's archive. TRANSCRIBE_UPDATE_URL points it at
// another server than GitHub (e2e tests).
func newUpdater(exe string) *update.Updater {
	if runtime.GOOS != "windows" {
		return &update.Updater{Current: version, Manual: true}
	}
	api := update.LatestURL
	if u := os.Getenv("TRANSCRIBE_UPDATE_URL"); u != "" {
		api = u
	}
	return &update.Updater{Current: version, API: api, Exe: exe, Verify: verifyExe}
}

// verifyExe runs the downloaded EXE with -version: it must start and be the version expected.
func verifyExe(path, want string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "-version").Output()
	if err != nil {
		return fmt.Errorf("the new version does not start: %w", err)
	}
	if got := strings.TrimSpace(string(out)); got != want {
		return fmt.Errorf("the download is version %q, not %q", got, want)
	}
	return nil
}

// relaunchArgs are the arguments for the new version: those of this start, on the port this app
// got (the open window reconnects there). With a tray icon it starts as at sign-in, without
// opening a second window: the open one reloads.
func relaunchArgs(args []string, port int, noBrowser bool) []string {
	var out []string
	background := false
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(strings.TrimLeft(args[i], "-"), "=")
		switch {
		case !strings.HasPrefix(args[i], "-"):
		case name == "port":
			if !hasValue {
				i++ // its value follows
			}
			continue
		case name == "background":
			background = !hasValue || value == "true"
		}
		out = append(out, args[i])
	}
	out = append(out, "-port", strconv.Itoa(port))
	if !noBrowser && !background {
		out = append(out, "-background")
	}
	return out
}

// relaunch starts exe, the new version, once this app has ended.
func relaunch(exe string, args []string) {
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		fail(fmt.Errorf("die neue Version ließ sich nicht starten (%s): %w", exe, err))
	}
	_ = cmd.Process.Release()
}

// updateMenuItem is the tray menu's entry for an update: "" while there is none.
func updateMenuItem(u *update.Updater) (string, bool) {
	if u == nil {
		return "", false
	}
	st := u.Status()
	v := strings.TrimPrefix(st.Latest, "v")
	switch st.State {
	case update.Available, update.Failed:
		return "Auf Version " + v + " aktualisieren", true
	case update.Downloading:
		return fmt.Sprintf("Version %s wird geladen … %d %%", v, int(st.Progress*100)), false
	case update.Waiting:
		return "Update startet, sobald Aufträge und Aufnahmen fertig sind", false
	}
	return "", false
}

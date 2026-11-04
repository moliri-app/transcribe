// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package update looks for a newer release of the app on GitHub and, when the user asks for it,
// downloads it, puts it in place of the running EXE and has the app start it.
package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// LatestURL is the GitHub API's latest release of the app: neither drafts nor pre-releases.
const LatestURL = "https://api.github.com/repos/moliri/transcribe/releases/latest"

// Asset is the name of the EXE attached to every release (.github/workflows/release.yml).
const Asset = "transcribe.exe"

// Every is how often a running app looks again.
const Every = 24 * time.Hour

// The states of Status.State.
const (
	Unknown     = ""            // not looked yet, or looking failed (offline, rate limit)
	Current     = "current"     // this is the newest release
	Available   = "available"   // Status.Latest is newer and can be installed
	Downloading = "downloading" // the new EXE is on its way (Status.Progress)
	Waiting     = "waiting"     // in place; the app restarts once no job or recording runs
	Restarting  = "restarting"  // the app ends and starts the new version
	Failed      = "failed"      // installing failed (Status.Error); Available again on the next try
)

// Status is what the settings page and the tray show.
type Status struct {
	Version  string  `json:"version"`          // of this app; "" or dev-… for a development build
	Checks   bool    `json:"checks"`           // false for a development build: it never looks
	State    string  `json:"state"`            // one of the states above
	Latest   string  `json:"latest,omitempty"` // the newest release, once known
	Page     string  `json:"page,omitempty"`   // its page on GitHub, with what is new
	Progress float64 `json:"progress,omitempty"`
	Error    string  `json:"error,omitempty"`
	Checked  int64   `json:"checked,omitempty"` // unix seconds of the last successful look
}

type release struct {
	version, page, url string
	size               int64
}

// Updater keeps the update state of the running app.
type Updater struct {
	Current string // the app's own version (main.version)
	API     string // where to ask for the latest release; LatestURL unless a test points elsewhere
	Exe     string // the running EXE
	Client  *http.Client
	// Idle reports whether no job or recording runs, so the app may restart.
	Idle func() bool
	// Restart ends the app so that it starts again from Exe (cmd/transcribe).
	Restart func()
	// Verify checks the downloaded EXE before it takes the old one's place; may be nil.
	Verify func(path, version string) error
	// Found is called once for every newer release found; may be nil.
	Found func(version string)

	mu      sync.Mutex
	st      Status
	rel     *release
	told    string // the version Found was called for
	running bool   // Install is under way
}

func (u *Updater) client() *http.Client {
	if u.Client != nil {
		return u.Client
	}
	return http.DefaultClient
}

// Status returns the current state.
func (u *Updater) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	st := u.st
	st.Version, st.Checks = u.Current, IsRelease(u.Current)
	return st
}

// Run looks at once and then every Every until ctx ends. A development build never looks.
func (u *Updater) Run(ctx context.Context) {
	if !IsRelease(u.Current) {
		return
	}
	t := time.NewTicker(Every)
	defer t.Stop()
	for {
		if err := u.Check(ctx); err != nil && ctx.Err() == nil {
			log.Println("update:", err) // offline or rate-limited: the UI stays quiet
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Check asks for the latest release. A failure leaves the state as it was.
func (u *Updater) Check(ctx context.Context) error {
	if !IsRelease(u.Current) {
		return errors.New("development build")
	}
	rel, err := u.latest(ctx)
	if err != nil {
		return err
	}
	u.mu.Lock()
	u.st.Checked = time.Now().Unix()
	tell := ""
	if !u.running && u.st.State != Waiting && u.st.State != Restarting {
		u.st.Error, u.st.Progress = "", 0
		if rel != nil && Compare(rel.version, u.Current) > 0 {
			u.rel = rel
			u.st.State, u.st.Latest, u.st.Page = Available, rel.version, rel.page
			if u.told != rel.version {
				u.told, tell = rel.version, rel.version
			}
		} else {
			u.rel = nil
			u.st.State, u.st.Latest, u.st.Page = Current, "", ""
			if rel != nil {
				u.st.Latest, u.st.Page = rel.version, rel.page
			}
		}
	}
	u.mu.Unlock()
	if tell != "" && u.Found != nil {
		u.Found(tell)
	}
	return nil
}

// latest returns the latest release with the EXE attached, nil if there is none.
func (u *Updater) latest(ctx context.Context) (*release, error) {
	api := u.API
	if api == "" {
		api = LatestURL
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "transcribe/"+u.Current)
	r, err := u.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode == http.StatusNotFound {
		return nil, nil // no release yet
	}
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", api, r.Status)
	}
	var got struct {
		Tag        string `json:"tag_name"`
		Page       string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&got); err != nil {
		return nil, err
	}
	if got.Draft || got.Prerelease || !IsRelease(got.Tag) {
		return nil, nil
	}
	for _, a := range got.Assets {
		if strings.EqualFold(a.Name, Asset) && a.URL != "" {
			return &release{version: got.Tag, page: got.Page, url: a.URL, size: a.Size}, nil
		}
	}
	return nil, nil // the release has no EXE (yet): the workflow attaches it after creating it
}

// Install downloads the newer release in the background, puts it in place and restarts the app
// once no job or recording runs. It returns at once; Status follows the progress.
func (u *Updater) Install() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.running || u.st.State == Waiting || u.st.State == Restarting {
		return nil // already under way
	}
	if u.rel == nil {
		return errors.New("Es gibt keine neuere Version.")
	}
	u.running = true
	u.st.State, u.st.Progress, u.st.Error = Downloading, 0, ""
	go u.install(*u.rel)
	return nil
}

func (u *Updater) install(rel release) {
	err := u.fetch(rel)
	if err == nil && u.Verify != nil {
		err = u.Verify(newPath(u.Exe), rel.version)
	}
	if err == nil {
		err = Swap(u.Exe, newPath(u.Exe))
	}
	u.mu.Lock()
	u.running = false
	if err != nil {
		_ = os.Remove(newPath(u.Exe))
		log.Println("update:", err)
		u.st.State, u.st.Progress, u.st.Error = Failed, 0, explain(err)
		u.mu.Unlock()
		return
	}
	log.Printf("update: %s is in place of %s, restarting once idle", rel.version, u.Current)
	u.st.State, u.st.Progress = Waiting, 1
	u.mu.Unlock()

	for u.Idle != nil && !u.Idle() {
		time.Sleep(2 * time.Second)
	}
	u.mu.Lock()
	u.st.State = Restarting
	u.mu.Unlock()
	if u.Restart != nil {
		u.Restart()
	}
}

// fetch downloads the release's EXE to newPath(u.Exe).
func (u *Updater) fetch(rel release) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rel.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "transcribe/"+u.Current)
	r, err := u.client().Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", r.Status)
	}
	size := rel.size
	if size <= 0 {
		size = r.ContentLength
	}
	f, err := os.Create(newPath(u.Exe))
	if err != nil {
		return err
	}
	head := make([]byte, 2)
	n, err := io.Copy(f, io.TeeReader(r.Body, &progress{u: u, size: size, head: head}))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	switch {
	case err != nil:
		return err
	case rel.size > 0 && n != rel.size:
		return fmt.Errorf("download: %d of %d bytes", n, rel.size)
	case !bytes.Equal(head, []byte("MZ")):
		return errors.New("download: not a Windows program")
	}
	return nil
}

// progress counts the downloaded bytes into the status and keeps the first two (the EXE's "MZ").
type progress struct {
	u    *Updater
	size int64
	n    int64
	head []byte
}

func (p *progress) Write(b []byte) (int, error) {
	for i := p.n; i < int64(len(p.head)) && i-p.n < int64(len(b)); i++ {
		p.head[i] = b[i-p.n]
	}
	p.n += int64(len(b))
	if p.size > 0 {
		p.u.mu.Lock()
		p.u.st.Progress = min(float64(p.n)/float64(p.size), 1)
		p.u.mu.Unlock()
	}
	return len(b), nil
}

// explain says in the UI's words why installing failed.
func explain(err error) string {
	switch {
	case errors.Is(err, os.ErrPermission):
		return "Transcribe darf seinen Programmordner nicht ändern. Lade die neue Version von der Releases-Seite und ersetze die EXE von Hand."
	case errors.Is(err, context.DeadlineExceeded):
		return "Der Download hat zu lange gedauert. Versuch es später noch einmal."
	}
	return "Das Update ließ sich nicht installieren: " + err.Error()
}

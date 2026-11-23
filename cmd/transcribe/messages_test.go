// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	appmsg "github.com/moliri-app/transcribe/internal/msg"
	"github.com/moliri-app/transcribe/internal/prefs"
)


var poIndex = regexp.MustCompile(`\{(\d+)\}`)

// readPO reads frontend/src/locales/<locale>.po: the translations by English text, with the
// placeholders wuchale numbers ({0}, named in a comment "#. 0: a.service") named as in Go.
func readPO(t *testing.T, locale string) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "frontend", "src", "locales", locale+".po"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string]string{}
	names := map[string]string{}
	var id string
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "#. "):
			for _, part := range strings.Split(strings.TrimPrefix(line, "#. "), "; ") {
				if i, name, ok := strings.Cut(part, ": "); ok {
					names[i] = strings.TrimPrefix(name, "a.")
				}
			}
		case strings.HasPrefix(line, "msgid "):
			id, _ = strconv.Unquote(strings.TrimPrefix(line, "msgid "))
		case strings.HasPrefix(line, "msgstr "):
			s, _ := strconv.Unquote(strings.TrimPrefix(line, "msgstr "))
			named := func(s string) string {
				return poIndex.ReplaceAllStringFunc(s, func(p string) string {
					if n, ok := names[p[1:len(p)-1]]; ok {
						return "{" + n + "}"
					}
					return p
				})
			}
			if id != "" {
				out[named(id)] = named(s)
			}
			id, names = "", map[string]string{}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// The messages the dictation window translates say what the pages say for them
// (frontend/src/lib/meldungen.js), in every language as the frontend's catalogues have it.
func TestMessagesAgreeWithTheFrontend(t *testing.T) {
	js, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "lib", "meldungen.js"))
	if err != nil {
		t.Fatal(err)
	}
	for key, m := range messages {
		// the frontend writes ${a.service} where Go writes {service}
		frontend := regexp.MustCompile(`\{([a-z]+)\}`).ReplaceAllString(m.en, "$${a.$1}")
		if !strings.Contains(string(js), "case '"+key+"':") || !strings.Contains(string(js), frontend) {
			t.Errorf("%s: meldungen.js does not say %q", key, frontend)
		}
	}
	for _, l := range prefs.UILocales[1:] {
		po := readPO(t, l)
		for key, m := range messages {
			want := po[m.en]
			if want == "" {
				t.Errorf("%s: %s.po has no translation of %q", key, l, m.en)
				continue
			}
			if got := m.tr[l]; got != want {
				t.Errorf("%s in %s is %q; the frontend says %q", key, l, got, want)
			}
		}
	}
}

// keyMade finds the messages a Go file makes.
var keyMade = regexp.MustCompile(`\bmsg\.New\("([A-Za-z]+\.[A-Za-z.]+)"`)

// Every message that can end a dictation has its texts: those of the recording (internal/live) and
// of the speech recognition services (internal/sttapi). The rest of internal/live is about the
// notes on the Live page and the background mode, which a dictation does not touch.
func TestDictationErrorsTranslated(t *testing.T) {
	notDictation := map[string]string{
		"live.noSuchNote":       "deleting or opening a note on the Live page",
		"live.noteRunning":      "deleting a note on the Live page",
		"live.alreadyRecording": "the recording started on the Live page",
		"live.noOrigin":         "a page asking for the app",
		"live.off":              "programs listening while the background mode is off; dictating records either way",
	}
	found := map[string]bool{}
	for _, dir := range []string{"live", "sttapi"} {
		files, _ := filepath.Glob(filepath.Join("..", "..", "internal", dir, "*.go"))
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range keyMade.FindAllStringSubmatch(string(b), -1) {
				found[m[1]] = true
				if _, ok := messages[m[1]]; !ok && notDictation[m[1]] == "" {
					t.Errorf("%s: message %q can end a dictation but has no texts in messages.go", f, m[1])
				}
			}
		}
	}
	if len(found) < 20 {
		t.Fatalf("found only %d messages; does the search still match how they are made?", len(found))
	}
	for key := range notDictation {
		if !found[key] {
			t.Errorf("%q is no longer made; drop it from notDictation", key)
		}
	}
	for key, m := range messages {
		want := placeholders(m.en)
		for _, l := range prefs.UILocales[1:] {
			if got := placeholders(m.tr[l]); !slices.Equal(got, want) {
				t.Errorf("%s in %s has placeholders %v, not %v", key, l, got, want)
			}
		}
	}
}

func placeholders(s string) []string {
	p := regexp.MustCompile(`\{[a-z]+\}`).FindAllString(s, -1)
	slices.Sort(p)
	return p
}

func TestErrorTextInInterfaceLanguage(t *testing.T) {
	noMic := appmsg.New("capture.noDevice", "Aufnahme fehlgeschlagen: kein Mikrofon gefunden")
	for _, c := range []struct {
		locale string
		err    error
		want   string
	}{
		{"en", noMic, "Recording failed: no microphone found"},
		{"de", noMic, "Aufnahme fehlgeschlagen: kein Mikrofon gefunden"},
		{"fr", fmt.Errorf("%w", noMic), "L’enregistrement a échoué : aucun microphone trouvé"},
		{"en", appmsg.New("capture.failed", "Aufnahme fehlgeschlagen: {reason}", "reason", "device busy"), "Recording failed: device busy"},
		{"en", appmsg.New("capture.format", "Audio: unbekanntes Format ({bits} Bit)", "bits", 12), "Audio: unknown format (12 bit)"},
		// the service's own words come after the text, if it said any
		{"en", appmsg.New("api.keyRejected", "{service} hat den API-Schlüssel abgelehnt. {detail}", "service", "ElevenLabs", "detail", ""), "ElevenLabs rejected the API key."},
		{"en", appmsg.New("api.error", "{service}: Fehler {status}: {detail}", "service", "OpenAI", "status", 500, "detail", "overloaded"), "OpenAI: error 500: overloaded"},
		// a message within a message is translated too
		{"en", appmsg.New("api.refused", "{service} lehnt ab: {reason}", "service", "ElevenLabs", "reason", appmsg.New("api.quota", "Guthaben oder Anfragen sind aufgebraucht.")), "ElevenLabs refuses: Credit or requests are used up."},
		// what the catalogue does not know keeps its German text
		{"en", appmsg.New("live.noSuchNote", "Diese Notiz gibt es nicht."), "Diese Notiz gibt es nicht."},
		{"en", errors.New("model german is not installed"), "model german is not installed"},
		{"en", fmt.Errorf("Mikrofon: %w", noMic), "Mikrofon: Aufnahme fehlgeschlagen: kein Mikrofon gefunden"},
	} {
		if got := errorTextIn(c.locale, c.err); got != c.want {
			t.Errorf("%s %v: %q, want %q", c.locale, c.err, got, c.want)
		}
	}
}

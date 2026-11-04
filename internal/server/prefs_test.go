// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/internal/queue"
)

// Where speech is recognised: the key is kept but never shown, and a job takes the default or
// the engine asked for, as far as the settings allow.
func TestRecognitionPrefs(t *testing.T) {
	s, d := newServer(t)
	add := func(body string) *httptest.ResponseRecorder { return do(s, "POST", "/api/queue/add", body, nil) }
	if w := do(s, "POST", "/api/prefs", `{"recognition":"both","fileEngine":"api"}`, nil); w.Code != 200 {
		t.Fatalf("prefs: %d %s", w.Code, w.Body)
	}
	if w := add(`{"audio":["Über uns ä.mp3"]}`); w.Code != 400 || !strings.Contains(w.Body.String(), "kein Schlüssel") {
		t.Errorf("without a key: %d %s", w.Code, w.Body)
	}
	w := do(s, "POST", "/api/prefs", `{"api":{"key":" sk_0123456789abcd "}}`, nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "sk_") || !strings.Contains(w.Body.String(), `"keyHint":"abcd"`) ||
		!strings.Contains(w.Body.String(), `"provider":"openai","url":"https://api.openai.com/v1","model":"whisper-1"`) {
		t.Fatalf("key: %d %s", w.Code, w.Body)
	}
	if b := do(s, "GET", "/api/prefs", "", nil).Body.String(); strings.Contains(b, "sk_") || !strings.Contains(b, `"keySet":true`) ||
		!strings.Contains(b, `"apiProviders":[{"id":"openai"`) {
		t.Errorf("shown: %s", b)
	}
	// another service: its own address and model, and the key of the one before is gone
	b := do(s, "POST", "/api/prefs", `{"api":{"provider":"elevenlabs"}}`, nil).Body.String()
	if !strings.Contains(b, `"provider":"elevenlabs","url":"https://api.elevenlabs.io","model":"scribe_v2","keySet":false`) {
		t.Errorf("other service: %s", b)
	}
	if w := do(s, "POST", "/api/prefs", `{"api":{"provider":"nope"}}`, nil); w.Code != 400 {
		t.Errorf("unknown service: %d", w.Code)
	}
	do(s, "POST", "/api/prefs", `{"api":{"key":"sk_0123456789abcd"}}`, nil)
	if w := add(`{"audio":["Über uns ä.mp3"]}`); w.Code != 200 {
		t.Fatalf("add: %d %s", w.Code, w.Body)
	}
	var q struct{ Jobs []queue.Job }
	_ = json.Unmarshal(do(s, "GET", "/api/queue", "", nil).Body.Bytes(), &q)
	if len(q.Jobs) != 1 || q.Jobs[0].Options.Engine != "api" {
		t.Fatalf("default engine: %+v", q.Jobs)
	}
	_ = os.WriteFile(filepath.Join(d, "audio", "b.mp3"), []byte("0123456789"), 0o644)
	if w := add(`{"audio":["b.mp3"],"engine":"local"}`); w.Code != 200 {
		t.Fatalf("add local: %d %s", w.Code, w.Body)
	}
	_ = json.Unmarshal(do(s, "GET", "/api/queue", "", nil).Body.Bytes(), &q)
	if len(q.Jobs) != 2 || q.Jobs[1].Options.Engine != "local" {
		t.Errorf("chosen engine: %+v", q.Jobs)
	}
}

// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A recording gets a name of its own, tags and can be hidden; the library lists all of it, the
// export names its file after it, and deleting the recording forgets it.
func TestShelveRecording(t *testing.T) {
	s, d := newServer(t)
	_ = os.WriteFile(filepath.Join(d, "audio", "talk.wav"), []byte("RIFF"), 0o644) // results/talk/_final.json exists
	post := func(stem, body string, hdr map[string]string) int {
		return do(s, "POST", "/api/recording?stem="+stem, body, hdr).Code
	}
	entry := func() map[string]any {
		var items []map[string]any
		if err := json.Unmarshal(do(s, "GET", "/api/index", "", nil).Body.Bytes(), &items); err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			if it["stem"] == "talk" {
				return it
			}
		}
		t.Fatal("talk not in the index")
		return nil
	}

	if c := post("talk", `{"title":"x"}`, map[string]string{"Origin": "https://evil.example"}); c != 403 {
		t.Errorf("foreign page: %d", c)
	}
	for _, stem := range []string{"", "..", "missing", "a%2Fb"} {
		if c := post(stem, `{}`, nil); c != 404 {
			t.Errorf("%q: %d", stem, c)
		}
	}
	if c := post("talk", `{"title":"  Gespräch   mit Frau Berger ","tags":["Interview","interview"," Papier "]}`, nil); c != 200 {
		t.Fatalf("shelve: %d", c)
	}
	it := entry()
	if it["title"] != "Gespräch mit Frau Berger" || it["file"] != "talk" || it["hidden"] != false {
		t.Errorf("index: %v %v %v", it["title"], it["file"], it["hidden"])
	}
	if tags, _ := json.Marshal(it["tags"]); string(tags) != `["Interview","Papier"]` {
		t.Errorf("tags: %s", tags)
	}
	w := do(s, "GET", "/api/export?stem=talk&format=txt", "", nil)
	if cd := w.Header().Get("Content-Disposition"); cd != "attachment; filename*=UTF-8''Gespr%C3%A4ch%20mit%20Frau%20Berger.txt" {
		t.Errorf("export name: %s", cd)
	}

	// only what is sent changes; an empty title gives the file's name back
	if c := post("talk", `{"hidden":true}`, nil); c != 200 {
		t.Fatal(c)
	}
	if it := entry(); it["hidden"] != true || it["title"] != "Gespräch mit Frau Berger" {
		t.Errorf("hidden: %v", it)
	}
	if c := post("talk", `{"title":"","hidden":false,"tags":[]}`, nil); c != 200 {
		t.Fatal(c)
	}
	if it := entry(); it["title"] != "talk" || it["hidden"] != false {
		t.Errorf("reset: %v", it)
	}

	_ = post("talk", `{"title":"Weg"}`, nil)
	if c := do(s, "POST", "/api/delete?name=talk.wav", "", nil).Code; c != 200 {
		t.Fatal(c)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "library.json")); string(b) != "{\n  \"recordings\": {}\n}" {
		t.Errorf("library.json after delete: %s", b)
	}
}

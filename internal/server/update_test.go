// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"encoding/json"
	"testing"
)

// Without an updater the app is a development build: it shows no version and never looks.
func TestUpdateWithoutUpdater(t *testing.T) {
	s, _ := newServer(t)
	var st struct {
		Version string
		Checks  bool
		State   string
	}
	w := do(s, "GET", "/api/update", "", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil || w.Code != 200 {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
	if st.Checks || st.Version != "" || st.State != "" {
		t.Errorf("got %+v", st)
	}
	if w := do(s, "POST", "/api/update", `{"action":"check"}`, nil); w.Code != 409 {
		t.Errorf("check: got %d, want 409", w.Code)
	}
	if w := do(s, "POST", "/api/update", `{"action":"install"}`, nil); w.Code != 409 {
		t.Errorf("install: got %d, want 409", w.Code)
	}
	if w := do(s, "POST", "/api/update", `{"action":"install"}`, map[string]string{"Origin": "https://evil.example"}); w.Code != 403 {
		t.Errorf("foreign page: got %d, want 403", w.Code)
	}
}

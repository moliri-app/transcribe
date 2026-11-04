// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package models

import (
	"bytes"
	"testing"
)

func TestRestrictLanguages(t *testing.T) {
	graph := appendBytes(nil, 7, bytes.Repeat([]byte{1}, 300))
	model := append([]byte{8, 7}, graph...) // ir_version 7, then the graph
	model = append(model, metaField("all_language_tokens", "50259,50260,50261,50262")...)
	model = append(model, metaField("n_mels", "80")...)
	model = append(model, metaField("all_language_codes", "en,zh,de,es")...)
	out, err := restrictLanguages(model, []string{"de", "en", "nds"})
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{8, 7}, graph...)
	want = append(want, metaField("all_language_tokens", "50259,50261")...)
	want = append(want, metaField("n_mels", "80")...)
	want = append(want, metaField("all_language_codes", "en,de")...)
	if !bytes.Equal(out, want) {
		t.Errorf("rewritten model differs")
	}
	if _, err := restrictLanguages(model, []string{"de", "nds"}); err == nil {
		t.Error("one known language accepted")
	}
}

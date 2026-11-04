// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package fileutil has the small file helpers every part of the app shares.
package fileutil

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

// WriteAtomic writes data next to path and renames it into place, so readers never see a half-written file.
func WriteAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// JSON encodes like Python's json.dumps(ensure_ascii=False): no HTML escaping. indent "" = compact.
func JSON(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// WriteJSON writes v atomically, indented by one space like the Python version.
func WriteJSON(path string, v any) error {
	b, err := JSON(v, " ")
	if err != nil {
		return err
	}
	return WriteAtomic(path, b)
}

// ReadJSON decodes path into v; a missing file is reported as os.ErrNotExist.
func ReadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

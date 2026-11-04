// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package osocr

// New gives nil outside Windows: the engine "os" is listed only there (macOS Vision may follow).
func New() *Engine { return nil }

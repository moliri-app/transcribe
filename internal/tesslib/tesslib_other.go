// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package tesslib

import (
	"context"
	"errors"
)

// Outside Windows the app uses the tesseract program of the system (ocr.Tesseract without Lib).

var errNoLib = errors.New("the Tesseract library is only downloaded on Windows")

func load(string) (string, error) { return "", errNoLib }

func (l *Lib) TSV(context.Context, []byte, string, string, int) ([]byte, error) {
	return nil, errNoLib
}

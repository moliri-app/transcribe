// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package pick shows the system's dialog to choose a folder. The pages run in a browser, which
// never tells where a folder is; the app asks the system instead.
package pick

import "errors"

// ErrUnsupported is returned where the app has no dialog to show (everywhere but Windows).
var ErrUnsupported = errors.New("no folder dialog on this system")

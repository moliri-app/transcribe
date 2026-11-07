// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package llm

import "os"

// bindToApp does nothing here: a server whose app crashed runs on until it is ended by hand.
func bindToApp(*os.Process) {}

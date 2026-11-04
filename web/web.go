// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package web embeds the browser UI that frontend/ builds into dist/ (pnpm build). Without a
// frontend build, dist holds only a placeholder file, so go build and go test work on a plain
// checkout.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

// Dist is the build of the UI (frontend/).
var Dist, _ = fs.Sub(files, "dist")

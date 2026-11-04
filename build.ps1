# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Builds the portable app: first the new UI (frontend/ -> web/dist), then the EXE, which embeds it.
# The Windows EXE is a GUI app (no console window).
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    pnpm --dir frontend install --frozen-lockfile
    if ($LASTEXITCODE -ne 0) { throw 'pnpm install ist fehlgeschlagen' }
    pnpm --dir frontend build
    if ($LASTEXITCODE -ne 0) { throw 'pnpm build ist fehlgeschlagen' }

    # a development build: its version is not a release's, so it never looks for updates
    $version = try { git describe --tags --always --dirty 2>$null } catch { $null }
    if (-not $version) { $version = 'unknown' }
    $env:CGO_ENABLED = '0'
    go build -ldflags "-s -w -H=windowsgui -X main.version=dev-$version" -o transcribe.exe ./cmd/transcribe
    if ($LASTEXITCODE -ne 0) { throw 'go build ist fehlgeschlagen' }
} finally {
    Pop-Location
}

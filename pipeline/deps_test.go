// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// enginePackages are the packages other programs import (Moliri's server/transcribe among them).
var enginePackages = []string{"annotate", "audio", "export", "fileutil", "jobs", "model", "models",
	"ocr", "ocr/ocrtest", "ort", "pipeline", "proc", "sherpa", "stream"}

// The engine depends on nothing of the desktop app (internal/) and builds for Linux without cgo,
// as the server builds it.
func TestEngineStandsOnItsOwn(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the engine twice")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on the PATH")
	}
	args := []string{"list", "-deps", "-test"}
	for _, p := range enginePackages {
		args = append(args, "github.com/moliri-app/transcribe/"+p)
	}
	for _, goos := range []string{"linux", "windows"} {
		cmd := exec.Command("go", args...)
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH=amd64", "CGO_ENABLED=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go list for %s: %v\n%s", goos, err, out)
		}
		for _, dep := range strings.Fields(string(out)) {
			if strings.HasPrefix(dep, "github.com/moliri-app/transcribe/internal/") || strings.HasPrefix(dep, "github.com/moliri-app/transcribe/cmd/") {
				t.Errorf("%s: the engine imports %s", goos, dep)
			}
		}
	}
	build := exec.Command("go", append([]string{"build"}, args[3:]...)...)
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Errorf("CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build: %v\n%s", err, out)
	}
}

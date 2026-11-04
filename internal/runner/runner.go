// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package runner is the recognition of the app: the engine (package pipeline) bound to the data
// directory and the settings. It chooses what recognises (the models on this computer or a
// service, internal/sttapi), loads the runtime (the GPU build when the user chose it, a download
// when it is missing) and keeps the results in <data>/results (internal/results).
package runner

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/internal/results"
	"github.com/moliri-app/transcribe/internal/setup"
	"github.com/moliri-app/transcribe/internal/sttapi"
	"github.com/moliri-app/transcribe/jobs"
	"github.com/moliri-app/transcribe/model"
	"github.com/moliri-app/transcribe/models"
	"github.com/moliri-app/transcribe/pipeline"
	"github.com/moliri-app/transcribe/sherpa"
	"github.com/moliri-app/transcribe/stream"
)

// quality is what recognition on this computer always does: "best" fills the gaps the main
// model leaves (singing, other languages) with Whisper turbo. The quality a job asks for is no
// longer heeded.
const quality = "best"

// Runner is the recognition bound to a data directory; it implements queue.Runner,
// annotate.Retranscriber, live.Opener (OpenLive) and live.Warmer.
type Runner struct {
	DataDir string
	LibDir  string // the runtime's folder once loaded; set beforehand, it is used as it is
	engine  *pipeline.Runner
}

// New returns the recognition of the app with its data in dataDir.
func New(dataDir string) *Runner {
	r := &Runner{DataDir: dataDir}
	r.engine = &pipeline.Runner{
		Models:  models.Set{Dir: filepath.Join(dataDir, "models")},
		Runtime: runtime{r},
		Voice: func() pipeline.Voice {
			p := prefs.Load(dataDir)
			return pipeline.Voice{Words: p.LiveWords, Commands: !p.LiveNoCommands}
		},
	}
	return r
}

// Engine is the pipeline the runner drives.
func (r *Runner) Engine() *pipeline.Runner { return r.engine }

// runtime loads the speech recognition runtime for the engine (pipeline.Runtime).
type runtime struct{ r *Runner }

func (rt runtime) Load(ctx context.Context) (string, error) {
	r := rt.r
	if err := r.init(ctx); err != nil {
		return "", err
	}
	return r.LibDir, nil
}

func (rt runtime) Present() bool { return rt.r.libPresent() }

func (r *Runner) init(ctx context.Context) error {
	if r.LibDir == "" {
		if dir := r.cudaLib(); dir != "" {
			sherpa.SetLibDir(dir)
			sherpa.SetProvider("cuda")
			if err := sherpa.Load(dir); err == nil {
				r.LibDir = dir
				log.Println("GPU: CUDA runtime loaded from", dir)
				return nil
			} else {
				log.Println("GPU: CUDA runtime failed, using CPU:", err)
				sherpa.SetProvider("cpu")
			}
		}
		r.LibDir = r.findLib()
		if r.LibDir == "" {
			r.LibDir = filepath.Join(r.DataDir, "lib")
			if err := models.EnsureLib(ctx, r.LibDir, nil); err != nil {
				r.LibDir = ""
				return err
			}
		}
	}
	sherpa.SetLibDir(r.LibDir)
	return sherpa.Load(r.LibDir)
}

// Runtime loads the speech recognition runtime, downloading it when it is missing, and returns its
// folder and whether it runs on the graphics card. Text recognition (internal/ort) loads the
// onnxruntime of the same folder: a process holds only one, and speech recognition would else find
// the wrong one loaded.
func (r *Runner) Runtime(ctx context.Context) (string, bool, error) {
	if err := r.init(ctx); err != nil {
		return "", false, err
	}
	return r.LibDir, sherpa.Provider() == "cuda", nil
}

// RuntimePresent reports whether Runtime can load the runtime without downloading it. Unlike
// libPresent it does not ask for the GPU (nvidia-smi takes seconds): it is asked for every list of
// the text recognition engines.
func (r *Runner) RuntimePresent() bool {
	return r.LibDir != "" || r.findLib() != "" || models.FindLib(setup.CudaLibDir(r.DataDir)) != ""
}

// WantsGPU reports whether the runtime loads (or would load) on the graphics card: the user chose
// it and its CUDA build is installed.
func (r *Runner) WantsGPU() bool {
	if sherpa.Loaded() {
		return sherpa.Provider() == "cuda"
	}
	return prefs.Load(r.DataDir).Accel == "cuda" && models.FindLib(setup.CudaLibDir(r.DataDir)) != ""
}

// findLib is the folder of the speech recognition runtime on the CPU; "" if it has not been
// downloaded yet.
func (r *Runner) findLib() string {
	exe, _ := os.Executable()
	return models.FindLib(os.Getenv("SHERPA_LIB"), filepath.Dir(exe), filepath.Join(r.DataDir, "lib"))
}

// libPresent reports whether init can load the runtime without downloading it.
func (r *Runner) libPresent() bool { return r.LibDir != "" || r.cudaLib() != "" || r.findLib() != "" }

// cudaLib returns the CUDA runtime folder if the user chose GPU acceleration and everything it needs is present.
func (r *Runner) cudaLib() string {
	if prefs.Load(r.DataDir).Accel != "cuda" {
		return ""
	}
	dir := models.FindLib(setup.CudaLibDir(r.DataDir))
	if dir == "" {
		return ""
	}
	g := setup.DetectGPU(r.DataDir)
	if !g.Ready {
		log.Println("GPU: CUDA selected but not usable (missing:", g.Missing, "), using CPU")
		return ""
	}
	setup.UseLibDirs(g.Dirs)
	return dir
}

// Run executes events -> speakers -> text -> annotate on <data>/audio/<audioFile> and writes
// results/<stem>/: _machine.json, _final.json and the re-recognition cache.
func (r *Runner) Run(ctx context.Context, dataDir, audioFile string, o jobs.Options, p jobs.Progress) error {
	if o.Engine == prefs.API {
		return r.runAPI(ctx, dataDir, audioFile, o, p)
	}
	if !prefs.Load(r.DataDir).UsesModels() {
		return errors.New("Die Erkennung auf diesem Rechner ist ausgeschaltet (Einstellungen, Erkennung: nur externe API).")
	}
	stem := strings.TrimSuffix(audioFile, filepath.Ext(audioFile))
	ann, cache, err := results.Inputs(dataDir, stem)
	if err != nil {
		return err
	}
	o.Quality = quality
	eng := *r.engine
	eng.Models = models.Set{Dir: filepath.Join(dataDir, "models")}
	res, err := eng.Run(ctx, filepath.Join(dataDir, "audio", audioFile), o, ann, cache, p)
	if err != nil {
		return err
	}
	return results.Save(dataDir, stem, res, cache)
}

// Retranscribe implements annotate.Retranscriber for passages corrected in the editor: with
// whatever recognises whole recordings by default.
func (r *Runner) Retranscribe(ctx context.Context, audioPath string, a, b float64, lang string) ([]model.Word, error) {
	if p := prefs.Load(r.DataDir); p.Engine("", p.FileEngine) == prefs.API {
		return apiRetranscriber{r}.Retranscribe(ctx, audioPath, a, b, lang)
	}
	return r.engine.Retranscribe(ctx, audioPath, a, b, lang)
}

// OpenLive implements live.Opener, with the service (internal/sttapi) if that is what live transcription uses.
func (r *Runner) OpenLive(ctx context.Context, lang string) (stream.Stream, error) {
	p := prefs.Load(r.DataDir)
	if p.Engine("", p.LiveEngine) == prefs.API {
		return sttapi.OpenLive(ctx, apiConfig(p), lang)
	}
	return r.engine.OpenLive(ctx, lang)
}

// KeepWarm implements live.Warmer (pipeline.Runner.KeepWarm).
func (r *Runner) KeepWarm(lang string) bool { return r.engine.KeepWarm(lang) }

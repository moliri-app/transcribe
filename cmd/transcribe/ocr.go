// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"path/filepath"

	"github.com/moliri-app/transcribe/embed"
	"github.com/moliri-app/transcribe/internal/llm"
	"github.com/moliri-app/transcribe/internal/ocrapi"
	"github.com/moliri-app/transcribe/internal/osocr"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/internal/runner"
	"github.com/moliri-app/transcribe/internal/server"
	"github.com/moliri-app/transcribe/internal/setup"
	"github.com/moliri-app/transcribe/internal/tesslib"
	"github.com/moliri-app/transcribe/ocr"
	"github.com/moliri-app/transcribe/ort"
)

// ocrEngines are the text recognition engines of the app (docs/ocr-api.md). Tesseract is not
// shipped: on Windows its library and models are downloaded on first use; elsewhere the tesseract
// program of the system reads (with its own models). PP-OCRv5 (engine "onnx") downloads its models
// on first use and runs on the onnxruntime of speech recognition, on the graphics card when that
// uses it. On Windows the system's own text recognition (os) follows. Last the AI service (api)
// with the user's own key from the settings: it reads only when a request names it or the settings
// make it the default, never as the first usable engine (it costs money, the image leaves the
// computer). The default engine of requests without one is a setting too, read at every request.
func ocrEngines(dataDir string, pl *runner.Runner) *ocr.Registry {
	t := &ocr.Tesseract{Hint: "apt install tesseract-ocr tesseract-ocr-deu, brew install tesseract tesseract-lang"}
	if setup.TesseractLibSupported() {
		t = &ocr.Tesseract{
			TessData:     setup.TesseractDataDir(dataDir),
			Lib:          tesslib.New(dataDir),
			Downloadable: setup.TesseractModels(),
			Fetch: func(ctx context.Context, model string) error {
				return setup.FetchTesseractModel(ctx, dataDir, model)
			},
		}
	}
	p := &ocr.PPOCR{
		Nets:     &ort.Nets{Runtime: pl.Runtime, Present: pl.RuntimePresent, WantsGPU: pl.WantsGPU},
		ModelDir: setup.PPOCRDir(dataDir),
		Fetch: func(ctx context.Context, file string) error {
			return setup.FetchPPOCRModel(ctx, dataDir, file)
		},
	}
	engines := []ocr.Engine{t, p}
	if e := osocr.New(); e != nil {
		engines = append(engines, e)
	}
	engines = append(engines, &ocrapi.Engine{Config: func() ocrapi.Config { return ocrapi.Of(prefs.Load(dataDir).OCRAPI) }})
	r := ocr.NewRegistry("desktop", engines...)
	r.Default = func() string { return prefs.Load(dataDir).OCREngine }
	return r
}

// embedModel is EmbeddingGemma for Moliri's field search by meaning (docs/embed-api.md): its files
// come when Moliri or the settings page asks for them, and it runs on the onnxruntime of speech
// recognition, on the CPU.
func embedModel(dataDir string, pl *runner.Runner, threads int) *embed.Model {
	return &embed.Model{
		Dir:     setup.EmbedDir(dataDir),
		Threads: threads,
		Runtime: func(ctx context.Context) (string, error) {
			dir, _, err := pl.Runtime(ctx)
			return dir, err
		},
	}
}

// filterModel is Qwen 2.5 7B on llama.cpp's server for Moliri's filter in plain words
// (docs/filter-api.md): program and model come when Moliri or the settings page asks for them.
// nil where llama.cpp has no build for this system.
func filterModel(dataDir string) server.Chatter {
	if !setup.LLMSupported() {
		return nil
	}
	return &llm.Server{
		Program:   setup.LlamaServer(dataDir),
		Model:     setup.LLMModel(dataDir),
		CacheDir:  filepath.Join(setup.LLMDir(dataDir), "cache"),
		Installed: func() bool { return setup.LLMInstalled(dataDir) },
	}
}

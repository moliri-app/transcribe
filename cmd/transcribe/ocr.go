// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"context"

	"github.com/moliri-app/transcribe/internal/models"
	"github.com/moliri-app/transcribe/internal/ocr"
	"github.com/moliri-app/transcribe/internal/ocrapi"
	"github.com/moliri-app/transcribe/internal/ort"
	"github.com/moliri-app/transcribe/internal/osocr"
	"github.com/moliri-app/transcribe/internal/pipeline"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/internal/tesslib"
)

// ocrEngines are the text recognition engines of the app (docs/ocr-api.md). Tesseract is not
// shipped: on Windows its library and models are downloaded on first use; elsewhere the tesseract
// program of the system reads (with its own models). PP-OCRv5 (engine "onnx") downloads its models
// on first use and runs on the onnxruntime of speech recognition, on the graphics card when that
// uses it. On Windows the system's own text recognition (os) follows. Last the AI service (api)
// with the user's own key from the settings: it reads only when a request names it or the settings
// make it the default, never as the first usable engine (it costs money, the image leaves the
// computer). The default engine of requests without one is a setting too, read at every request.
func ocrEngines(dataDir string, pl *pipeline.Runner) *ocr.Registry {
	t := &ocr.Tesseract{Hint: "apt install tesseract-ocr tesseract-ocr-deu, brew install tesseract tesseract-lang"}
	if models.TesseractLibSupported() {
		t = &ocr.Tesseract{
			TessData:     models.TesseractDataDir(dataDir),
			Lib:          tesslib.New(dataDir),
			Downloadable: models.TesseractModels(),
			Fetch: func(ctx context.Context, model string) error {
				return models.FetchTesseractModel(ctx, dataDir, model)
			},
		}
	}
	p := &ocr.PPOCR{
		Nets:     &ort.Nets{Runtime: pl.Runtime, Present: pl.RuntimePresent, WantsGPU: pl.WantsGPU},
		ModelDir: models.PPOCRDir(dataDir),
		Fetch: func(ctx context.Context, file string) error {
			return models.FetchPPOCRModel(ctx, dataDir, file)
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

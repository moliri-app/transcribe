// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package models

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// PP-OCRv5 for text recognition (OCR, engine "onnx") is not shipped either: its two models come on
// first use, each pinned by its checksum, as RapidOCR publishes them converted to ONNX. They run
// on the onnxruntime of the speech recognition runtime (lib/ or lib-cuda/).

// PPOCRDir holds the models.
func PPOCRDir(dataDir string) string { return filepath.Join(dataDir, "ppocr") }

const ppocrBase = "https://www.modelscope.cn/models/RapidAI/RapidOCR/resolve/v3.10.0/onnx/PP-OCRv5/"

// ppocrModels are text detection (all scripts) and recognition of Latin script, the mobile models
// RapidOCR 3.10 measured in Moliri (docs/ki-einsatz-skizze.md §2.5).
var ppocrModels = map[string]tessModel{
	"ch_PP-OCRv5_det_mobile.onnx":    {ppocrBase + "det/ch_PP-OCRv5_det_mobile.onnx", "4d97c44a20d30a81aad087d6a396b08f786c4635742afc391f6621f5c6ae78ae"},
	"latin_PP-OCRv5_rec_mobile.onnx": {ppocrBase + "rec/latin_PP-OCRv5_rec_mobile.onnx", "b20bd37c168a570f583afbc8cd7925603890efbcdc000a59e22c269d160b5f5a"},
}

// PPOCRModels are the files of the models, sorted.
func PPOCRModels() []string {
	names := make([]string, 0, len(ppocrModels))
	for n := range ppocrModels {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// PPOCRInstalled reports whether all models are in place.
func PPOCRInstalled(dataDir string) bool {
	for f := range ppocrModels {
		if st, err := os.Stat(filepath.Join(PPOCRDir(dataDir), f)); err != nil || st.Size() == 0 {
			return false
		}
	}
	return true
}

// FetchPPOCRModel downloads one model file into PPOCRDir unless it is there.
func FetchPPOCRModel(ctx context.Context, dataDir, file string) error {
	m, ok := ppocrModels[file]
	if !ok {
		return fmt.Errorf("no download for the PP-OCR model %q", file)
	}
	tessMu.Lock()
	defer tessMu.Unlock()
	dir := PPOCRDir(dataDir)
	path := filepath.Join(dir, file)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	defer os.Remove(tmp)
	err := download(ctx, "ppocr", m.url, m.sum, tmp)
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		track("ppocr", "error", 0, 0, err)
		return err
	}
	if PPOCRInstalled(dataDir) {
		track("ppocr", "done", 0, 0, nil)
	}
	return nil
}

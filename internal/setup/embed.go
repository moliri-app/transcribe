// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package setup

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/moliri-app/transcribe/embed"
	"github.com/moliri-app/transcribe/models"
)

// EmbeddingGemma, for Moliri's field search by meaning (docs/embed-api.md), is not shipped either:
// its files (295 MB) come when Moliri first asks or on the settings page, each pinned by its
// checksum, from the revision of onnx-community's ONNX export that Moliri's browser model was
// measured with. It runs on the onnxruntime of the speech recognition runtime.

// EmbedDir holds the model.
func EmbedDir(dataDir string) string { return filepath.Join(dataDir, "embeddinggemma") }

const embedBase = "https://huggingface.co/onnx-community/embeddinggemma-300m-ONNX/resolve/5090578d9565bb06545b4552f76e6bc2c93e4a66/"

var embedFiles = map[string]string{
	"tokenizer.json":                 "4dda02faaf32bc91031dc8c88457ac272b00c1016cc679757d1c441b248b9c47",
	"onnx/model_quantized.onnx":      "172efde319fe1542dc41f31be6154910b05b78f7a861c265c4600eec906bd6d8",
	"onnx/model_quantized.onnx_data": "705626e28e4c23c82ade34566b4197d97f534c12275fa406dfb71e9937d388c0",
}

var embedMu sync.Mutex

// EmbedInstalled reports whether all of the model's files are in place.
func EmbedInstalled(dataDir string) bool {
	return (&embed.Model{Dir: EmbedDir(dataDir)}).Installed()
}

// FetchEmbedModel downloads the model's files that are missing. One download runs at a time; a
// second caller waits for it and then finds the files there.
func FetchEmbedModel(ctx context.Context, dataDir string) error {
	embedMu.Lock()
	defer embedMu.Unlock()
	if EmbedInstalled(dataDir) {
		return nil
	}
	dir := EmbedDir(dataDir)
	for _, f := range embed.Files {
		path := filepath.Join(dir, filepath.FromSlash(f))
		if st, err := os.Stat(path); err == nil && st.Size() > 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		tmp := path + ".tmp"
		err := download(ctx, "embed", embedBase+f, embedFiles[f], tmp)
		if err == nil {
			err = os.Rename(tmp, path)
		}
		os.Remove(tmp)
		if err != nil {
			models.Track("embed", "error", 0, 0, err)
			return err
		}
	}
	models.Track("embed", "done", 0, 0, nil)
	return nil
}

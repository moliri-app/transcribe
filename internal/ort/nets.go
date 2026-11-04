// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ort

import (
	"context"
	"sync"

	"github.com/moliri-app/transcribe/internal/ocr"
)

// Nets opens the models of text recognition (ocr.Nets) on the runtime Runtime names.
type Nets struct {
	// Runtime gives the folder of the runtime library and whether it is the CUDA build,
	// downloading it first when it is missing.
	Runtime func(ctx context.Context) (dir string, cuda bool, err error)
	// Present reports whether the runtime is there, nothing left to download.
	Present func() bool
	// WantsGPU reports whether models would run on the graphics card, before one is loaded.
	WantsGPU func() bool

	mu     sync.Mutex
	opened bool
	gpu    bool
}

func (n *Nets) Installed() bool {
	n.mu.Lock()
	opened := n.opened
	n.mu.Unlock()
	return opened || (n.Present != nil && n.Present())
}

func (n *Nets) GPU() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.opened {
		return n.gpu
	}
	return n.WantsGPU != nil && n.WantsGPU()
}

func (n *Nets) Open(ctx context.Context, path string, threads int) (ocr.Net, error) {
	dir, cuda, err := n.Runtime(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := Load(dir)
	if err != nil {
		return nil, err
	}
	s, err := rt.NewSession(path, threads, cuda)
	if err != nil {
		return nil, err
	}
	n.mu.Lock()
	n.opened, n.gpu = true, s.GPU
	n.mu.Unlock()
	return s, nil
}

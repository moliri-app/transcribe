// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package jobs

import (
	"context"
	"sync"
)

// Gate pauses a job while it runs: the work asks Hold at the points where it may stop (between
// pieces of audio), and waits there while the gate is shut. What the job holds (models, memory)
// stays, so it goes on where it stopped.
type Gate struct {
	mu   sync.Mutex
	shut chan struct{} // closed when the gate opens again; nil while it is open
}

// Shut pauses the work under the gate at its next Hold.
func (g *Gate) Shut() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.shut == nil {
		g.shut = make(chan struct{})
	}
}

// Open lets the work under the gate go on.
func (g *Gate) Open() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.shut != nil {
		close(g.shut)
		g.shut = nil
	}
}

func (g *Gate) wait() <-chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.shut
}

type gateKey struct{}

// WithGate returns a context under which Hold waits while g is shut.
func WithGate(ctx context.Context, g *Gate) context.Context {
	return context.WithValue(ctx, gateKey{}, g)
}

// Hold waits while the gate of the job running under ctx is shut, and returns ctx.Err(): work
// calls it where it would check ctx.Err() between pieces. Without a gate it waits for nothing.
func Hold(ctx context.Context) error {
	if g, ok := ctx.Value(gateKey{}).(*Gate); ok {
		for {
			c := g.wait()
			if c == nil {
				break
			}
			select {
			case <-c:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return ctx.Err()
}

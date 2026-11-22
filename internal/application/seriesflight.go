package application

import (
	"context"
	"sync"
	"time"
)

// blockFlightTimeout bounds a shared fill. It runs detached from the caller
// that started it, so one caller giving up never fails the others.
const blockFlightTimeout = 2 * time.Minute

// blockFlight gives every month block at most one reader at a time; other
// callers that need the block wait for that reader instead of reading it too.
type blockFlight struct {
	mu    sync.Mutex
	calls map[string]*blockCall
}

// blockCall is one in-flight read of one block.
type blockCall struct {
	done chan struct{}
	vals []float32
	err  error
}

// claim returns the in-flight call for each key, starting one where none
// runs; owned[i] reports that the caller started it and must finish it.
func (g *blockFlight) claim(keys []string) (calls []*blockCall, owned []bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.calls == nil {
		g.calls = map[string]*blockCall{}
	}
	calls, owned = make([]*blockCall, len(keys)), make([]bool, len(keys))
	for i, k := range keys {
		if call, ok := g.calls[k]; ok {
			calls[i] = call
			continue
		}
		calls[i], owned[i] = &blockCall{done: make(chan struct{})}, true
		g.calls[k] = calls[i]
	}
	return calls, owned
}

// finish publishes the owner's result and releases the key.
func (g *blockFlight) finish(key string, call *blockCall, vals []float32, err error) {
	call.vals, call.err = vals, err
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	close(call.done)
}

// wait blocks until the call finishes or ctx is done.
func (call *blockCall) wait(ctx context.Context) ([]float32, error) {
	select {
	case <-call.done:
		return call.vals, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

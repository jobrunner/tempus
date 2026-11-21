package omstore

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// lru is a small fixed-capacity least-recently-used map.
type lru[K comparable, V any] struct {
	mu    sync.Mutex
	cap   int
	order *list.List
	items map[K]*list.Element
}

type lruEntry[K comparable, V any] struct {
	key K
	val V
}

func newLRU[K comparable, V any](capacity int) *lru[K, V] {
	return &lru[K, V]{cap: capacity, order: list.New(), items: map[K]*list.Element{}}
}

func (c *lru[K, V]) get(k K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[k]; ok {
		c.order.MoveToFront(e)
		return e.Value.(*lruEntry[K, V]).val, true
	}
	var zero V
	return zero, false
}

func (c *lru[K, V]) put(k K, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[k]; ok {
		e.Value.(*lruEntry[K, V]).val = v
		c.order.MoveToFront(e)
		return
	}
	c.items[k] = c.order.PushFront(&lruEntry[K, V]{key: k, val: v})
	if c.order.Len() > c.cap {
		last := c.order.Back()
		c.order.Remove(last)
		delete(c.items, last.Value.(*lruEntry[K, V]).key)
	}
}

// sharedWorkTimeout bounds work that several requests wait on. It runs on a
// context detached from whichever request started it, so one caller giving
// up never fails the others.
const sharedWorkTimeout = 2 * time.Minute

// flightGroup collapses concurrent calls with the same key into one. The
// shared call runs on its own goroutine with a detached, bounded context;
// every caller — the one that started it too — waits only as long as its own
// context allows.
type flightGroup struct {
	mu    sync.Mutex
	calls map[string]*flightCall
}

type flightCall struct {
	done    chan struct{}
	val     any
	err     error
	waiters int // callers that joined after the start (guarded by mu)
}

func (g *flightGroup) do(ctx context.Context, key string, fn func(context.Context) (any, error)) (any, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = map[string]*flightCall{}
	}
	c, ok := g.calls[key]
	if !ok {
		c = &flightCall{done: make(chan struct{})}
		g.calls[key] = c
		go g.run(context.WithoutCancel(ctx), key, c, fn)
	} else {
		c.waiters++
	}
	g.mu.Unlock()
	select {
	case <-c.done:
		return c.val, c.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (g *flightGroup) run(detached context.Context, key string, c *flightCall, fn func(context.Context) (any, error)) {
	ctx, cancel := context.WithTimeout(detached, sharedWorkTimeout)
	defer cancel()
	c.val, c.err = fn(ctx)
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	close(c.done)
}

// joined reports how many callers joined the running call for key.
func (g *flightGroup) joined(key string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if c, ok := g.calls[key]; ok {
		return c.waiters
	}
	return 0
}

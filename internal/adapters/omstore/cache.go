package omstore

import (
	"container/list"
	"sync"
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

// flightGroup collapses concurrent calls with the same key into one.
type flightGroup struct {
	mu    sync.Mutex
	calls map[string]*flightCall
}

type flightCall struct {
	done chan struct{}
	val  any
	err  error
}

func (g *flightGroup) do(key string, fn func() (any, error)) (any, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = map[string]*flightCall{}
	}
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		<-c.done
		return c.val, c.err
	}
	c := &flightCall{done: make(chan struct{})}
	g.calls[key] = c
	g.mu.Unlock()
	c.val, c.err = fn()
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	close(c.done)
	return c.val, c.err
}

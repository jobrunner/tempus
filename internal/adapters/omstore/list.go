package omstore

import (
	"context"
	"sync"
	"time"
)

// invalidate drops the listing that contains key.
func (s *Store) invalidate(key string) {
	dir := key[:max(0, lastSlash(key))]
	s.mu.Lock()
	delete(s.catalogs, dir)
	s.mu.Unlock()
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// catalog returns the (cached) listing of data/<domain>/<variable>/.
func (s *Store) catalog(ctx context.Context, domain, variable string) (*catalog, error) {
	dir := "data/" + domain + "/" + variable
	s.mu.Lock()
	c, ok := s.catalogs[dir]
	s.mu.Unlock()
	if ok && s.now().Sub(c.fetched) < s.listTTL {
		return c, nil
	}
	return s.relist(ctx, dir)
}

// KeepListed lists the variable directories ("<domain>/<variable>") now and
// again whenever three quarters of the listing TTL have passed, until ctx is
// done. A query then never waits for a listing — on a cold path that is a
// full round trip to the bucket before any data read can start. A failed
// refresh is left to the next one; queries still list on demand.
func (s *Store) KeepListed(ctx context.Context, dirs []string) {
	for {
		var wg sync.WaitGroup
		for _, d := range dirs {
			// Directly, not through the flight group: shared work runs
			// detached from its caller, so a shutdown would not stop it.
			wg.Go(func() { _, _ = s.listInto(ctx, "data/"+d) })
		}
		wg.Wait()
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.listTTL / 4 * 3):
		}
	}
}

// relist fetches the listing of dir and caches it; concurrent callers share
// one listing.
func (s *Store) relist(ctx context.Context, dir string) (*catalog, error) {
	v, err := s.flight.do(ctx, "list:"+dir, func(ctx context.Context) (any, error) {
		return s.listInto(ctx, dir)
	})
	if err != nil {
		return nil, err
	}
	return v.(*catalog), nil
}

// listInto lists dir on ctx and caches the result.
func (s *Store) listInto(ctx context.Context, dir string) (*catalog, error) {
	objs, err := s.list(ctx, dir+"/")
	if err != nil {
		return nil, err
	}
	c := newCatalog(objs, s.now())
	s.mu.Lock()
	s.catalogs[dir] = c
	s.mu.Unlock()
	return c, nil
}

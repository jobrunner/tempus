package omstore

import (
	"context"
	"sync"
	"time"
)

// invalidate drops the listing that contains key. A listing that started
// before the invalidation must not bring it back when it finishes.
func (s *Store) invalidate(key string) {
	dir := key[:max(0, lastSlash(key))]
	s.mu.Lock()
	s.listSeq++
	s.dropped[dir] = s.listSeq
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

// listInto lists dir on ctx and caches the result, unless a newer listing
// or an invalidation got there first.
func (s *Store) listInto(ctx context.Context, dir string) (*catalog, error) {
	seq := s.beginListing()
	objs, err := s.list(ctx, dir+"/")
	if err != nil {
		return nil, err
	}
	c := newCatalog(objs, s.now())
	c.seq = seq
	return s.publish(dir, c), nil
}

// beginListing numbers a listing as it starts.
func (s *Store) beginListing() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listSeq++
	return s.listSeq
}

// publish caches c as dir's catalog when it is the newest view: no listing
// that started later has been published and the directory was not
// invalidated after c's listing started. A delayed background refresh thus
// never replaces a newer catalog with an older snapshot. It returns the
// catalog that is current afterwards.
func (s *Store) publish(dir string, c *catalog) *catalog {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cur, ok := s.catalogs[dir]; ok && cur.seq > c.seq {
		return cur
	}
	if s.dropped[dir] > c.seq {
		return c
	}
	s.catalogs[dir] = c
	return c
}

package omstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// get fetches [off, off+n) of obj. If-Match pins the ETag the cached metadata
// belongs to; a 412 means the file was rewritten, which invalidates the
// listing so the next read sees the new version.
func (s *Store) get(ctx context.Context, obj object, off, n uint64) ([]byte, error) {
	if n == 0 {
		return nil, nil
	}
	release, err := s.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/"+obj.Key, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+n-1))
	if obj.ETag != "" {
		req.Header.Set("If-Match", obj.ETag)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, upstream("%s: %w", obj.Key, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := s.checkRange(obj, resp.StatusCode); err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, upstream("%s: %w", obj.Key, err)
	}
	if uint64(len(b)) != n {
		return nil, upstream("%s: got %d of %d bytes", obj.Key, len(b), n)
	}
	s.stats.add(len(b))
	return b, nil
}

// acquire takes one of the MaxParallel request slots, or gives up when ctx
// ends first. The returned func frees the slot.
func (s *Store) acquire(ctx context.Context) (release func(), err error) {
	select {
	case s.sem <- struct{}{}:
		return func() { <-s.sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// checkRange classifies the status of a range GET: only 206 carries the
// requested bytes; 412 and 404 mean the file changed or vanished since the
// listing, which is dropped so the next read lists afresh.
func (s *Store) checkRange(obj object, status int) error {
	switch status {
	case http.StatusPartialContent:
		return nil
	case http.StatusPreconditionFailed, http.StatusNotFound:
		s.invalidate(obj.Key)
		return upstream("%s changed or vanished (status %d)", obj.Key, status)
	default:
		return upstream("%s: status %d", obj.Key, status)
	}
}

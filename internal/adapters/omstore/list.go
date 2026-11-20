package omstore

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
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
	v, err := s.flight.do(ctx, "list:"+dir, func() (any, error) {
		objs, err := s.list(ctx, dir+"/")
		if err != nil {
			return nil, err
		}
		c := newCatalog(objs, s.now())
		s.mu.Lock()
		s.catalogs[dir] = c
		s.mu.Unlock()
		return c, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*catalog), nil
}

type listResult struct {
	Contents []struct {
		Key  string `xml:"Key"`
		ETag string `xml:"ETag"`
		Size uint64 `xml:"Size"`
	} `xml:"Contents"`
	IsTruncated           bool   `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
}

// list runs S3 ListObjectsV2 over prefix, following continuation tokens.
func (s *Store) list(ctx context.Context, prefix string) ([]object, error) {
	var objs []object
	token := ""
	for {
		res, err := s.listPage(ctx, prefix, token)
		if err != nil {
			return nil, err
		}
		for _, c := range res.Contents {
			objs = append(objs, object{Key: c.Key, ETag: c.ETag, Size: c.Size})
		}
		if !res.IsTruncated || res.NextContinuationToken == "" {
			return objs, nil
		}
		token = res.NextContinuationToken
	}
}

// listPage fetches one ListObjectsV2 page; token is empty for the first.
func (s *Store) listPage(ctx context.Context, prefix, token string) (listResult, error) {
	var res listResult
	q := url.Values{"list-type": {"2"}, "prefix": {prefix}}
	if token != "" {
		q.Set("continuation-token", token)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/?"+q.Encode(), nil)
	if err != nil {
		return res, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return res, upstream("list %s: %w", prefix, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return res, upstream("list %s: status %d", prefix, resp.StatusCode)
	}
	err = xml.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&res)
	return res, err
}

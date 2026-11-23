package omstore

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
)

// The S3 ListObjectsV2 call behind the catalogs.

type listResult struct {
	Contents []struct {
		Key  string `xml:"Key"`
		ETag string `xml:"ETag"`
		Size uint64 `xml:"Size"`
	} `xml:"Contents"`
	IsTruncated           bool   `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
}

// maxListPages bounds the continuation loop: a variable directory holds a few
// hundred files (1000 per page), so this is far beyond any real listing.
const maxListPages = 100

// list runs S3 ListObjectsV2 over prefix, following continuation tokens.
func (s *Store) list(ctx context.Context, prefix string) ([]object, error) {
	var objs []object
	token := ""
	for page := 0; ; page++ {
		if page >= maxListPages {
			return nil, upstream("list %s: more than %d pages", prefix, maxListPages)
		}
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

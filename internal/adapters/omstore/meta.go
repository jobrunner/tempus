package omstore

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jobrunner/tempus/internal/adapters/omfile"
)

// array returns the parsed root variable, cached per key and ETag — in
// memory, and as raw bytes in the persistent cache when there is one.
func (s *Store) array(ctx context.Context, obj object) (omfile.Array, error) {
	key := obj.Key + "@" + obj.ETag
	if a, ok := s.arrays.get(key); ok {
		return a, nil
	}
	v, err := s.flight.do(ctx, key, func(ctx context.Context) (any, error) {
		raw, err := s.persisted(ctx, obj, "array|"+key, func(ctx context.Context) ([]byte, error) { return s.fetchVariable(ctx, obj) })
		if err != nil {
			return nil, err
		}
		a, err := omfile.ParseArray(raw)
		if err != nil {
			return nil, fmt.Errorf("omstore: %s: %w", obj.Key, err)
		}
		s.arrays.put(key, a)
		return a, nil
	})
	if err != nil {
		return omfile.Array{}, err
	}
	return v.(omfile.Array), nil
}

// fetchVariable reads the file's tail, locates the root variable through the
// trailer and returns its bytes — fetching the variable on its own only when
// it does not lie inside the tail.
func (s *Store) fetchVariable(ctx context.Context, obj object) ([]byte, error) {
	n := min(obj.Size, tailBytes)
	tail, err := s.get(ctx, obj, obj.Size-n, n)
	if err != nil {
		return nil, err
	}
	off, size, err := omfile.ParseTrailer(tail)
	if err != nil {
		return nil, fmt.Errorf("omstore: %s: %w", obj.Key, err)
	}
	start := obj.Size - n
	if off < start || off+size > obj.Size {
		// The variable lies outside the tail; fetch it on its own.
		if tail, err = s.get(ctx, obj, off, size); err != nil {
			return nil, err
		}
		start = off
	}
	// A copy: a cache may keep the bytes, and the slice would pin the whole
	// tail buffer while only the slice is accounted for.
	return append([]byte(nil), tail[off-start:off-start+size]...), nil
}

// lut returns the plan's LUT blocks, cached per key, ETag and block range.
func (s *Store) lut(ctx context.Context, obj object, p omfile.Plan) ([]byte, error) {
	off, n := p.LUTRange()
	key := obj.Key + "@" + obj.ETag + "#" + strconv.FormatUint(off, 10) + "+" + strconv.FormatUint(n, 10)
	if b, ok := s.luts.get(key); ok {
		return b, nil
	}
	v, err := s.flight.do(ctx, key, func(ctx context.Context) (any, error) {
		b, err := s.persisted(ctx, obj, "lut|"+key, func(ctx context.Context) ([]byte, error) { return s.get(ctx, obj, off, n) })
		if err != nil {
			return nil, err
		}
		s.luts.put(key, b)
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

// metaTTL bounds how long file metadata stays in the persistent cache. The
// key carries the file's ETag, so a rewritten file is never served stale
// metadata; the TTL only lets entries of files nobody reads any more go.
const metaTTL = 30 * 24 * time.Hour

// metaVersion is part of every persistent metadata key — bump it when the
// stored form changes.
const metaVersion = "omstore-meta|v1|"

// persisted returns the bytes cached under key, or fetches, stores and
// returns them. Metadata of the files read so far then survives a restart,
// which saves the tail and LUT round trips of the first read after every
// deploy. Without a persistent cache, or for an object without an ETag —
// whose key would then not change when the file is replaced — it only
// fetches.
func (s *Store) persisted(ctx context.Context, obj object, key string, fetch func(context.Context) ([]byte, error)) ([]byte, error) {
	if s.meta == nil || obj.ETag == "" {
		return fetch(ctx)
	}
	if raw, ok, err := s.meta.Get(ctx, metaVersion+key); err == nil && ok {
		return raw, nil
	}
	raw, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	_ = s.meta.Set(ctx, metaVersion+key, raw, metaTTL)
	return raw, nil
}

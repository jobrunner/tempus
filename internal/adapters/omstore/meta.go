package omstore

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jobrunner/tempus/internal/adapters/omfile"
)

// array returns the parsed root variable, cached per key and ETag.
func (s *Store) array(ctx context.Context, obj object) (omfile.Array, error) {
	key := obj.Key + "@" + obj.ETag
	if a, ok := s.arrays.get(key); ok {
		return a, nil
	}
	v, err := s.flight.do(ctx, key, func(ctx context.Context) (any, error) {
		a, err := s.fetchArray(ctx, obj)
		if err != nil {
			return nil, err
		}
		s.arrays.put(key, a)
		return a, nil
	})
	if err != nil {
		return omfile.Array{}, err
	}
	return v.(omfile.Array), nil
}

// fetchArray reads the file's tail, locates the root variable through the
// trailer and parses it — fetching the variable on its own only when it does
// not lie inside the tail.
func (s *Store) fetchArray(ctx context.Context, obj object) (omfile.Array, error) {
	n := min(obj.Size, tailBytes)
	tail, err := s.get(ctx, obj, obj.Size-n, n)
	if err != nil {
		return omfile.Array{}, err
	}
	off, size, err := omfile.ParseTrailer(tail)
	if err != nil {
		return omfile.Array{}, fmt.Errorf("omstore: %s: %w", obj.Key, err)
	}
	start := obj.Size - n
	if off < start || off+size > obj.Size {
		// The variable lies outside the tail; fetch it on its own.
		if tail, err = s.get(ctx, obj, off, size); err != nil {
			return omfile.Array{}, err
		}
		start = off
	}
	a, err := omfile.ParseArray(tail[off-start : off-start+size])
	if err != nil {
		return omfile.Array{}, fmt.Errorf("omstore: %s: %w", obj.Key, err)
	}
	return a, nil
}

// lut returns the plan's LUT blocks, cached per key, ETag and block range.
func (s *Store) lut(ctx context.Context, obj object, p omfile.Plan) ([]byte, error) {
	off, n := p.LUTRange()
	key := obj.Key + "@" + obj.ETag + "#" + strconv.FormatUint(off, 10) + "+" + strconv.FormatUint(n, 10)
	if b, ok := s.luts.get(key); ok {
		return b, nil
	}
	v, err := s.flight.do(ctx, key, func(ctx context.Context) (any, error) {
		b, err := s.get(ctx, obj, off, n)
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

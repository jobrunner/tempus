package omfile

import "encoding/binary"

// unpackBase reads the n base values of a block in the layout interleaved
// selects (FORMAT.md §4.2).
func unpackBase(b []byte, dst []uint64, n, width int, interleaved bool) (int, error) {
	if interleaved {
		return unpackInterleaved(b, dst, width)
	}
	return unpackHorizontal(b, dst, n, width)
}

// unpackHorizontal reads n values of the given width from an LSB-first bit
// stream (FORMAT.md §4.2).
func unpackHorizontal(b []byte, dst []uint64, n, width int) (int, error) {
	if width > 64 {
		return 0, errCorrupt
	}
	nb := (n*width + 7) / 8
	if len(b) < nb {
		return 0, errTruncated
	}
	if width == 0 {
		clear(dst[:n])
		return 0, nil
	}
	mask := uint64(1)<<width - 1
	if width == 64 {
		mask = ^uint64(0)
	}
	bit := 0
	for i := range n {
		var v uint64
		got := 0
		for got < width {
			byteIdx, shift := bit/8, bit%8
			take := min(8-shift, width-got)
			v |= uint64(b[byteIdx]>>shift) & (1<<take - 1) << got
			got += take
			bit += take
		}
		dst[i] = v & mask
	}
	return nb, nil
}

// unpackInterleaved reads a full 128-value block packed into 8 lanes of 16-bit
// words: value i lives in lane i%8 at position i/8, each lane is an LSB-first
// bit stream over its words, and word k of lane l sits at byte (k*8+l)*2
// (FORMAT.md §4.2).
func unpackInterleaved(b []byte, dst []uint64, width int) (int, error) {
	if width > 16 {
		return 0, errCorrupt
	}
	nb := laneBlock * width / 8
	if len(b) < nb {
		return 0, errTruncated
	}
	if width == 0 {
		clear(dst[:laneBlock])
		return 0, nil
	}
	mask := uint32(1)<<width - 1
	for lane := range lanes {
		var acc uint32
		have, word := 0, 0
		for j := range laneBlock / lanes {
			if have < width {
				w := uint32(binary.LittleEndian.Uint16(b[(word*lanes+lane)*2:]))
				word++
				acc |= w << have
				have += 16
			}
			dst[j*lanes+lane] = uint64(acc & mask)
			acc >>= width
			have -= width
		}
	}
	return nb, nil
}

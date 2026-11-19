package omfile

import (
	"encoding/binary"
	"math/bits"
)

// Block header layout (FORMAT.md §4.3): the low six bits are the base bit width
// b, the two high bits select the mode.
const (
	modeMask     = 0xc0
	modeConstant = 0xc0 // every value equals one stored value
	modeBitmap   = 0x80 // exceptions located by a bitmap, high bits packed
	modeVarint   = 0x40 // exceptions as variable-byte values plus positions
	widthMask    = 0x3f

	// laneBlock is the number of values in one lane-interleaved block, lanes the
	// number of 16-bit lanes it is split into (FORMAT.md §4.2).
	laneBlock = 128
	lanes     = 8
)

// decodeBlock decodes one PFor block of n values into dst[:n] and returns the
// number of bytes consumed. interleaved selects the 8×16-bit lane layout of full
// 128-value blocks; otherwise the base values are packed horizontally.
func decodeBlock(b []byte, dst []uint64, n int, interleaved bool) (int, error) {
	if len(b) == 0 {
		return 0, errTruncated
	}
	header := b[0]
	width := int(header & widthMask)
	pos := 1
	switch header & modeMask {
	case modeConstant:
		nb := (width + 7) / 8
		if len(b) < pos+nb {
			return 0, errTruncated
		}
		var v uint64
		for i := nb - 1; i >= 0; i-- {
			v = v<<8 | uint64(b[pos+i])
		}
		for i := range n {
			dst[i] = v
		}
		return pos + nb, nil
	case modeBitmap:
		return decodeBitmapBlock(b, dst, n, width, interleaved)
	case modeVarint:
		return decodeVarintBlock(b, dst, n, width, interleaved)
	}
	used, err := unpackBase(b[pos:], dst, n, width, interleaved)
	return pos + used, err
}

// decodeBitmapBlock handles header|0x80: [header][bx][bitmap][exceptions][base].
func decodeBitmapBlock(b []byte, dst []uint64, n, width int, interleaved bool) (int, error) {
	pos := 1
	if len(b) < pos+1 {
		return 0, errTruncated
	}
	excWidth := int(b[pos])
	pos++
	mapLen := (n + 7) / 8
	if len(b) < pos+mapLen {
		return 0, errTruncated
	}
	bitmap := b[pos : pos+mapLen]
	pos += mapLen
	count := 0
	for _, m := range bitmap {
		count += bits.OnesCount8(m)
	}
	exc := make([]uint64, count)
	used, err := unpackHorizontal(b[pos:], exc, count, excWidth)
	if err != nil {
		return 0, err
	}
	pos += used
	used, err = unpackBase(b[pos:], dst, n, width, interleaved)
	if err != nil {
		return 0, err
	}
	pos += used
	k := 0
	for i := range n {
		if bitmap[i/8]&(1<<(i%8)) != 0 {
			dst[i] |= exc[k] << width
			k++
		}
	}
	return pos, nil
}

// decodeVarintBlock handles header|0x40: [header][count][base][values][positions].
func decodeVarintBlock(b []byte, dst []uint64, n, width int, interleaved bool) (int, error) {
	pos := 1
	if len(b) < pos+1 {
		return 0, errTruncated
	}
	count := int(b[pos])
	pos++
	used, err := unpackBase(b[pos:], dst, n, width, interleaved)
	if err != nil {
		return 0, err
	}
	pos += used
	exc := make([]uint64, count)
	for i := range exc {
		v, l, err := readExceptionVarint(b[pos:])
		if err != nil {
			return 0, err
		}
		exc[i] = v
		pos += l
	}
	if len(b) < pos+count {
		return 0, errTruncated
	}
	for i, p := range b[pos : pos+count] {
		if int(p) >= n {
			return 0, errCorrupt
		}
		dst[p] |= exc[i] << width
	}
	return pos + count, nil
}

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

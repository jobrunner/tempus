package omfile

import "math/bits"

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
		return decodeConstantBlock(b, dst, n, width)
	case modeBitmap:
		return decodeBitmapBlock(b, dst, n, width, interleaved)
	case modeVarint:
		return decodeVarintBlock(b, dst, n, width, interleaved)
	}
	used, err := unpackBase(b[pos:], dst, n, width, interleaved)
	return pos + used, err
}

// decodeConstantBlock handles header|0xc0: [header][value], the one value
// stored little-endian in ceil(width/8) bytes and repeated n times.
func decodeConstantBlock(b []byte, dst []uint64, n, width int) (int, error) {
	pos := 1
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

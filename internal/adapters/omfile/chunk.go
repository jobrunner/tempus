package omfile

import "math"

// nanMarker is the quantised value that stands for NaN.
const nanMarker = math.MaxInt16

// DecodeChunk decodes one pfor_delta_2d_int16 chunk whose actual (edge-clipped)
// shape is shape into dst, which must hold product(shape) values (FORMAT.md §5).
// The values are the quantised int16 bit patterns; Array.Float converts them.
// All arithmetic wraps at 16 bits, exactly like the encoder's int16 deltas.
func DecodeChunk(b []byte, shape []uint64, dst []uint16) error {
	total := uint64(1)
	for _, d := range shape {
		total *= d
	}
	if total > uint64(len(dst)) {
		return errCorrupt
	}
	vals := dst[:total]
	if len(vals) == 0 {
		return nil
	}
	if err := decodeDeltas(b, vals); err != nil {
		return err
	}
	undoRowDeltas(vals, len(vals[:shape[len(shape)-1]]))
	return nil
}

// decodeDeltas decodes the chunk's value stream into vals: a prefix varint
// holding the first value, then PFor blocks of zigzag-coded deltas that are
// summed up along the flattened chunk.
func decodeDeltas(b []byte, vals []uint16) error {
	first, pos, err := readPrefixVarint(b)
	if err != nil {
		return err
	}
	if first > math.MaxUint16 {
		return errCorrupt
	}
	zz := make([]uint64, laneBlock)
	prev := uint16(first)
	vals[0] = prev
	for i := 1; i < len(vals); {
		n := min(len(vals)-i, laneBlock)
		used, err := decodeBlock(b[pos:], zz, n, n == laneBlock)
		if err != nil {
			return err
		}
		pos += used
		for _, z := range zz[:n] {
			if z > math.MaxUint16 {
				return errCorrupt
			}
			prev += unzigzag16(uint16(z))
			vals[i] = prev
			i++
		}
	}
	return nil
}

// undoRowDeltas reverses the 2D delta step: every row of cols values was
// stored as its difference to the row above.
func undoRowDeltas(vals []uint16, cols int) {
	for r := cols; r < len(vals); r += cols {
		row, above := vals[r:r+cols], vals[r-cols:r]
		for c := range row {
			row[c] += above[c]
		}
	}
}

// unzigzag16 maps 0, 1, 2, 3, … to the two's-complement bit patterns of
// 0, −1, 1, −2, … so that adding it wraps like int16 arithmetic.
func unzigzag16(z uint16) uint16 {
	return z>>1 ^ -(z & 1)
}

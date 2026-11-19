package omfile

import "encoding/binary"

// readPrefixVarint decodes the length-prefixed integer that opens every chunk
// and every LUT block (FORMAT.md §4.1). The number of leading one bits k of the
// first byte says how many bytes follow; the remaining 7-k bits of the first
// byte are the most significant part of the value. The byte order of the
// following bytes depends on k and was established by experiment: k=1 one byte,
// k=2 little-endian, k=3 big-endian, k=4 little-endian. Longer forms have not
// been observed (they are not needed below 32 GiB offsets) and are rejected.
func readPrefixVarint(b []byte) (uint64, int, error) {
	if len(b) == 0 {
		return 0, 0, errTruncated
	}
	p := b[0]
	switch {
	case p < 0x80:
		return uint64(p), 1, nil
	case p < 0xc0:
		if len(b) < 2 {
			return 0, 0, errTruncated
		}
		return uint64(p&0x3f)<<8 | uint64(b[1]), 2, nil
	case p < 0xe0:
		if len(b) < 3 {
			return 0, 0, errTruncated
		}
		return uint64(p&0x1f)<<16 | uint64(binary.LittleEndian.Uint16(b[1:])), 3, nil
	case p < 0xf0:
		if len(b) < 4 {
			return 0, 0, errTruncated
		}
		return uint64(p&0x0f)<<24 | uint64(b[1])<<16 | uint64(b[2])<<8 | uint64(b[3]), 4, nil
	case p < 0xf8:
		if len(b) < 5 {
			return 0, 0, errTruncated
		}
		return uint64(p&0x07)<<32 | uint64(binary.LittleEndian.Uint32(b[1:])), 5, nil
	}
	return 0, 0, errUnknownVarint
}

// Offsets of the exception varint forms (FORMAT.md §4.4).
const (
	excOneByteLimit = 0xb1  // values below are stored as one byte
	excTwoByteLimit = 16561 // first value of the three-byte offset form
)

// readExceptionVarint decodes the variable-byte encoding of exception values in
// the 0x40 block mode (FORMAT.md §4.4). Two encoders exist side by side: the
// three-byte offset form (0xf1…0xf8) and the raw form (0xff + uint16). Only the
// forms observed for values up to 16 bits are known; any other first byte is
// reported, never guessed.
func readExceptionVarint(b []byte) (uint64, int, error) {
	if len(b) == 0 {
		return 0, 0, errTruncated
	}
	switch first := b[0]; {
	case first < excOneByteLimit:
		return uint64(first), 1, nil
	case first <= 0xf0:
		if len(b) < 2 {
			return 0, 0, errTruncated
		}
		return uint64(first-excOneByteLimit)<<8 + uint64(b[1]) + excOneByteLimit, 2, nil
	case first <= 0xf8:
		if len(b) < 3 {
			return 0, 0, errTruncated
		}
		return uint64(first-0xf1)<<16 + uint64(binary.LittleEndian.Uint16(b[1:3])) + excTwoByteLimit, 3, nil
	case first == 0xff:
		if len(b) < 3 {
			return 0, 0, errTruncated
		}
		return uint64(binary.LittleEndian.Uint16(b[1:3])), 3, nil
	default:
		return 0, 0, errUnknownVarint
	}
}

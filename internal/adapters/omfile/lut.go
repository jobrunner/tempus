package omfile

// DecodeLUTBlock decodes lookup-table block idx (its raw stride bytes) into
// absolute file offsets (FORMAT.md §3).
func (a Array) DecodeLUTBlock(b []byte, idx uint64) ([]uint64, error) {
	entries := a.NumChunks() + 1
	start := idx * LUTBlockEntries
	if start >= entries {
		return nil, errCorrupt
	}
	first, pos, err := readPrefixVarint(b)
	if err != nil {
		return nil, err
	}
	out := make([]uint64, min(LUTBlockEntries, entries-start))
	out[0] = first
	deltas := make([]uint64, len(out)-1)
	if len(deltas) == 0 {
		return out, nil
	}
	if _, err := decodeBlock(b[pos:], deltas, len(deltas), false); err != nil {
		return nil, err
	}
	for i, d := range deltas {
		out[i+1] = out[i] + d
	}
	return out, nil
}

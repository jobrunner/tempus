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

// Plan says which bytes to fetch to read one series along the last dimension.
type Plan struct {
	// FirstChunk and LastChunk are the inclusive linear chunk indices.
	FirstChunk, LastChunk uint64
	// FirstLUTBlock and LastLUTBlock are the inclusive LUT blocks holding the
	// offsets FirstChunk … LastChunk+1.
	FirstLUTBlock, LastLUTBlock uint64

	array  Array
	coord  []uint64
	t0, t1 uint64
}

// PlanSeries plans reading the values at the leading coordinate coord (one
// index per dimension except the last) for last-dimension indices [t0, t1).
// Because chunks are laid out in C order, the time chunks of one location are
// adjacent and the whole series is one contiguous byte range.
func (a Array) PlanSeries(coord []uint64, t0, t1 uint64) (Plan, error) {
	last := len(a.Dims) - 1
	if len(coord) != last || t0 >= t1 || t1 > a.Dims[last] {
		return Plan{}, errCorrupt
	}
	grid := a.chunkGrid()
	var base uint64
	for d, c := range coord {
		if c >= a.Dims[d] {
			return Plan{}, errCorrupt
		}
		base = base*grid[d] + c/a.Chunks[d]
	}
	base *= grid[last]
	p := Plan{
		FirstChunk: base + t0/a.Chunks[last],
		LastChunk:  base + (t1-1)/a.Chunks[last],
		array:      a, coord: append([]uint64(nil), coord...), t0: t0, t1: t1,
	}
	p.FirstLUTBlock = p.FirstChunk / LUTBlockEntries
	p.LastLUTBlock = (p.LastChunk + 1) / LUTBlockEntries
	return p, nil
}

// LUTRange is the byte range [off, off+n) of the plan's LUT blocks.
func (p Plan) LUTRange() (off, n uint64) {
	s := p.array.LUTStride()
	return p.array.LUTOffset + p.FirstLUTBlock*s, (p.LastLUTBlock - p.FirstLUTBlock + 1) * s
}

// Offsets decodes the plan's LUT bytes into the chunk start offsets
// FirstChunk … LastChunk+1.
func (p Plan) Offsets(lut []byte) ([]uint64, error) {
	s := p.array.LUTStride()
	var all []uint64
	for blk := p.FirstLUTBlock; blk <= p.LastLUTBlock; blk++ {
		lo := (blk - p.FirstLUTBlock) * s
		if uint64(len(lut)) < lo+s {
			return nil, errTruncated
		}
		offs, err := p.array.DecodeLUTBlock(lut[lo:lo+s], blk)
		if err != nil {
			return nil, err
		}
		all = append(all, offs...)
	}
	from := p.FirstChunk - p.FirstLUTBlock*LUTBlockEntries
	to := p.LastChunk + 1 - p.FirstLUTBlock*LUTBlockEntries
	if to >= uint64(len(all)) {
		return nil, errCorrupt
	}
	return all[from : to+1], nil
}

// Series decodes the series from data, the bytes [offsets[0], offsets[last]).
func (p Plan) Series(offsets []uint64, data []byte) ([]float32, error) {
	a := p.array
	last := len(a.Dims) - 1
	ct := a.Chunks[last]
	out := make([]float32, 0, p.t1-p.t0)
	shape := make([]uint64, len(a.Dims))
	var buf []uint16
	for i := range p.LastChunk - p.FirstChunk + 1 {
		lo, hi := offsets[i]-offsets[0], offsets[i+1]-offsets[0]
		if hi < lo || hi > uint64(len(data)) {
			return nil, errCorrupt
		}
		tc := p.t0/ct + i
		row := uint64(0)
		total := uint64(1)
		for d := range shape {
			c := tc
			if d < last {
				c = p.coord[d] / a.Chunks[d]
			}
			shape[d] = min(a.Chunks[d], a.Dims[d]-c*a.Chunks[d])
			total *= shape[d]
			if d < last {
				row = row*shape[d] + p.coord[d] - c*a.Chunks[d]
			}
		}
		if uint64(cap(buf)) < total {
			buf = make([]uint16, total)
		}
		vals := buf[:total]
		if err := DecodeChunk(data[lo:hi], shape, vals); err != nil {
			return nil, err
		}
		width := shape[last]
		from := max(p.t0, tc*ct) - tc*ct
		to := min(p.t1, tc*ct+width) - tc*ct
		for _, v := range vals[row*width+from : row*width+to] {
			out = append(out, a.Float(v))
		}
	}
	return out, nil
}

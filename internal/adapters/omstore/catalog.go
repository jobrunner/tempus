package omstore

import (
	"strconv"
	"strings"
	"time"
)

// hoursPerChunkFile is the time span of one chunk_N.om file: N is the number of
// hours since the Unix epoch divided by this length.
const hoursPerChunkFile = 504

// object is one file of a variable directory in the bucket.
type object struct {
	Key  string
	ETag string
	Size uint64
}

// catalog lists the files of one variable of one domain.
type catalog struct {
	years   map[int]object
	chunks  map[int64]object
	fetched time.Time
	// seq numbers the listing this catalog came from (Store.beginListing).
	seq uint64
}

func newCatalog(objs []object, fetched time.Time) *catalog {
	c := &catalog{years: map[int]object{}, chunks: map[int64]object{}, fetched: fetched}
	for _, o := range objs {
		name := o.Key[strings.LastIndexByte(o.Key, '/')+1:]
		stem, ok := strings.CutSuffix(name, ".om")
		if !ok {
			continue
		}
		if y, ok := strings.CutPrefix(stem, "year_"); ok {
			if n, err := strconv.Atoi(y); err == nil {
				c.years[n] = o
			}
		} else if ch, ok := strings.CutPrefix(stem, "chunk_"); ok {
			if n, err := strconv.ParseInt(ch, 10, 64); err == nil {
				c.chunks[n] = o
			}
		}
	}
	return c
}

// segment is a run of hours served by one file.
type segment struct {
	obj object
	// t0, t1 are indices along the file's time dimension.
	t0, t1 uint64
	// out is the index of t0 in the requested series.
	out int
}

// resolve maps the epoch hours [from, to) onto files. Year files take
// precedence over chunk files; a variable that has year files ignores chunk
// files older than its first year (stale leftovers, e.g. IFS chunk_817).
// Hours without a file are simply not covered (NaN for the caller).
func (c *catalog) resolve(from, to int64) []segment {
	firstYear := 0
	for y := range c.years {
		if firstYear == 0 || y < firstYear {
			firstYear = y
		}
	}
	var segs []segment
	// Every step covers at least one hour, so to−from steps always suffice;
	// the bound turns any regression into missing data instead of an endless
	// loop that grows segs without limit.
	for h, step := from, int64(0); h < to && step < to-from; step++ {
		t := time.Unix(h*3600, 0).UTC()
		if obj, ok := c.years[t.Year()]; ok {
			start := yearStartHour(t.Year())
			end := min(to, yearStartHour(t.Year()+1))
			segs = append(segs, segment{obj: obj, t0: index(h - start), t1: index(end - start), out: int(h - from)})
			h = end
			continue
		}
		n := floorDiv(h, hoursPerChunkFile)
		end := min(to, (n+1)*hoursPerChunkFile)
		if _, ok := c.years[t.Year()+1]; ok {
			end = min(end, yearStartHour(t.Year()+1))
		}
		if obj, ok := c.chunks[n]; ok && (firstYear == 0 || t.Year() >= firstYear) {
			base := n * hoursPerChunkFile
			segs = append(segs, segment{obj: obj, t0: index(h - base), t1: index(end - base), out: int(h - from)})
		}
		h = end
	}
	return segs
}

// floorDiv divides rounding towards negative infinity, so hours before the
// epoch land in the chunk that actually contains them.
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// index converts a non-negative hour offset into a file's time index.
func index(h int64) uint64 {
	if h < 0 {
		return 0
	}
	return uint64(h)
}

func yearStartHour(y int) int64 {
	return time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC).Unix() / 3600
}

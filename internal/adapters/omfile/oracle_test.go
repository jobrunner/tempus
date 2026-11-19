package omfile

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readSeries runs the full read path (trailer → array → plan → LUT → data)
// against an in-memory file, the way a range-fetching caller would.
func readSeries(t *testing.T, file []byte, coord []uint64, t0, t1 uint64) []float32 {
	t.Helper()
	a := parseFile(t, file)
	p, err := a.PlanSeries(coord, t0, t1)
	if err != nil {
		t.Fatalf("PlanSeries(%v, %d, %d): %v", coord, t0, t1, err)
	}
	off, n := p.LUTRange()
	offs, err := p.Offsets(file[off : off+n])
	if err != nil {
		t.Fatalf("Offsets: %v", err)
	}
	vals, err := p.Series(offs, file[offs[0]:offs[len(offs)-1]])
	if err != nil {
		t.Fatalf("Series(%v): %v", coord, err)
	}
	return vals
}

func parseFile(t *testing.T, file []byte) Array {
	t.Helper()
	off, size, err := ParseTrailer(file)
	if err != nil {
		t.Fatalf("ParseTrailer: %v", err)
	}
	a, err := ParseArray(file[off : off+size])
	if err != nil {
		t.Fatalf("ParseArray: %v", err)
	}
	return a
}

func readF32(t *testing.T, path string) []float32 {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]float32, len(raw)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
	}
	return out
}

func sameBits(a, b float32) bool {
	if math.IsNaN(float64(a)) || math.IsNaN(float64(b)) {
		return math.IsNaN(float64(a)) && math.IsNaN(float64(b))
	}
	return math.Float32bits(a) == math.Float32bits(b)
}

// TestOracleFixtures decodes every fixture written by the black-box oracle
// (scripts/om-conformance/fixtures.py) and requires bit-identical values for
// every location, over the full series and over ranges cutting through chunks.
func TestOracleFixtures(t *testing.T) {
	files, err := filepath.Glob("testdata/oracle/*.om")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	for _, f := range files {
		t.Run(strings.TrimSuffix(filepath.Base(f), ".om"), func(t *testing.T) { checkFixture(t, f) })
	}
}

func checkFixture(t *testing.T, path string) {
	t.Helper()
	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := readF32(t, strings.TrimSuffix(path, ".om")+".f32")
	a := parseFile(t, file)
	last := len(a.Dims) - 1
	nt := a.Dims[last]
	ranges := [][2]uint64{{0, nt}, {nt / 3, nt - nt/4}, {nt - 1, nt}}
	coord := make([]uint64, last)
	for row := uint64(0); ; row++ {
		for _, r := range ranges {
			if r[0] < r[1] {
				compareSeries(t, coord, r[0], readSeries(t, file, coord, r[0], r[1]), want[row*nt+r[0]:])
			}
		}
		if !next(coord, a.Dims[:last]) {
			return
		}
	}
}

func compareSeries(t *testing.T, coord []uint64, t0 uint64, got, want []float32) {
	t.Helper()
	for i, v := range got {
		if !sameBits(v, want[i]) {
			t.Fatalf("coord %v t=%d: got %v want %v", coord, t0+uint64(i), v, want[i])
		}
	}
}

// next advances coord like an odometer over dims; false when it wraps.
func next(coord, dims []uint64) bool {
	for d := len(coord) - 1; d >= 0; d-- {
		coord[d]++
		if coord[d] < dims[d] {
			return true
		}
		coord[d] = 0
	}
	return false
}

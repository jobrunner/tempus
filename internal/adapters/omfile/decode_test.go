package omfile

import (
	"errors"
	"math"
	"os"
	"testing"
)

func TestReadPrefixVarint(t *testing.T) {
	tests := []struct {
		in   []byte
		want uint64
		n    int
	}{
		{[]byte{0x05}, 5, 1},
		{[]byte{0x83, 0xe8}, 1000, 2},
		{[]byte{0xc0, 0x20, 0x4e}, 20000, 3},
		{[]byte{0xe8, 0xe5, 0xca, 0x12}, 0x08e5ca12, 4},
		{[]byte{0xf3, 0xf5, 0xad, 0x6e, 0xdc}, 3<<32 | 0xdc6eadf5, 5},
	}
	for _, tt := range tests {
		got, n, err := readPrefixVarint(tt.in)
		if err != nil || got != tt.want || n != tt.n {
			t.Errorf("readPrefixVarint(% x) = %d, %d, %v; want %d, %d", tt.in, got, n, err, tt.want, tt.n)
		}
		if _, _, err := readPrefixVarint(tt.in[:len(tt.in)-1]); !errors.Is(err, errTruncated) {
			t.Errorf("truncated % x: err = %v", tt.in, err)
		}
	}
	if _, _, err := readPrefixVarint([]byte{0xf8, 0, 0, 0, 0, 0}); !errors.Is(err, errUnknownVarint) {
		t.Errorf("unknown prefix: err = %v", err)
	}
}

func TestReadExceptionVarint(t *testing.T) {
	tests := []struct {
		in   []byte
		want uint64
	}{
		{[]byte{0xb0}, 176},
		{[]byte{0xb1, 0x00}, 177},
		{[]byte{0xf0, 0xff}, 16560},
		{[]byte{0xf1, 0x84, 0xbe}, 65333},
		{[]byte{0xff, 0xb1, 0x40}, 16561},
	}
	for _, tt := range tests {
		got, n, err := readExceptionVarint(tt.in)
		if err != nil || got != tt.want || n != len(tt.in) {
			t.Errorf("readExceptionVarint(% x) = %d, %d, %v; want %d", tt.in, got, n, err, tt.want)
		}
		if _, _, err := readExceptionVarint(tt.in[:len(tt.in)-1]); !errors.Is(err, errTruncated) {
			t.Errorf("truncated % x: err = %v", tt.in, err)
		}
	}
	if _, _, err := readExceptionVarint([]byte{0xf9, 0, 0}); !errors.Is(err, errUnknownVarint) {
		t.Errorf("unknown prefix: err = %v", err)
	}
}

func TestParseRejectsUnsupportedAndBroken(t *testing.T) {
	file, err := os.ReadFile("testdata/oracle/two_rows.om")
	if err != nil {
		t.Fatal(err)
	}
	off, size, err := ParseTrailer(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ParseTrailer(file[:10]); !errors.Is(err, errTruncated) {
		t.Errorf("short trailer: %v", err)
	}
	bad := append([]byte(nil), file...)
	bad[len(bad)-TrailerSize+2] = 2
	if _, _, err := ParseTrailer(bad); !errors.Is(err, ErrUnsupported) {
		t.Errorf("version 2: %v", err)
	}
	bad[len(bad)-TrailerSize] = 'X'
	if _, _, err := ParseTrailer(bad); !errors.Is(err, errCorrupt) {
		t.Errorf("bad magic: %v", err)
	}
	v := file[off : off+size]
	if _, err := ParseArray(v[:20]); !errors.Is(err, errTruncated) {
		t.Errorf("short variable: %v", err)
	}
	if _, err := ParseArray(v[:len(v)-6]); !errors.Is(err, errTruncated) {
		t.Errorf("cut name: %v", err)
	}
	other := append([]byte(nil), v...)
	other[1] = 3
	if _, err := ParseArray(other); !errors.Is(err, ErrUnsupported) {
		t.Errorf("other codec: %v", err)
	}
	a, err := ParseArray(v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.PlanSeries([]uint64{2}, 0, 1); !errors.Is(err, errCorrupt) {
		t.Errorf("coord out of range: %v", err)
	}
	if _, err := a.PlanSeries([]uint64{0}, 3, 3); !errors.Is(err, errCorrupt) {
		t.Errorf("empty range: %v", err)
	}
	if !math.IsNaN(float64(a.Float(nanMarker))) {
		t.Error("NaN marker not decoded as NaN")
	}
}

func TestDecodeChunkRejectsTruncatedInput(t *testing.T) {
	file, err := os.ReadFile("testdata/oracle/weather_like_20.om")
	if err != nil {
		t.Fatal(err)
	}
	a := parseFile(t, file)
	p, err := a.PlanSeries([]uint64{0, 0}, 0, 504)
	if err != nil {
		t.Fatal(err)
	}
	off, n := p.LUTRange()
	offs, err := p.Offsets(file[off : off+n])
	if err != nil {
		t.Fatal(err)
	}
	chunk := file[offs[0]:offs[1]]
	dst := make([]uint16, 6*504)
	for cut := range len(chunk) - 1 {
		if err := DecodeChunk(chunk[:cut], []uint64{1, 6, 504}, dst); err == nil {
			t.Fatalf("cut at %d of %d: no error", cut, len(chunk))
		}
	}
	if _, err := p.Offsets(file[off : off+n-1]); !errors.Is(err, errTruncated) {
		t.Errorf("short LUT: %v", err)
	}
}

// FuzzDecodeChunk only asks that arbitrary bytes never panic or overrun dst.
func FuzzDecodeChunk(f *testing.F) {
	for _, s := range [][]byte{{0}, {0x05, 0x04, 0x9a, 0x0a}, {0xc0, 0xff, 0x7f, 0x40, 0x21, 0xf1, 0x84, 0xbe}} {
		f.Add(s, uint8(3), uint8(200))
	}
	f.Fuzz(func(_ *testing.T, b []byte, rows, cols uint8) {
		shape := []uint64{uint64(rows%8) + 1, uint64(cols) + 1}
		dst := make([]uint16, shape[0]*shape[1])
		_ = DecodeChunk(b, shape, dst)
	})
}

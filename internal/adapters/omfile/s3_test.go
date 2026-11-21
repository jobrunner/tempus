package omfile

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"testing"
)

// The replay test runs the reader against byte ranges recorded from the public
// Open-Meteo bucket and compares with values the black-box oracle read from the
// same files (testdata/s3/cases.json, scripts/om-conformance/s3_fixture_cases.py).
// TEMPUS_OM_RECORD=1 re-records testdata/s3/ranges.json from the live bucket.

const (
	s3BaseURL     = "https://openmeteo.s3.amazonaws.com/data/"
	casesFile     = "testdata/s3/cases.json"
	rangesFile    = "testdata/s3/ranges.json"
	tailReadBytes = 4096
)

type s3Case struct {
	Path  string   `json:"path"`
	Coord []uint64 `json:"coord"`
	T0    uint64   `json:"t0"`
	T1    uint64   `json:"t1"`
	Want  string   `json:"want"`
}

type recordedFile struct {
	Size   uint64          `json:"size"`
	Ranges []recordedRange `json:"ranges"`
}

type recordedRange struct {
	Off  uint64 `json:"off"`
	Data string `json:"data"`
}

// rangeSource serves byte ranges either live (recording them) or from a recording.
type rangeSource struct {
	record bool
	files  map[string]*recordedFile
}

func (s *rangeSource) size(t *testing.T, path string) uint64 {
	t.Helper()
	if !s.record {
		return s.files[path].Size
	}
	resp, err := http.Head(s3BaseURL + path)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	n, err := strconv.ParseUint(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if s.files[path] == nil {
		s.files[path] = &recordedFile{Size: n}
	}
	return n
}

func (s *rangeSource) read(t *testing.T, path string, off, n uint64) []byte {
	t.Helper()
	f := s.files[path]
	if !s.record {
		for _, r := range f.Ranges {
			raw, _ := base64.StdEncoding.DecodeString(r.Data)
			if r.Off == off && uint64(len(raw)) == n {
				return raw
			}
		}
		t.Fatalf("%s: range %d+%d was not recorded", path, off, n)
	}
	req, _ := http.NewRequest(http.MethodGet, s3BaseURL+path, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+n-1))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil || uint64(len(raw)) != n {
		t.Fatalf("%s: range %d+%d: %v (%d bytes)", path, off, n, err, len(raw))
	}
	f.Ranges = append(f.Ranges, recordedRange{Off: off, Data: base64.StdEncoding.EncodeToString(raw)})
	return raw
}

func TestRealFilesReplay(t *testing.T) {
	casesPath, rangesPath := envOr("TEMPUS_OM_CASES", casesFile), envOr("TEMPUS_OM_RANGES", rangesFile)
	var cases []s3Case
	loadJSON(t, casesPath, &cases)
	src := &rangeSource{record: os.Getenv("TEMPUS_OM_RECORD") == "1", files: map[string]*recordedFile{}}
	if !src.record {
		loadJSON(t, rangesPath, &src.files)
	}
	for _, c := range cases {
		t.Run(c.Path, func(t *testing.T) { replayCase(t, src, c) })
	}
	if src.record {
		out, _ := json.MarshalIndent(src.files, "", " ")
		if err := os.WriteFile(rangesPath, out, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func loadJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

// replayCase reads one series the way the store will: the file tail (trailer
// and variable), the LUT blocks of the plan, then one contiguous data range.
func replayCase(t *testing.T, src *rangeSource, c s3Case) {
	t.Helper()
	size := src.size(t, c.Path)
	tailLen := min(size, tailReadBytes)
	tail := src.read(t, c.Path, size-tailLen, tailLen)
	off, n, err := ParseTrailer(tail)
	if err != nil {
		t.Fatal(err)
	}
	rel := off - (size - tailLen)
	a, err := ParseArray(tail[rel : rel+n])
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.PlanSeries(c.Coord, c.T0, c.T1)
	if err != nil {
		t.Fatal(err)
	}
	lo, ln := p.LUTRange()
	offs, err := p.Offsets(src.read(t, c.Path, lo, ln))
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Series(offs, src.read(t, c.Path, offs[0], offs[len(offs)-1]-offs[0]))
	if err != nil {
		t.Fatal(err)
	}
	wantRaw, _ := base64.StdEncoding.DecodeString(c.Want)
	if len(got)*4 != len(wantRaw) {
		t.Fatalf("got %d values, want %d", len(got), len(wantRaw)/4)
	}
	want := make([]float32, len(got))
	for i := range want {
		want[i] = math.Float32frombits(binary.LittleEndian.Uint32(wantRaw[4*i:]))
	}
	compareSeries(t, c.Coord, c.T0, got, want)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

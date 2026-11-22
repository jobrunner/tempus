package reanalysis

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/adapters/omhttp"
	"github.com/jobrunner/tempus/internal/adapters/omstore"
	"github.com/jobrunner/tempus/internal/domain"
	ra "github.com/jobrunner/tempus/internal/domain/reanalysis"
)

// The Open-Meteo archive API is the black-box oracle of the opt-in live
// tests: tempus must reproduce its best_match output for the same coordinate
// and elevation. Nothing here reads Open-Meteo code; only the API answers.

const (
	liveEnv    = "TEMPUS_OM_LIVE"
	archiveURL = "https://archive-api.open-meteo.com/v1/archive"
	bucketURL  = "https://openmeteo.s3.amazonaws.com"
	kindHourly = "hourly"
	kindDaily  = "daily"
	dayLayout  = "2006-01-02"
)

// skipUnlessLive skips a test that talks to the bucket and the archive API.
func skipUnlessLive(t *testing.T) {
	t.Helper()
	if os.Getenv(liveEnv) != "1" {
		t.Skip("set " + liveEnv + "=1 to compare against the live Open-Meteo archive API")
	}
}

// liveService wires a Service onto the real open-data bucket the way the
// composition root does: the same parallelism and the retrying transport, so
// a reset connection is retried as in production instead of failing the run.
func liveService() *Service {
	client := &http.Client{
		Timeout:   60 * time.Second,
		Transport: omhttp.NewTransport(omhttp.Options{RetryAttempts: 3}),
	}
	return NewService(omstore.New(omstore.Options{BaseURL: bucketURL, HTTPClient: client, MaxParallel: 32}))
}

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// archiveQuery builds the oracle request: best_match (the API default), land
// cell selection (also the default, stated so a changed default shows) and
// the elevation tempus is given, so both sides correct to the same height.
func archiveQuery(p domain.GeoPoint, from, to time.Time, kind string, vars []string) url.Values {
	return url.Values{
		"latitude": {fmt.Sprint(p.Lat)}, "longitude": {fmt.Sprint(p.Lon)}, "elevation": {fmt.Sprint(p.Elevation)},
		"start_date": {from.Format(dayLayout)}, "end_date": {to.Format(dayLayout)},
		"cell_selection": {"land"}, "timezone": {"UTC"}, kind: {strings.Join(vars, ",")},
	}
}

// archive queries the oracle for the days from … to (inclusive) and returns
// each variable's values; JSON null (missing) arrives as a nil pointer.
func archive(t *testing.T, c *http.Client, p domain.GeoPoint, from, to time.Time, kind string, vars []string) map[string][]*float64 {
	t.Helper()
	days := int(to.Sub(from)/(24*time.Hour)) + 1
	want := days
	if kind == kindHourly {
		want = ra.HoursPerDay * days
	}
	q := archiveURL + "?" + archiveQuery(p, from, to, kind, vars).Encode()
	for attempt := range 5 {
		if out, ok := fetchArchive(t, c, q, kind, vars, want); ok {
			return out
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
	}
	t.Fatalf("archive API unavailable: %s", q)
	return nil
}

// fetchArchive makes one oracle call; ok is false on a transport failure or a
// non-200 answer (worth a retry). A well-formed answer of the wrong shape is
// fatal, not retried.
func fetchArchive(t *testing.T, c *http.Client, q, kind string, vars []string, want int) (map[string][]*float64, bool) {
	t.Helper()
	resp, err := c.Get(q)
	if err != nil {
		t.Logf("oracle: %v", err)
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Logf("oracle: HTTP %d", resp.StatusCode)
		return nil, false
	}
	var body map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Logf("oracle: %v", err)
		return nil, false
	}
	var block map[string]json.RawMessage
	if err := json.Unmarshal(body[kind], &block); err != nil {
		t.Fatalf("oracle: no %s block: %v", kind, err)
	}
	out := make(map[string][]*float64, len(vars))
	for _, v := range vars {
		var s []*float64
		if err := json.Unmarshal(block[v], &s); err != nil || len(s) != want {
			t.Fatalf("oracle %s %s: %d values (want %d), err %v", kind, v, len(s), want, err)
		}
		out[v] = s
	}
	return out, true
}

// tally counts, per variable and per group (e.g. time window), how many
// values equal the oracle after its rounding, and how many of the misses are
// one step in the last printed decimal.
type tally struct {
	t        *testing.T
	vars     map[string]*count
	groups   map[string]*count
	misses   int
	maxShown int
}

type count struct{ equal, lastDigit, total int }

func newTally(t *testing.T) *tally {
	return &tally{t: t, vars: map[string]*count{}, groups: map[string]*count{}, maxShown: 20}
}

func (c *tally) bucket(m map[string]*count, k string) *count {
	if m[k] == nil {
		m[k] = &count{}
	}
	return m[k]
}

// compare rounds tempus's values to the oracle's decimals and records each
// hour (or day). The first misses are logged with their position.
func (c *tally) compare(label, group, variable string, mine []float64, api []*float64, decimals int) {
	if len(mine) != len(api) {
		c.t.Errorf("%s %s: tempus %d values, oracle %d", label, variable, len(mine), len(api))
		return
	}
	for i, a := range api {
		v := ra.Round(float32(mine[i]), decimals)
		cv, cg := c.bucket(c.vars, variable), c.bucket(c.groups, group)
		cv.total++
		cg.total++
		if same(a, v) {
			cv.equal++
			cg.equal++
			continue
		}
		if lastDigit(a, v, decimals) {
			cv.lastDigit++
			cg.lastDigit++
		}
		c.misses++
		if c.misses <= c.maxShown {
			c.t.Logf("MISS %s %s #%d: tempus %v oracle %v", label, variable, i, v, deref(a))
		}
	}
}

// report logs the counts and fails every variable whose equal share is below
// minShare.
func (c *tally) report(minShare float64) {
	for _, part := range []struct {
		title string
		m     map[string]*count
	}{{"variable", c.vars}, {"window", c.groups}} {
		names := make([]string, 0, len(part.m))
		for k := range part.m {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			s := part.m[k]
			c.t.Logf("%-8s %-24s %6d/%-6d equal (%.3f %%), misses ±1 last decimal %d, larger %d",
				part.title, k, s.equal, s.total, 100*float64(s.equal)/float64(max(s.total, 1)),
				s.lastDigit, s.total-s.equal-s.lastDigit)
		}
	}
	for k, s := range c.vars {
		if float64(s.equal) < minShare*float64(s.total) {
			c.t.Errorf("%s: only %d of %d equal", k, s.equal, s.total)
		}
	}
}

// same compares in float32: the API prints the float32 result of its own
// rounding, which parses back to the same float32.
func same(api *float64, mine float32) bool {
	if api == nil {
		return math.IsNaN(float64(mine))
	}
	return float32(*api) == mine
}

// lastDigit reports a miss of exactly one step in the last printed decimal.
func lastDigit(api *float64, mine float32, decimals int) bool {
	if api == nil || math.IsNaN(float64(mine)) {
		return false
	}
	step := math.Pow10(-decimals)
	d := math.Abs(*api - float64(mine))
	return d > step/2 && d < 1.5*step
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

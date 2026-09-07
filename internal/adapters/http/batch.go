package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
)

// BatchLimits bounds the batch endpoint; zero values fall back to the
// defaults below (wired from config in the composition root).
type BatchLimits struct {
	MaxPoints     int
	MaxSyncPoints int
}

const (
	defaultBatchMaxPoints     = 10000
	defaultBatchMaxSyncPoints = 1000
	// batchBytesPerPoint sizes the request-body cap: points are small JSON
	// objects; 512 B each plus fixed headroom is generous.
	batchBytesPerPoint = 512
	batchBodyHeadroom  = 64 * 1024
)

func (l BatchLimits) maxPoints() int {
	if l.MaxPoints > 0 {
		return l.MaxPoints
	}
	return defaultBatchMaxPoints
}

func (l BatchLimits) maxSyncPoints() int {
	if l.MaxSyncPoints > 0 {
		return l.MaxSyncPoints
	}
	return defaultBatchMaxSyncPoints
}

// batchRequest mirrors the single-query parameters plus the points array.
type batchRequest struct {
	Providers []string     `json:"providers"`
	GDDBase   *float64     `json:"gddBase"`
	RefPeriod string       `json:"refPeriod"`
	Points    []batchPoint `json:"points"`
}

type batchPoint struct {
	ID       string   `json:"id"`
	Lat      *float64 `json:"lat"`
	Lon      *float64 `json:"lon"`
	Datetime string   `json:"datetime"`
}

func (s *Server) handleQueryBatch(w http.ResponseWriter, r *http.Request) {
	maxPts := s.batchLimits.maxPoints()
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxPts)*batchBytesPerPoint+batchBodyHeadroom)

	var breq batchRequest
	if err := json.NewDecoder(r.Body).Decode(&breq); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			s.writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		s.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(breq.Points) == 0 {
		s.writeError(w, http.StatusBadRequest, "points must not be empty")
		return
	}
	if len(breq.Points) > maxPts {
		s.writeError(w, http.StatusBadRequest,
			fmt.Sprintf("too many points: %d > %d", len(breq.Points), maxPts))
		return
	}
	stream := prefersNDJSON(r)
	if !stream && len(breq.Points) > s.batchLimits.maxSyncPoints() {
		s.writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf(
			"more than %d points require streaming; retry with Accept: application/x-ndjson",
			s.batchLimits.maxSyncPoints()))
		return
	}

	points := resolveBatchPoints(breq)
	if stream {
		s.streamBatchNDJSON(w, r, points)
		return
	}
	s.writeBatchSync(w, r, points)
}

// resolveBatchPoints validates every point through the exact single-query
// parser; invalid points become error items and never reach the query path.
func resolveBatchPoints(breq batchRequest) []input.BatchPoint {
	gdd := ""
	if breq.GDDBase != nil {
		gdd = strconv.FormatFloat(*breq.GDDBase, 'f', -1, 64)
	}
	out := make([]input.BatchPoint, 0, len(breq.Points))
	for i, p := range breq.Points {
		id := p.ID
		if id == "" {
			id = strconv.Itoa(i)
		}
		lat, lon := "", ""
		if p.Lat != nil {
			lat = strconv.FormatFloat(*p.Lat, 'f', -1, 64)
		}
		if p.Lon != nil {
			lon = strconv.FormatFloat(*p.Lon, 'f', -1, 64)
		}
		req, err := domain.ParseQueryRequest(lat, lon, p.Datetime, gdd, breq.Providers)
		if err != nil {
			out = append(out, input.BatchPoint{ID: id, ParseError: err.Error()})
			continue
		}
		req.RefPeriod = breq.RefPeriod
		out = append(out, input.BatchPoint{ID: id, Req: &req})
	}
	return out
}

// prefersNDJSON reports whether the client asked for the NDJSON stream.
func prefersNDJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/x-ndjson")
}

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jobrunner/tempus/internal/ports/input"
)

// batchEnvelope is the synchronous batch response (ortus-conformant shape).
type batchEnvelope struct {
	Results          []input.BatchItem `json:"results"`
	Total            int               `json:"total"`
	ProcessingTimeMS int64             `json:"processing_time_ms"`
}

func (s *Server) writeBatchSync(w http.ResponseWriter, r *http.Request, points []input.BatchPoint) {
	start := s.clock.Now()
	results := make([]input.BatchItem, 0, len(points))
	err := s.batch.QueryBatch(r.Context(), points, func(it input.BatchItem) error {
		results = append(results, it)
		return nil
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.logger.Debug("batch query canceled by client")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "batch query failed")
		return
	}
	s.writeJSON(w, http.StatusOK, batchEnvelope{
		Results:          results,
		Total:            len(results),
		ProcessingTimeMS: s.clock.Now().Sub(start).Milliseconds(),
	})
}

// streamBatchNDJSON writes one result item per line in input order, flushing
// per line so clients can render progress. There is no trailing envelope; a
// client detects a broken stream by comparing line count to points sent.
func (s *Server) streamBatchNDJSON(w http.ResponseWriter, r *http.Request, points []input.BatchPoint) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	rc := http.NewResponseController(w)
	// Lift any server-wide WriteTimeout for this response: a batch stream can
	// legitimately run long, and a blanket deadline must not kill it mid-flight.
	// Not every ResponseWriter supports this (e.g. in tests); ignore the error.
	_ = rc.SetWriteDeadline(time.Time{})
	w.WriteHeader(http.StatusOK)
	// Flush the header immediately so the client sees a response before the
	// first (possibly slow) item arrives.
	_ = rc.Flush()
	enc := json.NewEncoder(w)
	err := s.batch.QueryBatch(r.Context(), points, func(it input.BatchItem) error {
		if err := enc.Encode(it); err != nil { // Encode appends the newline
			return err
		}
		return rc.Flush()
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		// Headers are sent; all we can do is log and abort the stream.
		s.logger.Warn("batch stream aborted", "error", err)
	}
}

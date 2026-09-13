package httpapi

import (
	"fmt"
	"net/http"
	"time"

	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// The router-level middleware chain (tracing→trace-id→logging→recovery) lives
// here so server.go stays about routing and lifecycle. CORS is deliberately
// NOT among these: it has to wrap the router from the outside to see preflight
// requests at all — see cors.go.

func (s *Server) traceIDHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
			w.Header().Set("X-Trace-Id", sc.TraceID().String())
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		fields := []any{
			"method", r.Method, "path", r.URL.Path,
			keyStatus, wrapped.statusCode, "duration", time.Since(start),
		}
		if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
			fields = append(fields, "trace_id", sc.TraceID().String())
		}
		s.logger.Info("request", fields...)
	})
}

func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
					span.RecordError(fmt.Errorf("panic: %v", err), trace.WithStackTrace(true))
					span.SetStatus(otelcodes.Error, "panic recovered")
				}
				s.logger.Error("panic recovered", "error", err, "path", r.URL.Path)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseWriter captures the status code for the logging middleware.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController (used for streaming Flush, e.g. by
// handleQueryBatch's NDJSON writer) reach the underlying ResponseWriter's
// Flush/Hijack support through this logging wrapper.
func (rw *responseWriter) Unwrap() http.ResponseWriter { return rw.ResponseWriter }

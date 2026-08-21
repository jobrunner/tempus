package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
)

// failingFeatures reports a query error, which the stub in server_test.go cannot.
type failingFeatures struct{ err error }

func (f failingFeatures) Query(context.Context, domain.QueryRequest) (domain.QueryResult, error) {
	return domain.QueryResult{}, f.err
}

type notReadyHealth struct{}

func (notReadyHealth) Ready(context.Context) bool { return false }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func serverWith(features input.FeatureService, health input.HealthChecker, opts Options) *Server {
	return NewServer("127.0.0.1:0", features, stubProviders{}, health, fixedClock{}, discardLogger(), opts)
}

func TestHealthEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		health     input.HealthChecker
		wantStatus int
		wantBody   string
	}{
		{"health", "/health", stubHealth{}, http.StatusOK, "ok"},
		{"live", "/health/live", stubHealth{}, http.StatusOK, "ok"},
		{"ready", "/health/ready", stubHealth{}, http.StatusOK, "ok"},
		// A readiness probe that reports not-ready must fail closed, so an
		// orchestrator stops sending traffic instead of assuming health.
		{"ready_reports_not_ready", "/health/ready", notReadyHealth{}, http.StatusServiceUnavailable, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := serverWith(stubFeatures{}, tc.health, Options{})
			rr := httptest.NewRecorder()
			srv.Router().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, tc.path, nil))

			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tc.wantStatus)
			}
			if got := rr.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			var body map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if tc.wantBody != "" && body["status"] != tc.wantBody {
				t.Errorf("status field = %q, want %q", body["status"], tc.wantBody)
			}
			if tc.wantStatus != http.StatusOK {
				// The uniform error envelope, the one documented in openapi.yaml.
				if body["error"] == "" || body["message"] == "" {
					t.Errorf("error envelope incomplete: %v", body)
				}
			}
		})
	}
}

func TestHandleQuery_InvalidRequestIsBadRequest(t *testing.T) {
	srv := serverWith(stubFeatures{}, stubHealth{}, Options{})
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/query?lat=999&lon=9.93", nil))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["message"] == nil || body["message"] == "" {
		t.Errorf("want a message explaining the validation failure, got %v", body)
	}
}

func TestHandleQuery_ProviderErrorIsInternalError(t *testing.T) {
	srv := serverWith(failingFeatures{errors.New("boom")}, stubHealth{}, Options{})
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr,
		httptest.NewRequest(http.MethodGet, "/api/v1/query?lat=49.79&lon=9.93&datetime=2025-06-15T13:00:00Z", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

// A client that disconnects mid-query is not an error to report: the handler
// returns without writing anything, so the recorder keeps its default 200 and
// an empty body rather than a 500 nobody is listening for.
func TestHandleQuery_ClientCancellationWritesNothing(t *testing.T) {
	srv := serverWith(failingFeatures{context.Canceled}, stubHealth{}, Options{})
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr,
		httptest.NewRequest(http.MethodGet, "/api/v1/query?lat=49.79&lon=9.93&datetime=2025-06-15T13:00:00Z", nil))

	if rr.Body.Len() != 0 {
		t.Errorf("body = %q, want empty for a cancelled client", rr.Body.String())
	}
	// "Writes nothing" has to mean the status line and headers too: a handler
	// could call WriteHeader(500) and write no body, which an empty-body
	// assertion alone would wave through. The recorder's default 200 with no
	// Content-Type is what an untouched ResponseWriter looks like.
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want the recorder's untouched default 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "" {
		t.Errorf("Content-Type = %q, want unset for a cancelled client", ct)
	}
}

// With a tracer provider wired, otelmux creates a span and the middleware
// surfaces its trace ID, which is how a caller correlates a response with a
// trace. Without tracing the header must stay absent.
func TestTraceIDHeader(t *testing.T) {
	t.Run("present when tracing is enabled", func(t *testing.T) {
		tp := sdktrace.NewTracerProvider()
		t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
		srv := serverWith(stubFeatures{}, stubHealth{}, Options{TracerProvider: tp})

		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil))

		if got := rr.Header().Get("X-Trace-Id"); got == "" {
			t.Error("X-Trace-Id header missing although tracing is enabled")
		}
	})

	t.Run("absent when tracing is disabled", func(t *testing.T) {
		srv := serverWith(stubFeatures{}, stubHealth{}, Options{})
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil))

		if got := rr.Header().Get("X-Trace-Id"); got != "" {
			t.Errorf("X-Trace-Id = %q, want absent without tracing", got)
		}
	})
}

// A panicking handler must not take the process down, and the client must get a
// 500 rather than a dropped connection.
func TestRecoveryMiddleware_TurnsPanicIntoInternalError(t *testing.T) {
	srv := serverWith(stubFeatures{}, stubHealth{}, Options{})
	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("handler exploded")
	})

	rr := httptest.NewRecorder()
	srv.recoveryMiddleware(panicking).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/query", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

// Start blocks until Shutdown, and reports the shutdown as ErrServerClosed —
// the signal Run relies on to tell a clean stop from a real failure.
func TestStartAndShutdown(t *testing.T) {
	srv := serverWith(stubFeatures{}, stubHealth{}, Options{})

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	// No synchronisation with the listener coming up is needed: whether Shutdown
	// wins the race or the listener does, ListenAndServe reports ErrServerClosed
	// either way, which is exactly the property under test.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Start returned %v, want http.ErrServerClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Start did not return after Shutdown")
	}
}

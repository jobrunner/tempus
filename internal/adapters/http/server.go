package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// keyStatus is the JSON key used in health/status response envelopes.
const keyStatus = "status"

// Server wraps the HTTP server and its router. It holds only driving ports.
type Server struct {
	server         *http.Server
	router         *mux.Router
	features       input.FeatureService
	providers      input.ProviderLister
	clock          output.Clock
	health         input.HealthChecker
	logger         *slog.Logger
	serviceName    string
	tracerProvider trace.TracerProvider // may be nil (tracing disabled)
	frontendPage   []byte
}

// Options carries optional dependencies (tracing, service name, …).
type Options struct {
	TracerProvider trace.TracerProvider
	ServiceName    string
	// Version is substituted into the frontend footer (e.g. from -ldflags). When
	// empty, "dev" is shown.
	Version string
}

// NewServer builds the server, wires routes, and prepares the http.Server.
func NewServer(addr string, features input.FeatureService, providers input.ProviderLister, health input.HealthChecker, clock output.Clock, logger *slog.Logger, opts Options) *Server {
	name := opts.ServiceName
	if name == "" {
		name = "tempus"
	}
	version := opts.Version
	if version == "" {
		version = "dev"
	}
	s := &Server{
		features:       features,
		providers:      providers,
		clock:          clock,
		health:         health,
		logger:         logger,
		serviceName:    name,
		tracerProvider: opts.TracerProvider,
		frontendPage:   renderFrontend(version),
	}
	s.router = s.setupRoutes()
	s.server = &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

// setupRoutes registers every route. Keep it flat and greppable: the contract
// test walks exactly what you register here.
func (s *Server) setupRoutes() *mux.Router {
	r := mux.NewRouter()

	// Tracing first so later middleware/handlers see the span context. otelmux
	// uses the matched route template as span name (low cardinality).
	if s.tracerProvider != nil {
		r.Use(otelmux.Middleware(s.serviceName, otelmux.WithTracerProvider(s.tracerProvider)))
		r.Use(s.traceIDHeaderMiddleware)
	}
	r.Use(s.loggingMiddleware)
	r.Use(s.recoveryMiddleware)

	// Health/probe endpoints — never rate-limited, intentionally NOT in the
	// OpenAPI business contract (the contract test skips /health*).
	r.HandleFunc("/health", s.handleHealth).Methods(http.MethodGet)
	r.HandleFunc("/health/live", s.handleLiveness).Methods(http.MethodGet)
	r.HandleFunc("/health/ready", s.handleReadiness).Methods(http.MethodGet)

	// Versioned business surface. Every route under here MUST be documented in
	// openapi.yaml (enforced by TestRoutesMatchOpenAPISpec).
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/query", s.handleQuery).Methods(http.MethodGet)
	api.HandleFunc("/providers", s.handleProviders).Methods(http.MethodGet)

	// OpenAPI spec and Swagger UI — root-level, NOT under /api/v1 (not in the
	// business contract, so the contract test ignores them).
	r.HandleFunc("/openapi.json", s.handleOpenAPI).Methods(http.MethodGet)
	r.HandleFunc("/docs", s.handleSwaggerUI).Methods(http.MethodGet)

	// Frontend — serves the web UI at the root. Matches only GET / exactly;
	// does not shadow /api/v1/*, /health*, /openapi.json, or /docs.
	r.HandleFunc("/", s.handleIndex).Methods(http.MethodGet)

	return r
}

// Router exposes the router so tests (and the contract fitness function) can
// walk the registered routes.
func (s *Server) Router() *mux.Router { return s.router }

// Start / Shutdown manage the lifecycle (called by the composition root).
func (s *Server) Start() error { return s.server.ListenAndServe() }

func (s *Server) Shutdown(ctx context.Context) error { return s.server.Shutdown(ctx) }

// --- handlers ----------------------------------------------------------------

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var providerFilter []string
	if p := q.Get("providers"); p != "" {
		providerFilter = strings.Split(p, ",")
	}
	req, err := domain.ParseQueryRequest(
		q.Get("lat"), q.Get("lon"), q.Get("datetime"), q.Get("timezone"), providerFilter, s.clock.Now(),
	)
	if err != nil {
		var ve domain.ValidationError
		if errors.As(err, &ve) {
			s.writeError(w, http.StatusBadRequest, ve.Error())
			return
		}
		s.writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	result, err := s.features.Query(r.Context(), req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.logger.Debug("query canceled by client")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{"providers": s.providers.Providers(r.Context())})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{keyStatus: "ok"})
}

func (s *Server) handleLiveness(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{keyStatus: "ok"})
}

func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if !s.health.Ready(r.Context()) {
		s.writeError(w, http.StatusServiceUnavailable, "not ready")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{keyStatus: "ok"})
}

// --- response envelope -------------------------------------------------------

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes the single, uniform error envelope every handler uses.
// Documenting THIS shape once (in openapi.yaml components/schemas/Error) keeps
// the spec honest across all endpoints.
func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]any{
		"error":   http.StatusText(status),
		"message": message,
	})
}

// --- middleware --------------------------------------------------------------

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

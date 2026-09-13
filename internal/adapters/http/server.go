package httpapi

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel/trace"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// keyStatus is the JSON key used in health/status response envelopes.
const keyStatus = "status"

// Server wraps the HTTP server and its router. It holds only driving ports.
type Server struct {
	server *http.Server
	router *mux.Router
	// handler is what the server actually serves: the router, wrapped in CORS
	// when origins are configured. Router() still exposes the bare router for
	// route walking; Handler() is what tests and the composition root serve.
	handler            http.Handler
	corsAllowedOrigins []string
	features           input.FeatureService
	batch              input.BatchService
	batchLimits        BatchLimits
	providers          input.ProviderLister
	clock              output.Clock
	health             input.HealthChecker
	logger             *slog.Logger
	serviceName        string
	tracerProvider     trace.TracerProvider // may be nil (tracing disabled)
	frontendPage       []byte
}

// Options carries optional dependencies (tracing, service name, …).
type Options struct {
	TracerProvider trace.TracerProvider
	ServiceName    string
	// Version is substituted into the frontend footer (e.g. from -ldflags). When
	// empty, "dev" is shown.
	Version string
	// Batch bounds POST /api/v1/query/batch; zero fields fall back to defaults.
	Batch BatchLimits
	// CORSAllowedOrigins enables cross-origin access for the listed browser
	// origins (exact or "*.example.com"). Empty disables CORS entirely, which
	// is the default — the bundled frontend is served from the same origin.
	CORSAllowedOrigins []string
	// ReadTimeout bounds reading the whole request (headers plus body) and is
	// plumbed through from config: a key that is declared and documented but
	// never read is worse than a missing knob, because an operator who sets it
	// gets silence instead of an error. Zero means no limit.
	ReadTimeout time.Duration
}

// NewServer builds the server, wires routes, and prepares the http.Server.
func NewServer(addr string, features input.FeatureService, batch input.BatchService, providers input.ProviderLister, health input.HealthChecker, clock output.Clock, logger *slog.Logger, opts Options) *Server {
	name := cmp.Or(opts.ServiceName, "tempus")
	version := cmp.Or(opts.Version, "dev")
	s := &Server{
		features:           features,
		batch:              batch,
		batchLimits:        opts.Batch,
		providers:          providers,
		clock:              clock,
		health:             health,
		logger:             logger,
		serviceName:        name,
		tracerProvider:     opts.TracerProvider,
		frontendPage:       renderFrontend(version),
		corsAllowedOrigins: opts.CORSAllowedOrigins,
	}
	s.router = s.setupRoutes()
	s.handler = s.wrapCORS(s.router)
	// No blanket WriteTimeout here: batch NDJSON streaming can legitimately run
	// long, and a server-wide write deadline would kill it mid-stream. The
	// batch handler lifts any per-connection write deadline itself for its
	// response (see streamBatchNDJSON in batch_render.go).
	s.server = &http.Server{
		Addr:              addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       opts.ReadTimeout,
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
	api.HandleFunc("/query/batch", s.handleQueryBatch).Methods(http.MethodPost)
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

// Handler is what the server serves: the router plus any outer middleware that
// must see requests the router would not match (currently CORS preflights).
// Serve this — not Router() — or cross-origin behaviour silently disappears.
func (s *Server) Handler() http.Handler { return s.handler }

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
		q.Get("lat"), q.Get("lon"), q.Get("datetime"), q.Get("gddBase"), providerFilter,
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
	req.RefPeriod = q.Get("refPeriod")
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

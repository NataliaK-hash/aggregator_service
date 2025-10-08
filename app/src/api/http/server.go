package httpapi

import (
	"net/http"

	chi "aggregator-service/app/src/api/chi"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
)

// Server exposes the HTTP transport for the aggregator application.
type Server struct {
	handler http.Handler
}

// NewServer constructs an HTTP server that forwards requests to the application service.
func NewServer(service domain.AggregatorService, logger *infra.Logger) *Server {
	router := chi.NewRouter()
	handler := &handler{service: service, logger: logger}
	registerRoutes(router, handler)

	router.Use(infra.HTTPMiddleware(func(r *http.Request) string {
		if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
			if pattern := routeCtx.RoutePattern(); pattern != "" {
				return pattern
			}
		}
		return r.URL.Path
	}))

	return &Server{handler: router}
}

// Router returns the configured HTTP handler for reuse in tests or external HTTP servers.
func (s *Server) Router() http.Handler {
	return s.handler
}

// ServeHTTP allows Server to satisfy the http.Handler interface directly.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

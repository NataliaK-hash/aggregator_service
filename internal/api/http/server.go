package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"aggregator-service-project/internal/domain"
)

// Server exposes the HTTP transport for the aggregator application.
type Server struct {
	router chi.Router
}

// NewServer constructs a chi based HTTP server that forwards requests to the application service.
func NewServer(service domain.AggregatorService) *Server {
	router := chi.NewRouter()
	handler := &handler{service: service}
	registerRoutes(router, handler)

	return &Server{router: router}
}

// Router returns the configured chi router for reuse in tests or external HTTP servers.
func (s *Server) Router() http.Handler {
	return s.router
}

// ServeHTTP allows Server to satisfy the http.Handler interface directly.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

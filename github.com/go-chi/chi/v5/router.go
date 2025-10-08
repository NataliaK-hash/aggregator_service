package chi

import "net/http"

type route struct {
	method  string
	pattern string
	handler http.Handler
}

// Router defines the minimal methods used by the project.
type Router interface {
	http.Handler
	Get(pattern string, handler http.HandlerFunc)
}

// Mux is a lightweight HTTP multiplexer compatible with the chi Router interface used in this project.
type Mux struct {
	routes []route
}

// NewRouter constructs a new Router instance.
func NewRouter() Router {
	return &Mux{}
}

// ServeHTTP matches the incoming request by method and path.
func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, rt := range m.routes {
		if r.Method == rt.method && r.URL.Path == rt.pattern {
			rt.handler.ServeHTTP(w, r)
			return
		}
	}

	http.NotFound(w, r)
}

// Get registers a handler for the GET method and provided pattern.
func (m *Mux) Get(pattern string, handler http.HandlerFunc) {
	m.routes = append(m.routes, route{method: http.MethodGet, pattern: pattern, handler: handler})
}

package chi

import (
	"context"
	"net/http"
	"sort"
	"strings"
)

type contextKey struct{}

var routeCtxKey = contextKey{}

type RoutingContext struct {
	routePattern string
}

func (rc *RoutingContext) RoutePattern() string {
	if rc == nil {
		return ""
	}
	return rc.routePattern
}

func RouteContext(ctx context.Context) *RoutingContext {
	if ctx == nil {
		return nil
	}
	if rc, ok := ctx.Value(routeCtxKey).(*RoutingContext); ok {
		return rc
	}
	return nil
}

type Mux struct {
	middlewares      []func(http.Handler) http.Handler
	routes           map[string]map[string]http.Handler
	notFound         http.Handler
	methodNotAllowed http.Handler
}

func NewRouter() *Mux {
	return &Mux{
		routes: make(map[string]map[string]http.Handler),
	}
}

func (m *Mux) Use(middlewares ...func(http.Handler) http.Handler) {
	if len(middlewares) == 0 {
		return
	}
	m.middlewares = append(m.middlewares, middlewares...)
}

func (m *Mux) Get(pattern string, handler http.HandlerFunc) {
	m.Method(http.MethodGet, pattern, handler)
}

func (m *Mux) Method(method, pattern string, handler http.HandlerFunc) {
	if method == "" || pattern == "" || handler == nil {
		return
	}
	method = strings.ToUpper(method)
	if m.routes[method] == nil {
		m.routes[method] = make(map[string]http.Handler)
	}
	m.routes[method][pattern] = handler
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r == nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	method := r.Method
	path := r.URL.Path

	if handlers, ok := m.routes[method]; ok {
		if handler, ok := handlers[path]; ok {
			m.serveWithMiddlewares(path, handler, w, r)
			return
		}
	}

	if allowed := m.allowedMethods(path); allowed != "" {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m.methodNotAllowed != nil {
				m.methodNotAllowed.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Allow", allowed)
			w.WriteHeader(http.StatusMethodNotAllowed)
		})
		m.serveWithMiddlewares(path, handler, w, r)
		return
	}

	if m.notFound != nil {
		m.serveWithMiddlewares(path, m.notFound, w, r)
		return
	}
	m.serveWithMiddlewares(path, http.HandlerFunc(http.NotFound), w, r)
}

func (m *Mux) serveWithMiddlewares(pattern string, handler http.Handler, w http.ResponseWriter, r *http.Request) {
	rc := &RoutingContext{routePattern: pattern}
	ctx := context.WithValue(r.Context(), routeCtxKey, rc)
	req := r.WithContext(ctx)

	var final http.Handler = handler
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		if mw := m.middlewares[i]; mw != nil {
			final = mw(final)
		}
	}
	final.ServeHTTP(w, req)
}

func (m *Mux) NotFound(handler http.HandlerFunc) {
	m.notFound = handler
}

func (m *Mux) MethodNotAllowed(handler http.HandlerFunc) {
	m.methodNotAllowed = handler
}

func (m *Mux) allowedMethods(path string) string {
	var methods []string
	for method, handlers := range m.routes {
		if _, ok := handlers[path]; ok {
			methods = append(methods, method)
		}
	}
	if len(methods) == 0 {
		return ""
	}
	sort.Strings(methods)
	return strings.Join(methods, ", ")
}

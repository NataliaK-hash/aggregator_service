package prometheus

import "sync"

// Collector represents a Prometheus collector. In this stub it is an empty interface.
type Collector interface{}

// Desc represents a Prometheus metric descriptor placeholder.
type Desc struct{}

// Metric represents a Prometheus metric placeholder.
type Metric interface{}

// Registerer matches the Prometheus registerer interface.
type Registerer interface {
	Register(Collector) error
}

// AlreadyRegisteredError is returned when attempting to register the same collector twice.
type AlreadyRegisteredError struct {
	ExistingCollector Collector
}

func (e AlreadyRegisteredError) Error() string {
	return "collector already registered"
}

// registry is a simple in-memory implementation of Registerer used in tests.
type registry struct {
	mu         sync.Mutex
	collectors map[Collector]struct{}
}

// NewRegistry creates a new registry instance.
func NewRegistry() Registerer {
	return &registry{collectors: make(map[Collector]struct{})}
}

// Register adds the collector to the registry if it hasn't been registered yet.
func (r *registry) Register(c Collector) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.collectors == nil {
		r.collectors = make(map[Collector]struct{})
	}
	if _, exists := r.collectors[c]; exists {
		return AlreadyRegisteredError{ExistingCollector: c}
	}
	r.collectors[c] = struct{}{}
	return nil
}

// defaultRegistry is used when no explicit registerer is provided.
var defaultRegistry Registerer = NewRegistry()

// DefaultRegisterer is the package-level registerer used by default.
var DefaultRegisterer Registerer = defaultRegistry

package grpc_prometheus

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

// ServerMetrics is a lightweight stub that provides interceptors compatible with the real library.
type ServerMetrics struct {
	registerer prometheus.Registerer
}

// NewServerMetrics creates a new ServerMetrics instance.
func NewServerMetrics() *ServerMetrics {
	return &ServerMetrics{}
}

// UnaryServerInterceptor returns a passthrough unary interceptor.
func (m *ServerMetrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}
}

// StreamServerInterceptor returns a passthrough stream interceptor.
func (m *ServerMetrics) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, stream)
	}
}

// InitializeMetrics is a no-op in the stub implementation.
func (m *ServerMetrics) InitializeMetrics(*grpc.Server) {}

// Describe implements the prometheus.Collector interface in a minimal fashion.
func (m *ServerMetrics) Describe(chan<- *prometheus.Desc) {}

// Collect implements the prometheus.Collector interface in a minimal fashion.
func (m *ServerMetrics) Collect(chan<- prometheus.Metric) {}

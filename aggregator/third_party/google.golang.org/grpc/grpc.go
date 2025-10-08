package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
)

// ErrServerStopped is returned by Serve after the server has been stopped.
var ErrServerStopped = errors.New("grpc: the server has been stopped")

// ServerOption configures a stub gRPC server.
type ServerOption interface {
	apply(*Server)
}

type serverOption func(*Server)

func (o serverOption) apply(s *Server) {
	if o != nil {
		o(s)
	}
}

// ChainUnaryInterceptor aggregates unary interceptors into server options.
func ChainUnaryInterceptor(interceptors ...UnaryServerInterceptor) ServerOption {
	return serverOption(func(s *Server) {
		s.unaryInterceptors = append(s.unaryInterceptors, interceptors...)
	})
}

// ChainStreamInterceptor aggregates stream interceptors into server options.
func ChainStreamInterceptor(interceptors ...StreamServerInterceptor) ServerOption {
	return serverOption(func(s *Server) {
		s.streamInterceptors = append(s.streamInterceptors, interceptors...)
	})
}

// Server represents a very small in-memory stub of gRPC server used for compilation and basic coordination in tests.
type Server struct {
	unaryInterceptors  []UnaryServerInterceptor
	streamInterceptors []StreamServerInterceptor

	mu       sync.RWMutex
	stopOnce sync.Once
	stopCh   chan struct{}

	services []registeredService
}

type registeredService struct {
	desc *ServiceDesc
	impl interface{}
}

// NewServer creates a new stub server with the provided options applied.
func NewServer(opts ...ServerOption) *Server {
	s := &Server{stopCh: make(chan struct{})}
	for _, opt := range opts {
		if opt != nil {
			opt.apply(s)
		}
	}
	return s
}

type serverRegistrar interface {
	RegisterServer(*Server)
}

// Serve blocks until the server is stopped. For the in-process stub it simply notifies bufconn listener about the server.
func (s *Server) Serve(l net.Listener) error {
	if registrar, ok := l.(serverRegistrar); ok {
		registrar.RegisterServer(s)
	}
	<-s.stopCh
	return ErrServerStopped
}

// GracefulStop requests graceful shutdown of the server.
func (s *Server) GracefulStop() {
	s.Stop()
}

// Stop immediately stops the server.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

// UnaryHandler defines the handler invoked by unary interceptors in the stub implementation.
type UnaryHandler func(context.Context, interface{}) (interface{}, error)

// UnaryServerInfo mirrors the corresponding structure from the real gRPC package.
type UnaryServerInfo struct {
	Server     interface{}
	FullMethod string
}

// UnaryServerInterceptor represents a unary interceptor signature.
type UnaryServerInterceptor func(context.Context, interface{}, *UnaryServerInfo, UnaryHandler) (interface{}, error)

// StreamServerInfo mirrors stream metadata for interceptors.
type StreamServerInfo struct {
	Server         interface{}
	FullMethod     string
	IsClientStream bool
	IsServerStream bool
}

// StreamHandler represents a streaming handler function.
type StreamHandler func(interface{}, ServerStream) error

// StreamServerInterceptor represents a stream interceptor signature.
type StreamServerInterceptor func(interface{}, ServerStream, *StreamServerInfo, StreamHandler) error

// ServerStream is the minimal interface required by stream interceptors.
type ServerStream interface {
	Context() context.Context
}

// ServiceRegistrar allows registration of services on the server.
type ServiceRegistrar interface {
	RegisterService(*ServiceDesc, interface{})
}

// RegisterService stores the service descriptor for potential introspection in tests.
func (s *Server) RegisterService(desc *ServiceDesc, impl interface{}) {
	if desc == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services = append(s.services, registeredService{desc: desc, impl: impl})
}

// ServiceDesc describes an RPC service.
type ServiceDesc struct {
	ServiceName string
	HandlerType interface{}
	Methods     []MethodDesc
	Streams     []StreamDesc
	Metadata    interface{}
}

// MethodDesc describes a single unary RPC method.
type MethodDesc struct {
	MethodName string
	Handler    methodHandler
}

type methodHandler func(interface{}, context.Context, func(interface{}) error, UnaryServerInterceptor) (interface{}, error)

// StreamDesc describes a streaming RPC method.
type StreamDesc struct {
	StreamName    string
	Handler       StreamHandler
	ServerStreams bool
	ClientStreams bool
}

// DialOption configures client connections in the stub implementation.
type DialOption interface {
	apply(*dialOptions)
}

type dialOption func(*dialOptions)

func (o dialOption) apply(opts *dialOptions) {
	if o != nil {
		o(opts)
	}
}

type dialOptions struct {
	server *Server
}

// WithInProcessServer configures the dialer to use the provided in-process server.
func WithInProcessServer(server *Server) DialOption {
	return dialOption(func(opts *dialOptions) {
		opts.server = server
	})
}

// CallOption is a placeholder for real gRPC call options.
type CallOption interface{}

// ClientConnInterface matches the subset of the real gRPC interface required by generated code.
type ClientConnInterface interface {
	Invoke(context.Context, string, interface{}, interface{}, ...CallOption) error
}

// ClientConn represents an in-process client connection.
type ClientConn struct {
	server *Server
}

// DialContext creates a client connection backed by the provided in-process server.
func DialContext(_ context.Context, _ string, opts ...DialOption) (*ClientConn, error) {
	cfg := dialOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt.apply(&cfg)
		}
	}
	if cfg.server == nil {
		return nil, errors.New("grpc: dial requires WithInProcessServer option")
	}
	return &ClientConn{server: cfg.server}, nil
}

// Invoke executes a unary RPC against the in-process server.
func (cc *ClientConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, _ ...CallOption) error {
	if cc == nil || cc.server == nil {
		return errors.New("grpc: client connection is not initialized")
	}

	resp, err := cc.server.invokeUnary(ctx, method, args)
	if err != nil {
		return err
	}
	if reply == nil || resp == nil {
		return nil
	}

	rv := reflect.ValueOf(reply)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("grpc: reply must be a non-nil pointer")
	}

	respValue := reflect.ValueOf(resp)
	if respValue.Kind() == reflect.Ptr {
		if respValue.IsNil() {
			rv.Elem().Set(reflect.Zero(rv.Elem().Type()))
			return nil
		}
		if !respValue.Type().AssignableTo(rv.Type()) {
			return fmt.Errorf("grpc: cannot assign response of type %T to %T", resp, reply)
		}
		rv.Elem().Set(respValue.Elem())
		return nil
	}
	if !respValue.Type().AssignableTo(rv.Elem().Type()) {
		return fmt.Errorf("grpc: cannot assign response of type %T to %T", resp, reply)
	}
	rv.Elem().Set(respValue)
	return nil
}

// Close releases resources associated with the client connection.
func (cc *ClientConn) Close() error {
	return nil
}

func (s *Server) invokeUnary(ctx context.Context, fullMethod string, req interface{}) (interface{}, error) {
	s.mu.RLock()
	services := append([]registeredService(nil), s.services...)
	s.mu.RUnlock()

	for _, svc := range services {
		prefix := "/" + svc.desc.ServiceName + "/"
		if !strings.HasPrefix(fullMethod, prefix) {
			continue
		}
		methodName := strings.TrimPrefix(fullMethod, prefix)
		for _, method := range svc.desc.Methods {
			if method.MethodName != methodName {
				continue
			}

			decode := func(target interface{}) error {
				if target == nil || req == nil {
					return nil
				}
				targetValue := reflect.ValueOf(target)
				if targetValue.Kind() != reflect.Ptr || targetValue.IsNil() {
					return errors.New("grpc: decode target must be a pointer")
				}
				reqValue := reflect.ValueOf(req)
				if reqValue.Kind() != reflect.Ptr || reqValue.IsNil() {
					return nil
				}
				if !reqValue.Type().AssignableTo(targetValue.Type()) {
					return fmt.Errorf("grpc: cannot assign %T to %T", req, target)
				}
				targetValue.Elem().Set(reqValue.Elem())
				return nil
			}

			interceptor := s.chainUnaryInterceptors()
			return method.Handler(svc.impl, ctx, decode, interceptor)
		}
	}

	return nil, fmt.Errorf("grpc: method %s not found", fullMethod)
}

func (s *Server) chainUnaryInterceptors() UnaryServerInterceptor {
	if len(s.unaryInterceptors) == 0 {
		return nil
	}

	interceptors := append([]UnaryServerInterceptor(nil), s.unaryInterceptors...)
	return func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		chained := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			current := interceptors[i]
			next := chained
			chained = func(currentCtx context.Context, currentReq interface{}) (interface{}, error) {
				return current(currentCtx, currentReq, info, next)
			}
		}
		return chained(ctx, req)
	}
}

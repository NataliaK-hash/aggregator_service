package bufconn

import (
	"context"
	"errors"
	"net"
	"sync"

	"google.golang.org/grpc"
)

// Listener provides an in-memory listener compatible with the stub gRPC implementation.
type Listener struct {
	mu     sync.Mutex
	server *grpc.Server
	closed bool
	ready  chan struct{}
}

// Listen creates a new buffer-based listener. The size argument is ignored in the stub implementation.
func Listen(_ int) *Listener {
	return &Listener{ready: make(chan struct{})}
}

// RegisterServer attaches the given gRPC server to this listener.
func (l *Listener) RegisterServer(server *grpc.Server) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.server = server
	select {
	case <-l.ready:
	default:
		close(l.ready)
	}
}

// DialContext creates a client connection to the in-process server.
func (l *Listener) DialContext(ctx context.Context, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	l.mu.Lock()
	server := l.server
	ready := l.ready
	l.mu.Unlock()
	if server == nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ready:
			l.mu.Lock()
			server = l.server
			l.mu.Unlock()
			if server == nil {
				return nil, errors.New("bufconn: server not registered")
			}
		}
	}
	opts = append(opts, grpc.WithInProcessServer(server))
	return grpc.DialContext(ctx, "bufconn", opts...)
}

// Accept implements the net.Listener interface. It is not supported in the stub.
func (l *Listener) Accept() (net.Conn, error) {
	return nil, errors.New("bufconn: Accept is not supported")
}

// Close marks the listener as closed.
func (l *Listener) Close() error {
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
	return nil
}

// Addr returns a dummy network address for logging purposes.
func (l *Listener) Addr() net.Addr {
	return stubAddr("bufconn")
}

// stubAddr provides a trivial implementation of net.Addr.
type stubAddr string

func (a stubAddr) Network() string { return string(a) }

func (a stubAddr) String() string { return string(a) }

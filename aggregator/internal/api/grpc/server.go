package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"aggregator/internal/logging"
	"aggregator/pkg/api"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

const defaultShutdownTimeout = 10 * time.Second

// Options описывает параметры запуска gRPC сервера.
type Options struct {
	// Address — адрес, по которому должен слушать сервер (например, ":50051").
	Address string
	// ShutdownTimeout задаёт максимальное время корректного завершения активных соединений.
	ShutdownTimeout time.Duration
	// Registerer позволяет зарегистрировать метрики в пользовательском реестре Prometheus.
	// Если не задан, используется prometheus.DefaultRegisterer.
	Registerer prometheus.Registerer
}

// Server инкапсулирует gRPC сервер агрегатора и управляет его жизненным циклом.
type Server struct {
	address         string
	logger          *logging.Logger
	grpcServer      *grpc.Server
	listener        net.Listener
	shutdownTimeout time.Duration
}

type traceIDKey struct{}

// ContextWithTraceID добавляет traceId в контекст запроса, чтобы интерцепторы могли использовать его в логах.
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// NewServer создаёт новый экземпляр gRPC сервера с настроенными интерцепторами логирования и метрик.
func NewServer(logger *logging.Logger, service api.AggregatorServiceServer, opts Options) (*Server, error) {
	if service == nil {
		return nil, errors.New("aggregator service is required")
	}

	address := opts.Address
	if address == "" {
		return nil, errors.New("address is required")
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", address, err)
	}

	shutdownTimeout := opts.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}

	metrics := grpc_prometheus.NewServerMetrics()
	registerer := opts.Registerer
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}
	if registerer != nil {
		if err := registerer.Register(metrics); err != nil {
			if alreadyRegistered, ok := err.(prometheus.AlreadyRegisteredError); ok {
				if existing, ok := alreadyRegistered.ExistingCollector.(*grpc_prometheus.ServerMetrics); ok {
					metrics = existing
				} else {
					return nil, fmt.Errorf("register metrics: %w", err)
				}
			} else {
				return nil, fmt.Errorf("register metrics: %w", err)
			}
		}
	}

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingUnaryInterceptor(logger),
			metrics.UnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			loggingStreamInterceptor(logger),
			metrics.StreamServerInterceptor(),
		),
	)

	api.RegisterAggregatorServiceServer(server, service)
	metrics.InitializeMetrics(server)

	return &Server{
		address:         address,
		logger:          logger,
		grpcServer:      server,
		listener:        listener,
		shutdownTimeout: shutdownTimeout,
	}, nil
}

// Serve запускает gRPC сервер и ожидает завершения контекста для корректного остановки.
func (s *Server) Serve(ctx context.Context) error {
	if s == nil {
		return errors.New("server is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	defer s.listener.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.grpcServer.Serve(s.listener)
	}()

	if s.logger != nil {
		s.logger.Info("gRPC server started", "address", s.listener.Addr().String())
	}

	select {
	case <-ctx.Done():
		if s.logger != nil {
			s.logger.Info("gRPC server shutdown initiated")
		}
		shutdownErr := s.shutdown()
		serveErr := <-errCh
		if errors.Is(serveErr, grpc.ErrServerStopped) {
			serveErr = nil
		}
		if serveErr != nil && shutdownErr == nil {
			shutdownErr = serveErr
		}
		if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(ctxErr, context.Canceled) && shutdownErr == nil {
			shutdownErr = ctxErr
		}
		return shutdownErr
	case err := <-errCh:
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	}
}

func (s *Server) shutdown() error {
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		if s.logger != nil {
			s.logger.Info("gRPC server stopped gracefully")
		}
		return nil
	case <-time.After(s.shutdownTimeout):
		if s.logger != nil {
			s.logger.Warn("gRPC server graceful shutdown timed out, forcing stop", "timeout", s.shutdownTimeout.String())
		}
		s.grpcServer.Stop()
		return fmt.Errorf("graceful shutdown exceeded %s", s.shutdownTimeout)
	}
}

func loggingUnaryInterceptor(logger *logging.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		log := attachTrace(logger, ctx)
		if log != nil {
			ctx = log.WithContext(ctx)
		}

		resp, err := handler(ctx, req)

		if log != nil {
			fields := []any{"method", info.FullMethod, "duration", time.Since(start)}
			if err != nil {
				fields = logging.AttachError(err, fields...)
				log.Error("gRPC unary call completed", fields...)
			} else {
				log.Info("gRPC unary call completed", fields...)
			}
		}

		return resp, err
	}
}

func loggingStreamInterceptor(logger *logging.Logger) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		baseCtx := stream.Context()
		log := attachTrace(logger, baseCtx)
		if log != nil {
			baseCtx = log.WithContext(baseCtx)
		}

		wrapped := stream
		if baseCtx != stream.Context() {
			wrapped = &streamWithContext{ServerStream: stream, ctx: baseCtx}
		}

		start := time.Now()
		err := handler(srv, wrapped)

		if log != nil {
			fields := []any{"method", info.FullMethod, "duration", time.Since(start)}
			if err != nil {
				fields = logging.AttachError(err, fields...)
				log.Error("gRPC stream call completed", fields...)
			} else {
				log.Info("gRPC stream call completed", fields...)
			}
		}

		return err
	}
}

func attachTrace(logger *logging.Logger, ctx context.Context) *logging.Logger {
	if logger == nil {
		return nil
	}
	traceID := traceIDFromContext(ctx)
	if traceID == "" {
		return logger
	}
	return logger.WithTraceID(traceID)
}

func traceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(traceIDKey{}).(string); ok {
		return value
	}
	return ""
}

type streamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *streamWithContext) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	if s.ServerStream != nil {
		return s.ServerStream.Context()
	}
	return context.Background()
}

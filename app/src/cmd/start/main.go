package main

import (
	_ "aggregator-service/app/src/infra/utils/autoload"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	grpcapi "aggregator-service/app/src/api/grpc"
	httpapi "aggregator-service/app/src/api/http"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"google.golang.org/grpc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, cleanup, err := initApplication(ctx, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialise application: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	cfg := app.Config
	logger := app.Logger

	infra.LogConfig(ctx, logger, cfg)
	infra.StartMetricsServer(logger)

	service := app.Service
	generator := app.Generator
	workerPool := app.WorkerPool

	bufferSize := cfg.PacketBufferSize
	if bufferSize <= 0 {
		bufferSize = 100
	}
	packets := make(chan domain.DataPacket, bufferSize)

	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		generator.Run(ctx, packets)
	}()
	go func() {
		defer workers.Done()
		workerPool.Run(ctx, packets)
	}()

	httpServer := newHTTPServer(cfg.HTTPPort, service, logger)

	httpListener, err := net.Listen("tcp", httpServer.Addr)
	if err != nil {
		stop()
		workers.Wait()
		logger.Fatalf(ctx, "failed to listen on HTTP port %s: %v", cfg.HTTPPort, err)
	}

	grpcServer := grpcapi.NewServer(service, logger)
	grpcAddr := fmt.Sprintf(":%s", cfg.GRPCPort)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		stop()
		workers.Wait()
		logger.Fatalf(ctx, "failed to listen on gRPC port %s: %v", cfg.GRPCPort, err)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf(ctx, "HTTP server shutdown error: %v", err)
		}

		grpcServer.GracefulStop()
	}()

	serverErrs := make(chan error, 2)
	var serverGroup sync.WaitGroup

	serverGroup.Add(1)
	go func() {
		defer serverGroup.Done()
		logger.Printf(ctx, "HTTP server listening on %s", httpListener.Addr())
		if err := httpServer.Serve(httpListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrs <- fmt.Errorf("http server: %w", err)
		}
	}()

	serverGroup.Add(1)
	go func() {
		defer serverGroup.Done()
		logger.Printf(ctx, "gRPC server listening on %s", grpcListener.Addr())
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serverErrs <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	logger.Println(ctx, "metrics server listening on :2112")

	var serveErr error

	select {
	case <-ctx.Done():
	case err := <-serverErrs:
		if err != nil {
			serveErr = err
		}
		stop()
	}

	stop()
	workers.Wait()
	serverGroup.Wait()

	if serveErr != nil {
		logger.Printf(ctx, "server error: %v", serveErr)
	}

	logger.Println(ctx, "server stopped")
}

func newHTTPServer(port string, service domain.AggregatorService, logger *infra.Logger) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           httpapi.NewServer(service, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

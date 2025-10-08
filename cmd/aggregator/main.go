package main

import (
        "context"
        "errors"
        "fmt"
        _ "aggregator-service-project/internal/pkg/dotenv/autoload"
        "log"
        "net/http"
        "os"
        "os/signal"
        "sync"
        "syscall"
        "time"

	httpapi "aggregator-service-project/internal/api/http"
	"aggregator-service-project/internal/application/aggregator"
	appgenerator "aggregator-service-project/internal/application/generator"
	appworker "aggregator-service-project/internal/application/worker"
	"aggregator-service-project/internal/domain"
)

func main() {
	cfg := loadConfig()
	logger := log.New(os.Stdout, "aggregator-service ", log.LstdFlags|log.LUTC)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logConfig(logger, cfg)

	if shouldCheckDatabase(cfg) {
		if err := waitForDatabase(ctx, cfg, logger); err != nil {
			logger.Printf("database connectivity check failed: %v", err)
		} else {
			logger.Println("database connectivity check succeeded")
		}
	} else {
		logger.Println("database connectivity check skipped (no DSN or host/port configured)")
	}

	repo, cleanup, err := setupPostgresRepository(ctx, cfg, logger)
	if err != nil {
		logger.Fatalf("failed to initialise repository: %v", err)
	}
	defer cleanup()

	service := aggregator.New(repo)

	generatorCfg := appgenerator.Config{
		Interval:   time.Duration(cfg.GeneratorIntervalMillis) * time.Millisecond,
		PacketSize: cfg.MeasurementsPerPacket,
	}
	generator := appgenerator.New(generatorCfg, logger)
	workerPool := appworker.New(cfg.WorkerCount, repo, logger)

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

	httpServer := newHTTPServer(cfg.HTTPPort, service)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	logger.Printf("HTTP server listening on %s", httpServer.Addr)
	logger.Printf("gRPC endpoint configured on port %s (not yet implemented)", cfg.GRPCPort)

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		stop()
		workers.Wait()
		logger.Fatalf("HTTP server failed: %v", err)
	}

	stop()
	workers.Wait()

	logger.Println("server stopped")
}

func newHTTPServer(port string, service domain.AggregatorService) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           httpapi.NewServer(service),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

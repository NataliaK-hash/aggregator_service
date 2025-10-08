package app

import (
	"context"
	"errors"

	"aggregator/internal/config"
	"aggregator/internal/generator"
	"aggregator/internal/logging"
	"aggregator/internal/storage"
)

// App представляет приложение агрегатора.
type App struct {
	config          *config.Config
	logger          *logging.Logger
	shutdownManager *ShutdownManager
	source          generator.Source
	workerPool      *WorkerPool
}

// New создаёт новый экземпляр App.
func New(cfg *config.Config, logger *logging.Logger, shutdownManager *ShutdownManager, source generator.Source, workerPool *WorkerPool, repository storage.Repository) *App {
	return &App{config: cfg, logger: logger, shutdownManager: shutdownManager, source: source, workerPool: workerPool, repository: repository}
}

// Run запускает жизненный цикл приложения.
func (a *App) Run(ctx context.Context) error {
	if a.logger != nil {
		a.logger.Info("starting aggregator service",
			"httpPort", a.config.HttpPort,
			"grpcPort", a.config.GrpcPort,
			"workerPoolSize", a.config.WorkerPoolSize,
		)
	}

	runCtx, cancel := a.shutdownManager.WithContext(ctx)
	defer cancel()
	defer a.shutdownManager.Close()

	var (
		resultsDone       <-chan struct{}
		processingStarted bool
	)
	if a.source != nil && a.workerPool != nil {
		packets := a.source.Start(runCtx)
		a.workerPool.Start(runCtx, packets)
		resultsDone = a.consumeResults()
		processingStarted = true
	}

	<-runCtx.Done()

	if a.logger != nil {
		a.logger.Info("shutdown initiated")
	}

	cleanupCtx, cleanupCancel := a.shutdownManager.CleanupContext()
	defer cleanupCancel()

	var shutdownErr error
	if processingStarted {
		if err := a.workerPool.Shutdown(cleanupCtx); err != nil {
			shutdownErr = err
		}
	}

	if processingStarted && resultsDone != nil {
		if err := a.shutdownManager.WaitFor(cleanupCtx, resultsDone); err != nil {
			if shutdownErr == nil {
				shutdownErr = err
			}
		}
	}

	if a.logger != nil {
		switch {
		case errors.Is(shutdownErr, context.DeadlineExceeded):
			a.logger.Warn("shutdown deadline exceeded", "timeout", a.shutdownManager.timeout.String())
		case shutdownErr != nil:
			a.logger.Error("shutdown completed with error", "error", shutdownErr.Error())
		default:
			a.logger.Info("shutdown completed")
		}
	}

	return nil
}

// consumeResults запускает обработчик результатов, записывающий информацию о максимумах пакетов в лог.
func (a *App) consumeResults(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		for result := range a.workerPool.Results() {
			if a.logger != nil {
				a.logger.Debug("packet processed", "packetId", result.ID, "maxValue", result.MaxValue)
			}
		}
	}()

	return done
}

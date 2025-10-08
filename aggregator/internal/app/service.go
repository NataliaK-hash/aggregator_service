package app

import (
	"context"
	"errors"

	"aggregator/internal/config"
	"aggregator/internal/logging"
)

// App представляет приложение агрегатора.
type App struct {
	config          *config.Config
	logger          *logging.Logger
	shutdownManager *ShutdownManager
}

// New создаёт новый экземпляр App.
func New(cfg *config.Config, logger *logging.Logger, shutdownManager *ShutdownManager) *App {
	return &App{config: cfg, logger: logger, shutdownManager: shutdownManager}
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

	<-runCtx.Done()

	if a.logger != nil {
		a.logger.Info("shutdown initiated")
	}

	cleanupCtx, cleanupCancel := a.shutdownManager.CleanupContext()
	defer cleanupCancel()

	<-cleanupCtx.Done()

	if a.logger != nil {
		if errors.Is(cleanupCtx.Err(), context.DeadlineExceeded) {
			a.logger.Warn("shutdown deadline exceeded", "timeout", a.shutdownManager.timeout.String())
		} else {
			a.logger.Info("shutdown completed")
		}
	}

	return nil
}

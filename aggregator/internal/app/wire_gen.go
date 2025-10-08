//go:build !wireinject
// +build !wireinject

package app

import (
	"aggregator/internal/config"
	"aggregator/internal/generator"
	"aggregator/internal/logging"
	"aggregator/internal/storage"
	"time"
)

func InitializeApp() (*App, error) {
	config, err := provideConfig()
	if err != nil {
		return nil, err
	}
	logger, err := provideLogger(config)
	if err != nil {
		return nil, err
	}
	shutdownManager := provideShutdownManager(logger)
	generatorConfig := provideGeneratorConfig(config)
	source := generator.NewRandomSource(generatorConfig)
	workerPool := provideWorkerPool(config)
	repository, err := storage.ProvideRepository(config, logger)
	if err != nil {
		return nil, err
	}
	app := New(config, logger, shutdownManager, source, workerPool, repository)
	return app, nil
}

func provideConfig() (*config.Config, error) { return config.Load() }

func provideLogger(cfg *config.Config) (*logging.Logger, error) {
	return logging.New(cfg.LogLevel)
}

func provideShutdownManager(l *logging.Logger) *ShutdownManager {
	const shutdownTimeout = 30 * time.Second
	return NewShutdownManager(shutdownTimeout, l)
}

func provideWorkerPool(cfg *config.Config) *WorkerPool {
	return NewWorkerPool(cfg.WorkerPoolSize)
}

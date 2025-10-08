//go:build wireinject
// +build wireinject

package app

import (
	"time"

	"github.com/google/wire"

	"aggregator/internal/config"
	"aggregator/internal/generator"
	"aggregator/internal/logging"
)

func InitializeApp() (*App, error) {
	panic(wire.Build(
		provideConfig,
		provideLogger,
		provideGeneratorConfig,
		generator.ProviderSet,
		provideShutdownManager,
		New,
	))
}

func provideConfig() (*config.Config, error) { return config.Load() }

func provideLogger(cfg *config.Config) (*logging.Logger, error) {
	return logging.New(cfg.LogLevel)
}

func provideShutdownManager(l *logging.Logger) *ShutdownManager {
	const shutdownTimeout = 30 * time.Second
	return NewShutdownManager(shutdownTimeout, l)
}

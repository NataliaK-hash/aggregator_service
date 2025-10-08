//go:build wireinject
// +build wireinject

package app

import (
	"time"

	"github.com/google/wire"

	"aggregator/internal/config"
	"aggregator/internal/logging"
)

func InitializeApp() (*App, error) {
	panic(wire.Build(
		provideConfig,
		provideLogger,
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

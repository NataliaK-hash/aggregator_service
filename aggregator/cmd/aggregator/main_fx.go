//go:build fxexample
// +build fxexample

package main

import (
	"context"
	"time"

	"go.uber.org/fx"

	"aggregator/internal/app"
	"aggregator/internal/config"
	"aggregator/internal/generator"
	"aggregator/internal/logging"
	"aggregator/internal/storage"
)

func main() {
	fx.New(
		fx.Provide(
			config.Load,
			func(cfg *config.Config) (*logging.Logger, error) { return logging.New(cfg.LogLevel) },
			func(cfg *config.Config) generator.Source {
				return generator.NewRandomSource(generator.Config{
					PayloadLen: cfg.Generator.PayloadLen,
					Interval:   cfg.Generator.Interval,
					BufferSize: 1024,
				})
			},
			func(l *logging.Logger) *app.ShutdownManager { return app.NewShutdownManager(30*time.Second, l) },
			func(cfg *config.Config) *app.WorkerPool { return app.NewWorkerPool(cfg.WorkerPoolSize) },
			func(cfg *config.Config, l *logging.Logger, sm *app.ShutdownManager, src generator.Source, wp *app.WorkerPool) *app.App {
				return app.New(cfg, l, sm, src, wp)
			},
		),
		fx.Invoke(func(lc fx.Lifecycle, a *app.App) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error { go a.Run(ctx); return nil },
				OnStop:  func(ctx context.Context) error { return nil },
			})
		}),
	).Run()
}

//go:build fxexample
// +build fxexample

package main

import (
	"context"
	"time"

	"go.uber.org/fx"

	"aggregator/internal/app"
	"aggregator/internal/config"
	"aggregator/internal/logging"
)

func main() {
	fx.New(
		fx.Provide(
			config.Load,
			func(cfg *config.Config) (*logging.Logger, error) { return logging.New(cfg.LogLevel) },
			func(l *logging.Logger) *app.ShutdownManager { return app.NewShutdownManager(30*time.Second, l) },
			func(cfg *config.Config, l *logging.Logger, sm *app.ShutdownManager) *app.App {
				return app.New(cfg, l, sm)
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

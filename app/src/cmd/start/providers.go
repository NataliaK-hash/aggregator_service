package main

import (
	"context"
	"io"
	"time"

	"aggregator-service/app/src/core"
	dbpostgres "aggregator-service/app/src/database"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
)

func provideConfig() infra.Config {
	return infra.LoadConfig()
}

func provideServiceName() string {
	return "aggregator-service"
}

func provideLogger(out io.Writer, serviceName string) *infra.Logger {
	return infra.NewLogger(out, serviceName)
}

func provideGeneratorConfig(cfg infra.Config) core.GeneratorConfig {
	return core.GeneratorConfig{
		Interval:   time.Duration(cfg.GeneratorIntervalMillis) * time.Millisecond,
		PacketSize: cfg.MeasurementsPerPacket,
	}
}

func provideGenerator(cfg core.GeneratorConfig, logger *infra.Logger) domain.PacketGenerator {
	return core.NewGenerator(cfg, logger)
}

func provideWorkerPool(cfg infra.Config, repo domain.PacketMaxRepository, logger *infra.Logger) domain.WorkerPool {
	return core.NewWorkerPool(cfg.WorkerCount, repo, logger)
}

func provideAggregatorService(repo domain.PacketMaxRepository) domain.AggregatorService {
	return core.NewAggregator(repo)
}

func provideRepository(ctx context.Context, cfg infra.Config, logger *infra.Logger) (domain.PacketMaxRepository, func(), error) {
	if dbpostgres.ShouldCheckDatabase(cfg) {
		if err := dbpostgres.WaitForDatabase(ctx, cfg, logger); err != nil {
			if logger != nil {
				logger.Printf(ctx, "database connectivity check failed: %v", err)
			}
		} else {
			if logger != nil {
				logger.Println(ctx, "database connectivity check succeeded")
			}
		}
	} else {
		if logger != nil {
			logger.Println(ctx, "database connectivity check skipped (no DSN or host/port configured)")
		}
	}

	return dbpostgres.SetupRepository(ctx, cfg, logger)
}

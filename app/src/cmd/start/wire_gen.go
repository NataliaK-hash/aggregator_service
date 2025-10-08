package main

import (
	"context"
	"io"

	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
)

func initApplication(ctx context.Context, out io.Writer) (*application, func(), error) {
	cfg, logger := setupBase(out)
	repo, cleanup, err := setupRepository(ctx, cfg, logger)
	if err != nil {
		return nil, nil, err
	}

	gen := setupGenerator(cfg, logger)
	pool := setupWorkerPool(cfg, repo, logger)
	svc := provideAggregatorService(repo)

	app := newApplication(cfg, logger, svc, gen, pool)
	return assembleApplication(app, cleanup)
}

func setupBase(out io.Writer) (infra.Config, *infra.Logger) {
	cfg := provideConfig()
	svcName := provideServiceName()
	log := provideLogger(out, svcName)
	return cfg, log
}

func setupRepository(ctx context.Context, cfg infra.Config, logger *infra.Logger) (domain.PacketMaxRepository, func(), error) {
	return provideRepository(ctx, cfg, logger)
}

func setupGenerator(cfg infra.Config, logger *infra.Logger) domain.PacketGenerator {
	genCfg := provideGeneratorConfig(cfg)
	return provideGenerator(genCfg, logger)
}

func setupWorkerPool(cfg infra.Config, repo domain.PacketMaxRepository, logger *infra.Logger) domain.WorkerPool {
	return provideWorkerPool(cfg, repo, logger)
}

package main

import (
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
)

type application struct {
	Config     infra.Config
	Logger     *infra.Logger
	Service    domain.AggregatorService
	Generator  domain.PacketGenerator
	WorkerPool domain.WorkerPool
}

func newApplication(cfg infra.Config, logger *infra.Logger, service domain.AggregatorService, generator domain.PacketGenerator, workerPool domain.WorkerPool) *application {
	return &application{
		Config:     cfg,
		Logger:     logger,
		Service:    service,
		Generator:  generator,
		WorkerPool: workerPool,
	}
}

func assembleApplication(app *application, cleanup func()) (*application, func(), error) {
	if cleanup == nil {
		cleanup = func() {}
	}
	return app, cleanup, nil
}

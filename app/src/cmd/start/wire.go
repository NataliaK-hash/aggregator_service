//go:build wireinject

package main

import (
	"context"
	"io"
	"time"

	"aggregator-service/app/src/core"
	dbpostgres "aggregator-service/app/src/database"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"github.com/google/wire"
)

func initApplication(ctx context.Context, out io.Writer) (*application, func(), error) {
	wire.Build(
		provideConfig,
		provideServiceName,
		provideLogger,
		provideGeneratorConfig,
		provideGenerator,
		provideWorkerPool,
		provideAggregatorService,
		provideRepository,
		newApplication,
		assembleApplication,
	)
	return nil, nil, nil
}

package domain

import (
	"context"
	"time"
)

// MeasurementWriter persists measurements produced by the worker pool.
type MeasurementWriter interface {
	Add(ctx context.Context, measurement Measurement) error
}

// MeasurementReader exposes queries used by the aggregator application.
type MeasurementReader interface {
	MaxBySource(ctx context.Context, id string) (Measurement, error)
	MaxInRange(ctx context.Context, from, to time.Time) (Measurement, error)
}

// MeasurementRepository aggregates the write and read capabilities required by the service.
type MeasurementRepository interface {
	MeasurementWriter
	MeasurementReader
}

// AggregatorResult represents the output of aggregator use-cases.
type AggregatorResult struct {
	SourceID  string
	Value     float64
	Timestamp time.Time
}

// AggregatorService describes the behaviour exposed to transport layers.
type AggregatorService interface {
	MaxBySource(ctx context.Context, id string) (AggregatorResult, error)
	MaxInRange(ctx context.Context, from, to time.Time) (AggregatorResult, error)
}

// PacketGenerator produces data packets that will be processed by workers.
type PacketGenerator interface {
	Run(ctx context.Context, out chan<- DataPacket)
}

// WorkerPool consumes packets and stores their measurements via the repository.
type WorkerPool interface {
	Run(ctx context.Context, packets <-chan DataPacket)
}

package domain

import (
	"context"
	"time"
)

// PacketMaxWriter persists packet maxima produced by the worker pool.
type PacketMaxWriter interface {
	Add(ctx context.Context, packetMax PacketMax) error
}

// PacketMaxReader exposes queries used by the aggregator application.
type PacketMaxReader interface {
	PacketMaxByID(ctx context.Context, packetID string) (PacketMax, error)
	PacketMaxInRange(ctx context.Context, from, to time.Time) ([]PacketMax, error)
}

// PacketMaxRepository aggregates the write and read capabilities required by the service.
type PacketMaxRepository interface {
	PacketMaxWriter
	PacketMaxReader
}

// AggregatorResult represents the output of aggregator use-cases.
type AggregatorResult struct {
	PacketID  string
	SourceID  string
	Value     float64
	Timestamp time.Time
}

// AggregatorService describes the behaviour exposed to transport layers.
type AggregatorService interface {
	MaxByPacketID(ctx context.Context, packetID string) (AggregatorResult, error)
	MaxInRange(ctx context.Context, from, to time.Time) ([]AggregatorResult, error)
}

// PacketGenerator produces data packets that will be processed by workers.
type PacketGenerator interface {
	Run(ctx context.Context, out chan<- DataPacket)
}

// WorkerPool consumes packets and stores their measurements via the repository.
type WorkerPool interface {
	Run(ctx context.Context, packets <-chan DataPacket)
}

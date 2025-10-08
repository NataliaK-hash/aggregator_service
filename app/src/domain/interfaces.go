package domain

import (
	"context"
	"time"
)

type PacketMaxWriter interface {
	Add(ctx context.Context, packetMax PacketMax) error
}

type PacketMaxReader interface {
	PacketMaxByID(ctx context.Context, packetID string) (PacketMax, error)
	PacketMaxInRange(ctx context.Context, from, to time.Time) ([]PacketMax, error)
}

type PacketMaxRepository interface {
	PacketMaxWriter
	PacketMaxReader
}

type AggregatorResult struct {
	PacketID  string
	SourceID  string
	Value     float64
	Timestamp time.Time
}

type AggregatorService interface {
	MaxByPacketID(ctx context.Context, packetID string) (AggregatorResult, error)
	MaxInRange(ctx context.Context, from, to time.Time) ([]AggregatorResult, error)
}

type PacketGenerator interface {
	Run(ctx context.Context, out chan<- DataPacket)
}

type WorkerPool interface {
	Run(ctx context.Context, packets <-chan DataPacket)
}

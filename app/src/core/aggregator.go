package core

import (
	"context"
	"time"

	"aggregator-service/app/src/domain"
)

// Aggregator orchestrates access to packet max statistics.
type Aggregator struct {
	repo domain.PacketMaxReader
}

// New creates a new Aggregator service instance.
func NewAggregator(repo domain.PacketMaxReader) *Aggregator {
	return &Aggregator{repo: repo}
}

// MaxByPacketID returns the maximum measurement associated with the provided packet identifier.
func (a *Aggregator) MaxByPacketID(ctx context.Context, packetID string) (domain.AggregatorResult, error) {
	packetMax, err := a.repo.PacketMaxByID(ctx, packetID)
	if err != nil {
		return domain.AggregatorResult{}, err
	}

	return toResult(packetMax), nil
}

// MaxInRange returns all packet maxima observed within the provided time interval.
func (a *Aggregator) MaxInRange(ctx context.Context, from, to time.Time) ([]domain.AggregatorResult, error) {
	packetMaxes, err := a.repo.PacketMaxInRange(ctx, from, to)
	if err != nil {
		return nil, err
	}

	results := make([]domain.AggregatorResult, len(packetMaxes))
	for i, packetMax := range packetMaxes {
		results[i] = toResult(packetMax)
	}

	return results, nil
}

func toResult(packetMax domain.PacketMax) domain.AggregatorResult {
	return domain.AggregatorResult{
		PacketID:  packetMax.PacketID,
		SourceID:  packetMax.SourceID,
		Value:     packetMax.Value,
		Timestamp: packetMax.Timestamp,
	}
}

var _ domain.AggregatorService = (*Aggregator)(nil)

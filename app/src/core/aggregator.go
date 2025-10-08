package core

import (
	"context"
	"time"

	"aggregator-service/app/src/domain"
)

type Aggregator struct {
	repo domain.PacketMaxReader
}

func NewAggregator(repo domain.PacketMaxReader) *Aggregator {
	return &Aggregator{repo: repo}
}

func (a *Aggregator) MaxByPacketID(ctx context.Context, packetID string) (domain.AggregatorResult, error) {
	packetMax, err := a.repo.PacketMaxByID(ctx, packetID)
	if err != nil {
		return domain.AggregatorResult{}, err
	}
	return toResult(packetMax), nil
}

func (a *Aggregator) MaxInRange(ctx context.Context, from, to time.Time) ([]domain.AggregatorResult, error) {
	packetMaxes, err := a.repo.PacketMaxInRange(ctx, from, to)
	if err != nil {
		return nil, err
	}

	results := make([]domain.AggregatorResult, len(packetMaxes))
	for i, p := range packetMaxes {
		results[i] = toResult(p)
	}
	return results, nil
}

func toResult(p domain.PacketMax) domain.AggregatorResult {
	return domain.AggregatorResult{
		PacketID:  p.PacketID,
		SourceID:  p.SourceID,
		Value:     p.Value,
		Timestamp: p.Timestamp,
	}
}

var _ domain.AggregatorService = (*Aggregator)(nil)

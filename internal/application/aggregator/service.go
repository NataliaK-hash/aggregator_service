package aggregator

import (
	"context"
	"time"

	"aggregator-service-project/internal/domain"
)

// Aggregator orchestrates access to measurement statistics.
type Aggregator struct {
	repo domain.MeasurementReader
}

// New creates a new Aggregator service instance.
func New(repo domain.MeasurementReader) *Aggregator {
	return &Aggregator{repo: repo}
}

// MaxBySource returns the maximum measurement for the provided source identifier.
func (a *Aggregator) MaxBySource(ctx context.Context, id string) (domain.AggregatorResult, error) {
	measurement, err := a.repo.MaxBySource(ctx, id)
	if err != nil {
		return domain.AggregatorResult{}, err
	}

	return domain.AggregatorResult{
		SourceID:  id,
		Value:     measurement.Value,
		Timestamp: measurement.Timestamp,
	}, nil
}

// MaxInRange returns the maximum measurement in the provided time interval.
func (a *Aggregator) MaxInRange(ctx context.Context, from, to time.Time) (domain.AggregatorResult, error) {
	measurement, err := a.repo.MaxInRange(ctx, from, to)
	if err != nil {
		return domain.AggregatorResult{}, err
	}

	return domain.AggregatorResult{
		SourceID:  measurement.SourceID,
		Value:     measurement.Value,
		Timestamp: measurement.Timestamp,
	}, nil
}

var _ domain.AggregatorService = (*Aggregator)(nil)

package aggregator

import (
	"context"
	"errors"
	"time"

	"aggregator/internal/domain"
)

// ErrNotFound is returned when no measurement exists for the provided filters.
var ErrNotFound = errors.New("measurement not found")

// Repository describes the data access requirements for the aggregator service.
type Repository interface {
	MaxBySource(ctx context.Context, id string) (domain.Measurement, error)
	MaxInRange(ctx context.Context, from, to time.Time) (domain.Measurement, error)
}

// Service exposes the use-cases that the transport layers rely on.
type Service interface {
	MaxBySource(ctx context.Context, id string) (Result, error)
	MaxInRange(ctx context.Context, from, to time.Time) (Result, error)
}

// Result represents the application output returned by the aggregator service.
type Result struct {
	SourceID  string
	Value     float64
	Timestamp time.Time
}

// Aggregator orchestrates access to measurement statistics.
type Aggregator struct {
	repo Repository
}

// New creates a new Aggregator service instance.
func New(repo Repository) *Aggregator {
	return &Aggregator{repo: repo}
}

// MaxBySource returns the maximum measurement for the provided source identifier.
func (a *Aggregator) MaxBySource(ctx context.Context, id string) (Result, error) {
	measurement, err := a.repo.MaxBySource(ctx, id)
	if err != nil {
		return Result{}, err
	}

	return Result{
		SourceID:  id,
		Value:     measurement.Value,
		Timestamp: measurement.Timestamp,
	}, nil
}

// MaxInRange returns the maximum measurement in the provided time interval.
func (a *Aggregator) MaxInRange(ctx context.Context, from, to time.Time) (Result, error) {
	measurement, err := a.repo.MaxInRange(ctx, from, to)
	if err != nil {
		return Result{}, err
	}

	return Result{
		SourceID:  measurement.SourceID,
		Value:     measurement.Value,
		Timestamp: measurement.Timestamp,
	}, nil
}

var _ Service = (*Aggregator)(nil)

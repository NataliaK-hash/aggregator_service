package memory

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"aggregator-service-project/internal/domain"
)

// Repository stores measurements in memory and satisfies the application repository contract.
type Repository struct {
	mu           sync.RWMutex
	measurements []domain.Measurement
}

// New creates an empty in-memory repository instance.
func New() *Repository {
	return &Repository{}
}

// Seed replaces the internal storage with the provided sample data.
func (r *Repository) Seed(measurements []domain.Measurement) {
	r.mu.Lock()
	defer r.mu.Unlock()

	copied := make([]domain.Measurement, len(measurements))
	copy(copied, measurements)
	r.measurements = copied
}

// Add stores a measurement in the repository.
func (r *Repository) Add(_ context.Context, measurement domain.Measurement) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.measurements = append(r.measurements, measurement)
	return nil
}

// MaxBySource returns the maximum measurement for the provided source identifier.
func (r *Repository) MaxBySource(_ context.Context, id string) (domain.Measurement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	maxValue := math.Inf(-1)
	var maxMeasurement *domain.Measurement

	for i := range r.measurements {
		measurement := r.measurements[i]
		if measurement.SourceID != id {
			continue
		}

		if maxMeasurement == nil || measurement.Value > maxValue {
			maxValue = measurement.Value
			copy := measurement
			maxMeasurement = &copy
		}
	}

	if maxMeasurement == nil {
		return domain.Measurement{}, domain.ErrNotFound
	}

	return *maxMeasurement, nil
}

// MaxInRange returns the maximum measurement within the provided time range.
func (r *Repository) MaxInRange(_ context.Context, from, to time.Time) (domain.Measurement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if from.After(to) {
		return domain.Measurement{}, domain.ErrNotFound
	}

	filtered := make([]domain.Measurement, 0, len(r.measurements))
	for i := range r.measurements {
		measurement := r.measurements[i]
		if measurement.Timestamp.Before(from) || measurement.Timestamp.After(to) {
			continue
		}
		filtered = append(filtered, measurement)
	}

	if len(filtered) == 0 {
		return domain.Measurement{}, domain.ErrNotFound
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Value > filtered[j].Value
	})

	return filtered[0], nil
}

var _ domain.MeasurementRepository = (*Repository)(nil)

package clickhouse

import (
	"context"
	"sort"
	"sync"
	"time"

	"aggregator/internal/domain"
)

type Repository struct {
	mu   sync.RWMutex
	data map[string]domain.PacketMax
}

func NewRepository() *Repository {
	return &Repository{data: make(map[string]domain.PacketMax)}
}

func (r *Repository) Save(_ context.Context, packets []domain.PacketMax) error {
	if len(packets) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, packet := range packets {
		r.data[packet.ID] = packet
	}

	return nil
}

func (r *Repository) GetByID(_ context.Context, id string) (*domain.PacketMax, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	value, ok := r.data[id]
	if !ok {
		return nil, nil
	}

	copy := value
	return &copy, nil
}

func (r *Repository) GetByTimeRange(_ context.Context, from, to time.Time) ([]domain.PacketMax, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]domain.PacketMax, 0)
	for _, value := range r.data {
		if !value.Timestamp.Before(from) && value.Timestamp.Before(to) {
			results = append(results, value)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})

	return results, nil
}

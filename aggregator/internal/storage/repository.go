package storage

import (
	"context"
	"time"

	"aggregator/internal/domain"
)

type Repository interface {
	Save(ctx context.Context, packets []domain.PacketMax) error
	GetByID(ctx context.Context, id string) (*domain.PacketMax, error)
	GetByTimeRange(ctx context.Context, from, to time.Time) ([]domain.PacketMax, error)
}

type Closer interface {
	Close(ctx context.Context) error
}

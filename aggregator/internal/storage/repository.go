package storage

import (
	"context"
	"time"

	"aggregator/internal/domain"
)

// Repository описывает абстракцию слоя хранения агрегированных результатов.
type Repository interface {
	Save(ctx context.Context, packets []domain.PacketMax) error
	GetByID(ctx context.Context, id string) (*domain.PacketMax, error)
	GetByTimeRange(ctx context.Context, from, to time.Time) ([]domain.PacketMax, error)
}

// Closer описывает репозиторий, который может корректно завершать работу, сбрасывая буферы.
type Closer interface {
	Close(ctx context.Context) error
}
